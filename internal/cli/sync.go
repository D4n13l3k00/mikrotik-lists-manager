package cli

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

var syncFlags connFlags
var syncDryRun bool
var syncFormat string
var syncVerbose bool
var syncConcurrency int
var syncWatch bool
var syncWatchInterval int

var syncCmd = &cobra.Command{
	Use:   "sync [file]",
	Short: "Синхронизировать address-list из файла в MikroTik",
	Long: `Читает файл (или stdin если '-'), вычисляет diff с текущим состоянием
address-list на MikroTik и применяет изменения.

При нескольких списках один и тот же файл синхронизируется в каждый список.

Примеры:
  mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes
  mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l vpn-routes -n
  mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l list1,list2
  mikrotik-lists-manager sync vpn.list -H 192.168.1.1 -u admin -l list1 -l list2`,
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

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(syncFlags.skipTLSVerify))
	ctx := cmd.Context()

	if info, err := client.GetRouterInfo(ctx); err == nil {
		output.RouterBanner(routerBannerInfo(info, host))
	}

	doSync := func() error {
		content, err := readFileOrStdin(args[0])
		if err != nil {
			return fmt.Errorf("чтение файла: %w", err)
		}
		entries, err := parseContent(content, effectiveFormat)
		if err != nil {
			return err
		}
		g := new(errgroup.Group)
		for _, listName := range listNames {
			g.Go(func() error {
				output.Header(fmt.Sprintf("Синхронизация %q на %s", listName, host))
				current, err := client.GetList(ctx, listName)
				if err != nil {
					return fmt.Errorf("получение списка %q: %w", listName, err)
				}
				output.Info(fmt.Sprintf("На роутере: %d записей, в файле: %d записей", len(current), len(entries)))
				changes, duplicates := syncer.Diff(entries, current)
				for _, addr := range duplicates {
					output.Warn(fmt.Sprintf("дубль в файле: %s (используется последнее вхождение)", addr))
				}
				if len(changes) == 0 {
					output.Info("Уже синхронизировано.")
					return nil
				}
				output.Header("Изменения")
				if syncDryRun {
					output.Info("(dry run — изменения не будут применены)")
				}
				return syncer.Apply(ctx, client, listName, changes, syncDryRun, syncVerbose, syncConcurrency)
			})
		}
		return g.Wait()
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
