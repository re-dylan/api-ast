package parser

import (
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/scanner"
	"github.com/zeromicro/api-ast/token"
)

type parser struct {
	file    *token.File
	errors  scanner.ErrorList
	scanner scanner.Scanner

	// Tracing/debugging
	mode   Mode
	trace  bool
	indent int

	// Comments
	comments    []*ast.CommentGroup
	leadComment *ast.CommentGroup
	lineComment *ast.CommentGroup

	pos token.Pos
	tok token.Token
	lit string

	// Error recovery
	// (used to limit the number of calls to parser.advance
	// w/o making scanning progress - avoids potential endless
	// loops across multiple parser functions during error recovery)
	syncPos token.Pos // last synchronization position
	syncCnt int       // number of parser.advance calls without progress

	// Non-syntactic parser control
	exprLev int // < 0: in control clause, >= 0: in expression

	imports []*ast.ImportSpec // list of imports
}

func (p *parser) init(fileSet *token.FileSet, filename string, src []byte, mode Mode) {
	p.file = fileSet.AddFile(filename, -1, len(src))
	var m scanner.Mode
	if mode&ParseComments != 0 {
		m = scanner.ScanComments
	}

	p.scanner.Init(p.file, src, func(pos token.Position, msg string) {
		p.errors.Add(pos, msg)
	}, m)

	p.mode = mode
	p.trace = mode&Trace != 0
	p.next()
}

// Advance to the next non-comment token. In the process, collect
// any comment groups encountered, and remember the last lead and
// line comments.
//
// A lead comment is a comment group that starts and ends in a
// line without any other tokens and that is followed by a non-comment
// token on the line immediately after the comment group.
//
// A line comment is a comment group that follows a non-comment
// token on the same line, and that has no tokens after it on the line
// where it ends.
//
// Lead and line comments may be considered documentation that is
// stored in the AST.
//
func (p *parser) next() {
	p.leadComment = nil
	p.lineComment = nil
	prev := p.pos
	p.next0()

	if p.tok == token.COMMENT {
		var comment *ast.CommentGroup
		var endline int

		if p.file.Line(p.pos) == p.file.Line(prev) {
			// The comment is on same line as the previous token; it
			// cannot be a lead comment but may be a line comment.
			comment, endline = p.consumeCommentGroup(0)
			if p.file.Line(p.pos) != endline || p.tok == token.EOF {
				// The next token is on a different line, thus
				// the last comment group is a line comment.
				p.lineComment = comment
			}
		}

		// consume successor comments, if any
		endline = -1
		for p.tok == token.COMMENT {
			comment, endline = p.consumeCommentGroup(1)
		}

		if endline+1 == p.file.Line(p.pos) {
			// The next token is following on the line immediately after the
			// comment group, thus the last comment group is a lead comment.
			p.leadComment = comment
		}
	}
}

// Consume a comment and return it and the line on which it ends.
func (p *parser) consumeComment() (comment *ast.Comment, endline int) {
	// /*-style comments may end on a different line than where they start.
	// Scan the comment for '\n' chars and adjust endline accordingly.
	endline = p.file.Line(p.pos)
	if p.lit[1] == '*' {
		// don't use range here - no need to decode Unicode code points
		for i := 0; i < len(p.lit); i++ {
			if p.lit[i] == '\n' {
				endline++
			}
		}
	}

	comment = &ast.Comment{Slash: p.pos, Text: p.lit}
	p.next0()

	return
}

// Consume a group of adjacent comments, add it to the parser's
// comments list, and return it together with the line at which
// the last comment in the group ends. A non-comment token or n
// empty lines terminate a comment group.
//
func (p *parser) consumeCommentGroup(n int) (comments *ast.CommentGroup, endline int) {
	var list []*ast.Comment
	endline = p.file.Line(p.pos)
	for p.tok == token.COMMENT && p.file.Line(p.pos) <= endline+n {
		var comment *ast.Comment
		comment, endline = p.consumeComment()
		list = append(list, comment)
	}

	// add comment group to the comments list
	comments = &ast.CommentGroup{List: list}
	p.comments = append(p.comments, comments)

	return
}

