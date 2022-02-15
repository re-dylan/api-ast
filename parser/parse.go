package parser

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
)

// ----------------------------------------------------------------------------
// syntax
func (p *parser) parseSyntax() *ast.SyntaxSpec {
	if p.trace {
		defer un(trace(p, "Syntax"))
	}
	pos := p.expect(token.SYNTAX)
	assignPos := p.expect(token.ASSIGN)

	namePos := p.pos
	var name string
	if p.tok == token.STRING {
		name = p.lit
		p.next()
	} else { // use expect() error handling
		p.expect(token.STRING)
	}
	p.expectSemi()

	return &ast.SyntaxSpec{
		TokPos: pos,
		Assign: assignPos,
		Name:   &ast.BasicLit{ValuePos: namePos, Kind: token.STRING, Value: name},
	}
}

// ----------------------------------------------------------------------------
// identifiers

func (p *parser) parseIdent() *ast.Ident {
	pos := p.pos
	var name string
	if p.tok == token.IDENT {
		name = p.lit
		p.next()
	} else {
		name = "_"
		p.expect(token.IDENT) // use expect() error handling
	}
	return &ast.Ident{NamePos: pos, Name: name}
}

// parseApiIdent api ident support more like "user-api" "/path/:name"
func (p *parser) parseApiIdent() *ast.Ident {
	if p.trace {
		defer un(trace(p, "ApiIdent"))
	}
	pos := p.pos
	var name string
	if p.tok == token.QUO { // colon for "/path/:name
		for p.tok != token.EOF {
			// current is Ident, need to check next is Ident. it should break
			var check bool
			switch p.tok {
			case token.QUO, token.COLON:
				name += p.tok.String()
			default:
				check = true
				name += p.lit
			}
			p.next()
			if check && (p.tok != token.QUO && p.tok != token.COLON) { //
				break
			}
		}
	} else if p.tok == token.ATSERVER || p.tok == token.DOC || p.tok == token.HANDLER { // support @
		name = p.lit
		p.next()
	} else if p.tok == token.IDENT {
		name = p.lit
		p.next()
		for p.tok == token.SUB { // support user-api
			name += p.lit
			p.next()
			name += p.lit
			p.next()
		}
	} else {
		name = "_"
		p.expect(token.IDENT) // use expect() error handling
	}
	return &ast.Ident{NamePos: pos, Name: name}
}

type parseSpecFunction func(doc *ast.CommentGroup, pos token.Pos, keyword token.Token, iota int) ast.Spec

// parseGenDecl parse for *ImportSpec, *ValueSpec, and *TypeSpec.
// import ()
// var ()
// type ()
func (p *parser) parseGenDecl(keyword token.Token, f parseSpecFunction) *ast.GenDecl {
	if p.trace {
		defer un(trace(p, "GenDecl("+keyword.String()+")"))
	}

	doc := p.leadComment
	pos := p.expect(keyword)
	var lparen, rparen token.Pos
	var list []ast.Spec
	if p.tok == token.LPAREN {
		lparen = p.pos
		p.next()
		for iota := 0; p.tok != token.RPAREN && p.tok != token.EOF; iota++ {
			list = append(list, f(p.leadComment, pos, keyword, iota))
		}
		rparen = p.expect(token.RPAREN)
		p.expectSemi()
	} else {
		list = append(list, f(doc, pos, keyword, 0))
	}
	return &ast.GenDecl{
		Doc:    doc,
		TokPos: pos,
		Tok:    keyword,
		Lparen: lparen,
		Specs:  list,
		Rparen: rparen,
	}
}

func isValidImport(lit string) bool {
	const illegalChars = `!"#$%&'()*,:;<=>?[\]^{|}` + "`\uFFFD"
	s, _ := strconv.Unquote(lit) // go/scanner returns a legal string literal
	for _, r := range s {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegalChars, r) {
			return false
		}
	}
	return s != ""
}

// ----------------------------------------------------------------------------
// Declarations

func (p *parser) parseImportSpec(doc *ast.CommentGroup, _ token.Pos, _ token.Token, _ int) ast.Spec {
	if p.trace {
		defer un(trace(p, "ImportSpec"))
	}

	pos := p.pos
	var path string
	if p.tok == token.STRING {
		path = p.lit
		if !isValidImport(path) {
			p.error(pos, "invalid import path: "+path)
		}
		p.next()
	} else {
		p.expect(token.STRING) // use expect() error handling
	}
	p.expectSemi()

	spec := &ast.ImportSpec{
		Doc:     doc,
		Path:    &ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: path},
		Comment: p.lineComment,
	}
	p.imports = append(p.imports, spec)
	return spec
}

