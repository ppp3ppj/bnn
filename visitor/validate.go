package visitor

import (
	"fmt"
	"strings"

	"github.com/ppp3ppj/bnn/ast"
)

var validRuntimes = map[ast.RuntimeKind]bool{
	ast.RuntimeMise:  true,
	ast.RuntimeBrew:  true,
	ast.RuntimeShell: true,
}

// ValidationErrors holds every rule violation found in a single pass.
type ValidationErrors struct {
	errs []string
}

func (e *ValidationErrors) add(msg string) { e.errs = append(e.errs, msg) }
func (e *ValidationErrors) any() bool      { return len(e.errs) > 0 }

func (e *ValidationErrors) Error() string {
	return strings.Join(e.errs, "\n")
}

// Validate walks the ManifestNode and checks all rules.
// Returns nil when the manifest is valid.
// Returns *ValidationErrors listing every violation when invalid.
func Validate(m *ast.ManifestNode) error {
	ve := &ValidationErrors{}

	// build name → index map (also catches duplicate names)
	index := make(map[string]int, len(m.Bunches))
	for i, b := range m.Bunches {
		if _, exists := index[b.Name]; exists {
			ve.add(fmt.Sprintf("duplicate bunch name %q", b.Name))
		} else {
			index[b.Name] = i
		}
	}

	for _, b := range m.Bunches {
		// runtime type is valid
		if !validRuntimes[b.Runtime.Type] {
			ve.add(fmt.Sprintf("bunch %q: unknown runtime %q (must be mise, brew, or shell)", b.Name, b.Runtime.Type))
		}

		// depends targets exist
		for _, dep := range b.Depends {
			if _, ok := index[dep]; !ok {
				ve.add(fmt.Sprintf("bunch %q: depends on unknown bunch %q", b.Name, dep))
			}
		}

		// steps not empty — at least one run() required
		hasRun := false
		for _, s := range b.Steps {
			if s.Kind == ast.StepRun {
				hasRun = true
				break
			}
		}
		if !hasRun {
			ve.add(fmt.Sprintf("bunch %q: steps must contain at least one run()", b.Name))
		}

		// check is not empty if declared
		if b.Check != "" && strings.TrimSpace(b.Check) == "" {
			ve.add(fmt.Sprintf("bunch %q: check command must not be blank", b.Name))
		}
	}

	// circular dependency detection (DFS)
	if cycles := findCycles(m, index); len(cycles) > 0 {
		for _, c := range cycles {
			ve.add(fmt.Sprintf("circular dependency: %s", c))
		}
	}

	if ve.any() {
		return ve
	}
	return nil
}

type state int

const (
	unvisited state = iota
	visiting
	visited
)

func findCycles(m *ast.ManifestNode, index map[string]int) []string {
	states := make(map[string]state, len(m.Bunches))
	var cycles []string

	var dfs func(name string, path []string)
	dfs = func(name string, path []string) {
		if states[name] == visited {
			return
		}
		if states[name] == visiting {
			// find where the cycle starts in path
			for i, n := range path {
				if n == name {
					cycle := append(path[i:], name)
					cycles = append(cycles, strings.Join(cycle, " → "))
					return
				}
			}
			return
		}

		states[name] = visiting
		path = append(path, name)

		idx, ok := index[name]
		if ok {
			for _, dep := range m.Bunches[idx].Depends {
				dfs(dep, path)
			}
		}

		states[name] = visited
	}

	for _, b := range m.Bunches {
		if states[b.Name] == unvisited {
			dfs(b.Name, nil)
		}
	}

	return cycles
}
