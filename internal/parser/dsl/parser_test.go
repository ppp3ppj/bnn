package dsl_test

import (
	"testing"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/internal/parser/dsl"
)

const sampleConf = `
% bnn.conf

bunch(ruby,
    runtime(mise, "3.3"),
    depends([]),
    check("mise current ruby | grep 3.3"),
    steps([
        pre("echo preparing ruby"),
        run("gem install bundler"),
        run("gem install rubocop"),
        post("echo ruby ready")
    ])
).

bunch(node,
    runtime(mise, "22"),
    depends([]),
    check("mise current node | grep 22"),
    steps([
        run("npm install -g pnpm"),
        run("npm install -g typescript")
    ])
).

bunch(rails,
    runtime(shell),
    depends([ruby, node]),
    steps([
        run("gem install rails")
    ])
).
`

func TestParse_sampleConf(t *testing.T) {
	m, err := dsl.Parse(sampleConf)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(m.Bunches) != 3 {
		t.Fatalf("expected 3 bunches, got %d", len(m.Bunches))
	}
}

func TestParse_ruby(t *testing.T) {
	m, err := dsl.Parse(sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	ruby := m.Bunches[0]

	if ruby.Name != "ruby" {
		t.Errorf("name: want ruby, got %s", ruby.Name)
	}
	if ruby.Runtime.Type != ast.RuntimeMise {
		t.Errorf("runtime type: want mise, got %s", ruby.Runtime.Type)
	}
	if ruby.Runtime.Version != "3.3" {
		t.Errorf("runtime version: want 3.3, got %s", ruby.Runtime.Version)
	}
	if ruby.Check != "mise current ruby | grep 3.3" {
		t.Errorf("check: unexpected value %q", ruby.Check)
	}
	if len(ruby.Steps) != 4 {
		t.Fatalf("steps: want 4, got %d", len(ruby.Steps))
	}
	if ruby.Steps[0].Kind != ast.StepPre || ruby.Steps[0].Command != "echo preparing ruby" {
		t.Errorf("step[0]: got %+v", ruby.Steps[0])
	}
	if ruby.Steps[1].Kind != ast.StepRun || ruby.Steps[1].Command != "gem install bundler" {
		t.Errorf("step[1]: got %+v", ruby.Steps[1])
	}
	if ruby.Steps[2].Kind != ast.StepRun || ruby.Steps[2].Command != "gem install rubocop" {
		t.Errorf("step[2]: got %+v", ruby.Steps[2])
	}
	if ruby.Steps[3].Kind != ast.StepPost || ruby.Steps[3].Command != "echo ruby ready" {
		t.Errorf("step[3]: got %+v", ruby.Steps[3])
	}
}

func TestParse_rails_depends(t *testing.T) {
	m, err := dsl.Parse(sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	rails := m.Bunches[2]

	if rails.Runtime.Type != ast.RuntimeShell {
		t.Errorf("runtime: want shell, got %s", rails.Runtime.Type)
	}
	if len(rails.Depends) != 2 || rails.Depends[0] != "ruby" || rails.Depends[1] != "node" {
		t.Errorf("depends: got %v", rails.Depends)
	}
	if rails.Check != "" {
		t.Errorf("check should be empty, got %q", rails.Check)
	}
}

func TestParse_runtimeNoVersion(t *testing.T) {
	src := `bunch(foo, runtime(shell), steps([run("echo hi")])).`
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if m.Bunches[0].Runtime.Version != "" {
		t.Errorf("version should be empty")
	}
}

func TestParse_emptyDepends(t *testing.T) {
	src := `bunch(foo, runtime(shell), depends([]), steps([run("x")])).`
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Bunches[0].Depends) != 0 {
		t.Errorf("expected empty depends, got %v", m.Bunches[0].Depends)
	}
}

func TestParse_comment(t *testing.T) {
	src := `
% this is a comment
bunch(foo,
    runtime(shell), % inline comment
    steps([run("echo hi")])
).
`
	_, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("comments should be ignored: %v", err)
	}
}

