package printer

import (
	"fmt"
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
	"io"
	"text/tabwriter"
)

// ----------------------------------------------------------------------------
// Public interface

// A Mode value is a set of flags (or 0). They control printing.
type Mode uint

const (
	RawFormat Mode = 1 << iota // do not use a tabwriter; if set, UseSpaces is ignored
	TabIndent                  // use tabs for indentation independent of UseSpaces
	UseSpaces                  // use spaces instead of tabs for alignment
	SourcePos                  // emit //line directives to preserve original source positions
)

// The mode below is not included in printer's public API because
// editing code text is deemed out of scope. Because this mode is
// unexported, it's also possible to modify or remove it based on
// the evolving needs of go/format and cmd/gofmt without breaking
// users. See discussion in CL 240683.
const (
	// normalizeNumbers means to canonicalize number
	// literal prefixes and exponents while printing.
	//
	// This value is known in and used by go/format and cmd/gofmt.
	// It is currently more convenient and performant for those
	// packages to apply number normalization during printing,
	// rather than by modifying the AST in advance.
	normalizeNumbers Mode = 1 << 30
)

// A Config node controls the output of Fprint.
type Config struct {
	Mode     Mode // default: 0
	TabWidth int  // default: 8
	Indent   int  // default: 0 (all code is indented at least by this much)
}

// Fprint "pretty-prints" an AST node to output.
// It calls Config.Fprint with default settings.
// Note that gofmt uses tabs for indentation but spaces for alignment;
// use format.Node (package go/format) for output that matches gofmt.
//
func Fprint(output io.Writer, fset *token.FileSet, node ast.Node) error {
	return (&Config{TabWidth: 8}).Fprint(output, fset, node)
}

func (cfg *Config) Fprint(out io.Writer, fset *token.FileSet, node ast.Node) error {
	return cfg.fprint(out, fset, node, make(map[ast.Node]int))
}

func (cfg *Config) fprint(output io.Writer, fSet *token.FileSet, node ast.Node, nodeSizes map[ast.Node]int) error {
	var p printer
	p.init(cfg, fSet, nodeSizes)
	if err := p.printNode(node); err != nil {
		return err
	}

	p.impliedSemi = false
	p.flush(token.Position{Offset: infinity, Line: infinity}, token.EOF)

	output = &trimmer{output: output}

	if cfg.Mode&RawFormat == 0 {
		minwidth := cfg.TabWidth

		padChar := byte('\t')
		if cfg.Mode&UseSpaces != 0 {
			padChar = ' '
		}

		twmode := tabwriter.DiscardEmptyColumns
		if cfg.Mode&TabIndent != 0 {
			minwidth = 0
			twmode |= tabwriter.TabIndent
		}

		output = tabwriter.NewWriter(output, minwidth, cfg.TabWidth, 1, padChar, twmode)
	}

	if _, err := output.Write(p.output); err != nil {
		return err
	}

	if tw, _ := output.(*tabwriter.Writer); tw != nil {
		return tw.Flush()
	}

	return nil
}

func (p *printer) internalError(msg ...interface{}) {
	if true {
		fmt.Print(p.pos.String() + ": ")
		fmt.Println(msg...)
		panic("go/printer")
	}
}
