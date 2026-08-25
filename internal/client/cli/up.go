package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/luynrs/justray/internal/client/tui/style"
	"github.com/luynrs/justray/internal/shared/rpc"
)

var upTunFlag, upProxyFlag bool

var upCmd = &cobra.Command{
	Use:               "up [id | name]",
	Short:             "Connect",
	GroupID:           cmdGroup,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeNode,
	RunE: func(cmd *cobra.Command, args []string) error {
		if upTunFlag && upProxyFlag {
			return fmt.Errorf("pick either --tun or --proxy")
		}
		mode := tunMode(upTunFlag, upProxyFlag)

		if len(args) > 0 {
			return connectNode(args[0], mode)
		}

		st, err := client.Status()
		if err != nil {
			return err
		}
		if st.Connected {
			if mode != nil {
				return switchMode(st, *mode)
			}
			report("Already "+state(st), st)
			return nil
		}

		id, err := client.Active()
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("no node selected yet; pick one: %s <id | name>", cmd.CommandPath())
		}
		return connectNode(id, mode)
	},
}

func init() {
	upCmd.Flags().BoolVar(&upTunFlag, "tun", false, "connect in TUN mode")
	upCmd.Flags().BoolVar(&upProxyFlag, "proxy", false, "connect in proxy mode")
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
	spinText := "Connecting to " + style.Sanitize(n.Name)
	if mode != nil {
		if _, err := runOp(spinText, func() (rpc.Status, error) { return client.SetTun(*mode) }, mode); err != nil {
			return err
		}
	}
	st, err := runOp(spinText, func() (rpc.Status, error) {
		return client.Connect(n.ID)
	}, mode)
	if err != nil {
		return err
	}
	text := state(st)
	report(upperFirst(text), st)
	return nil
}

// runOp waits out the daemon re-execing itself with tun caps
func runOp(text string, op func() (rpc.Status, error), want *bool) (rpc.Status, error) {
	stop := spin(text)
	st, err := op()
	stop()
	if err == nil || err.Error() != rpc.ElevateMsg {
		return st, err
	}
	stop = spin("Granting permissions")
	defer stop()
	return awaitElevate(client.Status, want, 3*time.Minute)
}

var elevatePoll = 500 * time.Millisecond

func awaitElevate(status func() (rpc.Status, error), want *bool, timeout time.Duration) (rpc.Status, error) {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		time.Sleep(elevatePoll)
		st, err := status()
		switch {
		case err != nil: // the daemon is mid exec-restart
		case st.Connected && (want == nil || st.Tun == *want):
			return st, nil
		case st.LastErr != "" && st.LastErr != rpc.ElevateMsg && !st.Connected:
			return st, errors.New(st.LastErr)
		}
	}
	return rpc.Status{}, errors.New("timed out waiting for permissions")
}

func switchMode(st rpc.Status, tun bool) error {
	if st.Tun == tun {
		report("Already "+state(st), st)
		return nil
	}
	next, err := runOp("Switching to "+strings.ToUpper(modeWord(tun)), func() (rpc.Status, error) {
		return client.SetTun(tun)
	}, &tun)
	if err != nil {
		return err
	}
	text := state(next)
	report(upperFirst(text), next)
	return nil
}

func report(headline string, st rpc.Status) {
	done(headline)
	nodeDetails(st)
	warn(st.LastErr)
}

func resolveNode(key string) (rpc.Node, error) {
	nodes, err := client.Nodes()
	if err != nil {
		return rpc.Node{}, err
	}
	return match(key, "node", nodes, func(n rpc.Node) (string, string) { return n.ID, n.Name })
}

func completeNode(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 || !completionDaemon() {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	nodes, err := client.Nodes()
	return completeNames(nodes, err, func(n rpc.Node) string { return n.Name })
}
