package plugin

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(kpdbug completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ kpdbug completion bash > /etc/bash_completion.d/kpdbug
  # macOS:
  $ kpdbug completion bash > /usr/local/etc/bash_completion.d/kpdbug

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ kpdbug completion zsh > "${fpath[1]}/_kpdbug"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ kpdbug completion fish | source

  # To load completions for each session, execute once:
  $ kpdbug completion fish > ~/.config/fish/completions/kpdbug.fish

PowerShell:

  PS> kpdbug completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> kpdbug completion powershell > kpdbug.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			_ = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			_ = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			_ = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)

	// Set up custom completions for flags
	setupCustomCompletions()
}

func setupCustomCompletions() {
	// Namespace completion
	_ = rootCmd.RegisterFlagCompletionFunc("namespace", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return getNamespaces(), cobra.ShellCompDirectiveNoFileComp
	})

	// Pod completion
	_ = rootCmd.RegisterFlagCompletionFunc("pod", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return getPods(), cobra.ShellCompDirectiveNoFileComp
	})

	// Profile completion
	_ = rootCmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"general", "restricted", "baseline", "privileged"}, cobra.ShellCompDirectiveNoFileComp
	})

	// Image completion (common debug images)
	_ = rootCmd.RegisterFlagCompletionFunc("image", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"debug:latest",
			"busybox:latest",
			"alpine:latest",
			"ubuntu:latest",
			"nicolaka/netshoot:latest",
		}, cobra.ShellCompDirectiveNoFileComp
	})
}

func getNamespaces() []string {
	cmd := ExecCommand("kubectl", "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return []string{"default"}
	}

	namespaces := strings.Fields(string(output))
	if len(namespaces) == 0 {
		return []string{"default"}
	}
	return namespaces
}

func getPods() []string {
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	cmd := ExecCommand("kubectl", "get", "pods", "-n", ns, "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	return strings.Fields(string(output))
}
