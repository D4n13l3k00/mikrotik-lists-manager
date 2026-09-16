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
  mlm completion bash > /etc/bash_completion.d/mlm
  # или для текущего пользователя:
  mlm completion bash >> ~/.bash_completion

Zsh:
  mlm completion zsh > "${fpath[1]}/_mlm"

Fish:
  mlm completion fish > ~/.config/fish/completions/mlm.fish

PowerShell:
  mlm completion powershell >> $PROFILE`,
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
