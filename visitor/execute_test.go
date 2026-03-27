package visitor_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/visitor"
)

// fakeRunner records every call as "method:arg" strings.
type fakeRunner struct {
	calls []string
}

func (f *fakeRunner) Install(name, version string) error {
	f.calls = append(f.calls, fmt.Sprintf("install:%s@%s", name, version))
	return nil
}
func (f *fakeRunner) SetGlobal(name, version string) error {
	f.calls = append(f.calls, fmt.Sprintf("global:%s@%s", name, version))
	return nil
}
func (f *fakeRunner) Exec(cmd string) error {
	f.calls = append(f.calls, "exec:"+cmd)
	return nil
}

func newExec(fr *fakeRunner, checkFn func(string) error) *visitor.Executor {
	e := visitor.NewExecutor(fr)
	if checkFn != nil {
		e.CheckRun = checkFn
	}
	return e
}

func checkFails(_ string) error { return errors.New("not configured") }
func checkPasses(_ string) error { return nil }

// --- VisitRuntime ---

func TestVisitRuntime_mise(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)

	rt := ast.RuntimeNode{Type: ast.RuntimeMise, Version: "3.3"}
	if err := e.VisitRuntime("ruby", rt); err != nil {
		t.Fatal(err)
	}

	want := []string{"install:ruby@3.3", "global:ruby@3.3"}
	if strings.Join(fr.calls, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, fr.calls)
	}
}

func TestVisitRuntime_mise_installBeforeGlobal(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)
	e.VisitRuntime("ruby", ast.RuntimeNode{Type: ast.RuntimeMise, Version: "3.3"})

	if len(fr.calls) < 2 || !strings.HasPrefix(fr.calls[0], "install:") {
		t.Errorf("install must come before global, got %v", fr.calls)
	}
}

func TestVisitRuntime_brew(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)

	if err := e.VisitRuntime("curl", ast.RuntimeNode{Type: ast.RuntimeBrew}); err != nil {
		t.Fatal(err)
	}
	if len(fr.calls) != 1 || fr.calls[0] != "exec:brew install curl" {
		t.Errorf("brew: want [exec:brew install curl], got %v", fr.calls)
	}
}

func TestVisitRuntime_shell_noop(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)

	e.VisitRuntime("rails", ast.RuntimeNode{Type: ast.RuntimeShell})
	if len(fr.calls) != 0 {
		t.Errorf("shell runtime should make no runner calls, got %v", fr.calls)
	}
}

// --- VisitStep ---

func TestVisitStep_run(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)

	e.VisitStep(ast.StepNode{Kind: ast.StepRun, Command: "gem install bundler"})
	if len(fr.calls) != 1 || fr.calls[0] != "exec:gem install bundler" {
		t.Errorf("got %v", fr.calls)
	}
}

func TestVisitStep_allKindsUseExec(t *testing.T) {
	for _, kind := range []ast.StepKind{ast.StepPre, ast.StepRun, ast.StepPost} {
		fr := &fakeRunner{}
		e := newExec(fr, nil)
		e.VisitStep(ast.StepNode{Kind: kind, Command: "echo hi"})
		if len(fr.calls) != 1 || fr.calls[0] != "exec:echo hi" {
			t.Errorf("kind %s: want exec call, got %v", kind, fr.calls)
		}
	}
}

// --- VisitBunch: check guard ---

func TestVisitBunch_checkPasses_skips(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, checkPasses)

	b := bunch("ruby", ast.RuntimeMise, nil, "mise current ruby | grep 3.3", run("gem install bundler"))
	b.Runtime.Version = "3.3"
	if err := e.VisitBunch(b); err != nil {
		t.Fatal(err)
	}
	if len(fr.calls) != 0 {
		t.Errorf("check passed → bunch must be skipped entirely, got calls: %v", fr.calls)
	}
}

func TestVisitBunch_checkFails_executes(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, checkFails)

	b := bunch("ruby", ast.RuntimeMise, nil, "mise current ruby | grep 3.3", run("gem install bundler"))
	b.Runtime.Version = "3.3"
	if err := e.VisitBunch(b); err != nil {
		t.Fatal(err)
	}
	if len(fr.calls) == 0 {
		t.Error("check failed → bunch must execute")
	}
}

func TestVisitBunch_noCheck_alwaysExecutes(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)

	b := bunch("ruby", ast.RuntimeShell, nil, "", run("echo hi"))
	e.VisitBunch(b)
	if len(fr.calls) == 0 {
		t.Error("no check → bunch must always execute")
	}
}

