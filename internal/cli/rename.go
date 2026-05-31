package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var renameFlags connFlags

var renameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Переименовать address-list на роутере",
	Long: `Переименовывает address-list на роутере, обновляя поле list у всех его записей.

Примеры:
  mikrotik-lists-manager rename vpn-old vpn-routes -H 192.168.1.1 -u admin`,
	Args: cobra.ExactArgs(2),
	RunE: runRename,
}

func init() {
	renameCmd.Flags().StringVarP(&renameFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	renameCmd.Flags().StringVarP(&renameFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	renameCmd.Flags().StringVarP(&renameFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	renameCmd.Flags().BoolVarP(&renameFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName, newName := args[0], args[1]

	host := resolve(renameFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(renameFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	pass, err := resolvePassword(renameFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(renameFlags.skipTLSVerify))
	ctx := cmd.Context()

	output.Info(fmt.Sprintf("Переименование %q → %q на %s...", oldName, newName, host))

	n, err := client.RenameList(ctx, oldName, newName)
	if err != nil {
		return fmt.Errorf("переименование: %w", err)
	}

	output.Info(fmt.Sprintf("Готово: обновлено %d записей.", n))
	return nil
}
