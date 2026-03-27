package dsl

import (
	"fmt"
	"strings"

	"github.com/ppp3ppj/bnn/ast"
	bnnlog "github.com/ppp3ppj/bnn/internal/log"
)

// Parser turns a token stream into a ManifestNode.
//
// Grammar:
//
//	manifest      = (var_binding | bunch_term)* EOF
//	var_binding   = VAR "=" concat_expr "."    e.g.  Label = "node-" ++ NodeVersion.
//	concat_expr   = string_or_var ("++" string_or_var)*
//	string_or_var = string | VAR
//	bunch_term    = "bunch" "(" atom "," bunch_args ")" "."
//	bunch_args    = bunch_arg ("," bunch_arg)*
//	bunch_arg     = local_var | runtime_arg | depends_arg | check_arg | steps_arg
//	local_var     = VAR "=" concat_expr        (no period — comma-separated arg)
//	runtime_arg   = "runtime" "(" atom ("," string_or_var)? ")"
//	depends_arg   = "depends" "(" "[" (atom ("," atom)*)? "]" ")"
//	check_arg     = "check"   "(" string_or_var ")"
//	steps_arg     = "steps"   "(" "[" (step ("," step)*)? "]" ")"
//	step          = ("pre"|"run"|"post") "(" string_or_var ")"
//
// ++ is only allowed in var bindings (Rule 5). Use ~Var~ interpolation in steps/check/runtime.
type Parser struct {
	tokens   []Token
	pos      int
	vars     map[string]string // bound variables: Name → value
	warnings []string          // Rule 6: non-fatal issues collected during parsing
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
			bnnlog.Debug("parser: var %s = %q", name, value)
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
	m.Warnings = p.warnings
	return m, nil
}

// parseVarBinding = VAR "=" concat_expr "."
func (p *Parser) parseVarBinding() (name, value string, err error) {
	nameTok := p.advance() // consume VAR token
	if _, err = p.expect(TOKEN_EQUALS, "="); err != nil {
		return "", "", err
	}
	value, err = p.resolveConcatExpr("variable value")
	if err != nil {
		return "", "", err
	}
	if _, err = p.expect(TOKEN_PERIOD, "."); err != nil {
		return "", "", err
	}
	return nameTok.Literal, value, nil
}

// resolveConcatOperand resolves one operand of ++.
// Gives Rule 1 error if an atom (bare identifier) is used — atoms are not strings.
func (p *Parser) resolveConcatOperand(side string) (string, error) {
	t := p.peek()
	if t.Type == TOKEN_ATOM {
		p.advance()
		return "", fmt.Errorf("[bnn] line %d:%d — '++' requires string on %s, got atom %q — use a quoted string or variable",
			t.Line, t.Col, side, t.Literal)
	}
	return p.resolveStringOrVar(side)
}

// resolveConcatExpr = string_or_var ("++" string_or_var)*
// Rule 1: both sides must be string type (not atom).
// Rule 3: result is always string type.
// Rule 6: warns if either side is an empty string (valid, but no effect).
func (p *Parser) resolveConcatExpr(context string) (string, error) {
	val, err := p.resolveConcatOperand(context)
	if err != nil {
		return "", err
	}
	for p.peek().Type == TOKEN_CONCAT {
		concatTok := p.advance() // consume ++
		right, err := p.resolveConcatOperand("right side of ++")
		if err != nil {
			return "", err
		}
		// Rule 6 — warn, not error
		if val == "" || right == "" {
			which := "left"
			if right == "" {
				which = "right"
			}
			p.warnings = append(p.warnings,
				fmt.Sprintf("[bnn] line %d:%d — concat with empty string has no effect (%s side is empty)",
					concatTok.Line, concatTok.Col, which))
		}
		before := val
		val = val + right
		bnnlog.Debug("parser: concat %q ++ %q → %q", before, right, val)
	}
	return val, nil
}

// forbidConcat enforces Rule 5: ++ is only allowed in variable assignments.
// Call this after resolving a string value in check/runtime/steps.
func (p *Parser) forbidConcat(context string) error {
	if p.peek().Type == TOKEN_CONCAT {
		t := p.peek()
		return fmt.Errorf("[bnn] line %d:%d — '++' is not allowed in %s — build the string in a variable first:\n  Label = A ++ B.\n  %s(Label)",
			t.Line, t.Col, context, context)
	}
	return nil
}