func (p *parser) next0() {
	if p.trace && p.pos.IsValid() {
		s := p.tok.String()
		switch {
		case p.tok.IsLiteral():
			p.printTrace(s, p.lit)
		case p.tok.IsOperator(), p.tok.IsKeyword():
			p.printTrace("\"" + s + "\"")
		default:
			p.printTrace(s)
		}
	}

	p.pos, p.tok, p.lit = p.scanner.Scan()
}

func (p *parser) parseFile() *ast.File {
	if p.trace {
		defer un(trace(p, "File"))
	}

	if p.errors.Len() != 0 {
		return nil
	}

	doc := p.leadComment
	var syntax *ast.SyntaxSpec
	if p.tok == token.SYNTAX { // syntax = "v1"
		syntax = p.parseSyntax()
	} else {
		// TODO: 未指定 语法版本的使用默认版本
		syntax = &ast.SyntaxSpec{
			TokPos: 0,
			Assign: 0,
			Name:   nil,
		}
		p.expectSemi()
		panic("not finished")
	}

	var decls []ast.Decl
	for p.tok != token.EOF {
		decls = append(decls, p.parseDecl(declStart))
	}

	f := &ast.File{
		Doc:        doc,
		Syntax:     syntax,
		Decls:      decls,
		Scope:      nil,
		Imports:    p.imports,
		Unresolved: nil,
		Comments:   p.comments,
	}

	var declErr func(token.Pos, string)
	if p.mode&DeclarationErrors != 0 {
		declErr = p.error
	}

	if p.mode&SkipObjectResolution == 0 {
		resolveFile(f, p.file, declErr)
	}

	return f
}

func (p *parser) expect(tok token.Token) token.Pos {
	pos := p.pos
	if p.tok != tok {
		p.errorExpected(pos, "'"+tok.String()+"'")
	}
	p.next()
	return pos
}

func (p *parser) expectSemi() {
	// semicolon is optional before a closing ')' or '}'
	if p.tok != token.RPAREN && p.tok != token.RBRACE {
		switch p.tok {
		case token.COMMA:
			// permit a ',' instead of a ';' but complain
			p.errorExpected(p.pos, "';'")
			fallthrough
		case token.SEMICOLON:
			p.next()
		default:
			p.errorExpected(p.pos, "';'")
			p.advance(stmtStart)
		}
	}
}

func (p *parser) advance(to map[token.Token]bool) {
	for ; p.tok != token.EOF; p.next() {
		if to[p.tok] {
			// Return only if parser made some progress since last
			// sync or if it has not reached 10 advance calls without
			// progress. Otherwise consume at least one token to
			// avoid an endless parser loop (it is possible that
			// both parseOperand and parseStmt call advance and
			// correctly do not advance, thus the need for the
			// invocation limit p.syncCnt).
			if p.pos == p.syncPos && p.syncCnt < 10 {
				p.syncCnt++
				return
			}
			if p.pos > p.syncPos {
				p.syncPos = p.pos
				p.syncCnt = 0
				return
			}
			// Reaching here indicates a parser bug, likely an
			// incorrect token list in this function, but it only
			// leads to skipping of possibly correct code if a
			// previous error is present, and thus is preferred
			// over a non-terminating parse.
		}
	}
}

var stmtStart = map[token.Token]bool{
	//token.BREAK:       true,
	//token.CONST:       true,
	//token.CONTINUE:    true,
	//token.DEFER:       true,
	//token.FALLTHROUGH: true,
	//token.FOR:         true,
	//token.GO:          true,
	//token.GOTO:        true,
	//token.IF:          true,
	//token.RETURN:      true,
	//token.SELECT:      true,
	//token.SWITCH:      true,
	token.TYPE: true,
	//token.VAR:         true,
	// TODO: add service?
}

//
var declStart = map[token.Token]bool{
	token.IMPORT:   true,
	token.INFO:     true,
	token.TYPE:     true,
	token.SYNTAX:   true,
	token.ATSERVER: true,
	token.SERVICE:  true,
}

//
var exprEnd = map[token.Token]bool{
	token.COMMA:     true,
	token.COLON:     true,
	token.SEMICOLON: true,
	token.RPAREN:    true,
	token.RBRACK:    true,
	token.RBRACE:    true,
}
