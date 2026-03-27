package dsl

import (
	"fmt"

	"github.com/ppp3ppj/bnn/ast"
)

// Parser turns a token stream into a ManifestNode.
//
// Grammar:
//
//	manifest    = (var_binding | bunch_term)* EOF
//	var_binding = VAR "=" string "."      e.g.  NodeVersion = "22".
//	bunch_term  = "bunch" "(" atom "," bunch_args ")" "."
//	bunch_args  = bunch_arg ("," bunch_arg)*
//	bunch_arg   = runtime_arg | depends_arg | check_arg | steps_arg
//	runtime_arg = "runtime" "(" atom ("," string_or_var)? ")"
//	depends_arg = "depends" "(" "[" (atom ("," atom)*)? "]" ")"
//	check_arg   = "check"   "(" string_or_var ")"
//	steps_arg   = "steps"   "(" "[" (step ("," step)*)? "]" ")"
//	step        = ("pre"|"run"|"post") "(" string_or_var ")"
//	string_or_var = string | VAR
type Parser struct {
	tokens []Token
	pos    int
	vars   map[string]string // bound variables: Name → value
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, vars: make(map[string]string)}
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
			return t, fmt.Errorf("[bnn] line %d:%d — expected %q but found %q", t.Line, t.Col, literal, t.Literal)
		}
		return t, fmt.Errorf("[bnn] line %d:%d — unexpected token %q", t.Line, t.Col, t.Literal)
	}
	if literal != "" && t.Literal != literal {
		return t, fmt.Errorf("[bnn] line %d:%d — expected %q but found %q", t.Line, t.Col, literal, t.Literal)
	}
	return t, nil
}

func (p *Parser) expectKeyword(kw string) error {
	_, err := p.expect(TOKEN_KEYWORD, kw)
	return err
}

// parseManifest = (var_binding | bunch_term)* EOF
func (p *Parser) parseManifest() (*ast.ManifestNode, error) {
	m := &ast.ManifestNode{Vars: make(map[string]string)}
	for {
		t := p.peek()
		if t.Type == TOKEN_EOF {
			break
		}
		// VarName = "value".
		if t.Type == TOKEN_VAR {
			name, value, err := p.parseVarBinding()
			if err != nil {
				return nil, err
			}
			if _, already := p.vars[name]; already {
				return nil, fmt.Errorf("[bnn] line %d:%d — variable %s is already bound (single-assignment only)", t.Line, t.Col, name)
			}
			p.vars[name] = value
			m.Vars[name] = value
			continue
		}
		if t.Type != TOKEN_KEYWORD || t.Literal != "bunch" {
			if t.Type == TOKEN_ATOM {
				return nil, fmt.Errorf("[bnn] line %d:%d — %q looks like a variable but variables must start with an uppercase letter or _ (e.g. %s%s)",
					t.Line, t.Col, t.Literal,
					string(t.Literal[0]-32), t.Literal[1:])
			}
			return nil, fmt.Errorf("[bnn] line %d:%d — expected a variable binding (e.g. NodeVersion = \"22\".) or bunch declaration, found %q", t.Line, t.Col, t.Literal)
		}
		b, err := p.parseBunch()
		if err != nil {
			return nil, err
		}
		m.Bunches = append(m.Bunches, b)
	}
	return m, nil
}

// parseVarBinding = VAR "=" string "."
func (p *Parser) parseVarBinding() (name, value string, err error) {
	nameTok := p.advance() // consume VAR token
	if _, err = p.expect(TOKEN_EQUALS, "="); err != nil {
		return "", "", err
	}
	valTok, err := p.expect(TOKEN_STRING, "")
	if err != nil {
		return "", "", fmt.Errorf("[bnn] line %d:%d — variable value must be a quoted string, found %q",
			p.tokens[p.pos-1].Line, p.tokens[p.pos-1].Col, p.tokens[p.pos-1].Literal)
	}
	if _, err = p.expect(TOKEN_PERIOD, "."); err != nil {
		return "", "", err
	}
	return nameTok.Literal, valTok.Literal, nil
}

