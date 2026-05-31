package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var listFlags connFlags
var listEntries string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать все address-list на роутере с количеством записей",
	Long: `Получает все статические записи с роутера и выводит сводку по спискам.
С флагом --entries показывает все записи конкретного списка.

Примеры:
  mikrotik-lists-manager list -H 192.168.1.1 -u admin
  mikrotik-lists-manager list -H 192.168.1.1 -u admin -e vpn-routes`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVarP(&listFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	listCmd.Flags().StringVarP(&listFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	listCmd.Flags().StringVarP(&listFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	listCmd.Flags().BoolVarP(&listFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	listCmd.Flags().StringVarP(&listEntries, "entries", "e", "", "Показать все записи указанного списка")
}

func runList(cmd *cobra.Command, args []string) error {
	host := resolve(listFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(listFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	pass, err := resolvePassword(listFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(listFlags.skipTLSVerify))
	ctx := cmd.Context()

	if listEntries != "" {
		return runListEntries(ctx, client, host, listEntries)
	}

	entries, err := client.GetAllEntries(ctx)
	if err != nil {
		return fmt.Errorf("получение списков: %w", err)
	}

	// group by list name
	type stat struct {
		total    int
		disabled int
	}
	stats := map[string]*stat{}
	for _, e := range entries {
		if stats[e.List] == nil {
			stats[e.List] = &stat{}
		}
		stats[e.List].total++
		if e.Disabled.Bool() {
			stats[e.List].disabled++
		}
	}

	if len(stats) == 0 {
		output.Info("Списков не найдено.")
		return nil
	}

	names := make([]string, 0, len(stats))
	for n := range stats {
		names = append(names, n)
	}
	sort.Strings(names)

	output.Header(fmt.Sprintf("Address-lists на %s", host))
	for _, name := range names {
		s := stats[name]
		output.ListRow(name, s.total, s.disabled)
	}
	fmt.Println()
	output.Info(fmt.Sprintf("Всего списков: %d", len(names)))
	return nil
}

func runListEntries(ctx context.Context, client *mikrotik.Client, host, listName string) error {
	entries, err := client.GetList(ctx, listName)
	if err != nil {
		return fmt.Errorf("получение списка %q: %w", listName, err)
	}

	if len(entries) == 0 {
		output.Info(fmt.Sprintf("Список %q пуст.", listName))
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Address < entries[j].Address
	})

	output.Header(fmt.Sprintf("%q на %s  (%d записей)", listName, host, len(entries)))
	disabled := 0
	for _, e := range entries {
		output.EntryRow(e.Address, e.Comment, e.Disabled.Bool())
		if e.Disabled.Bool() {
			disabled++
		}
	}
	fmt.Println()
	if disabled > 0 {
		output.Info(fmt.Sprintf("%d записей, %d отключено", len(entries), disabled))
	}
	return nil
}
