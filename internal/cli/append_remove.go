package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/dnsresolver"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/source"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

const appendRemoveProgressThreshold = 10

// ── append ────────────────────────────────────────────────────────────────────

var appendFlags connFlags
var appendDryRun bool
var appendFormat string
var appendConcurrency int
var appendResolveDomains bool
var appendDNS string
var appendFast bool
var appendBatch bool
var appendBatchSize int

var appendCmd = &cobra.Command{
	Use:   "append [file|url]",
	Short: "Добавить записи из файла или URL в список на роутере, пропустив дубли",
	Long: `Читает файл или URL, получает текущий список с роутера и добавляет только те записи,
которых ещё нет. Существующие записи не трогает.

Примеры:
  mlm append extra.list -H 192.168.1.1 -u admin -l vpn-routes
  mlm append extra.list -H 192.168.1.1 -u admin -l vpn-routes --fast
  mlm append https://example.com/ips.txt -H 192.168.1.1 -u admin -l vpn-routes
  mlm append domains.lst -H 192.168.1.1 -u admin -l vpn-routes --resolve-domains`,
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
	appendCmd.Flags().BoolVar(&appendResolveDomains, "resolve-domains", false, "Разрешать доменные имена в IP-адреса через DNS")
	appendCmd.Flags().StringVar(&appendDNS, "dns", "", "Пользовательский DNS-сервер для резолвинга (например: 1.1.1.1:53)")
	appendCmd.Flags().BoolVar(&appendFast, "fast", false, "Турбо-режим: добавление записей пакетами через RouterOS скрипты")
	appendCmd.Flags().BoolVar(&appendBatch, "batch", false, "Синоним --fast: добавление записей пакетами")
	appendCmd.Flags().IntVar(&appendBatchSize, "batch-size", syncer.DefaultBatchSize, "Размер пакета записей для --fast/--batch (по умолчанию 250)")
}

