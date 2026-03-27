package visitor_test

import (
	"testing"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/visitor"
)

func resolveNames(t *testing.T, m *ast.ManifestNode) []string {
	t.Helper()
	bunches, err := visitor.Resolve(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := make([]string, len(bunches))
	for i, b := range bunches {
		names[i] = b.Name
	}
	return names
}

func assertOrder(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: want %q, got %q\nfull order: %v", i, want[i], got[i], got)
			return
		}
	}
}

// before returns true if a appears before b in names.
func before(names []string, a, b string) bool {
	ai, bi := -1, -1
	for i, n := range names {
		if n == a {
			ai = i
		}
		if n == b {
			bi = i
		}
	}
	return ai != -1 && bi != -1 && ai < bi
}

func assertBefore(t *testing.T, names []string, a, b string) {
	t.Helper()
	if !before(names, a, b) {
		t.Errorf("expected %q before %q, got order: %v", a, b, names)
	}
}

// --- no dependencies ---

func TestResolve_noDeps_preservesOrder(t *testing.T) {
	m := manifest(
		bunch("rails", ast.RuntimeShell, nil, "", run("echo rails")),
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, nil, "", run("echo node")),
	)
	assertOrder(t, resolveNames(t, m), "rails", "ruby", "node")
}

func TestResolve_singleBunch(t *testing.T) {
	m := manifest(bunch("ruby", ast.RuntimeShell, nil, "", run("echo ruby")))
	assertOrder(t, resolveNames(t, m), "ruby")
}

func TestResolve_emptyManifest(t *testing.T) {
	names := resolveNames(t, manifest())
	if len(names) != 0 {
		t.Errorf("expected empty result, got %v", names)
	}
}

// --- linear chain ---

func TestResolve_linearChain(t *testing.T) {
	// declared: rails, ruby, node — but rails depends on ruby depends on node
	m := manifest(
		bunch("rails", ast.RuntimeShell, []string{"ruby"}, "", run("echo rails")),
		bunch("ruby", ast.RuntimeShell, []string{"node"}, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, nil, "", run("echo node")),
	)
	names := resolveNames(t, m)
	assertBefore(t, names, "node", "ruby")
	assertBefore(t, names, "ruby", "rails")
}

func TestResolve_readme_example(t *testing.T) {
	// The README example: declared rails, ruby, node → resolved node, ruby, rails
	m := manifest(
		bunch("rails", ast.RuntimeShell, []string{"ruby", "node"}, "", run("echo rails")),
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, nil, "", run("echo node")),
	)
	names := resolveNames(t, m)
	assertBefore(t, names, "ruby", "rails")
	assertBefore(t, names, "node", "rails")
}

// --- multiple dependencies ---

func TestResolve_twoDepsOnSameTarget(t *testing.T) {
	m := manifest(
		bunch("app", ast.RuntimeShell, []string{"ruby", "node"}, "", run("echo app")),
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, nil, "", run("echo node")),
	)
	names := resolveNames(t, m)
	assertBefore(t, names, "ruby", "app")
	assertBefore(t, names, "node", "app")
}

// --- diamond graph ---

func TestResolve_diamond(t *testing.T) {
	//     app
	//    /   \
	//  ruby  node
	//    \   /
	//    base
	m := manifest(
		bunch("app", ast.RuntimeShell, []string{"ruby", "node"}, "", run("echo app")),
		bunch("ruby", ast.RuntimeShell, []string{"base"}, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, []string{"base"}, "", run("echo node")),
		bunch("base", ast.RuntimeShell, nil, "", run("echo base")),
	)
	names := resolveNames(t, m)
	assertBefore(t, names, "base", "ruby")
	assertBefore(t, names, "base", "node")
	assertBefore(t, names, "ruby", "app")
	assertBefore(t, names, "node", "app")
}

// --- already in correct order ---

func TestResolve_alreadySorted(t *testing.T) {
	m := manifest(
		bunch("base", ast.RuntimeShell, nil, "", run("echo base")),
		bunch("ruby", ast.RuntimeShell, []string{"base"}, "", run("echo ruby")),
		bunch("rails", ast.RuntimeShell, []string{"ruby"}, "", run("echo rails")),
	)
	assertOrder(t, resolveNames(t, m), "base", "ruby", "rails")
}

// --- stable ordering at same level ---

func TestResolve_sameLevel_preservesDeclarationOrder(t *testing.T) {
	// ruby, node, python all independent — original order should be kept
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, nil, "", run("echo node")),
		bunch("python", ast.RuntimeShell, nil, "", run("echo python")),
		bunch("app", ast.RuntimeShell, []string{"ruby", "node", "python"}, "", run("echo app")),
	)
	names := resolveNames(t, m)
	// ruby, node, python appear before app and in their original relative order
	assertBefore(t, names, "ruby", "node")
	assertBefore(t, names, "node", "python")
	assertBefore(t, names, "python", "app")
}

// --- error: unknown dependency ---

func TestResolve_error_unknownDep(t *testing.T) {
	m := manifest(
		bunch("rails", ast.RuntimeShell, []string{"ruby"}, "", run("echo rails")),
	)
	_, err := visitor.Resolve(m)
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

// --- error: circular dependency ---

func TestResolve_error_cycle(t *testing.T) {
	m := manifest(
		bunch("a", ast.RuntimeShell, []string{"b"}, "", run("echo a")),
		bunch("b", ast.RuntimeShell, []string{"a"}, "", run("echo b")),
	)
	_, err := visitor.Resolve(m)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
}

func TestResolve_error_selfLoop(t *testing.T) {
	m := manifest(
		bunch("a", ast.RuntimeShell, []string{"a"}, "", run("echo a")),
	)
	_, err := visitor.Resolve(m)
	if err == nil {
		t.Fatal("expected error for self-loop")
	}
}
