package token

import (
	"go/token"
)

const (
	NoPos = token.NoPos
)

type (
	Pos = token.Pos

	Position = token.Position

	File = token.File

	FileSet = token.FileSet
)

func NewFileSet() *FileSet {
	return token.NewFileSet()
}