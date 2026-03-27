package visitor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/visitor"
)

func dryOut(m *ast.ManifestNode) string {
	var buf bytes.Buffer
	visitor.DryRun(m, &buf)
	return buf.String()
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q\ngot:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", want, got)
	}
}

// --- mise runtime ---

func TestDryRun_mise(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "",
			run("gem install bundler"),
			run("gem install rubocop"),
		),
	)
	m.Bunches[0].Runtime.Version = "3.3"
	out := dryOut(m)

	assertContains(t, out, "[dry] mise install ruby@3.3")
	assertContains(t, out, "[dry] mise global  ruby@3.3")
	assertContains(t, out, "[dry] run: gem install bundler")
	assertContains(t, out, "[dry] run: gem install rubocop")
}

func TestDryRun_mise_order(t *testing.T) {
	// mise install must come before mise global, and both before steps
	m := manifest(bunch("ruby", ast.RuntimeMise, nil, "", run("gem install bundler")))
	m.Bunches[0].Runtime.Version = "3.3"
	out := dryOut(m)

	iInstall := strings.Index(out, "mise install")
	iGlobal := strings.Index(out, "mise global")
	iStep := strings.Index(out, "[dry] run:")

	if iInstall > iGlobal {
		t.Error("mise install should appear before mise global")
	}
	if iGlobal > iStep {
		t.Error("mise global should appear before steps")
	}
}

// --- brew runtime ---

func TestDryRun_brew(t *testing.T) {
	m := manifest(
		bunch("curl", ast.RuntimeBrew, nil, "", run("echo installed")),
	)
	out := dryOut(m)

	assertContains(t, out, "[dry] brew install curl")
	assertNotContains(t, out, "mise")
}

// --- shell runtime ---

func TestDryRun_shell_noRuntimeLine(t *testing.T) {
	m := manifest(
		bunch("rails", ast.RuntimeShell, nil, "", run("gem install rails")),
	)
	out := dryOut(m)

	assertNotContains(t, out, "mise")
	assertNotContains(t, out, "brew")
	assertContains(t, out, "[dry] run: gem install rails")
}

// --- steps: pre / run / post ---

func TestDryRun_stepKinds(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "",
			pre("echo before"),
			run("gem install bundler"),
			post("echo after"),
		),
	)
	out := dryOut(m)

	assertContains(t, out, "[dry] pre: echo before")
	assertContains(t, out, "[dry] run: gem install bundler")
	assertContains(t, out, "[dry] post: echo after")
}

func TestDryRun_stepOrder(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "",
			pre("echo before"),
			run("gem install bundler"),
			post("echo after"),
		),
	)
	out := dryOut(m)

	iPre := strings.Index(out, "[dry] pre:")
	iRun := strings.Index(out, "[dry] run:")
	iPost := strings.Index(out, "[dry] post:")

	if iPre > iRun {
		t.Error("pre should appear before run")
	}
	if iRun > iPost {
		t.Error("run should appear before post")
	}
}

func TestDryRun_multipleRuns(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "",
			run("gem install bundler"),
			run("gem install rubocop"),
			run("gem install rails"),
		),
	)
	out := dryOut(m)

	assertContains(t, out, "[dry] run: gem install bundler")
	assertContains(t, out, "[dry] run: gem install rubocop")
	assertContains(t, out, "[dry] run: gem install rails")
}

// --- check guard ---

func TestDryRun_check_shown(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeMise, nil, "mise current ruby | grep 3.3", run("gem install bundler")),
	)
	m.Bunches[0].Runtime.Version = "3.3"
	out := dryOut(m)

	assertContains(t, out, "[dry] check: mise current ruby | grep 3.3")
	assertContains(t, out, "skip bunch if exits 0")
}

func TestDryRun_noCheck_notShown(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo hi")),
	)
	out := dryOut(m)

	assertNotContains(t, out, "[dry] check:")
}

// --- depends ---

func TestDryRun_depends_shown(t *testing.T) {
	m := manifest(
		bunch("base", ast.RuntimeShell, nil, "", run("echo base")),
		bunch("rails", ast.RuntimeShell, []string{"ruby", "node"}, "", run("gem install rails")),
	)
	out := dryOut(m)

	assertContains(t, out, "[dry] depends: ruby, node")
}

func TestDryRun_noDepends_notShown(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo hi")),
	)
	out := dryOut(m)

	assertNotContains(t, out, "[dry] depends:")
}

// --- bunch header ---

func TestDryRun_bunchHeader(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo hi")),
	)
	out := dryOut(m)
	assertContains(t, out, "--- bunch: ruby ---")
}

// --- multiple bunches: blank line separator ---

func TestDryRun_multipleBunches_separated(t *testing.T) {
	m := manifest(
		bunch("ruby", ast.RuntimeShell, nil, "", run("echo ruby")),
		bunch("node", ast.RuntimeShell, nil, "", run("echo node")),
	)
	out := dryOut(m)

	// both headers present
	assertContains(t, out, "--- bunch: ruby ---")
	assertContains(t, out, "--- bunch: node ---")

	// ruby section appears before node section
	iRuby := strings.Index(out, "--- bunch: ruby ---")
	iNode := strings.Index(out, "--- bunch: node ---")
	if iRuby > iNode {
		t.Error("ruby bunch should appear before node bunch")
	}
}

func TestDryRun_emptyManifest(t *testing.T) {
	out := dryOut(manifest())
	if out != "" {
		t.Errorf("empty manifest should produce no output, got %q", out)
	}
}