// interpolate scans s for ~VarName~ patterns and substitutes their values.
// ~~ is an escape sequence that produces a literal ~.
// Variable names inside ~...~ must start with uppercase or _.
func (p *Parser) interpolate(s string, line, col int) (string, error) {
	if !strings.ContainsRune(s, '~') {
		return s, nil // fast path: nothing to expand
	}
	var sb strings.Builder
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		if runes[i] != '~' {
			sb.WriteRune(runes[i])
			i++
			continue
		}
		// find closing ~
		j := i + 1
		for j < len(runes) && runes[j] != '~' {
			j++
		}
		if j >= len(runes) {
			return "", fmt.Errorf("[bnn] line %d:%d — unterminated interpolation: missing closing ~ in %q", line, col, s)
		}
		name := string(runes[i+1 : j])
		if name == "" {
			// ~~ → literal ~
			sb.WriteRune('~')
			i = j + 1
			continue
		}
		first := rune(name[0])
		if !isUpper(first) && first != '_' {
			return "", fmt.Errorf("[bnn] line %d:%d — ~%s~ is not a valid interpolation: variable names must start with an uppercase letter or _ (e.g. ~%s%s~)",
				line, col, name, string(first-32), name[1:])
		}
		val, ok := p.vars[name]
		if !ok {
			return "", fmt.Errorf("[bnn] line %d:%d — ~%s~ is not defined — declare it above with %s = \"value\".", line, col, name, name)
		}
		bnnlog.Debug("parser: interpolate ~%s~ → %q", name, val)
		sb.WriteString(val)
		i = j + 1
	}
	return sb.String(), nil
}

// resolveStringOrVar reads either a quoted string or a variable reference.
// It returns the resolved string value.
func (p *Parser) resolveStringOrVar(context string) (string, error) {
	t := p.peek()
	switch t.Type {
	case TOKEN_STRING:
		p.advance()
		return p.interpolate(t.Literal, t.Line, t.Col)
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

	// Save manifest-level vars and enter a local scope for this bunch.
	// Any VAR = "value" inside the bunch is visible only until the closing ")".
	// Shadowing a manifest-level name is a parse-time error (Erlang single-assignment).
	outerVars := make(map[string]string, len(p.vars))
	for k, v := range p.vars {
		outerVars[k] = v
	}
	localBound := make(map[string]bool) // tracks names bound in this bunch
	defer func() { p.vars = outerVars }()

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
		if err := p.parseBunchArg(&b, localBound, outerVars, nameTok.Literal); err != nil {
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

func (p *Parser) parseBunchArg(b *ast.BunchNode, localBound map[string]bool, outerVars map[string]string, bunchName string) error {
	t := p.peek()

	// local variable binding: Version = "22"  (no period — comma-separated arg)
	if t.Type == TOKEN_VAR {
		nameTok := p.advance()
		if _, err := p.expect(TOKEN_EQUALS, "="); err != nil {
			return err
		}
		value, err := p.resolveConcatExpr("variable value")
		if err != nil {
			return err
		}
		if _, inOuter := outerVars[nameTok.Literal]; inOuter {
			return fmt.Errorf("[bnn] line %d:%d — %s is already declared at manifest level — local variables cannot shadow global ones",
				nameTok.Line, nameTok.Col, nameTok.Literal)
		}
		if localBound[nameTok.Literal] {
			return fmt.Errorf("[bnn] line %d:%d — variable %s is already bound in this bunch (single-assignment only)",
				nameTok.Line, nameTok.Col, nameTok.Literal)
		}
		localBound[nameTok.Literal] = true
		p.vars[nameTok.Literal] = value
		bnnlog.Debug("parser: local var %s = %q (in bunch %s)", nameTok.Literal, value, bunchName)
		return nil
	}

	if t.Type != TOKEN_KEYWORD {
		return fmt.Errorf("[bnn] line %d:%d — expected a bunch argument (runtime, depends, check, steps, or a local variable), found %q", t.Line, t.Col, t.Literal)
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
		if err := p.forbidConcat("runtime"); err != nil {
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
	if err := p.forbidConcat("check"); err != nil {
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
	if err := p.forbidConcat("run/pre/post"); err != nil {
		return ast.StepNode{}, err
	}
	if _, err := p.expect(TOKEN_RPAREN, ")"); err != nil {
		return ast.StepNode{}, err
	}
	return ast.StepNode{Kind: kind, Command: cmd}, nil
}
