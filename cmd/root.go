package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/internal/parser/dsl"
	"github.com/ppp3ppj/bnn/visitor"
	"github.com/spf13/cobra"
)

// Execute is the entry point called from main.
func Execute() error {
	home, _ := os.UserHomeDir()
	defaultConf := filepath.Join(home, ".config", "bnn", "bnn.conf")
	return NewRootCmd(defaultConf, exec.LookPath).Execute()
}

// NewRootCmd builds the command tree. Exported for testing.
// conf is the default config path; tests pass a temp file path here.
func NewRootCmd(conf string, lookPath func(string) (string, error)) *cobra.Command {
	root := &cobra.Command{
		Use:           "bnn",
		Short:         "Declarative machine setup powered by mise",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// --config overrides the config path for all subcommands.
	// Subcommands read it via cmd.Root().PersistentFlags().GetString("config").
	root.PersistentFlags().String("config", conf, "path to config file (default ~/.config/bnn/bnn.conf)")

	root.AddCommand(newApplyCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newDoctorCmd(lookPath))
	return root
}

// cfgPath retrieves the resolved --config value from the root persistent flag.
func cfgPath(cmd *cobra.Command) string {
	path, _ := cmd.Root().PersistentFlags().GetString("config")
	return path
}

// loadConf reads, parses, and validates the config file at path.
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