// resolveStringOrVar reads either a quoted string or a variable reference.
// It returns the resolved string value.
func (p *Parser) resolveStringOrVar(context string) (string, error) {
	t := p.peek()
	switch t.Type {
	case TOKEN_STRING:
		p.advance()
		return t.Literal, nil
	case TOKEN_VAR:
		p.advance()
		val, ok := p.vars[t.Literal]
		if !ok {
			return "", fmt.Errorf("[bnn] line %d:%d — variable %s is not defined", t.Line, t.Col, t.Literal)
		}
		return val, nil
	case TOKEN_ATOM:
		p.advance()
		return "", fmt.Errorf("[bnn] line %d:%d — %q looks like a variable but variables must start with an uppercase letter or _ (e.g. %s%s)",
			t.Line, t.Col, t.Literal,
			string(t.Literal[0]-32), t.Literal[1:])
	default:
		p.advance()
		return "", fmt.Errorf("[bnn] line %d:%d — %s must be a quoted string or variable (e.g. \"value\" or NodeVersion), found %q", t.Line, t.Col, context, t.Literal)
	}
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
		return ast.BunchNode{}, fmt.Errorf("[bnn] line %d:%d — bunch name must be an identifier (no quotes), found %q",
			p.tokens[p.pos-1].Line, p.tokens[p.pos-1].Col, p.tokens[p.pos-1].Literal)
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
		return fmt.Errorf("[bnn] line %d:%d — expected a bunch argument (runtime, depends, check, or steps), found %q", t.Line, t.Col, t.Literal)
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
		return fmt.Errorf("[bnn] line %d:%d — unknown argument %q (valid: runtime, depends, check, steps)", t.Line, t.Col, t.Literal)
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
		return ast.RuntimeNode{}, fmt.Errorf("[bnn] line %d:%d — runtime type must be an identifier (mise, brew, or shell), found %q",
			p.tokens[p.pos-1].Line, p.tokens[p.pos-1].Col, p.tokens[p.pos-1].Literal)
	}
	rt := ast.RuntimeNode{Type: ast.RuntimeKind(typeTok.Literal)}

	// optional version string or variable
	if p.peek().Type == TOKEN_COMMA {
		p.advance() // consume comma
		ver, err := p.resolveStringOrVar("runtime version")
		if err != nil {
			return ast.RuntimeNode{}, err
		}
		rt.Version = ver
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
			return nil, fmt.Errorf("[bnn] line %d:%d — depends list must contain bunch names (no quotes), found %q",
				p.tokens[p.pos-1].Line, p.tokens[p.pos-1].Col, p.tokens[p.pos-1].Literal)
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

// check_arg = "check" "(" string_or_var ")"
func (p *Parser) parseCheck() (string, error) {
	if err := p.expectKeyword("check"); err != nil {
		return "", err
	}
	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return "", err
	}
	cmd, err := p.resolveStringOrVar("check command")
	if err != nil {
		return "", err
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return "", err
	}
	return cmd, nil
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
		return ast.StepNode{}, fmt.Errorf("[bnn] line %d:%d — expected a step command (pre, run, or post), found %q", t.Line, t.Col, t.Literal)
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
		return ast.StepNode{}, fmt.Errorf("[bnn] line %d:%d — %q is not a valid step keyword, use pre, run, or post", t.Line, t.Col, t.Literal)
	}

	if _, err := p.expect(TOKEN_LPAREN, "("); err != nil {
		return ast.StepNode{}, err
	}
	cmd, err := p.resolveStringOrVar("step command")
	if err != nil {
		return ast.StepNode{}, err
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return ast.StepNode{}, err
	}
	return ast.StepNode{Kind: kind, Command: cmd}, nil
}
