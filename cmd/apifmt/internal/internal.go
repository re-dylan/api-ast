package internal

import (
	"bytes"
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/parser"
	"github.com/zeromicro/api-ast/token"
)

func Parse(fSet *token.FileSet, filename string, src []byte, parserMode parser.Mode) (*ast.File, error) {
	file, err := parser.ParseFile(fSet, filename, src, parserMode)
	if err != nil {
		return nil, err
	}
	return file, err
}

func Format(fSet *token.FileSet, file *ast.File, src []byte) ([]byte, error) {
	// Complete source file.
	var buf bytes.Buffer
	err := cfg.Fprint(&buf, fSet, file)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
