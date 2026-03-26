package runner

import (
	"fmt"
	"os"
	"os/exec"
)

// Runner shells out to mise. MiseBin defaults to "mise" but can be overridden
// (e.g. in tests) to point at a fake binary.
type Runner struct {
	MiseBin string
}

func New() *Runner {
	return &Runner{MiseBin: "mise"}
}

// Install runs: mise install <name>@<version>
func (r *Runner) Install(name, version string) error {
	return r.run(r.MiseBin, "install", fmt.Sprintf("%s@%s", name, version))
}

// SetGlobal runs: mise global <name>@<version>
func (r *Runner) SetGlobal(name, version string) error {
	return r.run(r.MiseBin, "global", fmt.Sprintf("%s@%s", name, version))
}

// Exec runs a shell command inside the mise environment:
//
//	mise exec -- sh -c <cmd>
//
// Routing through sh -c means the command string can contain pipes,
// redirects, and other shell syntax.
func (r *Runner) Exec(cmd string) error {
	return r.run(r.MiseBin, "exec", "--", "sh", "-c", cmd)
}

func (r *Runner) run(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
