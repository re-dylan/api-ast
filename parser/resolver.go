package parser

import (
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
)

const debugResolve = false

func resolveFile(file *ast.File, handle *token.File, declErr func(token.Pos, string)) {
	// TODO: resolve Ident
}
