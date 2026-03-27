package visitor_test

import (
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/visitor"
)

// helpers

func bunch(name string, rt ast.RuntimeKind, depends []string, check string, steps ...ast.StepNode) ast.BunchNode {
	return ast.BunchNode{
		Name:    name,
		Runtime: ast.RuntimeNode{Type: rt},
		Depends: depends,
		Check:   check,
		Steps:   steps,
	}
}

func run(cmd string) ast.StepNode  { return ast.StepNode{Kind: ast.StepRun, Command: cmd} }
func pre(cmd string) ast.StepNode  { return ast.StepNode{Kind: ast.StepPre, Command: cmd} }
func post(cmd string) ast.StepNode { return ast.StepNode{Kind: ast.StepPost, Command: cmd} }

func manifest(bunches ...ast.BunchNode) *ast.ManifestNode {
	return &ast.ManifestNode{Bunches: bunches}
}

func assertValid(t *testing.T, m *ast.ManifestNode) {
	t.Helper()
	if err := visitor.Validate(m); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func assertInvalid(t *testing.T, m *ast.ManifestNode, wantSubstr string) {
	t.Helper()
	err := visitor.Validate(m)
	if err == nil {
		t.Fatalf("expected validation error containing %q, got nil", wantSubstr)
	}
	if !strings.Contains(err.Error(), wantSubstr) {
		t.Errorf("expected error containing %q, got:\n%s", wantSubstr, err.Error())
	}
}

// --- valid manifests ---

func TestValidate_valid_simple(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "mise current ruby | grep 3.3", run("gem install bundler")),
	)
	assertValid(t, m)
}

func TestValidate_valid_full(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "", run("gem install bundler")),
		bunch("node", ast.RuntimeMise, nil, "", run("npm i -g pnpm")),
		bunch("rails", ast.RuntimeShell, []string{"ruby", "node"}, "", run("gem install rails")),
	)
	assertValid(t, m)
}

func TestValidate_valid_brew(t *testing.T) {
	m := manifest(
		bunch("curl", ast.RuntimeBrew, nil, "", run("brew install curl")),
	)
	assertValid(t, m)
}

func TestValidate_valid_preAndPost(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "",
			pre("echo before"),
			run("gem install bundler"),
			post("echo after"),
		),
	)
	assertValid(t, m)
}

func TestValidate_valid_emptyManifest(t *testing.T) {
	assertValid(t, manifest())
}

// --- rule: depends target exists ---

func TestValidate_dependsUnknown(t *testing.T) {
	m := manifest(
		bunch("rails", ast.RuntimeShell, []string{"ruby"}, "", run("gem install rails")),
	)
	assertInvalid(t, m, "depends on 'ruby' which is not declared")
}

func TestValidate_dependsUnknown_multipleTargets(t *testing.T) {
	m := manifest(
		bunch("app", ast.RuntimeShell, []string{"ruby", "node"}, "", run("echo hi")),
	)
	err := visitor.Validate(m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "'ruby'") || !strings.Contains(err.Error(), "'node'") {
		t.Errorf("expected both missing deps reported, got: %v", err)
	}
}

// --- rule: runtime type is valid ---

func TestValidate_invalidRuntime(t *testing.T) {
	m := manifest(
		bunch("foo", ast.RuntimeKind("nvm"), nil, "", run("echo hi")),
	)
	assertInvalid(t, m, "unknown runtime 'nvm'")
}

func TestValidate_emptyRuntime(t *testing.T) {
	m := manifest(
		bunch("foo", ast.RuntimeKind(""), nil, "", run("echo hi")),
	)
	assertInvalid(t, m, "unknown runtime")
}

// --- rule: bunch name is unique ---

func TestValidate_duplicateName(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "", run("gem install bundler")),
		bunch("ruby", ast.RuntimeMise, nil, "", run("gem install rubocop")),
	)
	assertInvalid(t, m, "already declared above")
}

// --- rule: steps not empty (at least one run) ---

func TestValidate_noRunStep(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "", pre("echo hi")),
	)
	assertInvalid(t, m, "at least one run()")
}

func TestValidate_emptySteps(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, ""),
	)
	assertInvalid(t, m, "at least one run()")
}

func TestValidate_prePostWithoutRun(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "",
			pre("echo before"),
			post("echo after"),
		),
	)
	assertInvalid(t, m, "at least one run()")
}

// --- rule: check is valid string ---

func TestValidate_blankCheck(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "   ", run("gem install bundler")),
	)
	assertInvalid(t, m, "check command is blank")
}

func TestValidate_emptyCheckIsOk(t *testing.T) {
	// empty string means "no check declared" — valid
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "", run("gem install bundler")),
	)
	assertValid(t, m)
}

// --- rule: circular dependency ---

func TestValidate_selfLoop(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, []string{"ruby"}, "", run("echo hi")),
	)
	assertInvalid(t, m, "circular dependency")
}

func TestValidate_twoNodeCycle(t *testing.T) {
	m := manifest(
		bunch("a", ast.RuntimeShell, []string{"b"}, "", run("echo a")),
		bunch("b", ast.RuntimeShell, []string{"a"}, "", run("echo b")),
	)
	assertInvalid(t, m, "circular dependency")
}

func TestValidate_threeNodeCycle(t *testing.T) {
	// rails → ruby → gems → rails
	m := manifest(
		bunch("rails", ast.RuntimeShell, []string{"ruby"}, "", run("echo rails")),
		bunch("ruby", ast.RuntimeShell, []string{"gems"}, "", run("echo ruby")),
		bunch("gems", ast.RuntimeShell, []string{"rails"}, "", run("echo gems")),
	)
	assertInvalid(t, m, "circular dependency")
}

func TestValidate_diamondIsValid(t *testing.T) {
	//     app
	//    /   \
	//  ruby  node
	//    \   /
	//    base
	m := manifest(
		bunch("base", ast.RuntimeShell, nil, "", run("echo base")),
		bunch("ruby", ast.RuntimeShell, []string{"base"}, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, []string{"base"}, "", run("echo node")),
		bunch("app", ast.RuntimeShell, []string{"ruby", "node"}, "", run("echo app")),
	)
	assertValid(t, m)
}

// --- multiple errors reported together ---

func TestValidate_multipleErrors(t *testing.T) {
	m := manifest(
		bunch("foo", ast.RuntimeKind("bad"), []string{"missing"}, "", pre("only pre")),
	)
	err := visitor.Validate(m)
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown runtime") {
		t.Errorf("missing runtime error in: %s", msg)
	}
	if !strings.Contains(msg, "which is not declared") {
		t.Errorf("missing depends error in: %s", msg)
	}
	if !strings.Contains(msg, "at least one run()") {
		t.Errorf("missing steps error in: %s", msg)
	}
}
