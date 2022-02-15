package scanner

import (
	"fmt"
	"go/scanner"
	"io"
)

type (
	Error = scanner.Error

	ErrorList = scanner.ErrorList
)

func PrintError(w io.Writer, err error) {
	scanner.PrintError(w, err)
}

func (s *Scanner) error(offs int, msg string) {
	if s.err != nil {
		s.err(s.file.Position(s.file.Pos(offs)), msg)
	}
	s.ErrorCount++
}

func (s *Scanner) errorf(offs int, format string, args ...interface{}) {
	s.error(offs, fmt.Sprintf(format, args...))
}
