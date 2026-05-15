package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

var syncFlags connFlags
var syncDryRun bool
var syncFormat string

var syncCmd = &cobra.Command{
	Use:   "sync [file]",
	Short: "Синхронизировать address-list из файла в MikroTik",
	Long: `Читает файл (или stdin если '-'), вычисляет diff с текущим состоянием
address-list на MikroTik и применяет изменения.

Примеры:
  mikrotik-lists-manager sync vpn.list --host 192.168.1.1 --user admin --list vpn-routes
  mikrotik-lists-manager sync vpn.list --host 192.168.1.1 --user admin --list vpn-routes --dry-run
  cat vpn.list | mikrotik-lists-manager sync - --host 192.168.1.1 --user admin --list vpn-routes`,
	Args: cobra.ExactArgs(1),
	RunE: runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncFlags.host, "host", "H", "", "Адрес MikroTik (host или host:port) [$MT_HOST]")
	syncCmd.Flags().StringVarP(&syncFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	syncCmd.Flags().StringVarP(&syncFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS] (запросит интерактивно если не задан)")
	syncCmd.Flags().StringVarP(&syncFlags.listName, "list", "l", "", "Имя address-list [$MT_LIST]")
	syncCmd.Flags().BoolVarP(&syncFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "Показать изменения без применения")
	syncCmd.Flags().StringVarP(&syncFormat, "format", "f", "auto", "Формат входного файла: auto, native, mikrotik")
}

func runSync(cmd *cobra.Command, args []string) error {
	host := resolve(syncFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(syncFlags.user, "MT_USER", loadedConfig.User)
	listName := resolve(syncFlags.listName, "MT_LIST", loadedConfig.List)

	if host == "" {
		return fmt.Errorf("--host is required (or set MT_HOST / host in config)")
	}
	if user == "" {
		return fmt.Errorf("--user is required (or set MT_USER / user in config)")
	}
	if listName == "" {
		return fmt.Errorf("--list is required (or set MT_LIST / list in config)")
	}

	pass, err := resolvePassword(syncFlags.pass)
	if err != nil {
		return err
	}

	effectiveFormat := syncFormat
	if effectiveFormat == "auto" && loadedConfig.DefaultFormat != "" {
		effectiveFormat = loadedConfig.DefaultFormat
	}

	filePath := args[0]
	var content []byte
	content, err = readFileOrStdin(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	entries, err := parseContent(content, effectiveFormat)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(syncFlags.skipTLSVerify))
	output.Header(fmt.Sprintf("Получение списка %q с %s", listName, host))

	current, err := client.GetList(listName)
	if err != nil {
		return fmt.Errorf("fetching list: %w", err)
	}
	output.Info(fmt.Sprintf("На роутере: %d записей, в файле: %d записей", len(current), len(entries)))

	changes := syncer.Diff(entries, current)
	if len(changes) == 0 {
		output.Info("Уже синхронизировано.")
		return nil
	}

	output.Header("Изменения")
	if syncDryRun {
		output.Info("(dry run — изменения не будут применены)")
	}

	return syncer.Apply(client, listName, changes, syncDryRun)
}
