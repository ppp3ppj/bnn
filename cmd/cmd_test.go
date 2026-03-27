package cmd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/cmd"
	"github.com/spf13/cobra"
)

// ---- helpers ----

const simpleConf = `
bunch(ruby,
    runtime(shell),
    check("true"),
    steps([run("gem install bundler")])
).

bunch(node,
    runtime(shell),
    check("false"),
    steps([run("npm install -g pnpm")])
).

bunch(rails,
    runtime(shell),
    depends([ruby, node]),
    steps([run("gem install rails")])
).
`

const miseConf = `
bunch(ruby,
    runtime(mise, "3.3"),
    check("true"),
    steps([run("gem install bundler")])
).
`

func writeConf(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bnn.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newRoot(conf string) *cobra.Command {
	// use fake lookPath that always reports mise as found at /usr/bin/mise
	fakeLookPath := func(string) (string, error) { return "/usr/bin/mise", nil }
	return cmd.NewRootCmd(conf, fakeLookPath)
}

func runCmd(t *testing.T, root *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// ---- apply --dry ----

func TestApply_dry_allBunches(t *testing.T) {
	conf := writeConf(t, simpleConf)
	out, err := runCmd(t, newRoot(conf), "apply", "--dry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "--- bunch: ruby ---") {
		t.Errorf("missing ruby header in:\n%s", out)
	}
	if !strings.Contains(out, "--- bunch: node ---") {
		t.Errorf("missing node header in:\n%s", out)
	}
	if !strings.Contains(out, "--- bunch: rails ---") {
		t.Errorf("missing rails header in:\n%s", out)
	}
	if !strings.Contains(out, "[dry] run:") {
		t.Errorf("expected [dry] run: lines in:\n%s", out)
	}
}

func TestApply_dry_singleBunch(t *testing.T) {
	conf := writeConf(t, simpleConf)
	out, err := runCmd(t, newRoot(conf), "apply", "ruby", "--dry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "--- bunch: ruby ---") {
		t.Errorf("missing ruby header in:\n%s", out)
	}
	if strings.Contains(out, "--- bunch: node ---") {
		t.Errorf("node should not appear in single-bunch dry run:\n%s", out)
	}
	if strings.Contains(out, "--- bunch: rails ---") {
		t.Errorf("rails should not appear in single-bunch dry run:\n%s", out)
	}
}

func TestApply_dry_mise_showsInstallLines(t *testing.T) {
	conf := writeConf(t, miseConf)
	out, err := runCmd(t, newRoot(conf), "apply", "--dry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[dry] mise install ruby@3.3") {
		t.Errorf("expected mise install line in:\n%s", out)
	}
	if !strings.Contains(out, "[dry] mise global  ruby@3.3") {
		t.Errorf("expected mise global line in:\n%s", out)
	}
}

func TestApply_dry_check_shown(t *testing.T) {
	conf := writeConf(t, simpleConf)
	out, err := runCmd(t, newRoot(conf), "apply", "--dry")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[dry] check:") {
		t.Errorf("expected check line in dry run:\n%s", out)
	}
}

func TestApply_unknownBunch_error(t *testing.T) {
	conf := writeConf(t, simpleConf)
	_, err := runCmd(t, newRoot(conf), "apply", "python", "--dry")
	if err == nil {
		t.Error("expected error for unknown bunch")
	}
	if !strings.Contains(err.Error(), "'python'") {
		t.Errorf("error should mention bunch name, got: %v", err)
	}
}

func TestApply_missingConf_error(t *testing.T) {
	_, err := runCmd(t, newRoot("/no/such/bnn.conf"), "apply", "--dry")
	if err == nil {
		t.Error("expected error when bnn.conf missing")
	}
}

func TestApply_parseError(t *testing.T) {
	conf := writeConf(t, `this is not valid dsl`)
	_, err := runCmd(t, newRoot(conf), "apply", "--dry")
	if err == nil {
		t.Error("expected parse error")
	}
}

func TestApply_validationError(t *testing.T) {
	// rails depends on unknown bunch "missing"
	bad := `bunch(rails, runtime(shell), depends([missing]), steps([run("x")])).`
	conf := writeConf(t, bad)
	_, err := runCmd(t, newRoot(conf), "apply", "--dry")
	if err == nil {
		t.Error("expected validation error")
	}
}

// ---- status ----

func TestStatus_checkPass_showsTick(t *testing.T) {
	// "true" always exits 0
	conf := writeConf(t, `bunch(ruby, runtime(shell), check("true"), steps([run("x")])).`)
	out, err := runCmd(t, newRoot(conf), "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected ✓ for passing check in:\n%s", out)
	}
}