// --- VisitBunch: full sequence ---

func TestVisitBunch_order_runtimeBeforeSteps(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, checkFails)

	b := bunch("ruby", ast.RuntimeMise, nil, "check", run("gem install bundler"))
	b.Runtime.Version = "3.3"
	e.VisitBunch(b)

	if len(fr.calls) < 3 {
		t.Fatalf("expected at least 3 calls, got %v", fr.calls)
	}
	// install and global must precede the exec step
	iInstall := -1
	iExec := -1
	for i, c := range fr.calls {
		if strings.HasPrefix(c, "install:") {
			iInstall = i
		}
		if strings.HasPrefix(c, "exec:gem") {
			iExec = i
		}
	}
	if iInstall == -1 || iExec == -1 || iInstall > iExec {
		t.Errorf("install must precede step exec, got %v", fr.calls)
	}
}

func TestVisitBunch_stepsInOrder(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, checkFails)

	b := bunch("ruby", ast.RuntimeShell, nil, "check",
		pre("echo pre"),
		run("echo run1"),
		run("echo run2"),
		post("echo post"),
	)
	e.VisitBunch(b)

	want := []string{
		"exec:echo pre",
		"exec:echo run1",
		"exec:echo run2",
		"exec:echo post",
	}
	if strings.Join(fr.calls, ",") != strings.Join(want, ",") {
		t.Errorf("want %v, got %v", want, fr.calls)
	}
}

// --- Execute: dependency order ---

func TestExecute_resolvesOrder(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, checkFails)

	// declared: rails first, but rails depends on ruby
	m := manifest(
		bunch("rails", ast.RuntimeShell, []string{"ruby"}, "check", run("gem install rails")),
		bunch("ruby", ast.RuntimeShell, nil, "check", run("gem install bundler")),
	)
	if err := e.Execute(m); err != nil {
		t.Fatal(err)
	}

	// ruby's step must appear before rails' step
	iRuby := -1
	iRails := -1
	for i, c := range fr.calls {
		if c == "exec:gem install bundler" {
			iRuby = i
		}
		if c == "exec:gem install rails" {
			iRails = i
		}
	}
	if iRuby == -1 || iRails == -1 || iRuby > iRails {
		t.Errorf("ruby must execute before rails, calls: %v", fr.calls)
	}
}

func TestExecute_emptyManifest(t *testing.T) {
	fr := &fakeRunner{}
	e := newExec(fr, nil)
	if err := e.Execute(manifest()); err != nil {
		t.Fatalf("empty manifest: %v", err)
	}
	if len(fr.calls) != 0 {
		t.Errorf("no calls expected, got %v", fr.calls)
	}
}

// --- error propagation ---

func TestVisitRuntime_installError_stopsEarly(t *testing.T) {
	boom := errors.New("install failed")
	fr := &fakeRunner{}
	e := visitor.NewExecutor(&errorOnInstall{fr, boom})

	err := e.VisitRuntime("ruby", ast.RuntimeNode{Type: ast.RuntimeMise, Version: "3.3"})
	if err != boom {
		t.Errorf("expected install error to propagate, got %v", err)
	}
	// SetGlobal must NOT have been called
	for _, c := range fr.calls {
		if strings.HasPrefix(c, "global:") {
			t.Error("SetGlobal must not be called when Install fails")
		}
	}
}

func TestVisitStep_execError_propagates(t *testing.T) {
	boom := errors.New("exec failed")
	e := visitor.NewExecutor(&alwaysErrorRunner{boom})
	err := e.VisitStep(ast.StepNode{Kind: ast.StepRun, Command: "echo hi"})
	if err != boom {
		t.Errorf("expected exec error, got %v", err)
	}
}

// helpers for error injection

type errorOnInstall struct {
	fr  *fakeRunner
	err error
}

func (r *errorOnInstall) Install(_, _ string) error { return r.err }
func (r *errorOnInstall) SetGlobal(name, version string) error {
	return r.fr.SetGlobal(name, version)
}
func (r *errorOnInstall) Exec(cmd string) error { return r.fr.Exec(cmd) }

type alwaysErrorRunner struct{ err error }

func (r *alwaysErrorRunner) Install(_, _ string) error  { return r.err }
func (r *alwaysErrorRunner) SetGlobal(_, _ string) error { return r.err }
func (r *alwaysErrorRunner) Exec(_ string) error         { return r.err }
