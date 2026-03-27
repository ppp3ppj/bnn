package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/internal/parser/dsl"
	"github.com/ppp3ppj/bnn/visitor"
	"github.com/spf13/cobra"
)

// Execute is the entry point called from main.
func Execute() error {
	return NewRootCmd("bnn.conf", exec.LookPath).Execute()
}

// NewRootCmd builds the command tree. Exported for testing.
func NewRootCmd(conf string, lookPath func(string) (string, error)) *cobra.Command {
	root := &cobra.Command{
		Use:           "bnn",
		Short:         "Declarative machine setup powered by mise",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newApplyCmd(conf))
	root.AddCommand(newStatusCmd(conf))
	root.AddCommand(newDoctorCmd(conf, lookPath))
	root.AddCommand(newMigrateCmd())
	return root
}

// loadConf reads, parses, and validates bnn.conf at path.
func loadConf(path string) (*ast.ManifestNode, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("[bnn] cannot read %s — %w", path, err)
	}
	m, err := dsl.Parse(string(src))
	if err != nil {
		return nil, err
	}
	if err := visitor.Validate(m); err != nil {
		return nil, err
	}
	return m, nil
}
