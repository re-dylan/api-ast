package ast

import (
	"github.com/zeromicro/api-ast/token"
	"go/ast"
)

type (
	Node = ast.Node

	Expr interface {
		Node
		exprNode()
	}

	Decl interface {
		Node
		declNode()
	}
)

// ----------------------------------------------------------------------------
// API go-zero api

type (
	SyntaxSpec struct {
		TokPos token.Pos
		Assign token.Pos // position of '='
		Name   *BasicLit
	}
)

func (x *SyntaxSpec) Pos() token.Pos { return x.TokPos }
func (x *SyntaxSpec) End() token.Pos { return x.Name.End() }

// ----------------------------------------------------------------------------
// Comment and CommentGroup

type (
	Comment = ast.Comment

	CommentGroup = ast.CommentGroup
)

type (
	// A BadExpr node is a placeholder for an expression containing
	// syntax errors for which a correct expression node cannot be
	// created.
	//
	BadExpr struct {
		From, To token.Pos // position range of bad expression
	}

	// Ident node represents an identifier.
	Ident struct {
		NamePos token.Pos // identifier position
		Name    string    // identifier name
		// TODO: Obj
		//Obj     *Object   // denoted object; or nil
	}

	// BasicLit node represents a literal of basic type.
	BasicLit struct {
		ValuePos token.Pos   // literal position
		Kind     token.Token // token.INT, token.FLOAT, token.IMAG, token.CHAR, or token.STRING
		Value    string      // literal string; e.g. 42, 0x7f, 3.14, 1e-9, 2.4i, 'a', '\x7f', "foo" or `\m\n\o`
	}

	// A SelectorExpr node represents an expression followed by a selector.
	SelectorExpr struct {
		X   Expr   // expression
		Sel *Ident // field selector
	}

	// A StarExpr node represents an expression of the form "*" Expression.
	// Semantically it could be a unary "*" expression, or a pointer type.
	//
	StarExpr struct {
		Star token.Pos // position of "*"
		X    Expr      // operand
	}

	// A ParenExpr node represents a parenthesized expression.
	ParenExpr struct {
		Lparen token.Pos // position of "("
		X      Expr      // parenthesized expression
		Rparen token.Pos // position of ")"
	}
)

func (x *BadExpr) Pos() token.Pos { return x.From }
func (x *BadExpr) End() token.Pos { return x.To }
func (x *BadExpr) exprNode()      {}

func (x *Ident) Pos() token.Pos { return x.NamePos }
func (x *Ident) End() token.Pos { return token.Pos(int(x.NamePos) + len(x.Name)) }
func (x *Ident) exprNode()      {}

func (x *BasicLit) Pos() token.Pos { return x.ValuePos }
func (x *BasicLit) End() token.Pos { return token.Pos(int(x.ValuePos) + len(x.Value)) }
func (x *BasicLit) exprNode()      {}

func (x *SelectorExpr) Pos() token.Pos { return x.X.Pos() }
func (x *SelectorExpr) End() token.Pos { return x.Sel.End() }
func (x *SelectorExpr) exprNode()      {}

func (x *StarExpr) Pos() token.Pos { return x.Star }
func (x *StarExpr) End() token.Pos { return x.X.End() }
func (x *StarExpr) exprNode()      {}

func (x *ParenExpr) Pos() token.Pos { return x.Lparen }
func (x *ParenExpr) End() token.Pos { return x.Rparen }
func (x *ParenExpr) exprNode()      {}

// ----------------------------------------------------------------------------
// Declarations

type (
	Spec interface {
		Node
		specNode()
	}

	// An ImportSpec node represents a single package import.
	ImportSpec struct {
		Doc *CommentGroup // associated documentation; or nil
		//Name    *Ident        // local package name (including "."); or nil
		Path    *BasicLit     // import path
		Comment *CommentGroup // line comments; or nil
		EndPos  token.Pos     // end of spec (overrides Path.Pos if nonzero)
	}

	// A TypeSpec node represents a type declaration (TypeSpec production).
	TypeSpec struct {
		Doc  *CommentGroup // associated documentation; or nil
		Name *Ident        // type name
		//TypeParams *FieldList    // type parameters; or nil
		//Assign     token.Pos     // position of '=', if any
		Type    Expr          // *Ident, *ParenExpr, *SelectorExpr, *StarExpr, or any of the *XxxTypes
		Comment *CommentGroup // line comments; or nil
	}
)

func (x *ImportSpec) Pos() token.Pos { return x.Path.Pos() }
func (x *ImportSpec) End() token.Pos { return x.EndPos }
func (x *ImportSpec) specNode()      {}

