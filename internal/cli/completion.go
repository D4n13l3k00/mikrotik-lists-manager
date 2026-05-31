package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Вывести скрипт автодополнения для оболочки",
	Long: `Генерирует скрипт автодополнения для указанной оболочки.

Bash:
  mikrotik-lists-manager completion bash > /etc/bash_completion.d/mikrotik-lists-manager
  # или для текущего пользователя:
  mikrotik-lists-manager completion bash >> ~/.bash_completion

Zsh:
  mikrotik-lists-manager completion zsh > "${fpath[1]}/_mikrotik-lists-manager"

Fish:
  mikrotik-lists-manager completion fish > ~/.config/fish/completions/mikrotik-lists-manager.fish

PowerShell:
  mikrotik-lists-manager completion powershell >> $PROFILE`,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return root.GenZshCompletion(os.Stdout)
		case "fish":
			return root.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("неизвестная оболочка %q (поддерживаются: bash, zsh, fish, powershell)", args[0])
		}
	},
}
