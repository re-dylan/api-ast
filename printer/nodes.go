package printer

import (
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
)

// ----------------------------------------------------------------------------
//

func (p *printer) file(src *ast.File) {
	p.setComment(src.Doc)
	// for go it print package name; go-zero should just print decl
	//p.print(src.Pos(), token.PACKAGE, blank)
	//p.expr(src.Name)
	p.declList(src.Decls)
	p.print(newline)
}

func (p *printer) setComment(g *ast.CommentGroup) {
	if g == nil { // p.useNodeComments
		return
	}
	if p.comments == nil {
		p.comments = make([]*ast.CommentGroup, 1)
	} else if p.cindex < len(p.comments) {
		p.flush(p.posFor(g.List[0].Pos()), token.ILLEGAL)
		p.comments = p.comments[0:1]
		p.internalError("setComment found pending comments")
	}

	p.comments[0] = g
	p.cindex = 0

	if p.commentOffset == infinity {
		p.nextComment()
	}
}

func (p *printer) declList(list []ast.Decl) {
	tok := token.ILLEGAL
	for _, d := range list {
		prev := tok
		tok = declToken(d)
		if len(p.output) > 0 {
			min := 1
			if prev != tok || getDoc(d) != nil {
				min = 2
			}

			p.linebreak(p.lineFor(d.Pos()), min, ignore, p.numLines(d) > 1)
		}
		p.decl(d)
	}
}

// numLines returns the number of lines spanned by node n in the original source.
func (p *printer) numLines(n ast.Node) int {
	if from := n.Pos(); from.IsValid() {
		if to := n.End(); to.IsValid() {
			return p.lineFor(to) - p.lineFor(from) + 1
		}
	}
	return infinity
}

func (p *printer) linebreak(line, min int, ws whiteSpace, newSection bool) (nbreaks int) {
	n := nlimit(line - p.pos.Line)
	if n < min {
		n = min
	}

	if n > 0 {
		p.print(ws)
		if newSection {
			p.print(formfeed)
			n--
			nbreaks = 2
		}
		nbreaks += n
		for ; n > 0; n-- {
			p.print(newline)
		}
	}
	return
}

func (p *printer) decl(decl ast.Decl) {
	switch d := decl.(type) {
	case *ast.BadDecl:
		p.print(d.Pos(), "BadDecl")
	case *ast.GenDecl:
		p.genDecl(d)
	//case *ast.FuncDecl:
	//	p.funcDecl(d)
	// TODO: finish me
	default:
		panic("unreachable")
	}
}

func (p *printer) genDecl(d *ast.GenDecl) {
	p.setComment(d.Doc)
	p.print(d.Pos(), d.Tok, blank)

	if d.Lparen.IsValid() || len(d.Specs) > 1 {
		p.print(indent, formfeed)
		if n := len(d.Specs); n > 0 {

		} else {
			var line int
			for i, s := range d.Specs {
				if i > 0 {
					p.linebreak(p.lineFor(s.Pos()), 1, ignore, p.lines)
				}
				p.recordLine(&line)
			}
		}
		p.print(d.Rparen, token.RPAREN)
	}
}