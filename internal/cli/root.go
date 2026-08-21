package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/luynrs/justray/internal/cli/detach"
	"github.com/luynrs/justray/internal/daemon"
	"github.com/luynrs/justray/internal/tui"
)

var client *daemon.Client

const cmdGroup = "commands"

var rootCmd = &cobra.Command{
	Use:  "justray <command>",
	Long: `A fast, lightweight, and modern VPN client that just works.`,

	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		for c := cmd; c != nil; c = c.Parent() {
			if c.Name() == "completion" || c.Name() == "help" {
				return nil
			}
		}
		return connectDaemon()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(client)
	},
}

var boldStyle = lipgloss.NewStyle().Bold(true)

func bold(s string) string { return boldStyle.Render(s) }

func cmdLine(c *cobra.Command) string {
	return fmt.Sprintf("%-*s", c.NamePadding()+1, c.Name()+":")
}

const usageTemplate = `{{bold "USAGE"}}
  {{.UseLine}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{bold "AVAILABLE COMMANDS"}}{{range $cmds}}{{if .IsAvailableCommand}}
  {{cmdLine .}} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{bold .Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) .IsAvailableCommand)}}
  {{cmdLine .}} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{bold "ADDITIONAL COMMANDS"}}{{range $cmds}}{{if (and (eq .GroupID "") .IsAvailableCommand)}}
  {{cmdLine .}} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{bold "FLAGS"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{bold "GLOBAL FLAGS"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} <command> --help" for more information about a command.{{end}}
`

func init() {
	cobra.EnableCommandSorting = false
	cobra.AddTemplateFunc("bold", bold)
	cobra.AddTemplateFunc("cmdLine", cmdLine)
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.AddGroup(&cobra.Group{ID: cmdGroup, Title: "AVAILABLE COMMANDS"})
	rootCmd.AddCommand(upCmd, downCmd, statusCmd, subCmd)
}

// Execute runs the justray CLI. The caller (cmd/justray) handles the error.
func Execute() error {
	rootCmd.Use = filepath.Base(os.Args[0]) + " <command>"

	rootCmd.InitDefaultCompletionCmd()
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" {
			c.Short = "Generate shell completion scripts"
		}
	}
	setHelpText(rootCmd)

	return rootCmd.Execute()
}

func setHelpText(c *cobra.Command) {
	c.InitDefaultHelpFlag()
	c.Flags().Lookup("help").Usage = "Show help for command"
	for _, sub := range c.Commands() {
		setHelpText(sub)
	}
}

func connectDaemon() error {
	dir, err := daemon.Dir()
	if err != nil {
		return fmt.Errorf("resolve config dir: %w", err)
	}
	if err := daemon.EnsureDir(dir); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	client = daemon.NewClient(daemon.Socket(dir))
	if client.Ping() == nil {
		return nil
	}
	if err := spawn(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	if err := wait(client, 10*time.Second); err != nil {
		return fmt.Errorf("%w — see %s", err, daemon.DaemonLog(dir))
	}
	return nil
}

func spawn() error {
	bin, err := justrayd()
	if err != nil {
		return err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(bin)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	detach.Cmd(cmd)
	return cmd.Start() // detached on purpose
}

func justrayd() (string, error) {
	if bin := nextToSelf("justrayd"); bin != "" {
		return bin, nil
	}
	bin, err := exec.LookPath(exeName("justrayd"))
	if err != nil {
		return "", fmt.Errorf("justrayd not found next to justray or in PATH; build it with \"go build ./cmd/justrayd\"")
	}
	return bin, nil
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func nextToSelf(name string) string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(self), exeName(name))
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

func wait(c *daemon.Client, timeout time.Duration) error {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		if c.Ping() == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not come up within %s", timeout)
}

func completionDaemon() bool {
	if client != nil {
		return true
	}
	d, err := daemon.Dir()
	if err != nil {
		return false
	}
	client = daemon.NewClient(daemon.Socket(d))
	return client.Ping() == nil
}

func match[T any](key string, items []T, idName func(T) (id, name string)) (T, error) {
	key = strings.ToLower(key)
	var hits []T
	var names []string
	for _, it := range items {
		id, name := idName(it)
		if id == key || strings.HasPrefix(id, key) || strings.Contains(strings.ToLower(name), key) {
			hits = append(hits, it)
			names = append(names, name)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		var zero T
		return zero, fmt.Errorf("no match for %q", key)
	default:
		var zero T
		return zero, fmt.Errorf("%q matches multiple: %s", key, strings.Join(names, ", "))
	}
}

func completeNames[T any](items []T, err error, name func(T) string) ([]string, cobra.ShellCompDirective) {
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = name(it)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
