package migrator_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ppp3ppj/bnn/migrator"
)

// ── fake helpers ─────────────────────────────────────────────────────────────

// cmdMap maps "tool arg" → stdout output. Missing key → error.
func fakeRunCmd(cmds map[string]string) func(string, ...string) (string, error) {
	return func(name string, args ...string) (string, error) {
		key := name
		if len(args) > 0 {
			key += " " + args[0]
		}
		if out, ok := cmds[key]; ok {
			return out, nil
		}
		return "", errors.New("not found")
	}
}

// fileMap maps path → content.
func fakeReadFile(files map[string]string) func(string) (string, error) {
	return func(path string) (string, error) {
		if c, ok := files[path]; ok {
			return c, nil
		}
		return "", errors.New("no such file")
	}
}

type fakeMise struct {
	calls []string
	fail  map[string]bool // "install:ruby@3.3" → true means return error
}

func (f *fakeMise) Install(name, version string) error {
	key := "install:" + name + "@" + version
	f.calls = append(f.calls, key)
	if f.fail[key] {
		return errors.New("install failed")
	}
	return nil
}

func (f *fakeMise) SetGlobal(name, version string) error {
	key := "global:" + name + "@" + version
	f.calls = append(f.calls, key)
	if f.fail[key] {
		return errors.New("global failed")
	}
	return nil
}

func newMigrator(s *migrator.Scanner, m *fakeMise) (*migrator.Migrator, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	mg := &migrator.Migrator{Scanner: s, Runner: m, Out: buf}
	return mg, buf
}

// ── Scanner tests ─────────────────────────────────────────────────────────────

func TestScanner_nothingFound(t *testing.T) {
	s := migrator.NewScannerWith("/home/user", fakeRunCmd(nil), fakeReadFile(nil))
	if got := s.Scan(); len(got) != 0 {
		t.Errorf("expected no tools, got %v", got)
	}
}

func TestScanner_rvm(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "ruby-3.3.0\n"}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if len(found) != 1 {
		t.Fatalf("want 1, got %d: %v", len(found), found)
	}
	if found[0].Name != "rvm" || found[0].Version != "3.3.0" || found[0].Runtime != "ruby" {
		t.Errorf("unexpected: %+v", found[0])
	}
}

func TestScanner_rvm_stripsPrefix(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "ruby-3.2.1"}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if found[0].Version != "3.2.1" {
		t.Errorf("want 3.2.1, got %s", found[0].Version)
	}
}

func TestScanner_rvm_system_skipped(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "system"}),
		fakeReadFile(nil),
	)
	if got := s.Scan(); len(got) != 0 {
		t.Errorf("system ruby should not be detected, got %v", got)
	}
}

func TestScanner_rbenv(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rbenv version": "3.3.0 (set by ~/.rbenv/version)"}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if len(found) != 1 || found[0].Name != "rbenv" || found[0].Version != "3.3.0" {
		t.Errorf("unexpected: %v", found)
	}
}

func TestScanner_nvm(t *testing.T) {
	// nvm has no binary — detected via alias/default file
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(nil),
		fakeReadFile(map[string]string{"/home/user/.nvm/alias/default": "v22.0.0\n"}),
	)
	found := s.Scan()
	if len(found) != 1 || found[0].Name != "nvm" || found[0].Version != "22.0.0" {
		t.Errorf("unexpected: %v", found)
	}
}

func TestScanner_nvm_ltsAlias_skipped(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(nil),
		fakeReadFile(map[string]string{"/home/user/.nvm/alias/default": "lts/*\n"}),
	)
	if got := s.Scan(); len(got) != 0 {
		t.Errorf("lts/* alias should be skipped, got %v", got)
	}
}

func TestScanner_nvm_stripsV(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(nil),
		fakeReadFile(map[string]string{"/home/user/.nvm/alias/default": "v20.5.0"}),
	)
	if found := s.Scan(); found[0].Version != "20.5.0" {
		t.Errorf("want 20.5.0, got %s", found[0].Version)
	}
}

func TestScanner_nodenv(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"nodenv version": "22.0.0 (set by ~/.nodenv/version)"}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if len(found) != 1 || found[0].Name != "nodenv" || found[0].Version != "22.0.0" {
		t.Errorf("unexpected: %v", found)
	}
}

func TestScanner_fnm(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"fnm current": "v22.0.0"}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if len(found) != 1 || found[0].Name != "fnm" || found[0].Version != "22.0.0" {
		t.Errorf("unexpected: %v", found)
	}
}

