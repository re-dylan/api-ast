package main

import (
	"errors"
	"flag"
	"fmt"

	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/zeromicro/api-ast/cmd/apifmt/internal"
	"github.com/zeromicro/api-ast/parser"
	"github.com/zeromicro/api-ast/token"
)

var (
	list  = flag.Bool("l", false, "list files whose formatting differs from apifmt's")
	write = flag.Bool("w", false, "write result to (source) file instead of stdout")
	// TODO: not support
	rewirteRule = flag.String("r", "", "rewrite rule, not support for now")
	simplifyAST = flag.Bool("s", false, "simple code")
	doDiff      = flag.Bool("d", false, "display diffs instead of rewriting files")
	allErrors   = flag.Bool("e", false, "report all errors (not just the first 10 on different lines)")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")
)

const (
	tabWidth = 0
	// TODO:
	printerMode int = 0
)

var (
	parserMode parser.Mode
)

func main() {
	maxWeight := (2 << 20) * int64(runtime.GOMAXPROCS(0))
	s := internal.NewSequencer(maxWeight, os.Stdout, os.Stderr)

	apiftmMain(s)
	os.Exit(s.GetExitCode())
}

func apiftmMain(s *internal.Sequencer) {
	flag.Usage = usage
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			s.AddReport(errors.New("create cpu profile: " + err.Error()))
			return
		}
		defer func() {
			_ = f.Close()
		}()
		_ = pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	initParserMode()
	initRewrite()

	args := flag.Args()
	if len(args) == 0 {
		if *write {
			s.AddReport(fmt.Errorf("error: cannot use -w with standard iput"))
			return
		}
		s.Add(0, func(r *internal.Reporter) error {
			return processFile("<standard input>", nil, os.Stdin, r)
		})
	}

	for _, arg := range args {
		switch info, err := os.Stat(arg); {
		case err != nil:
			s.AddReport(err)
		case !info.IsDir():
			arg := arg
			s.Add(fileWeight(arg, info), func(r *internal.Reporter) error {
				return processFile(arg, info, nil, r)
			})
		default:
			err := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
				if err != nil || !isApiFile(d) {
					return err
				}

				info, err := d.Info()
				if err != nil {
					s.AddReport(err)
					return nil
				}

				s.Add(fileWeight(path, info), func(r *internal.Reporter) error {
					return processFile(path, info, nil, r)
				})
				return nil
			})
			if err != nil {
				s.AddReport(err)
			}
		}
	}
}

// ----

func usage() {
	_, _ = fmt.Fprintf(os.Stderr, "usage: apifmt [flags] [path ...]\n")
	flag.PrintDefaults()
}

func isApiFile(f fs.DirEntry) bool {
	name := f.Name()
	return !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go") && !f.IsDir()
}

func initParserMode() {
	parserMode = parser.ParseComments
	if *allErrors {
		parserMode |= parser.AllErrors
	}
}

func fileWeight(path string, info fs.FileInfo) int64 {
	if info == nil {
		return -1
	}
	if info.Mode().Type() == fs.ModeSymlink {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			return -1

		}
	}
	if !info.Mode().IsRegular() {
		return -1
	}
	return info.Size()
}

func processFile(filename string, info fs.FileInfo, in io.Reader, r *internal.Reporter) error {
	if in == nil {
		var err error
		in, err = os.Open(filename)
		if err != nil {
			return err
		}
	}

	var src []byte
	size := -1
	if info != nil && info.Mode().IsRegular() && int64(int(info.Size())) == info.Size() {
		size = int(info.Size())
	}

	if size+1 > 0 {
		// If we have the FileInfo from filepath.WalkDir, use it to make
		// a buffer of the right size and avoid ReadAll's reallocations.
		//
		// We try to read size+1 bytes so that we can detect modifications: if we
		// read more than size bytes, then the file was modified concurrently.
		// (If that happens, we could, say, append to src to finish the read, or
		// proceed with a truncated buffer — but the fact that it changed at all
		// indicates a possible race with someone editing the file, so we prefer to
		// stop to avoid corrupting it.)
		src = make([]byte, size+1)
		n, err := io.ReadFull(in, src)
		if err != nil && err != io.ErrUnexpectedEOF {
			return err
		}
		if n < size {
			return fmt.Errorf("error: size of %s changed during reading (from %d to %d bytes)", filename, size, n)
		} else if n > size {
			return fmt.Errorf("error: size of %s changed during reading (from %d to >=%d bytes)", filename, size, len(src))
		}
		src = src[:n]
	} else {
		// The file is not known to be regular, so we don't have a reliable size for it.
		var err error
		src, err = io.ReadAll(in)
		if err != nil {
			return err
		}
	}

	fileSet := token.NewFileSet()
	file, err := internal.Parse(fileSet, filename, src, parserMode)
	if err != nil {
		return err
	}

	// TODO: Sort Imports

	if *simplifyAST {
		internal.Simplify(file)
	}
}
