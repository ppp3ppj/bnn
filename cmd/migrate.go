package cmd

import (
	"github.com/ppp3ppj/bnn/migrator"
	"github.com/ppp3ppj/bnn/runner"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Detect rvm/nvm/rbenv/pyenv and migrate active versions to mise",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m := &migrator.Migrator{
				Scanner: migrator.NewScanner(),
				Runner:  runner.New(),
				Out:     cmd.OutOrStdout(),
			}
			results := m.Run()
			for _, r := range results {
				if r.Err != nil {
					return r.Err
				}
			}
			return nil
		},
	}
}