func (x *TypeSpec) Pos() token.Pos { return x.Name.Pos() }
func (x *TypeSpec) End() token.Pos { return x.Type.Pos() }
func (x *TypeSpec) specNode()      {}

type (
	BadDecl struct {
		From, To token.Pos // position range of bad declaration
	}

	// A GenDecl node (generic declaration node) represents an import,
	// constant, type or variable declaration. A valid Lparen position
	// (Lparen.IsValid()) indicates a parenthesized declaration.
	//
	// Relationship between Tok value and Specs element type:
	//
	//	token.IMPORT  *ImportSpec
	//	token.CONST   *ValueSpec
	//	token.TYPE    *TypeSpec
	//	token.VAR     *ValueSpec
	//
	GenDecl struct {
		Doc    *CommentGroup // associated documentation; or nil
		TokPos token.Pos     // position of Tok
		Tok    token.Token   // IMPORT, CONST, TYPE, or VAR
		Lparen token.Pos     // position of '(', if any
		Specs  []Spec
		Rparen token.Pos // position of ')', if any
	}
)

func (x *BadDecl) Pos() token.Pos { return x.From }
func (x *BadDecl) End() token.Pos { return x.To }
func (x *BadDecl) declNode()      {}

func (x *GenDecl) Pos() token.Pos { return x.TokPos }
func (x *GenDecl) End() token.Pos {
	if x.Rparen.IsValid() {
		return x.Rparen + 1
	}
	return x.Specs[0].End()
}
func (x *GenDecl) declNode() {}

// ----------------------------------------------------------------------------
// Expressions and types

type (

	// A Field represents a Field declaration list in a struct type,
	// a method list in an interface type, or a parameter/result declaration
	// in a signature.
	// Field.Names is nil for unnamed parameters (parameter lists which only contain types)
	// and embedded struct fields. In the latter case, the field name is the type name.
	// Field.Names contains a single name "type" for elements of interface type lists.
	// Types belonging to the same type list share the same "type" identifier which also
	// records the position of that keyword.
	//
	Field struct {
		Doc     *CommentGroup // associated documentation; or nil
		Names   []*Ident      // field/method/(type) parameter names, or type "type"; or nil
		Type    Expr          // field/method/parameter type, type list type; or nil
		Tag     *BasicLit     // field tag; or nil
		Comment *CommentGroup // line comments; or nil
	}

	FieldList struct {
		Opening token.Pos
		List    []*Field
		Closing token.Pos
	}
)

func (f *FieldList) Pos() token.Pos {
	if f.Opening.IsValid() {
		return f.Opening
	}
	// the list should not be empty in this case;
	// be conservative and guard against bad ASTs
	if len(f.List) > 0 {
		return f.List[0].Pos()
	}
	return token.NoPos
}

func (f *FieldList) End() token.Pos {
	if f.Closing.IsValid() {
		return f.Closing + 1
	}
	// the list should not be empty in this case;
	// be conservative and guard against bad ASTs
	if n := len(f.List); n > 0 {
		return f.List[n-1].End()
	}
	return token.NoPos
}

func (f *Field) Pos() token.Pos {
	if len(f.Names) > 0 {
		return f.Names[0].Pos()
	}
	if f.Type != nil {
		return f.Type.Pos()
	}
	return token.NoPos
}

func (f *Field) End() token.Pos {
	if f.Tag != nil {
		return f.Tag.End()
	}
	if f.Type != nil {
		return f.Type.End()
	}
	if len(f.Names) > 0 {
		return f.Names[len(f.Names)-1].End()
	}
	return token.NoPos
}

// NumFields returns the number of parameters or struct fields represented by a FieldList.
func (f *FieldList) NumFields() int {
	n := 0
	if f != nil {
		for _, g := range f.List {
			m := len(g.Names)
			if m == 0 {
				m = 1
			}
			n += m
		}
	}
	return n
}

type (
	// An ArrayType node represents an array or slice type.
	ArrayType struct {
		Lbrack token.Pos // position of "["
		Len    Expr      // Ellipsis node for [...]T array types, nil for slice types
		Elt    Expr      // element type
	}

	StructType struct {
		Struct token.Pos
		Fields *FieldList
		//Incomplete bool
	}

	// A MapType node represents a map type.
	MapType struct {
		Map   token.Pos // position of "map" keyword
		Key   Expr
		Value Expr
	}
)

func (x *ArrayType) Pos() token.Pos { return x.Lbrack }
func (x *ArrayType) End() token.Pos { return x.Elt.End() }
func (x *ArrayType) exprNode()      {}

