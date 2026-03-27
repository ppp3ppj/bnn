package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show installed vs declared state for each bunch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := loadConf(cfgPath(cmd))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			for _, b := range m.Bunches {
				if b.Check == "" {
					fmt.Fprintf(w, "%-15s  ?  no check declared\n", b.Name)
					continue
				}
				if err := exec.Command("sh", "-c", b.Check).Run(); err == nil {
					fmt.Fprintf(w, "%-15s  ✓  %s\n", b.Name, b.Check)
				} else {
					fmt.Fprintf(w, "%-15s  ✗  %s\n", b.Name, b.Check)
				}
			}
			return nil
		},
	}
}