func TestParse_error_unterminatedString(t *testing.T) {
	src := `bunch(foo, runtime(shell), steps([run("oops)])).`
	_, err := dsl.Parse(src)
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

func TestParse_error_unexpectedToken(t *testing.T) {
	src := `notbunch(foo).`
	_, err := dsl.Parse(src)
	if err == nil {
		t.Error("expected error for unknown top-level keyword")
	}
}

func TestParse_node_check(t *testing.T) {
	m, err := dsl.Parse(sampleConf)
	if err != nil {
		t.Fatal(err)
	}
	node := m.Bunches[1]
	if node.Name != "node" {
		t.Errorf("name: want node, got %s", node.Name)
	}
	if node.Runtime.Type != ast.RuntimeMise {
		t.Errorf("runtime type: want mise, got %s", node.Runtime.Type)
	}
	if node.Runtime.Version != "22" {
		t.Errorf("runtime version: want 22, got %s", node.Runtime.Version)
	}
	if node.Check != "mise current node | grep 22" {
		t.Errorf("check: got %q", node.Check)
	}
	if len(node.Steps) != 2 {
		t.Fatalf("steps: want 2, got %d", len(node.Steps))
	}
}

func TestParse_runtimeBrew(t *testing.T) {
	src := `bunch(foo, runtime(brew), steps([run("brew install foo")])).`
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if m.Bunches[0].Runtime.Type != ast.RuntimeBrew {
		t.Errorf("runtime: want brew, got %s", m.Bunches[0].Runtime.Type)
	}
	if m.Bunches[0].Runtime.Version != "" {
		t.Errorf("version should be empty for brew")
	}
}

func TestParse_emptyManifest(t *testing.T) {
	m, err := dsl.Parse(`% just a comment`)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Bunches) != 0 {
		t.Errorf("expected 0 bunches, got %d", len(m.Bunches))
	}
}

func TestParse_stringEscape(t *testing.T) {
	src := `bunch(foo, runtime(shell), steps([run("echo \"hello\"")])).`
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if m.Bunches[0].Steps[0].Command != `echo "hello"` {
		t.Errorf("escape: got %q", m.Bunches[0].Steps[0].Command)
	}
}

func TestParse_error_missingPeriod(t *testing.T) {
	src := `bunch(foo, runtime(shell), steps([run("x")]))`
	_, err := dsl.Parse(src)
	if err == nil {
		t.Error("expected error for missing period")
	}
}

func TestParse_error_dependsKeywordInList(t *testing.T) {
	// "run" is a keyword, not an atom — should be rejected in depends list
	src := `bunch(foo, runtime(shell), depends([run]), steps([run("x")])).`
	_, err := dsl.Parse(src)
	if err == nil {
		t.Error("expected error: keyword used as depends atom")
	}
}

func TestLexer_tokens(t *testing.T) {
	tokens, err := dsl.Tokenize(`bunch(ruby, "3.3", [node]).`)
	if err != nil {
		t.Fatal(err)
	}
	// expect: KEYWORD LPAREN ATOM COMMA STRING COMMA LBRACKET ATOM RBRACKET RPAREN PERIOD EOF
	want := []dsl.TokenType{
		dsl.TOKEN_KEYWORD,
		dsl.TOKEN_LPAREN,
		dsl.TOKEN_ATOM,
		dsl.TOKEN_COMMA,
		dsl.TOKEN_STRING,
		dsl.TOKEN_COMMA,
		dsl.TOKEN_LBRACKET,
		dsl.TOKEN_ATOM,
		dsl.TOKEN_RBRACKET,
		dsl.TOKEN_RPAREN,
		dsl.TOKEN_PERIOD,
		dsl.TOKEN_EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("token count: want %d, got %d — %v", len(want), len(tokens), tokens)
	}
	for i, tt := range want {
		if tokens[i].Type != tt {
			t.Errorf("token[%d]: want type %d, got %d (%q)", i, tt, tokens[i].Type, tokens[i].Literal)
		}
	}
}