func (x *StructType) Pos() token.Pos { return x.Struct }
func (x *StructType) End() token.Pos { return x.Fields.End() }
func (x *StructType) exprNode()      {}

func (x *MapType) Pos() token.Pos { return x.Map }
func (x *MapType) End() token.Pos { return x.Value.End() }
func (x *MapType) exprNode()      {}

// ----------------------------------------------------------------------------
// API AST

// API Info
type (
	KeyValueExpr struct {
		Key   *Ident
		Colon token.Pos // position of ":"
		Value Expr      // *BasicLit for info or *Ident for server
	}

	InfoType struct {
		TokPos token.Pos
		Kvs    []*KeyValueExpr
		RParen token.Pos
	}
)

func (x *InfoType) Pos() token.Pos { return x.TokPos }
func (x *InfoType) End() token.Pos { return x.RParen }
func (x *InfoType) declNode()      {}

func (x *KeyValueExpr) Pos() token.Pos { return x.Key.Pos() }
func (x *KeyValueExpr) End() token.Pos { return x.Value.End() }
func (x *KeyValueExpr) exprNode()      {}

// Server
type (
	Service struct {
		AtServer   *AtServer // optional , can be nil
		ServiceApi *ServiceApi
	}

	AtServer struct {
		TokPos token.Pos
		//LParen     token.Pos
		Kvs    []*KeyValueExpr
		RParen token.Pos
	}

	ServiceApi struct {
		TokPos       token.Pos
		Name         *Ident
		LBrace       token.Pos
		ServiceRoute []*ServiceRoute
		RBrace       token.Pos
	}

	ServiceRoute struct {
		TokPos    token.Pos
		AtDoc     *KeyValueExpr
		AtHandler *KeyValueExpr
		Route     *Route
	}

	//AtDoc struct {
	//	TokPos token.Pos
	//	Value  *Ident
	//}
	//
	//AtHandler = AtDoc

	Route struct {
		Method    *Ident
		Path      *Ident
		Req       *ParenExpr
		ReturnPos token.Pos
		Resp      *ParenExpr
		RPos      token.Pos // because Resp Req is optional, need this for EndPos
	}
)

func (x *Service) Pos() token.Pos {
	if x.AtServer != nil {
		return x.AtServer.Pos()
	}
	return x.ServiceApi.Pos()
}
func (x *Service) End() token.Pos { return x.ServiceApi.End() }
func (x *Service) declNode()      {}

func (x *AtServer) Pos() token.Pos { return x.TokPos }
func (x *AtServer) End() token.Pos { return x.RParen }

func (x *ServiceApi) Pos() token.Pos { return x.TokPos }
func (x *ServiceApi) End() token.Pos { return x.RBrace }

func (x *ServiceRoute) Pos() token.Pos { return x.TokPos }
func (x *ServiceRoute) End() token.Pos { return x.Route.End() }

func (x *Route) Pos() token.Pos { return x.Method.Pos() }
func (x *Route) End() token.Pos { return x.RPos }

// ----------------------------------------------------------------------------
// Files and packages

// A File node represents a Go source file.
//
// The Comments list contains all comments in the source file in order of
// appearance, including the comments that are pointed to from other nodes
// via Doc and Comment fields.
//
// For correct printing of source code containing comments (using packages
// go/format and go/printer), special care must be taken to update comments
// when a File's syntax tree is modified: For printing, comments are interspersed
// between tokens based on their position. If syntax tree nodes are
// removed or moved, relevant comments in their vicinity must also be removed
// (from the File.Comments list) or moved accordingly (by updating their
// positions). A CommentMap may be used to facilitate some of these operations.
//
// Whether and how a comment is associated with a node depends on the
// interpretation of the syntax tree by the manipulating program: Except for Doc
// and Comment comments directly associated with nodes, the remaining comments
// are "free-floating" (see also issues #18593, #20744).
//
type File struct {
	Doc *CommentGroup // associated documentation; or nil
	//Package    token.Pos       // position of "package" keyword
	//Name       *Ident          // package name
	Syntax *SyntaxSpec

	Decls      []Decl          // top-level declarations; or nil
	Scope      *Scope          // package scope (this file only)
	Imports    []*ImportSpec   // imports in this file
	Unresolved []*Ident        // unresolved identifiers in this file
	Comments   []*CommentGroup // list of all comments in the source file
}

func (f *File) Pos() token.Pos { return f.Syntax.Pos() } //return f.Package
func (f *File) End() token.Pos {
	if n := len(f.Decls); n > 0 {
		return f.Decls[n-1].End()
	}
	return f.Syntax.End()
}