func runAppend(cmd *cobra.Command, args []string) error {
	defer func() {
		appendFlags = connFlags{}
		appendDryRun = false
		appendFormat = ""
		appendConcurrency = 5
		appendResolveDomains = false
		appendDNS = ""
		appendFast = false
		appendBatch = false
		appendBatchSize = 0
	}()

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

	ctx := cmd.Context()
	proxyURL := resolveProxy(proxyFlag)

	content, err := readFileOrStdin(ctx, args[0], proxyURL)
	if err != nil {
		return err
	}

	entries, err := parseContent(content, appendFormat)
	if err != nil {
		return err
	}

	if appendResolveDomains {
		res, _ := dnsresolver.ResolveEntries(ctx, entries, appendDNS, proxyURL)
		for _, w := range res.Warnings {
			output.Warn(w)
		}
		entries = res.Entries
	}

	client := newClient(host, user, pass, resolveSkipTLS(appendFlags.skipTLSVerify))
	if info, err := client.GetRouterInfo(ctx); err == nil {
		output.RouterBanner(routerBannerInfo(info, host))
	}

	for _, listName := range listNames {
		current, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		existing := make(map[string]bool, len(current))
		for _, e := range current {
			existing[parser.NormalizeAddr(e.Address)] = true
		}

		seenToAdd := make(map[string]bool)
		var toAdd []parser.Entry
		for _, e := range entries {
			key := parser.NormalizeAddr(e.Address)
			if !existing[key] && !seenToAdd[key] {
				seenToAdd[key] = true
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

		if appendFast || appendBatch {
			changes := make([]syncer.Change, len(toAdd))
			for i, e := range toAdd {
				changes[i] = syncer.Change{
					Action:      syncer.ActionAdd,
					Address:     e.Address,
					NewComment:  e.Comment,
					NewDisabled: e.Disabled,
				}
			}
			bs := appendBatchSize
			if bs <= 0 {
				bs = syncer.DefaultBatchSize
			}
			if err := syncer.ApplyBatch(ctx, client, listName, changes, appendDryRun, false, bs); err != nil {
				return err
			}
			if skipped > 0 {
				output.Info(fmt.Sprintf("%d записей уже существовало.", skipped))
			}
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
			if gctx.Err() != nil {
				break
			}
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
var removeFast bool
var removeBatch bool
var removeBatchSize int

var removeCmd = &cobra.Command{
	Use:   "remove [file]",
	Short: "Удалить с роутера записи из файла, остальные не трогать",
	Long: `Читает файл, получает текущий список с роутера и удаляет только те записи,
которые есть в файле. Записи которых нет в файле — не трогает.

Примеры:
  mlm remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes
  mlm remove telegram.list -H 192.168.1.1 -u admin -l vpn-routes --fast
  mlm remove telegram.list -H 192.168.1.1 -u admin -l list1,list2 -n`,
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
	removeCmd.Flags().BoolVar(&removeFast, "fast", false, "Турбо-режим: удаление записей пакетами через RouterOS скрипты")
	removeCmd.Flags().BoolVar(&removeBatch, "batch", false, "Синоним --fast: удаление записей пакетами")
	removeCmd.Flags().IntVar(&removeBatchSize, "batch-size", syncer.DefaultBatchSize, "Размер пакета записей для --fast/--batch (по умолчанию 250)")
}

func runRemove(cmd *cobra.Command, args []string) error {
	defer func() {
		removeFlags = connFlags{}
		removeDryRun = false
		removeFormat = ""
		removeConcurrency = 5
		removeFast = false
		removeBatch = false
		removeBatchSize = 0
	}()

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

	ctx := cmd.Context()
	proxyURL := resolveProxy(proxyFlag)

	content, err := readFileOrStdin(ctx, args[0], proxyURL)
	if err != nil {
		return err
	}

	entries, err := parseContent(content, removeFormat)
	if err != nil {
		return err
	}

	toRemove := make(map[string]bool, len(entries))
	notFoundOrig := make(map[string]string, len(entries))
	for _, e := range entries {
		key := parser.NormalizeAddr(e.Address)
		toRemove[key] = true
		notFoundOrig[key] = e.Address
	}

	client := newClient(host, user, pass, resolveSkipTLS(removeFlags.skipTLSVerify))
	if info, err := client.GetRouterInfo(ctx); err == nil {
		output.RouterBanner(routerBannerInfo(info, host))
	}

	for _, listName := range listNames {
		current, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		output.Header(fmt.Sprintf("Удаление из %q на %s", listName, host))
		if removeDryRun {
			output.Info("(dry run — изменения не будут применены)")
		}

		notFoundSet := make(map[string]string, len(notFoundOrig))
		for k, v := range notFoundOrig {
			notFoundSet[k] = v
		}

		var toDelete []mikrotik.AddressListEntry
		for _, e := range current {
			key := parser.NormalizeAddr(e.Address)
			if toRemove[key] {
				toDelete = append(toDelete, e)
				delete(notFoundSet, key)
			}
		}

		for _, origAddr := range notFoundSet {
			output.Warn(fmt.Sprintf("%s не найден в списке на роутере", origAddr))
		}

		if len(toDelete) == 0 {
			fmt.Println()
			output.Info("Нечего удалять.")
			continue
		}

		if removeFast || removeBatch {
			changes := make([]syncer.Change, len(toDelete))
			for i, e := range toDelete {
				changes[i] = syncer.Change{
					Action:     syncer.ActionDelete,
					Address:    e.Address,
					OldComment: e.Comment,
					ID:         e.ID,
				}
			}
			bs := removeBatchSize
			if bs <= 0 {
				bs = syncer.DefaultBatchSize
			}
			if err := syncer.ApplyBatch(ctx, client, listName, changes, removeDryRun, false, bs); err != nil {
				return err
			}
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
			if gctx.Err() != nil {
				break
			}
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

func readFileOrStdin(ctx context.Context, path string, proxyURL string) ([]byte, error) {
	return source.ReadWithProxy(ctx, path, proxyURL)
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