func TestStatus_checkFail_showsCross(t *testing.T) {
	// "false" always exits 1
	conf := writeConf(t, `bunch(ruby, runtime(shell), check("false"), steps([run("x")])).`)
	out, err := runCmd(t, newRoot(conf), "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected ✗ for failing check in:\n%s", out)
	}
}

func TestStatus_noCheck_showsQuestion(t *testing.T) {
	conf := writeConf(t, `bunch(ruby, runtime(shell), steps([run("x")])).`)
	out, err := runCmd(t, newRoot(conf), "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "?") {
		t.Errorf("expected ? for missing check in:\n%s", out)
	}
}

func TestStatus_showsBunchName(t *testing.T) {
	conf := writeConf(t, simpleConf)
	out, err := runCmd(t, newRoot(conf), "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ruby", "node", "rails"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected bunch name %q in status output:\n%s", name, out)
		}
	}
}

func TestStatus_mixedChecks(t *testing.T) {
	conf := writeConf(t, simpleConf) // ruby=true(✓), node=false(✗), rails=no check(?)
	out, err := runCmd(t, newRoot(conf), "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected at least one ✓:\n%s", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected at least one ✗:\n%s", out)
	}
	if !strings.Contains(out, "?") {
		t.Errorf("expected at least one ?:\n%s", out)
	}
}

// ---- doctor ----

func TestDoctor_miseFound_confFound(t *testing.T) {
	conf := writeConf(t, simpleConf)
	fakeLookPath := func(string) (string, error) { return "/usr/local/bin/mise", nil }
	root := cmd.NewRootCmd(conf, fakeLookPath)

	out, err := runCmd(t, root, "doctor")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected ✓ lines in:\n%s", out)
	}
	if !strings.Contains(out, "/usr/local/bin/mise") {
		t.Errorf("expected mise path in:\n%s", out)
	}
}

func TestDoctor_miseNotFound(t *testing.T) {
	conf := writeConf(t, simpleConf)
	fakeLookPath := func(string) (string, error) { return "", errors.New("not found") }
	root := cmd.NewRootCmd(conf, fakeLookPath)

	out, err := runCmd(t, root, "doctor")
	if err == nil {
		t.Error("expected error when mise not found")
	}
	if !strings.Contains(out, "mise not found") {
		t.Errorf("expected 'mise not found' in:\n%s", out)
	}
}

func TestDoctor_confMissing(t *testing.T) {
	fakeLookPath := func(string) (string, error) { return "/usr/bin/mise", nil }
	root := cmd.NewRootCmd("/no/such/bnn.conf", fakeLookPath)

	out, err := runCmd(t, root, "doctor")
	if err == nil {
		t.Error("expected error when bnn.conf missing")
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected ✗ for missing conf in:\n%s", out)
	}
}

func TestDoctor_bothFail_returnsError(t *testing.T) {
	fakeLookPath := func(string) (string, error) { return "", errors.New("not found") }
	root := cmd.NewRootCmd("/no/such/bnn.conf", fakeLookPath)

	_, err := runCmd(t, root, "doctor")
	if err == nil {
		t.Error("expected error when both checks fail")
	}
}
