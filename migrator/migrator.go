package migrator

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// miseRunner is the subset of runner.Runner we need.
type miseRunner interface {
	Install(name, version string) error
	SetGlobal(name, version string) error
}

// tool describes one legacy version manager.
type tool struct {
	Name     string // rvm, nvm, rbenv, …
	Runtime  string // ruby, node, python
	InitHint string // pattern users must remove from shell config
	detect   func(s *Scanner) (version string, ok bool)
}

// registeredTools is the full list bnn knows about.
var registeredTools = []tool{
	{
		Name:     "rvm",
		Runtime:  "ruby",
		InitHint: `[ -s "$HOME/.rvm/scripts/rvm" ]`,
		detect: func(s *Scanner) (string, bool) {
			out, err := s.runCmd("rvm", "current")
			if err != nil {
				return "", false
			}
			v := strings.TrimSpace(out)
			v = strings.TrimPrefix(v, "ruby-")
			if v == "" || v == "system" {
				return "", false
			}
			return v, true
		},
	},
	{
		Name:     "rbenv",
		Runtime:  "ruby",
		InitHint: `eval "$(rbenv init -)"`,
		detect: func(s *Scanner) (string, bool) {
			out, err := s.runCmd("rbenv", "version")
			if err != nil {
				return "", false
			}
			// "3.3.0 (set by ~/.rbenv/version)"
			return firstWord(out), true
		},
	},
	{
		Name:     "nvm",
		Runtime:  "node",
		InitHint: `source "$HOME/.nvm/nvm.sh"`,
		detect: func(s *Scanner) (string, bool) {
			// nvm is a shell function — read the default alias file instead
			content, err := s.readFile(s.HomeDir + "/.nvm/alias/default")
			if err != nil {
				return "", false
			}
			v := strings.TrimSpace(content)
			v = strings.TrimPrefix(v, "v")
			// skip symbolic aliases like "lts/*", "stable" — not real versions
			if v == "" || !isSemver(v) {
				return "", false
			}
			return v, true
		},
	},
	{
		Name:     "nodenv",
		Runtime:  "node",
		InitHint: `eval "$(nodenv init -)"`,
		detect: func(s *Scanner) (string, bool) {
			out, err := s.runCmd("nodenv", "version")
			if err != nil {
				return "", false
			}
			// "22.0.0 (set by ~/.nodenv/version)"
			return firstWord(out), true
		},
	},
	{
		Name:     "fnm",
		Runtime:  "node",
		InitHint: `eval "$(fnm env --use-on-cd)"`,
		detect: func(s *Scanner) (string, bool) {
			out, err := s.runCmd("fnm", "current")
			if err != nil {
				return "", false
			}
			v := strings.TrimSpace(out)
			v = strings.TrimPrefix(v, "v")
			if v == "" || v == "none" {
				return "", false
			}
			return v, true
		},
	},
	{
		Name:     "pyenv",
		Runtime:  "python",
		InitHint: `eval "$(pyenv init -)"`,
		detect: func(s *Scanner) (string, bool) {
			out, err := s.runCmd("pyenv", "version")
			if err != nil {
				return "", false
			}
			// "3.12.0 (set by ~/.pyenv/version)"
			return firstWord(out), true
		},
	},
}

// Found holds one detected legacy manager and its active version.
type Found struct {
	Name     string
	Runtime  string
	Version  string
	InitHint string
}

// Result is the outcome of migrating one Found tool.
type Result struct {
	Found Found
	Err   error
}

// Scanner detects which legacy tools are active.
// runCmd and readFile are injectable for testing.
type Scanner struct {
	HomeDir  string
	runCmd   func(name string, args ...string) (string, error)
	readFile func(path string) (string, error)
}

// NewScanner returns a Scanner backed by real os/exec.
func NewScanner() *Scanner {
	home, _ := os.UserHomeDir()
	return &Scanner{
		HomeDir:  home,
		runCmd:   defaultRunCmd,
		readFile: readFileString,
	}
}

// NewScannerWith creates a Scanner with injected runners (for tests).
func NewScannerWith(homeDir string, runCmd func(string, ...string) (string, error), readFile func(string) (string, error)) *Scanner {
	return &Scanner{HomeDir: homeDir, runCmd: runCmd, readFile: readFile}
}

// Scan returns all legacy tools that are currently active.
func (s *Scanner) Scan() []Found {
	var found []Found
	for _, t := range registeredTools {
		if v, ok := t.detect(s); ok {
			found = append(found, Found{
				Name:     t.Name,
				Runtime:  t.Runtime,
				Version:  v,
				InitHint: t.InitHint,
			})
		}
	}
	return found
}

// Migrator scans for legacy tools and installs matching versions via mise.
type Migrator struct {
	Scanner *Scanner
	Runner  miseRunner
	Out     io.Writer
}

// Run detects legacy tools, migrates each one, then prints cleanup warnings.
// Returns the list of results so callers can inspect errors.
func (m *Migrator) Run() []Result {
	found := m.Scanner.Scan()

	if len(found) == 0 {
		fmt.Fprintln(m.Out, "✓  no legacy version managers detected")
		return nil
	}

	var results []Result
	for _, f := range found {
		fmt.Fprintf(m.Out, "⚠  found: %s (%s) — active version: %s\n", f.Name, f.Runtime, f.Version)
		r := Result{Found: f}

		ref := f.Runtime + "@" + f.Version

		if err := m.Runner.Install(f.Runtime, f.Version); err != nil {
			r.Err = fmt.Errorf("[bnn] migrate — mise install %s failed: %w", ref, err)
			fmt.Fprintf(m.Out, "   ✗  install %s: %v\n", ref, err)
			results = append(results, r)
			continue
		}
		fmt.Fprintf(m.Out, "   →  installed %s via mise  ✓\n", ref)

		if err := m.Runner.SetGlobal(f.Runtime, f.Version); err != nil {
			r.Err = fmt.Errorf("[bnn] migrate — mise global %s failed: %w", ref, err)
			fmt.Fprintf(m.Out, "   ✗  set global %s: %v\n", ref, err)
			results = append(results, r)
			continue
		}
		fmt.Fprintf(m.Out, "   →  set %s as global  ✓\n\n", ref)

		results = append(results, r)
	}

	// shell config cleanup warnings
	fmt.Fprintln(m.Out, "─────────────────────────────────────────")
	fmt.Fprintln(m.Out, "⚠  ACTION REQUIRED: remove legacy init lines from your shell config")
	fmt.Fprintln(m.Out)
	for _, f := range found {
		fmt.Fprintf(m.Out, "  %s — remove or comment out:\n", f.Name)
		fmt.Fprintf(m.Out, "    %s\n\n", f.InitHint)
	}
	fmt.Fprintln(m.Out, "  Check files: ~/.bashrc  ~/.zshrc  ~/.bash_profile  ~/.profile")
	fmt.Fprintln(m.Out)
	fmt.Fprintln(m.Out, "  Then restart your shell:  exec $SHELL")

	return results
}

// ── helpers ──────────────────────────────────────────────────────────────────

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return s[:i]
	}
	return s
}

func defaultRunCmd(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

func readFileString(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// isSemver returns true if s looks like a real version (digits and dots only).
// Rejects symbolic aliases like "lts/*", "stable", "latest".
func isSemver(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch != '.' && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}
