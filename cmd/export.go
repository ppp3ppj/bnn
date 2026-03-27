package cmd

import (
	"os"

	"github.com/ppp3ppj/bnn/visitor"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write a bash script equivalent of bnn apply to stdout (or --output file)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := loadConf(cfgPath(cmd))
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if path, _ := cmd.Flags().GetString("output"); path != "" {
				f, err := os.Create(path)
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}

			return visitor.Export(m, out)
		},
	}
	cmd.Flags().StringP("output", "o", "", "write script to file instead of stdout")
	return cmd
}
