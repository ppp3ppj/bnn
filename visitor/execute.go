package visitor

import (
	"os/exec"

	"github.com/ppp3ppj/bnn/ast"
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
	if b.Check != "" {
		if err := e.CheckRun(b.Check); err == nil {
			return nil // already configured — skip
		}
	}
	if err := e.VisitRuntime(b.Name, b.Runtime); err != nil {
		return err
	}
	for _, s := range b.Steps {
		if err := e.VisitStep(s); err != nil {
			return err
		}
	}
	return nil
}

// VisitRuntime installs and activates the runtime.
//   - mise  → Install + SetGlobal
//   - brew  → Exec("brew install <name>")
//   - shell → no-op
func (e *Executor) VisitRuntime(name string, rt ast.RuntimeNode) error {
	switch rt.Type {
	case ast.RuntimeMise:
		if err := e.Runner.Install(name, rt.Version); err != nil {
			return err
		}
		return e.Runner.SetGlobal(name, rt.Version)
	case ast.RuntimeBrew:
		return e.Runner.Exec("brew install " + name)
	default: // shell
		return nil
	}
}

// VisitStep executes a single step command inside the mise environment.
func (e *Executor) VisitStep(s ast.StepNode) error {
	return e.Runner.Exec(s.Command)
}

// shellCheck runs cmd with sh -c and returns the exit error (nil = exit 0).
// Check commands run outside mise exec — they test whether setup is already done.
func shellCheck(cmd string) error {
	return exec.Command("sh", "-c", cmd).Run()
}
