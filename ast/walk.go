package ast

import "fmt"

type Visitor interface {
	Visit(Node) Visitor
}

func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}
	switch n := node.(type) {
	case *File:
		if n.Doc != nil {
			Walk(v, n.Doc)
		}
		//Walk(v, n.Syntax)
		for _, x := range n.Decls {
			Walk(v, x)
		}
	// TODO: finish me
	default:
		panic(fmt.Sprintf("ast.Walk: unexpected node type %T", n))
	}

	v.Visit(nil)
}
