package printer

import (
	"fmt"
	"github.com/zeromicro/api-ast/ast"
	"github.com/zeromicro/api-ast/token"
	"os"
)

type whiteSpace byte

const (
	ignore   = whiteSpace(0)
	blank    = whiteSpace(' ')
	vtab     = whiteSpace('\v')
	newline  = whiteSpace('\n')
	formfeed = whiteSpace('\f')
	indent   = whiteSpace('>')
	unindent = whiteSpace('<')
)

// pmode value represents the current printer mode.
type pmode int

const (
	noExtraBlank     pmode = 1 << iota // disables extra blank after /*-style comment
	noExtraLinebreak                   // disables extra line break after /*-style comment
)

type printer struct {
	config Config
	fSet   *token.FileSet

	// Current state
	output      []byte       // raw printer result
	indent      int          // current indentation
	level       int          // level == 0: outside composite literal; level > 0: inside composite literal
	mode        pmode        // current printer mode
	impliedSemi bool         // if set, a linebreak implies a semicolon
	lastTok     token.Token  // last token printed (token.ILLEGAL if it's whitespace)
	prevOpen    token.Token  // previous non-brace "open" token (, [, or token.ILLEGAL
	wsbuf       []whiteSpace // delayed white space

	// Positions
	// The out position differs from the pos position when the result
	// formatting differs from the source formatting (in the amount of
	// white space). If there's a difference and SourcePos is set in
	// ConfigMode, //line directives are used in the output to restore
	// original source positions for a reader.
	pos     token.Position // current position in AST (source) space
	out     token.Position // current position in output space
	last    token.Position // value of pos after calling writeString
	linePtr *int           // if set, record out.Line for the next token in *linePtr

	// comment
	commentInfo
	comments []*ast.CommentGroup // may be nil

	cachedPos  token.Pos
	cachedLine int // line corresponding to cachedPos
}

func (p *printer) init(cfg *Config, fSet *token.FileSet, nodeSize map[ast.Node]int) {
	p.config = *cfg
	p.fSet = fSet
	p.wsbuf = make([]whiteSpace, 0, 16)
	p.cachedPos = -1
}

func (p *printer) printNode(node ast.Node) error {
	if n, ok := node.(*ast.File); ok {
		p.comments = n.Comments
	}

	p.nextComment()

	p.print(pmode(0)) // pmode(0) just rest p.mode

	switch n := node.(type) {
	// TODO: finish me
	case ast.Expr:

	case ast.Decl:
	case ast.Spec:
	//case []ast.Decl:
	case *ast.File:
		p.file(n)
	}

	return fmt.Errorf("go/printer: unsupported node type %T", node)
}

// nlimit limits n to maxNewlines.
func nlimit(n int) int {
	if n > maxNewlines {
		n = maxNewlines
	}
	return n
}

func mayCombine(prev token.Token, next byte) (b bool) {
	switch prev {
	case token.INT:
		b = next == '.' // 1.
	case token.ADD:
		b = next == '+' // ++
	case token.SUB:
		b = next == '-' // --
	case token.QUO:
		b = next == '*' // /*
	case token.LSS:
		b = next == '-' || next == '<' // <- or <<
	case token.AND:
		b = next == '&' || next == '^' // && or &^
	}
	return
}

// flush prints any pending comments and whitespace occurring textually
// before the position of the next token tok. The flush result indicates
// if a newline was written or if a formfeed was dropped from the whitespace
// buffer.
//
func (p *printer) flush(next token.Position, tok token.Token) (wroteNewline, droppedFF bool) {
	if p.commentBefore(next) {
		wroteNewline, droppedFF = p.intersperseComments(next, tok)
	} else {
		p.writeWhitespace(len(p.wsbuf))
	}
	return
}

