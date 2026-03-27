package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCmd(lookPath func(string) (string, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check prerequisites: mise in PATH, bnn.conf exists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conf := cfgPath(cmd)
			w := cmd.OutOrStdout()
			ok := true

			// check 1: mise in PATH
			if path, err := lookPath("mise"); err != nil {
				fmt.Fprintln(w, "✗  mise not found in PATH")
				fmt.Fprintln(w, "   install: curl https://mise.run | sh")
				ok = false
			} else {
				fmt.Fprintf(w, "✓  mise found: %s\n", path)
			}

			// check 2: config file exists
			if _, err := loadConf(conf); err != nil {
				fmt.Fprintf(w, "✗  %s not found or invalid\n", conf)
				ok = false
			} else {
				fmt.Fprintf(w, "✓  bnn.conf found: %s\n", conf)
			}

			if !ok {
				return fmt.Errorf("[bnn] prerequisites not met — fix the issues above and re-run bnn doctor")
			}
			return nil
		},
	}
}
