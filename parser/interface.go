package parser

import (
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
)

// A Mode value is a set of flags (or 0).
// They control the amount of source code parsed and other optional
// parser functionality.
//
type Mode uint

const (
	iotaMode Mode = 1 << iota
	// PackageClauseOnly    Mode             = 1 << iota // stop parsing after package clause
	//ImportsOnly                           // stop parsing after import declarations
	ParseComments                         // parse comments and add them to AST
	Trace                                 // print a trace of parsed productions
	DeclarationErrors                     // report declaration errors
	SpuriousErrors                        // same as AllErrors, for backward-compatibility
	SkipObjectResolution                  // don't resolve identifiers to objects - see ParseFile
	AllErrors            = SpuriousErrors // report all errors (not just the first 10 on different lines)
)

// If src != nil, readSource converts src to a []byte if possible;
// otherwise it returns an error. If src == nil, readSource returns
// the result of reading the file specified by filename.
//
func readSource(filename string, src interface{}) ([]byte, error) {
	if src != nil {
		switch s := src.(type) {
		case string:
			return []byte(s), nil
		case []byte:
			return s, nil
		case *bytes.Buffer:
			// is io.Reader, but src is already available in []byte form
			if s != nil {
				return s.Bytes(), nil
			}
		case io.Reader:
			return io.ReadAll(s)
		}
		return nil, errors.New("invalid source")
	}
	return os.ReadFile(filename)
}

// ParseFile parses the source code of a single Go source file and returns
// the corresponding ast.File node. The source code may be provided via
// the filename of the source file, or via the src parameter.
//
// If src != nil, ParseFile parses the source from src and the filename is
// only used when recording position information. The type of the argument
// for the src parameter must be string, []byte, or io.Reader.
// If src == nil, ParseFile parses the file specified by filename.
//
// The mode parameter controls the amount of source text parsed and other
// optional parser functionality. If the SkipObjectResolution mode bit is set,
// the object resolution phase of parsing will be skipped, causing File.Scope,
// File.Unresolved, and all Ident.Obj fields to be nil.
//
// Position information is recorded in the file set fset, which must not be
// nil.
//
// If the source couldn't be read, the returned AST is nil and the error
// indicates the specific failure. If the source was read but syntax
// errors were found, the result is a partial AST (with ast.Bad* nodes
// representing the fragments of erroneous source code). Multiple errors
// are returned via a scanner.ErrorList which is sorted by source position.
//
func ParseFile(fset *token.FileSet, filename string, src interface{}, mode Mode) (f *ast.File, err error) {
	if fset == nil {
		panic("parser.ParseFile: not token.FileSet provided (fset == nil)")
	}

	text, err := readSource(filename, src)
	if err != nil {
		return nil, err
	}

	var p parser
	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(bailout); !ok {
				panic(e)
			}
		}

		if f == nil {
			f = &ast.File{
				// TODO: finish me
				//Name:  new(ast.Ident),
				//Scope: ast.NewScope(nil),
			}
		}

		p.errors.Sort()
		err = p.errors.Err()
	}()

	p.init(fset, filename, text, mode)
	f = p.parseFile()
	return
}
