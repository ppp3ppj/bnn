package visitor

import (
	"fmt"
	"io"
	"strings"

	"github.com/ppp3ppj/bnn/ast"
)

// DryRun walks the manifest and prints every command that would be executed,
// without running anything. Output is written to w.
func DryRun(m *ast.ManifestNode, w io.Writer) {
	for i, b := range m.Bunches {
		if i > 0 {
			fmt.Fprintln(w)
		}
		dryRunBunch(b, w)
	}
}

func dryRunBunch(b ast.BunchNode, w io.Writer) {
	fmt.Fprintf(w, "--- bunch: %s ---\n", b.Name)

	if len(b.Depends) > 0 {
		fmt.Fprintf(w, "[dry] depends: %s\n", strings.Join(b.Depends, ", "))
	}

	if b.Check != "" {
		fmt.Fprintf(w, "[dry] check: %s  (skip bunch if exits 0)\n", b.Check)
	}

	switch b.Runtime.Type {
	case ast.RuntimeMise:
		ref := b.Name + "@" + b.Runtime.Version
		fmt.Fprintf(w, "[dry] mise install %s\n", ref)
		fmt.Fprintf(w, "[dry] mise global  %s\n", ref)
	case ast.RuntimeBrew:
		fmt.Fprintf(w, "[dry] brew install %s\n", b.Name)
	case ast.RuntimeShell:
		// no runtime setup step
	}

	for _, s := range b.Steps {
		fmt.Fprintf(w, "[dry] %s: %s\n", s.Kind, s.Command)
	}
}
