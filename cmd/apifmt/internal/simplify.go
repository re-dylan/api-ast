package internal

import "github.com/zeromicro/api-ast/ast"

func Simplify(f *ast.File) {
	// TODO: finish me
	// remove empty declarations such as "const ()", etc
	//removeEmptyDeclGroups(f)

	var s simplifier
	ast.Walk(s, f)
}

type simplifier struct{}

func (s simplifier) Visit(node ast.Node) ast.Visitor {
	// TODO: finish me
	return s
}
