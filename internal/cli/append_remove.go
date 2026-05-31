package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

const appendRemoveProgressThreshold = 10

// ── append ────────────────────────────────────────────────────────────────────

var appendFlags connFlags
var appendDryRun bool
var appendFormat string
var appendConcurrency int

var appendCmd = &cobra.Command{
	Use:   "append [file]",
	Short: "Добавить записи из файла в список на роутере, пропустив дубли",
	Long: `Читает файл, получает текущий список с роутера и добавляет только те записи,
которых ещё нет. Существующие записи не трогает.

Примеры:
  mikrotik-lists-manager append extra.list -H 192.168.1.1 -u admin -l vpn-routes
  mikrotik-lists-manager append extra.list -H 192.168.1.1 -u admin -l list1,list2 -n`,
	Args: cobra.ExactArgs(1),
	RunE: runAppend,
}

func init() {
	appendCmd.Flags().StringVarP(&appendFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	appendCmd.Flags().StringVarP(&appendFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	appendCmd.Flags().StringVarP(&appendFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	appendCmd.Flags().StringArrayVarP(&appendFlags.listNames, "list", "l", nil, "Имя address-list, можно несколько [$MT_LIST]")
	appendCmd.Flags().BoolVarP(&appendFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	appendCmd.Flags().BoolVarP(&appendDryRun, "dry-run", "n", false, "Показать изменения без применения")
	appendCmd.Flags().StringVarP(&appendFormat, "format", "f", "auto", "Формат файла: auto, native, mikrotik")
	appendCmd.Flags().IntVarP(&appendConcurrency, "concurrency", "c", 5, "Число параллельных запросов к API (0 = последовательно)")
}

func runAppend(cmd *cobra.Command, args []string) error {
	host := resolve(appendFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(appendFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	listNames, err := resolveListNames(appendFlags.listNames, loadedConfig.List)
	if err != nil {
		return err
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
	ctx := cmd.Context()

	for _, listName := range listNames {
		current, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		existing := make(map[string]bool, len(current))
		for _, e := range current {
			existing[strings.ToLower(e.Address)] = true
		}

		var toAdd []parser.Entry
		for _, e := range entries {
			if !existing[strings.ToLower(e.Address)] {
				toAdd = append(toAdd, e)
			}
		}
		skipped := len(entries) - len(toAdd)

		output.Header(fmt.Sprintf("Добавление в %q на %s", listName, host))
		if appendDryRun {
			output.Info("(dry run — изменения не будут применены)")
		}

		if len(toAdd) == 0 {
			fmt.Println()
			output.Info(fmt.Sprintf("Все записи уже есть в списке (%d пропущено).", skipped))
			continue
		}

		useProgress := len(toAdd) >= appendRemoveProgressThreshold && !appendDryRun
		var bar *progressbar.ProgressBar
		if useProgress {
			bar = newProgressBar(len(toAdd), "Добавление...")
		}

		var mu sync.Mutex
		var added atomic.Int64

		g, gctx := errgroup.WithContext(ctx)
		if appendConcurrency > 0 {
			g.SetLimit(appendConcurrency)
		}

		for _, e := range toAdd {
			g.Go(func() error {
				if !useProgress {
					mu.Lock()
					output.Add(e.Address, e.Comment, e.Disabled)
					mu.Unlock()
				}
				if !appendDryRun {
					if err := client.AddEntry(gctx, listName, e.Address, e.Comment, e.Disabled); err != nil {
						return fmt.Errorf("добавление %s: %w", e.Address, err)
					}
				}
				added.Add(1)
				if useProgress {
					bar.Add(1) //nolint:errcheck
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			if useProgress {
				fmt.Fprintln(os.Stderr)
			}
			return err
		}
		if useProgress {
			fmt.Fprintln(os.Stderr)
		}

		fmt.Println()
		msg := fmt.Sprintf("+%d добавлено", added.Load())
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
var removeConcurrency int

var removeCmd = &cobra.Command{
	Use:   "remove [file]",
	Short: "Удалить с роутера записи из файла, остальные не трогать",
	Long: `Читает файл, получает текущий список с роутера и удаляет только те записи,
которые есть в файле. Записи которых нет в файле — не трогает.

Примеры:
  mikrotik-lists-manager remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes
  mikrotik-lists-manager remove telegram.list -H 192.168.1.1 -u admin -l list1,list2 -n`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().StringVarP(&removeFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	removeCmd.Flags().StringVarP(&removeFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	removeCmd.Flags().StringVarP(&removeFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	removeCmd.Flags().StringArrayVarP(&removeFlags.listNames, "list", "l", nil, "Имя address-list, можно несколько [$MT_LIST]")
	removeCmd.Flags().BoolVarP(&removeFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	removeCmd.Flags().BoolVarP(&removeDryRun, "dry-run", "n", false, "Показать изменения без применения")
	removeCmd.Flags().StringVarP(&removeFormat, "format", "f", "auto", "Формат файла: auto, native, mikrotik")
	removeCmd.Flags().IntVarP(&removeConcurrency, "concurrency", "c", 5, "Число параллельных запросов к API (0 = последовательно)")
}

func runRemove(cmd *cobra.Command, args []string) error {
	host := resolve(removeFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(removeFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	listNames, err := resolveListNames(removeFlags.listNames, loadedConfig.List)
	if err != nil {
		return err
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

	toRemove := make(map[string]bool, len(entries))
	for _, e := range entries {
		toRemove[strings.ToLower(e.Address)] = true
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(removeFlags.skipTLSVerify))
	ctx := cmd.Context()

	for _, listName := range listNames {
		current, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		output.Header(fmt.Sprintf("Удаление из %q на %s", listName, host))
		if removeDryRun {
			output.Info("(dry run — изменения не будут применены)")
		}

		notFoundSet := make(map[string]bool, len(toRemove))
		for k := range toRemove {
			notFoundSet[k] = true
		}

		var toDelete []mikrotik.AddressListEntry
		for _, e := range current {
			if toRemove[strings.ToLower(e.Address)] {
				toDelete = append(toDelete, e)
				delete(notFoundSet, strings.ToLower(e.Address))
			}
		}

		for addr := range notFoundSet {
			output.Warn(fmt.Sprintf("%s не найден в списке на роутере", addr))
		}

		if len(toDelete) == 0 {
			fmt.Println()
			output.Info("Нечего удалять.")
			continue
		}

		useProgress := len(toDelete) >= appendRemoveProgressThreshold && !removeDryRun
		var bar *progressbar.ProgressBar
		if useProgress {
			bar = newProgressBar(len(toDelete), "Удаление...")
		}

		var mu sync.Mutex
		var removed atomic.Int64

		g, gctx := errgroup.WithContext(ctx)
		if removeConcurrency > 0 {
			g.SetLimit(removeConcurrency)
		}

		for _, e := range toDelete {
			g.Go(func() error {
				if !useProgress {
					mu.Lock()
					output.Remove(e.Address, e.Comment)
					mu.Unlock()
				}
				if !removeDryRun {
					if err := client.DeleteEntry(gctx, e.ID); err != nil {
						return fmt.Errorf("удаление %s: %w", e.Address, err)
					}
				}
				removed.Add(1)
				if useProgress {
					bar.Add(1) //nolint:errcheck
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			if useProgress {
				fmt.Fprintln(os.Stderr)
			}
			return err
		}
		if useProgress {
			fmt.Fprintln(os.Stderr)
		}

		fmt.Println()
		msg := fmt.Sprintf("−%d удалено", removed.Load())
		if len(notFoundSet) > 0 {
			msg += fmt.Sprintf(", %d не найдено на роутере", len(notFoundSet))
		}
		if removeDryRun {
			msg += " (dry run)"
		}
		output.Info(msg)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newProgressBar(total int, desc string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionSetDescription("[cyan]"+desc+"[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func readFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		return readStdin()
	}
	return os.ReadFile(path)
}

func readStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
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
