package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/snapshot"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

var (
	purgeFlags       connFlags
	purgeAll         bool
	purgeDryRun      bool
	purgeForce       bool
	purgeVerbose     bool
	purgeConcurrency int
	purgeNoSnapshot  bool
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Очистить (удалить все записи) address-list на MikroTik",
	Long: `Удаляет ВСЕ записи из указанного address-list на роутере MikroTik.

Для предотвращения случайной потери данных:
  1. Создаётся автоматический снимок перед удалением (можно отменить через mlm rollback)
  2. Запрашивается подтверждение перед выполнением (пропускается с --force / -y)
  3. Поддерживается режим симуляции (--dry-run / -n)

Примеры:
  mlm purge -H 192.168.88.1 -u admin -l vpn-routes              # очистить список с подтверждением
  mlm purge -H 192.168.88.1 -u admin -l vpn-routes -y           # очистить список без подтверждения
  mlm purge -H 192.168.88.1 -u admin -l vpn-routes -n           # симуляция (dry-run)
  mlm purge -H 192.168.88.1 -u admin -l list1,list2 -y          # очистить несколько списков
  mlm purge -H 192.168.88.1 -u admin --all -y                  # очистить ВСЕ списки на роутере`,
	RunE: runPurge,
}

func init() {
	purgeCmd.Flags().StringVarP(&purgeFlags.host, "host", "H", "", "Адрес MikroTik (host или host:port) [$MT_HOST]")
	purgeCmd.Flags().StringVarP(&purgeFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	purgeCmd.Flags().StringVarP(&purgeFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	purgeCmd.Flags().StringArrayVarP(&purgeFlags.listNames, "list", "l", nil, "Имя address-list, можно несколько [$MT_LIST]")
	purgeCmd.Flags().BoolVar(&purgeAll, "all", false, "Очистить ВСЕ списки адресов на роутере")
	purgeCmd.Flags().BoolVarP(&purgeFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	purgeCmd.Flags().BoolVarP(&purgeDryRun, "dry-run", "n", false, "Показать изменения без применения")
	purgeCmd.Flags().BoolVarP(&purgeForce, "force", "y", false, "Не запрашивать подтверждение перед очисткой")
	purgeCmd.Flags().BoolVarP(&purgeVerbose, "verbose", "v", false, "Выводить каждую запись даже при прогресс-баре")
	purgeCmd.Flags().IntVarP(&purgeConcurrency, "concurrency", "c", 5, "Число параллельных запросов к API (0 = последовательно)")
	purgeCmd.Flags().BoolVar(&purgeNoSnapshot, "no-snapshot", false, "Не создавать автоматический снимок перед очисткой")
}

func runPurge(cmd *cobra.Command, args []string) error {
	defer func() {
		purgeFlags = connFlags{}
		purgeAll = false
		purgeDryRun = false
		purgeForce = false
		purgeVerbose = false
		purgeConcurrency = 5
		purgeNoSnapshot = false
	}()

	host := resolve(purgeFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(purgeFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	pass, err := resolvePassword(purgeFlags.pass)
	if err != nil {
		return err
	}

	client := newClient(host, user, pass, resolveSkipTLS(purgeFlags.skipTLSVerify))
	ctx := cmd.Context()

	if info, err := client.GetRouterInfo(ctx); err == nil {
		output.RouterBanner(routerBannerInfo(info, host))
	}

	var targetLists []string
	if purgeAll {
		entries, err := client.GetAllEntries(ctx)
		if err != nil {
			return fmt.Errorf("получение списков с роутера: %w", err)
		}
		listMap := make(map[string]bool)
		for _, e := range entries {
			if !listMap[e.List] {
				listMap[e.List] = true
				targetLists = append(targetLists, e.List)
			}
		}
		sort.Strings(targetLists)
		if len(targetLists) == 0 {
			output.Info("Списков на роутере не найдено.")
			return nil
		}

		if !purgeDryRun && !purgeForce {
			if term.IsTerminal(int(os.Stdin.Fd())) {
				var confirmed bool
				err := huh.NewConfirm().
					Title(fmt.Sprintf("ВНИМАНИЕ: Будет очищено ВСЕХ списков: %d (%s). Всего записей: %d. Продолжить?",
						len(targetLists), strings.Join(targetLists, ", "), len(entries))).
					Value(&confirmed).
					Run()
				if err != nil || !confirmed {
					return fmt.Errorf("очистка отменена пользователем")
				}
			} else {
				return fmt.Errorf("безопасность: очистка ВСЕХ списков (%d списков, %d записей) требует подтверждения. Используйте --force (-y)", len(targetLists), len(entries))
			}
		}
	} else {
		targetLists, err = resolveListNames(purgeFlags.listNames, loadedConfig.List)
		if err != nil {
			return err
		}
		if len(targetLists) == 0 {
			return fmt.Errorf("укажите имя списка через --list (-l) или используйте --all для очистки всех списков")
		}
	}

	for _, listName := range targetLists {
		if err := ctx.Err(); err != nil {
			return err
		}

		current, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		output.Header(fmt.Sprintf("Очистка списка %q на %s", listName, host))

		if len(current) == 0 {
			output.Info(fmt.Sprintf("Список %q уже пуст.", listName))
			continue
		}

		output.Info(fmt.Sprintf("На роутере: %d записей", len(current)))

		if !purgeAll && !purgeDryRun && !purgeForce {
			if term.IsTerminal(int(os.Stdin.Fd())) {
				var confirmed bool
				err := huh.NewConfirm().
					Title(fmt.Sprintf("Внимание: будут удалены ВСЕ записи (%d шт.) из списка %q на %s. Продолжить?", len(current), listName, host)).
					Value(&confirmed).
					Run()
				if err != nil || !confirmed {
					return fmt.Errorf("очистка списка %q отменена пользователем", listName)
				}
			} else {
				return fmt.Errorf("безопасность: очистка списка %q (%d записей) требует подтверждения. Используйте --force (-y)", listName, len(current))
			}
		}

		if !purgeDryRun && !purgeNoSnapshot {
			snapMeta, sErr := snapshot.Save(host, listName, current)
			if sErr != nil {
				output.Warn(fmt.Sprintf("не удалось создать автоматический снимок: %v", sErr))
			} else {
				output.Info(fmt.Sprintf("Создан снимок для отката: %s (записей: %d)", snapMeta.ID, snapMeta.Total))
			}
		}

		if purgeDryRun {
			output.Info("(dry run — изменения не будут применены)")
		}

		changes := make([]syncer.Change, len(current))
		for i, e := range current {
			changes[i] = syncer.Change{
				Action:     syncer.ActionDelete,
				Address:    e.Address,
				OldComment: e.Comment,
				ID:         e.ID,
			}
		}

		if !purgeDryRun {
			script := fmt.Sprintf("/ip firewall address-list remove [find where list=%q]", listName)
			if err := client.Execute(ctx, script); err == nil {
				output.Summary(0, len(current), 0, false)
				continue
			} else {
				output.Warn(fmt.Sprintf("Быстрая очистка через скрипт не удалась: %v. Переключение на стандартное удаление...", err))
			}
		}

		if err := syncer.Apply(ctx, client, listName, changes, purgeDryRun, purgeVerbose, purgeConcurrency); err != nil {
			return err
		}
	}

	return nil
}
