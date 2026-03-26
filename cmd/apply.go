package cmd

import (
	"fmt"
	"io"

	"github.com/ppp3ppj/bnn/ast"
	"github.com/ppp3ppj/bnn/runner"
	"github.com/ppp3ppj/bnn/visitor"
	"github.com/spf13/cobra"
)

func newApplyCmd(conf string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [bunch]",
		Short: "Apply all bunches, or a specific bunch, from bnn.conf",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dry, _ := cmd.Flags().GetBool("dry")

			m, err := loadConf(conf)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				return applySingle(m, args[0], dry, cmd.OutOrStdout())
			}
			return applyAll(m, dry, cmd.OutOrStdout())
		},
	}
	cmd.Flags().Bool("dry", false, "Print what would happen without executing")
	return cmd
}

// applyAll: parse → validate → resolve → execute (or dry-run)
func applyAll(m *ast.ManifestNode, dry bool, w io.Writer) error {
	if dry {
		visitor.DryRun(m, w)
		return nil
	}
	r := runner.New()
	e := visitor.NewExecutor(r)
	return e.Execute(m)
}

// applySingle: parse → validate → pick one bunch → execute (or dry-run)
func applySingle(m *ast.ManifestNode, name string, dry bool, w io.Writer) error {
	var target *ast.BunchNode
	for i := range m.Bunches {
		if m.Bunches[i].Name == name {
			b := m.Bunches[i]
			target = &b
			break
		}
	}
	if target == nil {
		return fmt.Errorf("bunch %q not found in bnn.conf", name)
	}

	if dry {
		visitor.DryRun(&ast.ManifestNode{Bunches: []ast.BunchNode{*target}}, w)
		return nil
	}
	r := runner.New()
	e := visitor.NewExecutor(r)
	return e.VisitBunch(*target)
}