func (p *parser) parseTypeSpec(doc *ast.CommentGroup, _ token.Pos, _ token.Token, _ int) ast.Spec {
	if p.trace {
		defer un(trace(p, "TypeSpec"))
	}

	ident := p.parseIdent()
	spec := &ast.TypeSpec{
		Doc:  doc,
		Name: ident,
		//TypeParams: nil,
		//Comment:    nil,
	}
	// no need generics @see go/parser/parser.go:2560
	spec.Type = p.parseType()

	p.expectSemi()
	spec.Comment = p.lineComment
	return spec
}

func (p *parser) parseType() ast.Expr {
	if p.trace {
		defer un(trace(p, "Type"))
	}

	typ := p.tryIdentOrType()

	if typ == nil {
		pos := p.pos
		p.errorExpected(pos, "type")
		p.advance(exprEnd)
		return &ast.BadExpr{From: pos, To: p.pos}
	}
	return typ
}

func (p *parser) tryIdentOrType() ast.Expr {
	switch p.tok {
	case token.IDENT:
		return p.parseTypeName(nil)
	case token.LBRACK:
		lbrack := p.expect(token.LBRACK)
		return p.parseArrayType(lbrack, nil)
	case token.STRUCT, token.LBRACE: // support "type User {"  or "type User struct {"
		return p.parseStructType()
	case token.MUL:
		return p.parsePointerType()
	//case token.FUNC:
	//	typ := p.parseFuncType()
	//	return typ
	//case token.INTERFACE:
	//	return p.parseInterfaceType()
	case token.MAP:
		return p.parseMapType()
	//case token.CHAN, token.ARROW:
	//	return p.parseChanType()
	case token.LPAREN:
		lparen := p.pos
		p.next()
		typ := p.parseType()
		rparen := p.expect(token.RPAREN)
		return &ast.ParenExpr{Lparen: lparen, X: typ, Rparen: rparen}
	}

	// no type found
	return nil
}

func (p *parser) parseStructType() *ast.StructType {
	if p.trace {
		defer un(trace(p, "StructType"))
	}
	var pos, lbrace token.Pos
	if p.tok == token.STRUCT {
		pos = p.expect(token.STRUCT)
		lbrace = p.expect(token.LBRACE)
	} else {
		pos = p.expect(token.LBRACE)
		lbrace = pos
	}

	var list []*ast.Field
	for p.tok == token.IDENT || p.tok == token.MUL || p.tok == token.LPAREN {
		list = append(list, p.parseFieldDecl())
	}
	rbrace := p.expect(token.RBRACE)

	return &ast.StructType{
		Struct: pos,
		Fields: &ast.FieldList{
			Opening: lbrace,
			List:    list,
			Closing: rbrace,
		},
	}
}

func (p *parser) parseFieldDecl() *ast.Field {
	if p.trace {
		defer un(trace(p, "FieldDecl"))
	}

	doc := p.leadComment

	var names []*ast.Ident
	var typ ast.Expr
	if p.tok == token.IDENT {
		name := p.parseIdent()
		if p.tok == token.PERIOD || p.tok == token.STRING || p.tok == token.SEMICOLON || p.tok == token.RBRACE {
			// embedded type
			typ = name
			if p.tok == token.PERIOD { // for package.User
				typ = p.parseQualifiedIdent(name)
			}
		} else {
			// name1, name2, ... T
			names = []*ast.Ident{name}
			for p.tok == token.COMMA {
				p.next()
				names = append(names, p.parseIdent())
			}
			// no need this just parse type
			//// Careful dance: We don't know if we have an embedded instantiated
			//// type T[P1, P2, ...] or a field T of array type []E or [P]E.
			//if len(names) == 1 && p.tok == token.LBRACK {
			//	name, typ = p.parseArrayFieldOrTypeInstance(name)
			//	if name == nil {
			//		names = nil
			//	}
			//} else {
			//	// T P
			//	typ = p.parseType()
			//}
			typ = p.parseType()
		}
	} else {
		typ = p.parseType()
	}

	// tag
	var tag *ast.BasicLit
	if p.tok == token.STRING {
		tag = &ast.BasicLit{
			ValuePos: p.pos,
			Kind:     p.tok,
			Value:    p.lit,
		}
		p.next()
	}

	p.expectSemi()
	return &ast.Field{
		Doc:     doc,
		Names:   names,
		Type:    typ,
		Tag:     tag,
		Comment: p.lineComment,
	}
}