func (p *printer) print(args ...interface{}) {
	for _, arg := range args {
		// information about the current arg
		var data string
		var isLit bool
		var impliedSemi bool // value for p.impliedSemi after this arg

		// record previous opening token, if any
		switch p.lastTok {
		case token.ILLEGAL:
			// ignore (white space)
		case token.LPAREN, token.LBRACK:
			p.prevOpen = p.lastTok
		default:
			// other tokens followed any opening token
			p.prevOpen = token.ILLEGAL
		}

		switch x := arg.(type) {
		case pmode:
			// toggle printer mode
			p.mode ^= x
			continue

		case whiteSpace:
			if x == ignore {
				// don't add ignore's to the buffer; they
				// may screw up "correcting" unindents (see
				// LabeledStmt)
				continue
			}
			i := len(p.wsbuf)
			if i == cap(p.wsbuf) {
				// Whitespace sequences are very short so this should
				// never happen. Handle gracefully (but possibly with
				// bad comment placement) if it does happen.
				p.writeWhitespace(i)
				i = 0
			}
			p.wsbuf = p.wsbuf[0 : i+1]
			p.wsbuf[i] = x
			if x == newline || x == formfeed {
				// newlines affect the current state (p.impliedSemi)
				// and not the state after printing arg (impliedSemi)
				// because comments can be interspersed before the arg
				// in this case
				p.impliedSemi = false
			}
			p.lastTok = token.ILLEGAL
			continue

		case *ast.Ident:
			data = x.Name
			impliedSemi = true
			p.lastTok = token.IDENT

		case *ast.BasicLit:
			data = x.Value
			isLit = true
			impliedSemi = true
			p.lastTok = x.Kind

		case token.Token:
			s := x.String()
			if mayCombine(p.lastTok, s[0]) {
				// the previous and the current token must be
				// separated by a blank otherwise they combine
				// into a different incorrect token sequence
				// (except for token.INT followed by a '.' this
				// should never happen because it is taken care
				// of via binary expression formatting)
				if len(p.wsbuf) != 0 {
					p.internalError("whitespace buffer not empty")
				}
				p.wsbuf = p.wsbuf[0:1]
				p.wsbuf[0] = ' '
			}
			data = s
			// some keywords followed by a newline imply a semicolon
			switch x {
			case token.INC, token.DEC, token.RPAREN, token.RBRACK, token.RBRACE: //token.BREAK, token.CONTINUE, token.FALLTHROUGH, token.RETURN
				impliedSemi = true
			}
			p.lastTok = x

		case token.Pos:
			if x.IsValid() {
				p.pos = p.posFor(x) // accurate position of next item
			}
			continue

		case string:
			// incorrect AST - print error message
			data = x
			isLit = true
			impliedSemi = true
			p.lastTok = token.STRING

		default:
			_, _ = fmt.Fprintf(os.Stderr, "print: unsupported argument %v (%T)\n", arg, arg)
			panic("go/printer type")
		}
		// data != ""

		next := p.pos // estimated/accurate position of next item
		wroteNewline, droppedFF := p.flush(next, p.lastTok)

		// intersperse extra newlines if present in the source and
		// if they don't cause extra semicolons (don't do this in
		// flush as it will cause extra newlines at the end of a file)
		if !p.impliedSemi {
			n := nlimit(next.Line - p.pos.Line)
			// don't exceed maxNewlines if we already wrote one
			if wroteNewline && n == maxNewlines {
				n = maxNewlines - 1
			}
			if n > 0 {
				ch := byte('\n')
				if droppedFF {
					ch = '\f' // use formfeed since we dropped one before
				}
				p.writeByte(ch, n)
				impliedSemi = false
			}
		}

		// the next token starts now - record its line number if requested
		if p.linePtr != nil {
			*p.linePtr = p.out.Line
			p.linePtr = nil
		}

		p.writeString(next, data, isLit)
		p.impliedSemi = impliedSemi
	}
}

// ----------------------------------------------------------------------------
// write

// whiteWhitespace writes the first n whitespace entries.
func (p *printer) writeWhitespace(n int) {
	// write entries
	for i := 0; i < n; i++ {
		switch ch := p.wsbuf[i]; ch {
		case ignore:
			// ignore!
		case indent:
			p.indent++
		case unindent:
			p.indent--
			if p.indent < 0 {
				p.internalError("negative indentation:", p.indent)
				p.indent = 0
			}
		case newline, formfeed:
			// A line break immediately followed by a "correcting"
			// unindent is swapped with the unindent - this permits
			// proper label positioning. If a comment is between
			// the line break and the label, the unindent is not
			// part of the comment whitespace prefix and the comment
			// will be positioned correctly indented.
			if i+1 < n && p.wsbuf[i+1] == unindent {
				// Use a formfeed to terminate the current section.
				// Otherwise, a long label name on the next line leading
				// to a wide column may increase the indentation column
				// of lines before the label; effectively leading to wrong
				// indentation.
				p.wsbuf[i], p.wsbuf[i+1] = unindent, formfeed
				i-- // do it again
				continue
			}
			fallthrough
		default:
			p.writeByte(byte(ch), 1)
		}
	}

	// shift remaining entries down
	l := copy(p.wsbuf, p.wsbuf[n:])
	p.wsbuf = p.wsbuf[:l]
}

// writeByte writes ch n times to p.output and updates p.pos.
// Only used to write formatting (white space) characters.
func (p *printer) writeByte(ch byte, n int) {
	// TODO: finish me
	//if p.endAlignment {
	//	// Ignore any alignment control character;
	//	// and at the end of the line, break with
	//	// a formfeed to indicate termination of
	//	// existing columns.
	//	switch ch {
	//	case '\t', '\v':
	//		ch = ' '
	//	case '\n', '\f':
	//		ch = '\f'
	//		p.endAlignment = false
	//	}
	//}
	//
	//if p.out.Column == 1 {
	//	// no need to write line directives before white space
	//	p.writeIndent()
	//}
	//
	//for i := 0; i < n; i++ {
	//	p.output = append(p.output, ch)
	//}
	//
	//// update positions
	//p.pos.Offset += n
	//if ch == '\n' || ch == '\f' {
	//	p.pos.Line += n
	//	p.out.Line += n
	//	p.pos.Column = 1
	//	p.out.Column = 1
	//	return
	//}
	//p.pos.Column += n
	//p.out.Column += n
}

// writeString writes the string s to p.output and updates p.pos, p.out,
// and p.last. If isLit is set, s is escaped w/ tabwriter.Escape characters
// to protect s from being interpreted by the tabwriter.
//
// Note: writeString is only used to write Go tokens, literals, and
// comments, all of which must be written literally. Thus, it is correct
// to always set isLit = true. However, setting it explicitly only when
// needed (i.e., when we don't know that s contains no tabs or line breaks)
// avoids processing extra escape characters and reduces run time of the
// printer benchmark by up to 10%.
//
func (p *printer) writeString(pos token.Position, s string, isLit bool) {
	// TODO: finish me
}
