package visitor

import (
	"os/exec"

	"github.com/ppp3ppj/bnn/ast"
	bnnlog "github.com/ppp3ppj/bnn/internal/log"
)

// MiseRunner is the interface Execute depends on.
// *runner.Runner satisfies it automatically via Go's structural typing.
type MiseRunner interface {
	Install(name, version string) error
	SetGlobal(name, version string) error
	Exec(cmd string) error
}

// Executor walks a resolved ManifestNode and calls the mise runner.
type Executor struct {
	Runner   MiseRunner
	CheckRun func(cmd string) error // injectable; defaults to plain sh -c
}

func NewExecutor(r MiseRunner) *Executor {
	return &Executor{
		Runner:   r,
		CheckRun: shellCheck,
	}
}

// Execute resolves dependency order then visits every bunch in sequence.
func (e *Executor) Execute(m *ast.ManifestNode) error {
	sorted, err := Resolve(m)
	if err != nil {
		return err
	}
	for _, b := range sorted {
		if err := e.VisitBunch(b); err != nil {
			return err
		}
	}
	return nil
}

// VisitBunch runs the check guard first; skips the bunch when check exits 0.
func (e *Executor) VisitBunch(b ast.BunchNode) error {
	bnnlog.Debug("execute: bunch %s — enter", b.Name)
	if b.Check != "" {
		bnnlog.Debug("execute: bunch %s — check: %s", b.Name, b.Check)
		if err := e.CheckRun(b.Check); err == nil {
			bnnlog.Debug("execute: bunch %s — check passed, skipping", b.Name)
			return nil // already configured — skip
		}
		bnnlog.Debug("execute: bunch %s — check failed, running", b.Name)
	}
	if err := e.VisitRuntime(b.Name, b.Runtime); err != nil {
		return err
	}
	for _, s := range b.Steps {
		if err := e.VisitStep(s); err != nil {
			return err
		}
	}
	bnnlog.Debug("execute: bunch %s — done", b.Name)
	return nil
}

// VisitRuntime installs and activates the runtime.
//   - mise  → Install + SetGlobal
//   - brew  → Exec("brew install <name>")
//   - shell → no-op
func (e *Executor) VisitRuntime(name string, rt ast.RuntimeNode) error {
	switch rt.Type {
	case ast.RuntimeMise:
		bnnlog.Debug("execute: runtime mise install %s@%s", name, rt.Version)
		if err := e.Runner.Install(name, rt.Version); err != nil {
			return err
		}
		bnnlog.Debug("execute: runtime mise global  %s@%s", name, rt.Version)
		return e.Runner.SetGlobal(name, rt.Version)
	case ast.RuntimeBrew:
		bnnlog.Debug("execute: runtime brew install %s", name)
		return e.Runner.Exec("brew install " + name)
	default: // shell
		bnnlog.Debug("execute: runtime shell — no-op for %s", name)
		return nil
	}
}

// VisitStep executes a single step command inside the mise environment.
func (e *Executor) VisitStep(s ast.StepNode) error {
	bnnlog.Debug("execute: step %s: %s", s.Kind, s.Command)
	return e.Runner.Exec(s.Command)
}

// shellCheck runs cmd with sh -c and returns the exit error (nil = exit 0).
// Check commands run outside mise exec — they test whether setup is already done.
func shellCheck(cmd string) error {
	return exec.Command("sh", "-c", cmd).Run()
}