func (p *parser) parseQualifiedIdent(ident *ast.Ident) ast.Expr {
	if p.trace {
		defer un(trace(p, "QualifiedIdent"))
	}

	typ := p.parseTypeName(ident)

	return typ
}

// If the result is an identifier, it is not resolved.
func (p *parser) parseTypeName(ident *ast.Ident) ast.Expr {
	if p.trace {
		defer un(trace(p, "TypeName"))
	}

	if ident == nil {
		ident = p.parseIdent()
	}

	if p.tok == token.PERIOD {
		// ident is a package name
		p.next()
		sel := p.parseIdent()
		return &ast.SelectorExpr{X: ident, Sel: sel}
	}

	return ident
}

// parseArrayType "[" has already been consumed, and lbrack is its position.
// If len != nil it is the already consumed array length.
func (p *parser) parseArrayType(lbrack token.Pos, len ast.Expr) *ast.ArrayType {
	if p.trace {
		defer un(trace(p, "ArrayType"))
	}

	if len == nil {
		// TODO: finish me support [...]p or [5]p or [Rhs]p
		//p.exprLev++
		//// always permit ellipsis for more fault-tolerant parsing
		//if p.tok == token.ELLIPSIS { // [...]p
		//	len = &ast.Ellipsis{Ellipsis: p.pos}
		//	p.next()
		//} else if p.tok != token.RBRACK {
		//	len = p.parseRhs()
		//}
		//p.exprLev--
	}
	p.expect(token.LBRACK)
	elt := p.parseType()
	return &ast.ArrayType{
		Lbrack: lbrack,
		Len:    len,
		Elt:    elt,
	}
}

func (p *parser) parsePointerType() *ast.StarExpr {
	if p.trace {
		defer un(trace(p, "PointerType"))
	}

	star := p.expect(token.MUL)
	base := p.parseType()

	return &ast.StarExpr{
		Star: star,
		X:    base,
	}
}

func (p *parser) parseMapType() *ast.MapType {
	if p.trace {
		defer un(trace(p, "MapType"))
	}

	pos := p.expect(token.MAP)
	p.expect(token.LBRACK)
	key := p.parseType()
	p.expect(token.RBRACK)
	value := p.parseType()

	return &ast.MapType{Map: pos, Key: key, Value: value}
}

func (p *parser) parseDecl(sync map[token.Token]bool) ast.Decl {
	if p.trace {
		defer un(trace(p, "Declaration"))
	}

	//var f parseSpecFunction
	switch p.tok {
	case token.TYPE:
		return p.parseGenDecl(p.tok, p.parseTypeSpec)
	case token.ATSERVER, token.SERVICE:
		return p.parseService()
	case token.INFO:
		return p.parseInfoType()
	case token.IMPORT:
		return p.parseGenDecl(token.IMPORT, p.parseImportSpec)
	default:
		pos := p.pos
		p.errorExpected(pos, "declaration")
		p.advance(sync)
		return &ast.BadDecl{
			From: pos,
			To:   p.pos,
		}
	}

	panic("not support")
}

// ----------------------------------------------------------------------------
// API-info

func (p *parser) parseInfoType() *ast.InfoType {
	if p.trace {
		defer un(trace(p, "InfoType"))
	}
	pos := p.expect(token.INFO)
	p.expect(token.LPAREN)
	kvs := p.parseElementList()
	endPos := p.expect(token.RPAREN)
	p.expectSemi()

	return &ast.InfoType{
		TokPos: pos,
		Kvs:    kvs,
		RParen: endPos,
	}
}

func (p *parser) parseElementList() []*ast.KeyValueExpr {
	if p.trace {
		defer un(trace(p, "ElementList"))
	}
	var kvs []*ast.KeyValueExpr
	for p.tok != token.RPAREN && p.tok != token.EOF {
		kvs = append(kvs, p.parseElement(true))
		p.expectSemi()
	}
	return kvs
}

