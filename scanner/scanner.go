package scanner

import (
	"fmt"
	"github.com/zeromicro/api-ast/token"
	"path/filepath"
	"unicode/utf8"
)

// ----------------------------------------------------------------------------
// Mode

// Mode A mode value is a set of flags (or 0).
// They control scanner behavior.
type Mode int

const (
	ScanComments    Mode = 1 << iota // return comments as COMMENT tokens
	dontInsertSemis                  // do not automatically insert semicolons - for testing only
)

// ----------------------------------------------------------------------------
// Scanner

// An ErrorHandler may be provided to Scanner.Init. If a syntax error is
// encountered and a handler was installed, the handler is called with a
// position and an error message. The position points to the beginning of
// the offending token.
//
type ErrorHandler func(pos token.Position, msg string)

type Scanner struct {
	// immutable state
	file *token.File  // source file handle
	dir  string       // directory portion of file.Name()
	src  []byte       // source
	err  ErrorHandler // error reporting; or nil
	mode Mode         // scanning mode

	// scanning state
	ch         rune // current character
	offset     int  // character offset
	rdOffset   int  // reading offset (position after current character)
	lineOffset int  // current line offset
	insertSemi bool // insert a semicolon before next newline

	// public state - ok to modify
	ErrorCount int // number of errors encountered
}

func (s *Scanner) Init(file *token.File, src []byte, err ErrorHandler, mode Mode) {
	if file.Size() != len(src) {
		panic(fmt.Sprintf("file size (%d) does not match src len (%d)", file.Size(), len(src)))
	}
	s.file = file
	s.dir, _ = filepath.Split(file.Name())
	s.src = src
	s.err = err
	s.mode = mode

	s.ch = ' '
	s.offset = 0
	s.rdOffset = 0
	s.lineOffset = 0
	s.insertSemi = false
	s.ErrorCount = 0

	s.next()
	if s.ch == bom {
		s.next() // ignore BOM at file beginning
	}
}

const (
	bom = 0xFEFF // byte order mark, only permitted as very first character
	eof = -1     // end of file
)

func (s *Scanner) next() {
	if s.rdOffset < len(s.src) {
		s.offset = s.rdOffset
		if s.ch == '\n' {
			s.lineOffset = s.offset
			s.file.AddLine(s.offset)
		}
		r, w := rune(s.src[s.rdOffset]), 1
		switch {
		case r == 0:
			s.error(s.offset, "illegal character NULL")
		case r >= utf8.RuneSelf:
			// not ASCII
			r, w = utf8.DecodeRune(s.src[s.rdOffset:])
			if r == utf8.RuneError && w == 1 {
				s.error(s.offset, "illegal UTF-8 encoding")
			} else if r == bom && s.offset > 0 {
				s.error(s.offset, "illegal byte order mark")
			}
		}
		s.rdOffset += w
		s.ch = r
	} else {
		s.offset = len(s.src)
		if s.ch == '\n' {
			s.lineOffset = s.offset
			s.file.AddLine(s.offset)
		}
		s.ch = eof
	}
}

func (s *Scanner) peek() byte {
	if s.rdOffset < len(s.src) {
		return s.src[s.rdOffset]
	}
	return 0
}

