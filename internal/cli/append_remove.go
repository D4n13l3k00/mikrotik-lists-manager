package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

// ── append ────────────────────────────────────────────────────────────────────

var appendFlags connFlags
var appendDryRun bool
var appendFormat string

var appendCmd = &cobra.Command{
	Use:   "append [file]",
	Short: "Добавить записи из файла в список на роутере, пропустив дубли",
	Long: `Читает файл, получает текущий список с роутера и добавляет только те записи,
которых ещё нет. Существующие записи не трогает.

Примеры:
  mikrotik-lists-manager append extra.list -H 192.168.1.1 -u admin -l vpn-routes
  mikrotik-lists-manager append extra.list -H 192.168.1.1 -u admin -l vpn-routes -n`,
	Args: cobra.ExactArgs(1),
	RunE: runAppend,
}

func init() {
	appendCmd.Flags().StringVarP(&appendFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	appendCmd.Flags().StringVarP(&appendFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	appendCmd.Flags().StringVarP(&appendFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	appendCmd.Flags().StringVarP(&appendFlags.listName, "list", "l", "", "Имя address-list [$MT_LIST]")
	appendCmd.Flags().BoolVarP(&appendFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	appendCmd.Flags().BoolVarP(&appendDryRun, "dry-run", "n", false, "Показать изменения без применения")
	appendCmd.Flags().StringVarP(&appendFormat, "format", "f", "auto", "Формат файла: auto, native, mikrotik")
}

func runAppend(cmd *cobra.Command, args []string) error {
	host := resolve(appendFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(appendFlags.user, "MT_USER", loadedConfig.User)
	listName := resolve(appendFlags.listName, "MT_LIST", loadedConfig.List)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}
	if listName == "" {
		return fmt.Errorf("--list обязателен")
	}

	pass, err := resolvePassword(appendFlags.pass)
	if err != nil {
		return err
	}

	content, err := readFileOrStdin(args[0])
	if err != nil {
		return err
	}

	entries, err := parseContent(content, appendFormat)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(appendFlags.skipTLSVerify))

	current, err := client.GetList(listName)
	if err != nil {
		return fmt.Errorf("получение списка: %w", err)
	}

	// build set of existing addresses
	existing := make(map[string]bool, len(current))
	for _, e := range current {
		existing[strings.ToLower(e.Address)] = true
	}

	output.Header(fmt.Sprintf("Добавление в %q на %s", listName, host))
	if appendDryRun {
		output.Info("(dry run — изменения не будут применены)")
	}

	added := 0
	skipped := 0
	for _, e := range entries {
		if existing[strings.ToLower(e.Address)] {
			skipped++
			continue
		}
		output.Add(e.Address, e.Comment, e.Disabled)
		if !appendDryRun {
			if err := client.AddEntry(listName, e.Address, e.Comment, e.Disabled); err != nil {
				return fmt.Errorf("добавление %s: %w", e.Address, err)
			}
		}
		added++
	}

	fmt.Println()
	if added == 0 {
		output.Info(fmt.Sprintf("Все записи уже есть в списке (%d пропущено).", skipped))
	} else {
		msg := fmt.Sprintf("+%d добавлено", added)
		if skipped > 0 {
			msg += fmt.Sprintf(", %d уже существовало", skipped)
		}
		if appendDryRun {
			msg += " (dry run)"
		}
		output.Info(msg)
	}
	return nil
}

// ── remove ────────────────────────────────────────────────────────────────────

var removeFlags connFlags
var removeDryRun bool
var removeFormat string

var removeCmd = &cobra.Command{
	Use:   "remove [file]",
	Short: "Удалить с роутера записи из файла, остальные не трогать",
	Long: `Читает файл, получает текущий список с роутера и удаляет только те записи,
которые есть в файле. Записи которых нет в файле — не трогает.

Примеры:
  mikrotik-lists-manager remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes
  mikrotik-lists-manager remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes -n`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().StringVarP(&removeFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	removeCmd.Flags().StringVarP(&removeFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	removeCmd.Flags().StringVarP(&removeFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	removeCmd.Flags().StringVarP(&removeFlags.listName, "list", "l", "", "Имя address-list [$MT_LIST]")
	removeCmd.Flags().BoolVarP(&removeFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	removeCmd.Flags().BoolVarP(&removeDryRun, "dry-run", "n", false, "Показать изменения без применения")
	removeCmd.Flags().StringVarP(&removeFormat, "format", "f", "auto", "Формат файла: auto, native, mikrotik")
}

func runRemove(cmd *cobra.Command, args []string) error {
	host := resolve(removeFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(removeFlags.user, "MT_USER", loadedConfig.User)
	listName := resolve(removeFlags.listName, "MT_LIST", loadedConfig.List)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}
	if listName == "" {
		return fmt.Errorf("--list обязателен")
	}

	pass, err := resolvePassword(removeFlags.pass)
	if err != nil {
		return err
	}

	content, err := readFileOrStdin(args[0])
	if err != nil {
		return err
	}

	entries, err := parseContent(content, removeFormat)
	if err != nil {
		return err
	}

	// build set of addresses to remove
	toRemove := make(map[string]bool, len(entries))
	for _, e := range entries {
		toRemove[strings.ToLower(e.Address)] = true
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(removeFlags.skipTLSVerify))

	current, err := client.GetList(listName)
	if err != nil {
		return fmt.Errorf("получение списка: %w", err)
	}

	output.Header(fmt.Sprintf("Удаление из %q на %s", listName, host))
	if removeDryRun {
		output.Info("(dry run — изменения не будут применены)")
	}

	removed := 0
	notFound := 0
	for _, e := range current {
		if !toRemove[strings.ToLower(e.Address)] {
			continue
		}
		output.Remove(e.Address, e.Comment)
		if !removeDryRun {
			if err := client.DeleteEntry(e.ID); err != nil {
				return fmt.Errorf("удаление %s: %w", e.Address, err)
			}
		}
		removed++
		delete(toRemove, strings.ToLower(e.Address))
	}

	// warn about addresses from file that weren't found on router
	for addr := range toRemove {
		output.Warn(fmt.Sprintf("%s не найден в списке на роутере", addr))
		notFound++
	}

	fmt.Println()
	if removed == 0 && notFound == 0 {
		output.Info("Нечего удалять.")
	} else {
		msg := fmt.Sprintf("−%d удалено", removed)
		if notFound > 0 {
			msg += fmt.Sprintf(", %d не найдено на роутере", notFound)
		}
		if removeDryRun {
			msg += " (dry run)"
		}
		output.Info(msg)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func readFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		return os.ReadFile("/dev/stdin")
	}
	return os.ReadFile(path)
}

func parseContent(content []byte, format string) ([]parser.Entry, error) {
	s := string(content)
	if format == "auto" {
		format = parser.DetectFormat(s)
	}
	r := strings.NewReader(s)
	switch format {
	case "mikrotik":
		return parser.ParseMikrotik(r)
	default:
		return parser.ParseNative(r)
	}
}
