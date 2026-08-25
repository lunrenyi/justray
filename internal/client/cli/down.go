package cli

import "github.com/spf13/cobra"

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
			done("Already disconnected")
			return nil
		}
		stop := spin("Disconnecting")
		_, err = client.Disconnect()
		stop()
		if err != nil {
			return err
		}
		done("Disconnected")
		return nil
	},
}