func (p *parser) parseElement(expectColon bool) *ast.KeyValueExpr {
	if p.trace {
		defer un(trace(p, "Element"))
	}

	key := p.parseApiIdent()
	var colon token.Pos
	if expectColon {
		colon = p.expect(token.COLON)
	}
	var value ast.Expr
	if p.tok == token.STRING {
		value = &ast.BasicLit{
			ValuePos: p.pos,
			Kind:     token.STRING,
			Value:    p.lit,
		}
		p.next()
	} else {
		value = p.parseIdent()
	}
	return &ast.KeyValueExpr{
		Key:   key,
		Colon: colon,
		Value: value,
	}
}

func (p *parser) parseService() *ast.Service {
	if p.trace {
		defer un(trace(p, "Service"))
	}

	var atServer *ast.AtServer
	if p.tok == token.ATSERVER {
		atServer = p.parseAtServer()
	}
	serviceApi := p.parseServiceApi()

	return &ast.Service{
		AtServer:   atServer,
		ServiceApi: serviceApi,
	}
}

func (p *parser) parseAtServer() *ast.AtServer {
	if p.trace {
		defer un(trace(p, "AtServer"))
	}

	pos := p.pos
	p.expect(token.LPAREN)
	kvs := p.parseElementList()
	rParen := p.expect(token.RPAREN)
	p.expectSemi()
	return &ast.AtServer{
		TokPos: pos,
		Kvs:    kvs,
		RParen: rParen,
	}
}

func (p *parser) parseServiceApi() *ast.ServiceApi {
	if p.trace {
		defer un(trace(p, "ServiceApi"))
	}

	pos := p.expect(token.SERVICE)
	name := p.parseApiIdent()
	lBrace := p.expect(token.LBRACE)
	serviceRoutes := p.parseServiceRouteList()
	rBrace := p.expect(token.RBRACE)
	p.expectSemi()

	return &ast.ServiceApi{
		TokPos:       pos,
		Name:         name,
		LBrace:       lBrace,
		ServiceRoute: serviceRoutes,
		RBrace:       rBrace,
	}
}

func (p *parser) parseServiceRouteList() []*ast.ServiceRoute {
	if p.trace {
		defer un(trace(p, "ServiceRouteList"))
	}

	var routes []*ast.ServiceRoute
	for p.tok != token.RBRACE && p.tok != token.EOF {
		routes = append(routes, p.parseServiceRoute())
	}
	return routes
}

func (p *parser) parseServiceRoute() *ast.ServiceRoute {
	if p.trace {
		defer un(trace(p, "ServiceRoute"))
	}

	pos := p.pos
	var atDoc, atHandler *ast.KeyValueExpr
	for p.tok == token.DOC || p.tok == token.HANDLER {
		if p.tok == token.DOC {
			atDoc = p.parseElement(false)
		} else if p.tok == token.HANDLER {
			atHandler = p.parseElement(false)
		}
		p.expectSemi()
	}
	route := p.parseRoute()

	return &ast.ServiceRoute{
		TokPos:    pos,
		AtDoc:     atDoc,
		AtHandler: atHandler,
		Route:     route,
	}
}

func (p *parser) parseRoute() *ast.Route {
	if p.trace {
		defer un(trace(p, "Route"))
	}

	method := p.parseIdent()
	path := p.parseApiIdent()
	rPos := path.End()

	var req *ast.ParenExpr
	if p.tok == token.LPAREN {
		lparen := p.pos
		p.next()
		typ := p.parseType()
		rparen := p.expect(token.RPAREN)
		req = &ast.ParenExpr{Lparen: lparen, X: typ, Rparen: rparen}
		rPos = rparen
	}

	var returnPos token.Pos
	if p.tok == token.RETURNS {
		returnPos = p.pos
		p.next()
		rPos = returnPos
	}

	var resp *ast.ParenExpr
	if p.tok == token.LPAREN {
		lparen := p.pos
		p.next()
		typ := p.parseType()
		rparen := p.expect(token.RPAREN)
		resp = &ast.ParenExpr{Lparen: lparen, X: typ, Rparen: rparen}
		rPos = rparen
	}
	p.expectSemi()

	return &ast.Route{
		Method:    method,
		Path:      path,
		Req:       req,
		ReturnPos: returnPos,
		Resp:      resp,
		RPos:      rPos,
	}
}
