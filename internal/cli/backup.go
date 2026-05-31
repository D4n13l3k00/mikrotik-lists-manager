package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var backupFlags connFlags
var backupOutputDir string
var backupFormat string

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Сохранить все address-list с роутера в папку (один файл на список)",
	Long: `Получает все статические address-list с роутера и сохраняет каждый в отдельный файл.

Форматы: native (по умолчанию) или mikrotik (.rsc).

Примеры:
  mikrotik-lists-manager backup -H 192.168.1.1 -u admin -o ./backup
  mikrotik-lists-manager backup -H 192.168.1.1 -u admin -o ./backup -f mikrotik`,
	RunE: runBackup,
}

func init() {
	backupCmd.Flags().StringVarP(&backupFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	backupCmd.Flags().StringVarP(&backupFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	backupCmd.Flags().StringVarP(&backupFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	backupCmd.Flags().BoolVarP(&backupFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	backupCmd.Flags().StringVarP(&backupOutputDir, "output", "o", ".", "Папка для сохранения файлов")
	backupCmd.Flags().StringVarP(&backupFormat, "format", "f", "native", "Формат: native или mikrotik")
}

func runBackup(cmd *cobra.Command, args []string) error {
	host := resolve(backupFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(backupFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	pass, err := resolvePassword(backupFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(backupFlags.skipTLSVerify))
	ctx := cmd.Context()

	entries, err := client.GetAllEntries(ctx)
	if err != nil {
		return fmt.Errorf("получение записей: %w", err)
	}

	// group by list name
	order := []string{}
	groups := map[string][]mikrotik.AddressListEntry{}
	for _, e := range entries {
		if _, exists := groups[e.List]; !exists {
			order = append(order, e.List)
		}
		groups[e.List] = append(groups[e.List], e)
	}

	if len(order) == 0 {
		output.Info("Списков не найдено.")
		return nil
	}

	if err := os.MkdirAll(backupOutputDir, 0o755); err != nil {
		return fmt.Errorf("создание папки %s: %w", backupOutputDir, err)
	}

	ext := ".lst"
	if backupFormat == "mikrotik" {
		ext = ".rsc"
	}

	output.Header(fmt.Sprintf("Резервное копирование с %s → %s", host, backupOutputDir))

	saved := 0
	for _, listName := range order {
		listEntries := groups[listName]
		var sb strings.Builder

		switch backupFormat {
		case "mikrotik":
			sb.WriteString("/ip firewall address-list\n")
			for _, e := range listEntries {
				line := fmt.Sprintf("add list=%s address=%s", listName, e.Address)
				if e.Comment != "" {
					line += fmt.Sprintf(" comment=%q", e.Comment)
				}
				if e.Disabled.Bool() {
					line += " disabled=yes"
				}
				sb.WriteString(line + "\n")
			}
		default:
			for _, e := range listEntries {
				prefix := ""
				if e.Disabled.Bool() {
					prefix = "!"
				}
				if e.Comment != "" {
					sb.WriteString(fmt.Sprintf("%s%s  ## %s\n", prefix, e.Address, e.Comment))
				} else {
					sb.WriteString(prefix + e.Address + "\n")
				}
			}
		}

		safeName := strings.ReplaceAll(listName, "/", "_")
		safeName = strings.ReplaceAll(safeName, "\\", "_")
		outPath := filepath.Join(backupOutputDir, safeName+ext)

		if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("запись %s: %w", outPath, err)
		}
		output.Info(fmt.Sprintf("%-32s → %s (%d записей)", listName, outPath, len(listEntries)))
		saved++
	}

	fmt.Println()
	output.Info(fmt.Sprintf("Сохранено %d списков в %s", saved, backupOutputDir))
	return nil
}