func TestScanner_fnm_none_skipped(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"fnm current": "none"}),
		fakeReadFile(nil),
	)
	if got := s.Scan(); len(got) != 0 {
		t.Errorf("fnm 'none' should not be detected")
	}
}

func TestScanner_pyenv(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"pyenv version": "3.12.0 (set by ~/.pyenv/version)"}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if len(found) != 1 || found[0].Name != "pyenv" || found[0].Version != "3.12.0" {
		t.Errorf("unexpected: %v", found)
	}
}

func TestScanner_multiple(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{
			"rvm current":    "ruby-3.3.0",
			"pyenv version":  "3.12.0",
		}),
		fakeReadFile(nil),
	)
	found := s.Scan()
	if len(found) != 2 {
		t.Fatalf("want 2, got %d: %v", len(found), found)
	}
}

// ── Migrator output tests ─────────────────────────────────────────────────────

func TestMigrator_noneFound_message(t *testing.T) {
	s := migrator.NewScannerWith("/home/user", fakeRunCmd(nil), fakeReadFile(nil))
	fm := &fakeMise{}
	mg, buf := newMigrator(s, fm)
	mg.Run()
	if !strings.Contains(buf.String(), "no legacy version managers") {
		t.Errorf("expected 'no legacy' message, got:\n%s", buf.String())
	}
	if len(fm.calls) != 0 {
		t.Errorf("no mise calls expected when nothing found")
	}
}

func TestMigrator_callsInstallAndGlobal(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "ruby-3.3.0"}),
		fakeReadFile(nil),
	)
	fm := &fakeMise{}
	mg, _ := newMigrator(s, fm)
	mg.Run()

	if len(fm.calls) != 2 {
		t.Fatalf("want 2 calls (install+global), got %v", fm.calls)
	}
	if fm.calls[0] != "install:ruby@3.3.0" {
		t.Errorf("first call: want install:ruby@3.3.0, got %s", fm.calls[0])
	}
	if fm.calls[1] != "global:ruby@3.3.0" {
		t.Errorf("second call: want global:ruby@3.3.0, got %s", fm.calls[1])
	}
}

func TestMigrator_output_showsFoundLine(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "ruby-3.3.0"}),
		fakeReadFile(nil),
	)
	mg, buf := newMigrator(s, &fakeMise{})
	mg.Run()
	out := buf.String()
	if !strings.Contains(out, "rvm") || !strings.Contains(out, "3.3.0") {
		t.Errorf("expected found line with rvm+version in:\n%s", out)
	}
}

func TestMigrator_output_showsCleanupWarning(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "ruby-3.3.0"}),
		fakeReadFile(nil),
	)
	mg, buf := newMigrator(s, &fakeMise{})
	mg.Run()
	out := buf.String()
	if !strings.Contains(out, "ACTION REQUIRED") {
		t.Errorf("expected cleanup warning in:\n%s", out)
	}
	if !strings.Contains(out, ".rvm") {
		t.Errorf("expected rvm init hint in:\n%s", out)
	}
}

func TestMigrator_installError_skipsGlobal(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"rvm current": "ruby-3.3.0"}),
		fakeReadFile(nil),
	)
	fm := &fakeMise{fail: map[string]bool{"install:ruby@3.3.0": true}}
	mg, _ := newMigrator(s, fm)
	results := mg.Run()

	if results[0].Err == nil {
		t.Error("expected error when install fails")
	}
	for _, c := range fm.calls {
		if strings.HasPrefix(c, "global:") {
			t.Error("SetGlobal must not be called when Install fails")
		}
	}
}

func TestMigrator_result_noError_onSuccess(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{"pyenv version": "3.12.0"}),
		fakeReadFile(nil),
	)
	mg, _ := newMigrator(s, &fakeMise{})
	results := mg.Run()

	if len(results) != 1 || results[0].Err != nil {
		t.Errorf("expected successful result, got %+v", results)
	}
}

func TestMigrator_multipleTools_allMigrated(t *testing.T) {
	s := migrator.NewScannerWith("/home/user",
		fakeRunCmd(map[string]string{
			"rvm current":   "ruby-3.3.0",
			"pyenv version": "3.12.0",
		}),
		fakeReadFile(nil),
	)
	fm := &fakeMise{}
	mg, _ := newMigrator(s, fm)
	results := mg.Run()

	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if len(fm.calls) != 4 { // install+global × 2
		t.Errorf("want 4 mise calls, got %v", fm.calls)
	}
}
