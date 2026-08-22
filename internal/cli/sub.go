package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/luynrs/justray/internal/rpc"
	"github.com/luynrs/justray/internal/tui/style"
)

var subCmd = &cobra.Command{
	Use:     "subscription <command>",
	Aliases: []string{"sub"},
	Short:   "Manage subscriptions",
	GroupID: cmdGroup,
}

var subAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a subscription",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sub, err := client.AddSub(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Added %s (%d nodes)\n", sub.Name, sub.Nodes)
		return nil
	},
}

var subRemoveCmd = &cobra.Command{
	Use:               "remove <id | name>",
	Short:             "Remove a subscription",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeSub,
	RunE: func(cmd *cobra.Command, args []string) error {
		sub, err := resolveSub(args[0])
		if err != nil {
			return err
		}
		return client.RemoveSub(sub.ID)
	},
}

var subListCmd = &cobra.Command{
	Use:   "list",
	Short: "List subscriptions and their nodes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		subs, err := client.Subs()
		if err != nil {
			return err
		}
		nodes, err := client.Nodes()
		if err != nil {
			return err
		}
		showTree(subs, nodes)
		return nil
	},
}

func init() {
	subCmd.AddCommand(subAddCmd, subRemoveCmd, subListCmd)
}

func resolveSub(key string) (rpc.Sub, error) {
	subs, err := client.Subs()
	if err != nil {
		return rpc.Sub{}, err
	}
	return match(key, subs, func(s rpc.Sub) (string, string) { return s.ID, s.Name })
}

func completeSub(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 || !completionDaemon() {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	subs, err := client.Subs()
	return completeNames(subs, err, func(s rpc.Sub) string { return s.Name })
}

func showTree(subs []rpc.Sub, nodes []rpc.Node) {
	for i, s := range subs {
		if i > 0 {
			fmt.Println()
		}
		ns := filterBySub(nodes, s.ID)

		if s.Direct && len(ns) == 1 {
			fmt.Println(nodeLine(ns[0], "", 0, 0))
			continue
		}

		fmt.Println(bold(s.Name) + "  " + style.Dim.Render(s.ID))
		fmt.Println(style.Dim.Render(subMeta(s)))

		nameW, infoW := 0, 0
		for _, n := range ns {
			nameW = max(nameW, lipgloss.Width(n.Name))
			infoW = max(infoW, lipgloss.Width(serverProto(n)))
		}
		for j, n := range ns {
			branch := "├─"
			if j == len(ns)-1 {
				branch = "└─"
			}
			fmt.Println(nodeLine(n, branch, nameW, infoW))
		}
	}
}

func nodeLine(n rpc.Node, branch string, nameW, infoW int) string {
	name := style.Pad(n.Name, nameW)
	info := style.Dim.Render(style.Pad(serverProto(n), infoW))
	id := style.Dim.Render(n.ID)
	if branch == "" {
		return fmt.Sprintf("%s  %s  %s", n.Name, info, id)
	}
	return fmt.Sprintf("%s %s  %s  %s", style.Dim.Render(branch), name, info, id)
}

func serverProto(n rpc.Node) string {
	return fmt.Sprintf("%s:%d · %s", n.Server, n.Port, n.Protocol)
}

func subMeta(s rpc.Sub) string {
	if t := traffic(s); t != "" {
		return t + " · " + style.Since(s.UpdatedAt)
	}
	return style.Since(s.UpdatedAt)
}

func traffic(s rpc.Sub) string {
	used := s.Traffic.UploadBytes + s.Traffic.DownloadBytes
	switch {
	case s.Traffic.TotalBytes > 0:
		return fmt.Sprintf("%s / %s", style.Bytes(used), style.Bytes(s.Traffic.TotalBytes))
	case used > 0:
		return style.Bytes(used) + " used"
	}
	return ""
}

func filterBySub(nodes []rpc.Node, sub string) []rpc.Node {
	var out []rpc.Node
	for _, n := range nodes {
		if n.Sub == sub {
			out = append(out, n)
		}
	}
	return out
}
