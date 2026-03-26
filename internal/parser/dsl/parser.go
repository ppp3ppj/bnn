package dsl

import (
	"fmt"

	"github.com/ppp3ppj/bnn/ast"
)

// Parser turns a token stream into a ManifestNode.
//
// Grammar:
//
//	manifest    = bunch_term* EOF
//	bunch_term  = "bunch" "(" atom "," bunch_args ")" "."
//	bunch_args  = bunch_arg ("," bunch_arg)*
//	bunch_arg   = runtime_arg | depends_arg | check_arg | steps_arg
//	runtime_arg = "runtime" "(" atom ("," string)? ")"
//	depends_arg = "depends" "(" "[" (atom ("," atom)*)? "]" ")"
//	check_arg   = "check"   "(" string ")"
//	steps_arg   = "steps"   "(" "[" (step ("," step)*)? "]" ")"
//	step        = ("pre"|"run"|"post") "(" string ")"
type Parser struct {
	tokens []Token
	pos    int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse is the entry point.
func Parse(src string) (*ast.ManifestNode, error) {
	tokens, err := Tokenize(src)
	if err != nil {
		return nil, err
	}
	p := NewParser(tokens)
	return p.parseManifest()
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TOKEN_EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	t := p.peek()
	if t.Type != TOKEN_EOF {
		p.pos++
	}
	return t
}

func (p *Parser) expect(tt TokenType, literal string) (Token, error) {
	t := p.advance()
	if t.Type != tt {
		if literal != "" {
			return t, fmt.Errorf("line %d:%d: expected %q, got %q", t.Line, t.Col, literal, t.Literal)
		}
		return t, fmt.Errorf("line %d:%d: unexpected token %q (type %d)", t.Line, t.Col, t.Literal, t.Type)
	}
	if literal != "" && t.Literal != literal {
		return t, fmt.Errorf("line %d:%d: expected %q, got %q", t.Line, t.Col, literal, t.Literal)
	}
	return t, nil
}

func (p *Parser) expectKeyword(kw string) error {
	_, err := p.expect(TOKEN_KEYWORD, kw)
	return err
}

// parseManifest = bunch_term* EOF
func (p *Parser) parseManifest() (*ast.ManifestNode, error) {
	m := &ast.ManifestNode{}
	for {
		t := p.peek()
		if t.Type == TOKEN_EOF {
			break
		}
		if t.Type != TOKEN_KEYWORD || t.Literal != "bunch" {
			return nil, fmt.Errorf("line %d:%d: expected 'bunch', got %q", t.Line, t.Col, t.Literal)
		}
		b, err := p.parseBunch()
		if err != nil {
			return nil, err
		}
		m.Bunches = append(m.Bunches, b)
	}
	return m, nil
}

// bunch_term = "bunch" "(" atom "," bunch_args ")" "."
func (p *Parser) parseBunch() (ast.BunchNode, error) {
	if err := p.expectKeyword("bunch"); err != nil {
		return ast.BunchNode{}, err
	}
	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return ast.BunchNode{}, err
	}

	nameTok, err := p.expect(TOKEN_ATOM, "")
	if err != nil {
		return ast.BunchNode{}, fmt.Errorf("bunch name must be an atom: %w", err)
	}

	b := ast.BunchNode{Name: nameTok.Literal}

	// parse comma-separated bunch args
	for {
		t := p.peek()
		if t.Type == TOKEN_RPAREN {
			break
		}
		if _, err := p.expect(TOKEN_COMMA, ","); err != nil {
			return ast.BunchNode{}, err
		}
		t = p.peek()
		if t.Type == TOKEN_RPAREN {
			break
		}
		if err := p.parseBunchArg(&b); err != nil {
			return ast.BunchNode{}, err
		}
	}

	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return ast.BunchNode{}, err
	}
	if _, err := p.expect(TOKEN_PERIOD, "."); err != nil {
		return ast.BunchNode{}, err
	}
	return b, nil
}

func (p *Parser) parseBunchArg(b *ast.BunchNode) error {
	t := p.peek()
	if t.Type != TOKEN_KEYWORD {
		return fmt.Errorf("line %d:%d: expected bunch arg keyword, got %q", t.Line, t.Col, t.Literal)
	}
	switch t.Literal {
	case "runtime":
		r, err := p.parseRuntime()
		if err != nil {
			return err
		}
		b.Runtime = r
	case "depends":
		deps, err := p.parseDepends()
		if err != nil {
			return err
		}
		b.Depends = deps
	case "check":
		cmd, err := p.parseCheck()
		if err != nil {
			return err
		}
		b.Check = cmd
	case "steps":
		steps, err := p.parseSteps()
		if err != nil {
			return err
		}
		b.Steps = steps
	default:
		return fmt.Errorf("line %d:%d: unknown bunch arg %q", t.Line, t.Col, t.Literal)
	}
	return nil
}

