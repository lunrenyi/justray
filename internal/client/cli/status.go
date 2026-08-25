package cli

import "github.com/spf13/cobra"

var statusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show status",
	GroupID: cmdGroup,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := client.Status()
		if err != nil {
			return err
		}
		stateHeadline(st)
		warn(st.LastErr)

		if st.Connected {
			nodeDetails(st)
			return nil
		}

		id, err := client.Active()
		if err != nil || id == "" {
			return nil
		}
		n, err := resolveNode(id)
		if err != nil || n.ID == "" {
			return nil
		}
		last := [][2]string{{"Last node", nodeName(n.Name, n.ID)}}
		fields(append(last, nodeFields(n)...)...)
		return nil
	},
}
