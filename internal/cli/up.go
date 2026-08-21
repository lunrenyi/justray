package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/luynrs/justray/internal/daemon"
)

var upTunFlag, upProxyFlag bool

var upCmd = &cobra.Command{
	Use:               "up [id | name]",
	Short:             "Connect",
	GroupID:           cmdGroup,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeNode,
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := tunMode(upTunFlag, upProxyFlag)

		if len(args) > 0 {
			return connectNode(args[0], mode)
		}

		st, err := client.Status()
		if err != nil {
			return err
		}
		if st.Connected {
			if mode == nil {
				fmt.Println("Already connected")
				return nil
			}
			return switchMode(st, *mode)
		}

		id, err := client.Active()
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("no active node yet; specify one: jray up <id | name>")
		}
		return connectNode(id, mode)
	},
}

func init() {
	upCmd.Flags().BoolVar(&upTunFlag, "tun", false, "connect in TUN mode")
	upCmd.Flags().BoolVar(&upProxyFlag, "proxy", false, "connect in proxy mode")
	upCmd.MarkFlagsMutuallyExclusive("tun", "proxy")
}

func tunMode(tun, proxy bool) *bool {
	switch {
	case tun:
		return &tun
	case proxy:
		off := false
		return &off
	}
	return nil
}

func connectNode(key string, mode *bool) error {
	n, err := resolveNode(key)
	if err != nil {
		return err
	}
	fmt.Printf("Connecting to %s...\n", n.Name)
	if mode != nil {
		if _, err := client.SetTun(*mode); err != nil {
			return err
		}
	}
	st, err := client.Connect(n.ID)
	if err != nil {
		return err
	}
	fmt.Println("Connected")
	if st.LastErr != "" {
		fmt.Println(st.LastErr)
	}
	return nil
}

func switchMode(st daemon.Status, tun bool) error {
	if st.Tun == tun {
		fmt.Println("Already connected via " + strings.ToUpper(modeWord(tun)))
		return nil
	}
	fmt.Printf("Switching %s → %s...\n", modeWord(st.Tun), modeWord(tun))
	st2, err := client.SetTun(tun)
	if err != nil {
		return err
	}
	fmt.Println("Connected")
	if st2.LastErr != "" {
		fmt.Println(st2.LastErr)
	}
	return nil
}

func resolveNode(key string) (daemon.Node, error) {
	nodes, err := client.Nodes()
	if err != nil {
		return daemon.Node{}, err
	}
	return match(key, nodes, func(n daemon.Node) (string, string) { return n.ID, n.Name })
}

func completeNode(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 || !completionDaemon() {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	nodes, err := client.Nodes()
	return completeNames(nodes, err, func(n daemon.Node) string { return n.Name })
}
