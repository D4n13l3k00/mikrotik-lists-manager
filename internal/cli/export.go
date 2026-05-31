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

При нескольких списках и -o файл содержит все списки подряд.

Примеры:
  mikrotik-lists-manager export -H 192.168.1.1 -u admin -l vpn-routes
  mikrotik-lists-manager export -H 192.168.1.1 -u admin -l list1,list2 -f mikrotik -o backup.rsc`,
	RunE: runExport,
}

func init() {
	exportCmd.Flags().StringVarP(&exportFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	exportCmd.Flags().StringVarP(&exportFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	exportCmd.Flags().StringVarP(&exportFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	exportCmd.Flags().StringArrayVarP(&exportFlags.listNames, "list", "l", nil, "Имя address-list, можно несколько [$MT_LIST]")
	exportCmd.Flags().BoolVarP(&exportFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	exportCmd.Flags().StringVarP(&exportOutFormat, "format", "f", "native", "Формат вывода: native, mikrotik")
	exportCmd.Flags().StringVarP(&exportOutFile, "output", "o", "", "Записать в файл вместо stdout")
}

func runExport(cmd *cobra.Command, args []string) error {
	host := resolve(exportFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(exportFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	listNames, err := resolveListNames(exportFlags.listNames, loadedConfig.List)
	if err != nil {
		return err
	}

	pass, err := resolvePassword(exportFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(exportFlags.skipTLSVerify))
	ctx := cmd.Context()

	var sb strings.Builder
	totalEntries := 0

	for _, listName := range listNames {
		output.Header(fmt.Sprintf("Экспорт списка %q с %s", listName, host))

		entries, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}
		totalEntries += len(entries)

		switch exportOutFormat {
		case "mikrotik":
			if sb.Len() == 0 {
				sb.WriteString("/ip firewall address-list\n")
			}
			for _, e := range entries {
				line := fmt.Sprintf("add list=%s address=%s", listName, e.Address)
				if e.Comment != "" {
					line += fmt.Sprintf(" comment=%q", e.Comment)
				}
				sb.WriteString(line + "\n")
			}
		default:
			if len(listNames) > 1 {
				sb.WriteString(fmt.Sprintf("# ── %s ──\n", listName))
			}
			for _, e := range entries {
				if e.Comment != "" {
					sb.WriteString(fmt.Sprintf("%s  ## %s\n", e.Address, e.Comment))
				} else {
					sb.WriteString(e.Address + "\n")
				}
			}
		}
	}

	result := sb.String()

	if exportOutFile != "" {
		if err := os.WriteFile(exportOutFile, []byte(result), 0o644); err != nil {
			return fmt.Errorf("запись файла: %w", err)
		}
		output.Info(fmt.Sprintf("Записано %d записей в %s", totalEntries, exportOutFile))
	} else {
		fmt.Print(result)
	}
	return nil
}
