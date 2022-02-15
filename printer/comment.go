package printer

import (
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
	"strings"
)

const (
	maxNewlines = 2 // max. number of newlines between source text
	infinity    = 1 << 30
)

type commentInfo struct {
	cindex         int               // current comment index
	comment        *ast.CommentGroup // = printer.comments[cindex]; or nil
	commentOffset  int               // = printer.posFor(printer.comments[cindex].List[0].Pos()).Offset; or infinity
	commentNewline bool              // true if the comment group contains newlines
}

func (p *printer) nextComment() {
	for p.cindex < len(p.comments) {
		c := p.comments[p.cindex]
		p.cindex++
		if list := c.List; len(list) > 0 {
			p.comment = c
			p.commentOffset = p.posFor(list[0].Pos()).Offset
			p.commentNewline = p.commentsHaveNewline(list)
			return
		}
	}

	p.commentOffset = infinity
}

// commentsHaveNewline reports whether a list of comments belonging to
// an *ast.CommentGroup contains newlines. Because the position information
// may only be partially correct, we also have to read the comment text.
func (p *printer) commentsHaveNewline(list []*ast.Comment) bool {
	// len(list) > 0
	line := p.lineFor(list[0].Pos())
	for i, c := range list {
		if i > 0 && p.lineFor(list[i].Pos()) != line {
			// not all comments on the same line
			return true
		}
		if t := c.Text; len(t) >= 2 && (t[1] == '/' || strings.Contains(t, "\n")) {
			return true
		}
	}
	_ = line
	return false
}

func (p *printer) posFor(pos token.Pos) token.Position {
	return p.fSet.PositionFor(pos, false)
}

func (p *printer) lineFor(pos token.Pos) int {
	if pos != p.cachedPos {
		p.cachedPos = pos
		p.cachedLine = p.fSet.PositionFor(pos, false /* absolute position */).Line
	}
	return p.cachedLine
}

// commentBefore reports whether the current comment group occurs
// before the next position in the source code and printing it does
// not introduce implicit semicolons.
//
func (p *printer) commentBefore(next token.Position) bool {
	return p.commentOffset < next.Offset && (!p.impliedSemi || !p.commentNewline)
}

// containsLinebreak reports whether the whitespace buffer contains any line breaks.
func (p *printer) containsLinebreak() bool {
	for _, ch := range p.wsbuf {
		if ch == newline || ch == formfeed {
			return true
		}
	}
	return false
}

func (p *printer) intersperseComments(next token.Position, tok token.Token) (wroteNewLine, droppedFF bool) {
	var last *ast.Comment
	for p.commentBefore(next) {
		for _, c := range p.comment.List {
			p.writeCommentPrefix(p.posFor(c.Pos()), next, last, tok)
			p.writeComment(c)
			last = c
		}
		p.nextComment()
	}

	if last == nil {
		p.internalError("intersperseComments called without pending comments")
		return
	}

	needsLinebreak := false
	if p.mode&noExtraBlank == 0 &&
		last.Text[1] == '*' && p.lineFor(last.Pos()) == next.Line &&
		tok != token.COMMA &&
		(tok != token.RPAREN || p.prevOpen == token.LPAREN) &&
		(tok != token.RBRACK || p.prevOpen == token.LBRACK) {
		if p.containsLinebreak() && p.mode&noExtraLinebreak == 0 && p.level == 0 {
			needsLinebreak = true
		} else {
			p.writeByte(' ', 1)
		}
	}

	if last.Text[1] == '/' ||
		tok == token.EOF ||
		tok == token.RBRACE && p.mode&noExtraLinebreak == 0 {
		needsLinebreak = true
	}
	return p.writeCommentSuffix(needsLinebreak)
}

func (p *printer) writeCommentPrefix(pos, next token.Position, prev *ast.Comment, tok token.Token) {
	if len(p.output) == 0 {
		return
	}

	if pos.IsValid() && pos.Filename != p.last.Filename {
		p.writeByte('\f', maxNewlines)
		return
	}

	// TODO: finish me
	if pos.Line == p.last.Line && (prev == nil || prev.Text[1] != '/') {

	} else {

	}
}

func (p *printer) writeCommentSuffix(needsLinebreak bool) (wroteNewline, droppedFF bool) {
	for i, ch := range p.wsbuf {
		switch ch {
		case blank, vtab:
			// ignore trailing whitespace
			p.wsbuf[i] = ignore
		case indent, unindent:
			// don't lose indentation information
		case newline, formfeed:
			// if we need a line break, keep exactly one
			// but remember if we dropped any formfeeds
			if needsLinebreak {
				needsLinebreak = false
				wroteNewline = true
			} else {
				if ch == formfeed {
					droppedFF = true
				}
				p.wsbuf[i] = ignore
			}
		}
	}
	p.writeWhitespace(len(p.wsbuf))

	// make sure we have a line break
	if needsLinebreak {
		p.writeByte('\n', 1)
		wroteNewline = true
	}

	return
}

func (p *printer) writeComment(comment *ast.Comment) {
	// TODO: finish me
}

// ----------------------------------------------------------------------------
//

// getNode returns the ast.CommentGroup associated with n, if any.
func getDoc(n ast.Node) *ast.CommentGroup {
	switch n := n.(type) {
	case *ast.Field:
		return n.Doc
	case *ast.ImportSpec:
		return n.Doc
	//case *ast.ValueSpec:
	//	return n.Doc
	case *ast.TypeSpec:
		return n.Doc
	case *ast.GenDecl:
		return n.Doc
	//case *ast.FuncDecl:
	//	return n.Doc
	case *ast.File:
		return n.Doc
	}
	return nil
}

func getLastComment(n ast.Node) *ast.CommentGroup {
	switch n := n.(type) {
	case *ast.Field:
		return n.Comment
	case *ast.ImportSpec:
		return n.Comment
	//case *ast.ValueSpec:
	//	return n.Comment
	case *ast.TypeSpec:
		return n.Comment
	case *ast.GenDecl:
		if len(n.Specs) > 0 {
			return getLastComment(n.Specs[len(n.Specs)-1])
		}
	case *ast.File:
		if len(n.Comments) > 0 {
			return n.Comments[len(n.Comments)-1]
		}
	}
	return nil
}

func declToken(decl ast.Decl) (tok token.Token) {
	tok = token.ILLEGAL
	switch d := decl.(type) {
	case *ast.GenDecl:
		tok = d.Tok
		//case *ast.FuncDecl:
		//	tok = token.FUNC
		// TODO: finish me for server api
	}
	return
}
