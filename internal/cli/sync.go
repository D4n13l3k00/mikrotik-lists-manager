package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/dnsresolver"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/snapshot"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/source"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

var syncFlags connFlags
var syncDryRun bool
var syncFormat string
var syncVerbose bool
var syncConcurrency int
var syncWatch bool
var syncWatchInterval int
var syncForce bool
var syncResolveDomains bool
var syncDNS string
var syncNoSnapshot bool

var syncCmd = &cobra.Command{
	Use:   "sync [file|url]",
	Short: "Синхронизировать address-list из файла или URL в MikroTik",
	Long: `Читает файл или URL (или stdin если '-'), вычисляет diff с текущим состоянием
address-list на MikroTik и применяет изменения.

Перед применением изменений автоматически создается локальный снимок (snapshot) для отката.

При нескольких списках один и тот же источник синхронизируется в каждый список последовательно.

Примеры:
  mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes
  mlm sync https://example.com/vpn.lst -H 192.168.1.1 -u admin -l vpn-routes
  mlm sync domains.txt -H 192.168.1.1 -u admin -l vpn-routes --resolve-domains
  mlm sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -n
  mlm sync vpn.list -H 192.168.1.1 -u admin -l list1,list2 -y`,
	Args: cobra.ExactArgs(1),
	RunE: runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncFlags.host, "host", "H", "", "Адрес MikroTik (host или host:port) [$MT_HOST]")
	syncCmd.Flags().StringVarP(&syncFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	syncCmd.Flags().StringVarP(&syncFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS] (запросит интерактивно если не задан)")
	syncCmd.Flags().StringArrayVarP(&syncFlags.listNames, "list", "l", nil, "Имя address-list, можно несколько: -l a,b или -l a -l b [$MT_LIST]")
	syncCmd.Flags().BoolVarP(&syncFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	syncCmd.Flags().BoolVarP(&syncDryRun, "dry-run", "n", false, "Показать изменения без применения")
	syncCmd.Flags().StringVarP(&syncFormat, "format", "f", "auto", "Формат входного файла: auto, native, mikrotik")
	syncCmd.Flags().BoolVarP(&syncVerbose, "verbose", "v", false, "Выводить каждую запись даже при прогресс-баре")
	syncCmd.Flags().IntVarP(&syncConcurrency, "concurrency", "c", 5, "Число параллельных запросов к API (0 = последовательно)")
	syncCmd.Flags().BoolVarP(&syncWatch, "watch", "w", false, "Следить за файлом и пересинхронизировать при изменении")
	syncCmd.Flags().IntVar(&syncWatchInterval, "watch-interval", 3, "Интервал проверки файла в секундах (с --watch)")
	syncCmd.Flags().BoolVarP(&syncForce, "force", "y", false, "Не запрашивать подтверждение при массовом удалении записей")
	syncCmd.Flags().BoolVar(&syncResolveDomains, "resolve-domains", false, "Разрешать доменные имена в IP-адреса через DNS")
	syncCmd.Flags().StringVar(&syncDNS, "dns", "", "Пользовательский DNS-сервер для резолвинга (например: 1.1.1.1:53)")
	syncCmd.Flags().BoolVar(&syncNoSnapshot, "no-snapshot", false, "Не создавать автоматический снимок перед применением изменений")
}

func runSync(cmd *cobra.Command, args []string) error {
	if syncWatch && args[0] == "-" {
		return fmt.Errorf("--watch несовместим с чтением из stdin")
	}

	host := resolve(syncFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(syncFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	listNames, err := resolveListNames(syncFlags.listNames, loadedConfig.List)
	if err != nil {
		return err
	}

	pass, err := resolvePassword(syncFlags.pass)
	if err != nil {
		return err
	}

	effectiveFormat := syncFormat
	if effectiveFormat == "auto" && loadedConfig.DefaultFormat != "" {
		effectiveFormat = loadedConfig.DefaultFormat
	}

	client := newClient(host, user, pass, resolveSkipTLS(syncFlags.skipTLSVerify))
	ctx := cmd.Context()
	proxyURL := resolveProxy(proxyFlag)

	if info, err := client.GetRouterInfo(ctx); err == nil {
		output.RouterBanner(routerBannerInfo(info, host))
	}

	doSync := func() error {
		content, err := source.ReadWithProxy(ctx, args[0], proxyURL)
		if err != nil {
			return fmt.Errorf("чтение источника: %w", err)
		}
		entries, err := parseContent(content, effectiveFormat)
		if err != nil {
			return err
		}

		if syncResolveDomains {
			res, _ := dnsresolver.ResolveEntries(ctx, entries, syncDNS, proxyURL)
			for _, w := range res.Warnings {
				output.Warn(w)
			}
			entries = res.Entries
		}

		for _, listName := range listNames {
			if err := ctx.Err(); err != nil {
				return err
			}
			output.Header(fmt.Sprintf("Синхронизация %q на %s", listName, host))
			current, err := client.GetList(ctx, listName)
			if err != nil {
				return fmt.Errorf("получение списка %q: %w", listName, err)
			}
			output.Info(fmt.Sprintf("На роутере: %d записей, в источнике: %d записей", len(current), len(entries)))
			changes, duplicates := syncer.Diff(entries, current)
			for _, addr := range duplicates {
				output.Warn(fmt.Sprintf("дубль в источнике: %s (используется последнее вхождение)", addr))
			}
			if len(changes) == 0 {
				output.Info("Уже синхронизировано.")
				continue
			}

			deleteCount := 0
			for _, ch := range changes {
				if ch.Action == syncer.ActionDelete {
					deleteCount++
				}
			}

			if !syncDryRun && !syncForce && deleteCount > 0 {
				isBulkDelete := deleteCount > 50 || (len(current) > 0 && float64(deleteCount)/float64(len(current)) > 0.50)
				if isBulkDelete {
					if term.IsTerminal(int(os.Stdin.Fd())) {
						var confirmed bool
						err := huh.NewConfirm().
							Title(fmt.Sprintf("Внимание: будет удалено %d записей из %d в списке %q. Продолжить?", deleteCount, len(current), listName)).
							Value(&confirmed).
							Run()
						if err != nil || !confirmed {
							return fmt.Errorf("синхронизация отменена пользователем")
						}
					} else {
						return fmt.Errorf("безопасность: попытка массового удаления %d записей из %d в списке %q. Используйте --force (-y)", deleteCount, len(current), listName)
					}
				}
			}

			if !syncDryRun && !syncNoSnapshot && len(changes) > 0 {
				snapMeta, sErr := snapshot.Save(host, listName, current)
				if sErr != nil {
					output.Warn(fmt.Sprintf("не удалось создать автоматический снимок: %v", sErr))
				} else {
					output.Info(fmt.Sprintf("Создан снимок для отката: %s (записей: %d)", snapMeta.ID, snapMeta.Total))
				}
			}

			output.Header("Изменения")
			if syncDryRun {
				output.Info("(dry run — изменения не будут применены)")
			}
			if err := syncer.Apply(ctx, client, listName, changes, syncDryRun, syncVerbose, syncConcurrency); err != nil {
				return err
			}
		}
		return nil
	}

	if err := doSync(); err != nil {
		return err
	}
	if !syncWatch {
		return nil
	}

	filePath := args[0]
	var lastMod time.Time
	if fi, err := os.Stat(filePath); err == nil {
		lastMod = fi.ModTime()
	}

	output.Info(fmt.Sprintf("Слежение за %s (каждые %ds)...", filePath, syncWatchInterval))
	ticker := time.NewTicker(time.Duration(syncWatchInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fi, err := os.Stat(filePath)
			if err != nil {
				continue
			}
			if fi.ModTime().After(lastMod) {
				lastMod = fi.ModTime()
				output.Info("Файл изменён, синхронизирую...")
				if err := doSync(); err != nil {
					output.Warn(fmt.Sprintf("Ошибка синхронизации: %v", err))
				}
			}
		}
	}
}
