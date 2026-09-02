package cli

import "github.com/spf13/cobra"

var stopCmd = &cobra.Command{
	Use:     "stop",
	Short:   "Shut down",
	GroupID: cmdGroup,
	Args:    cobra.NoArgs,
}

func (a *app) stop(cmd *cobra.Command, args []string) error {
	c := a.daemon()
	if c == nil || c.Ping() != nil {
		done("Daemon is not running")
		return nil
	}
	stop := spin("Stopping daemon")
	err := c.Shutdown()
	stop()
	if err != nil {
		return err
	}
	done("Daemon stopped")
	return nil
}
