package cli

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/snapshot"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

var (
	rollbackFlags       connFlags
	rollbackID          string
	rollbackDryRun      bool
	rollbackForce       bool
	rollbackVerbose     bool
	rollbackConcurrency int
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Откатить address-list на MikroTik к сохранённому снимку",
	Long: `Восстанавливает состояние списка адресов из локального снимка.
По умолчанию восстанавливает последний снимок ('latest').

Примеры:
  mlm rollback -l vpn-routes -H 192.168.88.1 -u admin
  mlm rollback -l vpn-routes -H 192.168.88.1 -u admin -n
  mlm rollback -l vpn-routes --id 20260916-120000_abcd -y`,
	RunE: runRollback,
}

func init() {
	rollbackCmd.Flags().StringVarP(&rollbackFlags.host, "host", "H", "", "Адрес MikroTik (host или host:port) [$MT_HOST]")
	rollbackCmd.Flags().StringVarP(&rollbackFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	rollbackCmd.Flags().StringVarP(&rollbackFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	rollbackCmd.Flags().StringArrayVarP(&rollbackFlags.listNames, "list", "l", nil, "Имя address-list [$MT_LIST]")
	rollbackCmd.Flags().BoolVarP(&rollbackFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	rollbackCmd.Flags().StringVar(&rollbackID, "id", "latest", "ID снимка для отката (по умолчанию 'latest')")
	rollbackCmd.Flags().BoolVarP(&rollbackDryRun, "dry-run", "n", false, "Показать изменения без применения")
	rollbackCmd.Flags().BoolVarP(&rollbackForce, "force", "y", false, "Не запрашивать подтверждение при массовом удалении записей")
	rollbackCmd.Flags().BoolVarP(&rollbackVerbose, "verbose", "v", false, "Выводить каждую запись даже при прогресс-баре")
	rollbackCmd.Flags().IntVarP(&rollbackConcurrency, "concurrency", "c", 5, "Число параллельных запросов к API (0 = последовательно)")
}

func runRollback(cmd *cobra.Command, args []string) error {
	host := resolve(rollbackFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(rollbackFlags.user, "MT_USER", loadedConfig.User)
	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	listNames, err := resolveListNames(rollbackFlags.listNames, loadedConfig.List)
	if err != nil {
		return err
	}
	if len(listNames) == 0 {
		return fmt.Errorf("--list обязателен")
	}
	listName := listNames[0]

	pass, err := resolvePassword(rollbackFlags.pass)
	if err != nil {
		return err
	}

	snap, err := snapshot.Load(host, listName, rollbackID)
	if err != nil {
		return fmt.Errorf("загрузка снимка: %w", err)
	}

	client := newClient(host, user, pass, resolveSkipTLS(rollbackFlags.skipTLSVerify))
	ctx := cmd.Context()

	output.Header(fmt.Sprintf("Откат списка %q на %s к снимку %s (%s)",
		listName, host, snap.ID, snap.CreatedAt.Format("2006-01-02 15:04:05")))

	current, err := client.GetList(ctx, listName)
	if err != nil {
		return fmt.Errorf("получение списка с роутера: %w", err)
	}

	desired := make([]parser.Entry, 0, len(snap.Entries))
	for _, e := range snap.Entries {
		desired = append(desired, parser.Entry{
			Address:  e.Address,
			Comment:  e.Comment,
			Disabled: e.Disabled.Bool(),
		})
	}

	changes, duplicates := syncer.Diff(desired, current)
	for _, addr := range duplicates {
		output.Warn(fmt.Sprintf("дубль в снимке: %s", addr))
	}

	if len(changes) == 0 {
		output.Info("Список на роутере уже соответствует выбранному снимку (изменений нет).")
		return nil
	}

	deleteCount := 0
	for _, ch := range changes {
		if ch.Action == syncer.ActionDelete {
			deleteCount++
		}
	}

	if !rollbackDryRun && !rollbackForce && deleteCount > 0 {
		isBulkDelete := deleteCount > 50 || (len(current) > 0 && float64(deleteCount)/float64(len(current)) > 0.50)
		if isBulkDelete {
			if term.IsTerminal(int(os.Stdin.Fd())) {
				var confirmed bool
				err := huh.NewConfirm().
					Title(fmt.Sprintf("Внимание: при откате будет удалено %d записей из %d в списке %q. Продолжить?", deleteCount, len(current), listName)).
					Value(&confirmed).
					Run()
				if err != nil || !confirmed {
					return fmt.Errorf("откат отменен пользователем")
				}
			} else {
				return fmt.Errorf("безопасность: при откате будет удалено %d записей из %d в списке %q. Используйте --force (-y)", deleteCount, len(current), listName)
			}
		}
	}

	output.Header("Применение изменений отката")
	if rollbackDryRun {
		output.Info("(dry run — изменения не будут применены)")
	}

	return syncer.Apply(ctx, client, listName, changes, rollbackDryRun, rollbackVerbose, rollbackConcurrency)
}
