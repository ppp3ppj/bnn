package dsl_test

import (
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/internal/parser/dsl"
)

// ---- test helpers ----

func mustParse(t *testing.T, src string) *ast.ManifestNode {
	t.Helper()
	m, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	return m
}

func mustFail(t *testing.T, src, wantSubstr string) {
	t.Helper()
	_, err := dsl.Parse(src)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("expected error containing %q, got:\n%s", wantSubstr, err.Error())
	}
}

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

// ---- lexer: new tokens ----

func TestLexer_varToken(t *testing.T) {
	tokens, err := dsl.Tokenize(`NodeVersion = "22".`)
	if err != nil {
		t.Fatal(err)
	}
	// VAR EQUALS STRING PERIOD EOF
	if tokens[0].Type != dsl.TOKEN_VAR || tokens[0].Literal != "NodeVersion" {
		t.Errorf("want TOKEN_VAR 'NodeVersion', got %v", tokens[0])
	}
	if tokens[1].Type != dsl.TOKEN_EQUALS {
		t.Errorf("want TOKEN_EQUALS, got %v", tokens[1])
	}
}

func TestLexer_underscoreVarToken(t *testing.T) {
	tokens, err := dsl.Tokenize(`_Height = "3.3".`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Type != dsl.TOKEN_VAR || tokens[0].Literal != "_Height" {
		t.Errorf("want TOKEN_VAR '_Height', got %v", tokens[0])
	}
}

func TestLexer_concatToken(t *testing.T) {
	tokens, err := dsl.Tokenize(`"a" ++ "b"`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[1].Type != dsl.TOKEN_CONCAT || tokens[1].Literal != "++" {
		t.Errorf("want TOKEN_CONCAT '++', got %v", tokens[1])
	}
}

func TestLexer_singlePlus_error(t *testing.T) {
	_, err := dsl.Tokenize(`"a" + "b"`)
	if err == nil {
		t.Error("expected error for single '+'")
	}
	if !strings.Contains(err.Error(), "++") {
		t.Errorf("error should suggest '++', got: %v", err)
	}
}

// ---- manifest-level variables ----

func TestParse_var_manifestLevel(t *testing.T) {
	m := mustParse(t, `
NodeVersion = "22".
bunch(node, runtime(mise, NodeVersion), steps([run("x")])).
`)
	if m.Vars["NodeVersion"] != "22" {
		t.Errorf("Vars[NodeVersion]: want '22', got %q", m.Vars["NodeVersion"])
	}
	if m.Bunches[0].Runtime.Version != "22" {
		t.Errorf("runtime version: want '22', got %q", m.Bunches[0].Runtime.Version)
	}
}

func TestParse_var_usedInCheck(t *testing.T) {
	m := mustParse(t, `
Cmd = "mise current node | grep 22".
bunch(node, runtime(shell), check(Cmd), steps([run("x")])).
`)
	if m.Bunches[0].Check != "mise current node | grep 22" {
		t.Errorf("check: got %q", m.Bunches[0].Check)
	}
}

func TestParse_var_usedInStep(t *testing.T) {
	m := mustParse(t, `
Cmd = "npm install -g pnpm".
bunch(node, runtime(shell), steps([run(Cmd)])).
`)
	if m.Bunches[0].Steps[0].Command != "npm install -g pnpm" {
		t.Errorf("step command: got %q", m.Bunches[0].Steps[0].Command)
	}
}

func TestParse_var_referenceFromVar(t *testing.T) {
	m := mustParse(t, `
NodeVersion = "22".
Alias = NodeVersion.
bunch(node, runtime(mise, Alias), steps([run("x")])).
`)
	if m.Vars["Alias"] != "22" {
		t.Errorf("Alias: want '22', got %q", m.Vars["Alias"])
	}
}

func TestParse_var_singleAssignment_error(t *testing.T) {
	mustFail(t, `
NodeVersion = "22".
NodeVersion = "23".
bunch(node, runtime(shell), steps([run("x")])).
`, "already bound")
}

func TestParse_var_useBeforeDeclare_error(t *testing.T) {
	mustFail(t, `
bunch(node, runtime(mise, NodeVersion), steps([run("x")])).
NodeVersion = "22".
`, "not defined")
}

func TestParse_var_lowercaseSuggestion_error(t *testing.T) {
	mustFail(t, `nodeVersion = "22".`, "NodeVersion")
}

// ---- bunch-local variables ----

func TestParse_localVar_basic(t *testing.T) {
	m := mustParse(t, `
bunch(node,
    V = "22",
    runtime(mise, V),
    steps([run("x")])
).
`)
	if m.Bunches[0].Runtime.Version != "22" {
		t.Errorf("local var not resolved in runtime: got %q", m.Bunches[0].Runtime.Version)
	}
}

func TestParse_localVar_noShadowing_error(t *testing.T) {
	mustFail(t, `
NodeVersion = "22".
bunch(node,
    NodeVersion = "23",
    runtime(mise, NodeVersion),
    steps([run("x")])
).
`, "already declared at manifest level")
}

func TestParse_localVar_singleAssignment_error(t *testing.T) {
	mustFail(t, `
bunch(node,
    V = "22",
    V = "23",
    runtime(mise, V),
    steps([run("x")])
).
`, "already bound in this bunch")
}

func TestParse_localVar_sameName_differentBunches(t *testing.T) {
	// same local var name in sibling bunches is fine — each is its own scope
	mustParse(t, `
bunch(node,
    V = "22",
    runtime(mise, V),
    steps([run("x")])
).
bunch(ruby,
    V = "3.3",
    runtime(mise, V),
    steps([run("x")])
).
`)
}

func TestParse_localVar_notVisibleOutsideBunch(t *testing.T) {
	// V declared in first bunch must not bleed into second
	mustFail(t, `
bunch(node,
    V = "22",
    runtime(mise, V),
    steps([run("x")])
).
bunch(ruby,
    runtime(mise, V),
    steps([run("x")])
).
`, "not defined")
}

// ---- string interpolation ----

func TestParse_interpolation_basic(t *testing.T) {
	m := mustParse(t, `
NodeVersion = "22".
bunch(node, runtime(shell), check("grep ~NodeVersion~"), steps([run("x")])).
`)
	if m.Bunches[0].Check != "grep 22" {
		t.Errorf("interpolation: got %q", m.Bunches[0].Check)
	}
}

func TestParse_interpolation_inStep(t *testing.T) {
	m := mustParse(t, `
V = "22".
bunch(node, runtime(shell), steps([run("node ~V~ installed")])).
`)
	if m.Bunches[0].Steps[0].Command != "node 22 installed" {
		t.Errorf("interpolation in step: got %q", m.Bunches[0].Steps[0].Command)
	}
}

func TestParse_interpolation_tilde_escape(t *testing.T) {
	m := mustParse(t, `bunch(node, runtime(shell), steps([run("a~~b")])).`)
	if m.Bunches[0].Steps[0].Command != "a~b" {
		t.Errorf("~~ escape: got %q", m.Bunches[0].Steps[0].Command)
	}
}

func TestParse_interpolation_inVarBinding(t *testing.T) {
	m := mustParse(t, `
V = "22".
Label = "node-~V~".
bunch(node, runtime(shell), steps([run("x")])).
`)
	if m.Vars["Label"] != "node-22" {
		t.Errorf("interpolation in var binding: got %q", m.Vars["Label"])
	}
}

func TestParse_interpolation_undefined_error(t *testing.T) {
	mustFail(t, `bunch(node, runtime(shell), check("grep ~Missing~"), steps([run("x")])).`,
		"~Missing~ is not defined")
}

func TestParse_interpolation_unterminated_error(t *testing.T) {
	mustFail(t, `bunch(node, runtime(shell), check("grep ~Missing"), steps([run("x")])).`,
		"unterminated interpolation")
}

func TestParse_interpolation_lowercase_error(t *testing.T) {
	mustFail(t, `bunch(node, runtime(shell), check("grep ~ver~"), steps([run("x")])).`,
		"not a valid interpolation")
}

// ---- string concatenation ----

func TestParse_concat_basic(t *testing.T) {
	m := mustParse(t, `
Label = "node-" ++ "22".
bunch(node, runtime(shell), steps([run("x")])).
`)
	if m.Vars["Label"] != "node-22" {
		t.Errorf("concat: got %q", m.Vars["Label"])
	}
}

func TestParse_concat_chain(t *testing.T) {
	m := mustParse(t, `
A = "x" ++ "y" ++ "z".
bunch(node, runtime(shell), steps([run("x")])).
`)
	if m.Vars["A"] != "xyz" {
		t.Errorf("concat chain: got %q", m.Vars["A"])
	}
}

func TestParse_concat_withVarRef(t *testing.T) {
	m := mustParse(t, `
NodeVersion = "22".
Label = "node-" ++ NodeVersion.
bunch(node, runtime(shell), steps([run("x")])).
`)
	if m.Vars["Label"] != "node-22" {
		t.Errorf("concat with var: got %q", m.Vars["Label"])
	}
}

func TestParse_concat_buildFromPreviousVar(t *testing.T) {
	m := mustParse(t, `
Base  = "node-" ++ "22".
Full  = Base ++ "-lts".
bunch(node, runtime(shell), steps([run("x")])).
`)
	if m.Vars["Full"] != "node-22-lts" {
		t.Errorf("chained var concat: got %q", m.Vars["Full"])
	}
}

func TestParse_concat_localVar(t *testing.T) {
	m := mustParse(t, `
bunch(node,
    V = "22",
    Label = "node-" ++ V,
    runtime(shell),
    check("grep ~Label~"),
    steps([run("x")])
).
`)
	if m.Bunches[0].Check != "grep node-22" {
		t.Errorf("local concat: got %q", m.Bunches[0].Check)
	}
}

func TestParse_concat_emptyString_isValid_withWarning(t *testing.T) {
	m := mustParse(t, `
V = "" ++ "22".
bunch(node, runtime(shell), steps([run("x")])).
`)
	if m.Vars["V"] != "22" {
		t.Errorf("empty concat: got %q", m.Vars["V"])
	}
	if len(m.Warnings) == 0 {
		t.Error("expected warning for empty string concat")
	}
	if !strings.Contains(m.Warnings[0], "empty string") {
		t.Errorf("warning should mention empty string: %s", m.Warnings[0])
	}
}

func TestParse_concat_atomLHS_error(t *testing.T) {
	// Rule 1: atom is not a string
	mustFail(t, `
Label = node ++ "-22".
bunch(node, runtime(shell), steps([run("x")])).
`, "requires string")
}

func TestParse_concat_inStep_error(t *testing.T) {
	// Rule 5: ++ not allowed inside steps
	mustFail(t, `
V = "22".
bunch(node, runtime(shell), steps([run("node-" ++ V)])).
`, "not allowed")
}

func TestParse_concat_inCheck_error(t *testing.T) {
	// Rule 5: ++ not allowed inside check
	mustFail(t, `
V = "22".
bunch(node, runtime(shell), check("grep " ++ V), steps([run("x")])).
`, "not allowed")
}

func TestParse_concat_inRuntime_error(t *testing.T) {
	// Rule 5: ++ not allowed inside runtime
	mustFail(t, `
A = "2".
bunch(node, runtime(mise, "2" ++ A), steps([run("x")])).
`, "not allowed")
}

func TestParse_concat_undefinedVar_error(t *testing.T) {
	// Rule 7: every var in chain must be declared
	mustFail(t, `
NodeVersion = "22".
Label = "node-" ++ NodeVersion ++ Suffix.
bunch(node, runtime(shell), steps([run("x")])).
`, "not defined")
}

func TestParse_concat_useBeforeDeclare_error(t *testing.T) {
	// Rule 2: declare before use
	mustFail(t, `
Label = "node-" ++ NodeVersion.
NodeVersion = "22".
bunch(node, runtime(shell), steps([run("x")])).
`, "not defined")
}