// runtime_arg = "runtime" "(" atom ("," string)? ")"
func (p *Parser) parseRuntime() (ast.RuntimeNode, error) {
	if err := p.expectKeyword("runtime"); err != nil {
		return ast.RuntimeNode{}, err
	}
	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return ast.RuntimeNode{}, err
	}

	typeTok, err := p.expect(TOKEN_ATOM, "")
	if err != nil {
		return ast.RuntimeNode{}, fmt.Errorf("runtime type must be an atom: %w", err)
	}
	rt := ast.RuntimeNode{Type: ast.RuntimeKind(typeTok.Literal)}

	// optional version string
	if p.peek().Type == TOKEN_COMMA {
		p.advance() // consume comma
		verTok, err := p.expect(TOKEN_STRING, "")
		if err != nil {
			return ast.RuntimeNode{}, fmt.Errorf("runtime version must be a string: %w", err)
		}
		rt.Version = verTok.Literal
	}

	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return ast.RuntimeNode{}, err
	}
	return rt, nil
}

// depends_arg = "depends" "(" "[" (atom ("," atom)*)? "]" ")"
func (p *Parser) parseDepends() ([]string, error) {
	if err := p.expectKeyword("depends"); err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_LBRACKET, "["); err != nil {
		return nil, err
	}

	var deps []string
	for p.peek().Type != TOKEN_RBRACKET {
		t, err := p.expect(TOKEN_ATOM, "")
		if err != nil {
			return nil, fmt.Errorf("depends list must contain atoms: %w", err)
		}
		deps = append(deps, t.Literal)
		if p.peek().Type == TOKEN_COMMA {
			p.advance()
		} else {
			break
		}
	}

	if _, err := p.expect(TOKEN_RBRACKET, "]"); err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return nil, err
	}
	return deps, nil
}

// check_arg = "check" "(" string ")"
func (p *Parser) parseCheck() (string, error) {
	if err := p.expectKeyword("check"); err != nil {
		return "", err
	}
	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return "", err
	}
	cmd, err := p.expect(TOKEN_STRING, "")
	if err != nil {
		return "", fmt.Errorf("check argument must be a string: %w", err)
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return "", err
	}
	return cmd.Literal, nil
}

// steps_arg = "steps" "(" "[" (step ("," step)*)? "]" ")"
func (p *Parser) parseSteps() ([]ast.StepNode, error) {
	if err := p.expectKeyword("steps"); err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_LBRACKET, "["); err != nil {
		return nil, err
	}

	var steps []ast.StepNode
	for p.peek().Type != TOKEN_RBRACKET {
		s, err := p.parseStep()
		if err != nil {
			return nil, err
		}
		steps = append(steps, s)
		if p.peek().Type == TOKEN_COMMA {
			p.advance()
		} else {
			break
		}
	}

	if _, err := p.expect(TOKEN_RBRACKET, "]"); err != nil {
		return nil, err
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return nil, err
	}
	return steps, nil
}

// step = ("pre"|"run"|"post") "(" string ")"
func (p *Parser) parseStep() (ast.StepNode, error) {
	t := p.advance()
	if t.Type != TOKEN_KEYWORD {
		return ast.StepNode{}, fmt.Errorf("line %d:%d: expected step keyword (pre/run/post), got %q", t.Line, t.Col, t.Literal)
	}
	var kind ast.StepKind
	switch t.Literal {
	case "pre":
		kind = ast.StepPre
	case "run":
		kind = ast.StepRun
	case "post":
		kind = ast.StepPost
	default:
		return ast.StepNode{}, fmt.Errorf("line %d:%d: unknown step kind %q", t.Line, t.Col, t.Literal)
	}

	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return ast.StepNode{}, err
	}
	cmd, err := p.expect(TOKEN_STRING, "")
	if err != nil {
		return ast.StepNode{}, fmt.Errorf("step command must be a string: %w", err)
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return ast.StepNode{}, err
	}
	return ast.StepNode{Kind: kind, Command: cmd.Literal}, nil
}
