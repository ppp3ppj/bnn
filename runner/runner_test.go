package runner_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/runner"
)

// setupFakeMise writes a shell script that records its arguments to a file,
// then returns a Runner pointed at it and a function to read the recorded args.
func setupFakeMise(t *testing.T) (*runner.Runner, func() string) {
	t.Helper()

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	bin := filepath.Join(dir, "mise")

	script := "#!/bin/sh\nprintf '%s\n' \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatalf("could not write fake mise: %v", err)
	}

	r := &runner.Runner{MiseBin: bin}

	readArgs := func() string {
		b, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("args file not written — command may not have run: %v", err)
		}
		return strings.TrimSpace(string(b))
	}

	return r, readArgs
}

func TestInstall(t *testing.T) {
	r, args := setupFakeMise(t)
	if err := r.Install("ruby", "3.3"); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	got := args()
	if got != "install\nruby@3.3" {
		t.Errorf("Install args: got %q", got)
	}
}

func TestSetGlobal(t *testing.T) {
	r, args := setupFakeMise(t)
	if err := r.SetGlobal("ruby", "3.3"); err != nil {
		t.Fatalf("SetGlobal returned error: %v", err)
	}
	got := args()
	if got != "global\nruby@3.3" {
		t.Errorf("SetGlobal args: got %q", got)
	}
}

func TestExec(t *testing.T) {
	r, args := setupFakeMise(t)
	if err := r.Exec("gem install bundler"); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	got := args()
	if got != "exec\n--\nsh\n-c\ngem install bundler" {
		t.Errorf("Exec args: got %q", got)
	}
}

func TestExec_shellSyntax(t *testing.T) {
	// commands with pipes/redirects are passed as-is to sh -c
	r, args := setupFakeMise(t)
	if err := r.Exec("mise current ruby | grep 3.3"); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	got := args()
	if !strings.HasSuffix(got, "mise current ruby | grep 3.3") {
		t.Errorf("shell command not preserved: got %q", got)
	}
}

func TestInstall_ref_format(t *testing.T) {
	tests := []struct {
		name, version, wantRef string
	}{
		{"ruby", "3.3", "ruby@3.3"},
		{"node", "22", "node@22"},
		{"python", "3.12.0", "python@3.12.0"},
	}
	for _, tc := range tests {
		t.Run(tc.wantRef, func(t *testing.T) {
			r, args := setupFakeMise(t)
			if err := r.Install(tc.name, tc.version); err != nil {
				t.Fatal(err)
			}
			got := args()
			if !strings.Contains(got, tc.wantRef) {
				t.Errorf("want ref %q in args, got %q", tc.wantRef, got)
			}
		})
	}
}

func TestNew_defaultBin(t *testing.T) {
	r := runner.New()
	if r.MiseBin != "mise" {
		t.Errorf("default MiseBin: want %q, got %q", "mise", r.MiseBin)
	}
}

func TestInstall_error_badBin(t *testing.T) {
	r := &runner.Runner{MiseBin: "/no/such/binary"}
	if err := r.Install("ruby", "3.3"); err == nil {
		t.Error("expected error when binary does not exist")
	}
}
