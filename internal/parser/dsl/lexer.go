package dsl

import (
	"fmt"
	"strings"
)

type TokenType int

const (
	// literals
	TOKEN_ATOM    TokenType = iota // unquoted lowercase identifier
	TOKEN_STRING                   // "..."
	TOKEN_KEYWORD                  // bunch runtime depends check steps pre run post

	// punctuation
	TOKEN_LPAREN
	TOKEN_RPAREN
	TOKEN_LBRACKET
	TOKEN_RBRACKET
	TOKEN_COMMA
	TOKEN_PERIOD

	TOKEN_EOF
	TOKEN_ILLEGAL
)

var keywords = map[string]bool{
	"bunch":   true,
	"runtime": true,
	"depends": true,
	"check":   true,
	"steps":   true,
	"pre":     true,
	"run":     true,
	"post":    true,
}

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q, line=%d col=%d)", t.Type, t.Literal, t.Line, t.Col)
}

type Lexer struct {
	src  []rune
	pos  int
	line int
	col  int
}

func NewLexer(src string) *Lexer {
	return &Lexer{src: []rune(src), pos: 0, line: 1, col: 1}
}

func (l *Lexer) peek() (rune, bool) {
	if l.pos >= len(l.src) {
		return 0, false
	}
	return l.src[l.pos], true
}

func (l *Lexer) advance() rune {
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		ch, ok := l.peek()
		if !ok {
			return
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.advance()
		} else if ch == '%' {
			// comment — skip to end of line
			for {
				c, ok := l.peek()
				if !ok || c == '\n' {
					break
				}
				l.advance()
			}
		} else {
			return
		}
	}
}

func (l *Lexer) readString() (Token, error) {
	line, col := l.line, l.col
	l.advance() // consume opening "
	var sb strings.Builder
	for {
		ch, ok := l.peek()
		if !ok {
			return Token{}, fmt.Errorf("[bnn] line %d:%d — unterminated string literal", line, col)
		}
		l.advance()
		if ch == '"' {
			break
		}
		if ch == '\\' {
			// basic escape: \" \\
			next, ok := l.peek()
			if !ok {
				return Token{}, fmt.Errorf("[bnn] line %d:%d — unterminated escape sequence in string", line, col)
			}
			l.advance()
			switch next {
			case '"':
				sb.WriteRune('"')
			case '\\':
				sb.WriteRune('\\')
			default:
				sb.WriteRune('\\')
				sb.WriteRune(next)
			}
			continue
		}
		sb.WriteRune(ch)
	}
	return Token{Type: TOKEN_STRING, Literal: sb.String(), Line: line, Col: col}, nil
}

func isLower(ch rune) bool { return ch >= 'a' && ch <= 'z' }
func isAlnum(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_'
}

func (l *Lexer) readIdent() Token {
	line, col := l.line, l.col
	var sb strings.Builder
	for {
		ch, ok := l.peek()
		if !ok || !isAlnum(ch) {
			break
		}
		sb.WriteRune(l.advance())
	}
	lit := sb.String()
	tt := TOKEN_ATOM
	if keywords[lit] {
		tt = TOKEN_KEYWORD
	}
	return Token{Type: tt, Literal: lit, Line: line, Col: col}
}

func (l *Lexer) Next() (Token, error) {
	l.skipWhitespaceAndComments()

	line, col := l.line, l.col
	ch, ok := l.peek()
	if !ok {
		return Token{Type: TOKEN_EOF, Line: line, Col: col}, nil
	}

	switch {
	case ch == '"':
		return l.readString()
	case isLower(ch):
		return l.readIdent(), nil
	case ch == '(':
		l.advance()
		return Token{Type: TOKEN_LPAREN, Literal: "(", Line: line, Col: col}, nil
	case ch == ')':
		l.advance()
		return Token{Type: TOKEN_RPAREN, Literal: ")", Line: line, Col: col}, nil
	case ch == '[':
		l.advance()
		return Token{Type: TOKEN_LBRACKET, Literal: "[", Line: line, Col: col}, nil
	case ch == ']':
		l.advance()
		return Token{Type: TOKEN_RBRACKET, Literal: "]", Line: line, Col: col}, nil
	case ch == ',':
		l.advance()
		return Token{Type: TOKEN_COMMA, Literal: ",", Line: line, Col: col}, nil
	case ch == '.':
		l.advance()
		return Token{Type: TOKEN_PERIOD, Literal: ".", Line: line, Col: col}, nil
	default:
		l.advance()
		return Token{Type: TOKEN_ILLEGAL, Literal: string(ch), Line: line, Col: col},
			fmt.Errorf("[bnn] line %d:%d — unexpected character %q", line, col, ch)
	}
}

// Tokenize returns all tokens including the final EOF token, or an error.
func Tokenize(src string) ([]Token, error) {
	l := NewLexer(src)
	var tokens []Token
	for {
		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		if tok.Type == TOKEN_EOF {
			tokens = append(tokens, tok)
			return tokens, nil
		}
		tokens = append(tokens, tok)
	}
}