func (s *Scanner) Scan() (token.Pos, token.Token, string) {
scanAgain:
	s.skipWhitespace()

	pos := s.file.Pos(s.offset)
	var tok token.Token
	var lit string

	insertSemi := false
	switch ch := s.ch; {
	case isLetter(ch):
		lit = s.scanIdentifier()
		if len(lit) > 1 {
			tok = token.Lookup(lit)
			switch tok {
			case token.IDENT:
				insertSemi = true
			}
		} else {
			insertSemi = true
			tok = token.IDENT
		}
	case isDecimal(ch) || ch == '.' && isDecimal(rune(s.peek())):
		insertSemi = true
		tok, lit = s.scanNumber()
	case ch == '@': // for @ support scan
		s.next()
		lit = "@" + s.scanIdentifier()
		if len(lit) > 1 {
			tok = token.Lookup(lit)
			switch tok {
			case token.IDENT:
				insertSemi = true
			}
		} else {
			insertSemi = true
			tok = token.IDENT
		}
	default:
		s.next()
		switch ch {
		case -1:
			if s.insertSemi {
				s.insertSemi = false
				return pos, token.SEMICOLON, "\n"
			}
			tok = token.EOF
		case '\n':
			s.insertSemi = false
			return pos, token.SEMICOLON, "\n"
		case '"':
			insertSemi = true
			tok = token.STRING
			lit = s.scanString()
		case '\'':
			insertSemi = true
			tok = token.CHAR
			lit = s.scanRune()
		case '`':
			insertSemi = true
			tok = token.STRING
			lit = s.scanRawString()
		case ':':
			tok = s.switch2(token.COLON, token.DEFINE)
		case '.':
			// fractions starting with a '.' are handled by outer switch
			tok = token.PERIOD
			if s.ch == '.' && s.peek() == '.' {
				s.next()
				s.next() // consume last '.'
				tok = token.ELLIPSIS
			}
		case ',':
			tok = token.COMMA
		case ';':
			tok = token.SEMICOLON
			lit = ";"
		case '(':
			tok = token.LPAREN
		case ')':
			insertSemi = true
			tok = token.RPAREN
		case '[':
			tok = token.LBRACK
		case ']':
			insertSemi = true
			tok = token.RBRACK
		case '{':
			tok = token.LBRACE
		case '}':
			insertSemi = true
			tok = token.RBRACE
		case '+':
			tok = s.switch3(token.ADD, token.ADD_ASSIGN, '+', token.INC)
			if tok == token.INC {
				insertSemi = true
			}
		case '-':
			tok = s.switch3(token.SUB, token.SUB_ASSIGN, '-', token.DEC)
			if tok == token.DEC {
				insertSemi = true
			}
		case '*':
			tok = s.switch2(token.MUL, token.MUL_ASSIGN)
		case '/':
			if s.ch == '/' || s.ch == '*' {
				// comment
				if s.insertSemi && s.findLineEnd() {
					// reset position to the beginning of the comment
					s.ch = '/'
					s.offset = s.file.Offset(pos)
					s.rdOffset = s.offset + 1
					s.insertSemi = false // newline consumed
					return pos, token.SEMICOLON, "\n"
				}
				comment := s.scanComment()
				if s.mode&ScanComments == 0 {
					// skip comment
					s.insertSemi = false // newline consumed
					goto scanAgain
				}
				tok = token.COMMENT
				lit = comment
			} else {
				tok = s.switch2(token.QUO, token.QUO_ASSIGN)
			}
		case '%':
			tok = s.switch2(token.REM, token.REM_ASSIGN)
		case '^':
			tok = s.switch2(token.XOR, token.XOR_ASSIGN)
		case '<':
			if s.ch == '-' {
				s.next()
				tok = token.ARROW
			} else {
				tok = s.switch4(token.LSS, token.LEQ, '<', token.SHL, token.SHL_ASSIGN)
			}
		case '>':
			tok = s.switch4(token.GTR, token.GEQ, '>', token.SHR, token.SHR_ASSIGN)
		case '=':
			tok = s.switch2(token.ASSIGN, token.EQL)
		case '!':
			tok = s.switch2(token.NOT, token.NEQ)
		case '&':
			if s.ch == '^' {
				s.next()
				tok = s.switch2(token.AND_NOT, token.AND_NOT_ASSIGN)
			} else {
				tok = s.switch3(token.AND, token.AND_ASSIGN, '&', token.LAND)
			}
		case '|':
			tok = s.switch3(token.OR, token.OR_ASSIGN, '|', token.LOR)
		case '~':
			tok = token.TILDE
		default:
			// next reports unexpected BOMs - don't repeat
			if ch != bom {
				s.errorf(s.file.Offset(pos), "illegal character %#U", ch)
			}
			insertSemi = s.insertSemi // preserve insertSemi info
			tok = token.ILLEGAL
			lit = string(ch)
		}
	}

	if s.mode&dontInsertSemis == 0 {
		s.insertSemi = insertSemi
	}
	return pos, tok, lit
}
