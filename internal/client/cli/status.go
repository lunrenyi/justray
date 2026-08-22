package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

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

		if st.Connected {
			fmt.Println("Connected")
			printFields(
				field{"Node", st.NodeName},
				field{"ID", st.NodeID},
				field{"Mode", strings.ToUpper(modeWord(st.Tun))},
			)
			if st.LastErr != "" {
				fmt.Println(st.LastErr)
			}
			return nil
		}

		fmt.Println("Disconnected")
		id, err := client.Active()
		if err != nil || id == "" {
			return nil
		}
		n, err := resolveNode(id)
		if err != nil {
			return nil
		}
		printFields(field{"Last node", n.Name}, field{"ID", n.ID})
		return nil
	},
}

type field struct{ label, value string }

func printFields(fields ...field) {
	w := 0
	for _, f := range fields {
		w = max(w, len(f.label))
	}
	for _, f := range fields {
		fmt.Printf("%-*s %s\n", w+1, f.label+":", f.value)
	}
}

func modeWord(tun bool) string {
	if tun {
		return "tun"
	}
	return "proxy"
}
