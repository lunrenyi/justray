package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:     "down",
	Short:   "Disconnect",
	GroupID: cmdGroup,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := client.Status()
		if err != nil {
			return err
		}
		if !st.Connected {
			fmt.Println("Already disconnected")
			return nil
		}
		if _, err := client.Disconnect(); err != nil {
			return err
		}
		fmt.Println("Disconnected")
		return nil
	},
}
