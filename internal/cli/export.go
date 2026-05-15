package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var exportFlags connFlags
var exportOutFormat string
var exportOutFile string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Экспортировать address-list из MikroTik в файл или stdout",
	Long: `Получает текущий address-list из MikroTik и выводит его.

Форматы вывода:
  native    — удобочитаемый .list формат (по умолчанию)
  mikrotik  — команды в стиле /export MikroTik

Примеры:
  mikrotik-lists-manager export --host 192.168.1.1 --user admin --list vpn-routes
  mikrotik-lists-manager export --host 192.168.1.1 --user admin --list vpn-routes --format mikrotik -o backup.rsc`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVarP(&exportFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	exportCmd.Flags().StringVarP(&exportFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	exportCmd.Flags().StringVarP(&exportFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	exportCmd.Flags().StringVarP(&exportFlags.listName, "list", "l", "", "Имя address-list [$MT_LIST]")
	exportCmd.Flags().BoolVarP(&exportFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	exportCmd.Flags().StringVarP(&exportOutFormat, "format", "f", "native", "Формат вывода: native, mikrotik")
	exportCmd.Flags().StringVarP(&exportOutFile, "output", "o", "", "Записать в файл вместо stdout")
}

func runExport(cmd *cobra.Command, args []string) error {
	host := resolve(exportFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(exportFlags.user, "MT_USER", loadedConfig.User)
	listName := resolve(exportFlags.listName, "MT_LIST", loadedConfig.List)

	if host == "" {
		return fmt.Errorf("--host is required (or set MT_HOST / host in config)")
	}
	if user == "" {
		return fmt.Errorf("--user is required (or set MT_USER / user in config)")
	}
	if listName == "" {
		return fmt.Errorf("--list is required (or set MT_LIST / list in config)")
	}

	pass, err := resolvePassword(exportFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(exportFlags.skipTLSVerify))
	output.Header(fmt.Sprintf("Экспорт списка %q с %s", listName, host))

	entries, err := client.GetList(listName)
	if err != nil {
		return fmt.Errorf("fetching list: %w", err)
	}

	var sb strings.Builder
	switch exportOutFormat {
	case "mikrotik":
		sb.WriteString("/ip firewall address-list\n")
		for _, e := range entries {
			line := fmt.Sprintf("add list=%s address=%s", listName, e.Address)
			if e.Comment != "" {
				line += fmt.Sprintf(" comment=%q", e.Comment)
			}
			sb.WriteString(line + "\n")
		}
	default:
		for _, e := range entries {
			if e.Comment != "" {
				sb.WriteString(fmt.Sprintf("%s  ## %s\n", e.Address, e.Comment))
			} else {
				sb.WriteString(e.Address + "\n")
			}
		}
	}

	result := sb.String()

	if exportOutFile != "" {
		if err := os.WriteFile(exportOutFile, []byte(result), 0o644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		output.Info(fmt.Sprintf("Записано %d записей в %s", len(entries), exportOutFile))
	} else {
		fmt.Print(result)
	}
	return nil
}
