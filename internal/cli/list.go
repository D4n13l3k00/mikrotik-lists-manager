package cli

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var listFlags connFlags
var listEntries string
var listSort string
var listFilter string
var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать все address-list на роутере с количеством записей",
	Long: `Получает все статические записи с роутера и выводит сводку по спискам.
С флагом --entries показывает все записи конкретного списка.

Примеры:
  mikrotik-lists-manager list -H 192.168.1.1 -u admin
  mikrotik-lists-manager list -H 192.168.1.1 -u admin -e vpn-routes
  mikrotik-lists-manager list --json
  mikrotik-lists-manager list -e vpn-routes --json`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVarP(&listFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	listCmd.Flags().StringVarP(&listFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	listCmd.Flags().StringVarP(&listFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	listCmd.Flags().BoolVarP(&listFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	listCmd.Flags().StringVarP(&listEntries, "entries", "e", "", "Показать все записи указанного списка")
	listCmd.Flags().StringVar(&listSort, "sort", "name", "Сортировка: name (по имени) или size (по количеству записей)")
	listCmd.Flags().StringVarP(&listFilter, "filter", "F", "", "Фильтр по имени списка (подстрока, без учёта регистра)")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Вывод в формате JSON")
}

func parseAsPrefix(s string) (netip.Prefix, bool) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p, true
	}
	if ip, err := netip.ParseAddr(s); err == nil {
		return netip.PrefixFrom(ip, ip.BitLen()), true
	}
	return netip.Prefix{}, false
}

func compareAddresses(a, b string) bool {
	pa, aOk := parseAsPrefix(a)
	pb, bOk := parseAsPrefix(b)

	if aOk && bOk {
		if pa.Addr().Is4() && pb.Addr().Is6() {
			return true
		}
		if pa.Addr().Is6() && pb.Addr().Is4() {
			return false
		}
		cmp := pa.Addr().Compare(pb.Addr())
		if cmp != 0 {
			return cmp < 0
		}
		return pa.Bits() < pb.Bits()
	}
	if aOk && !bOk {
		return true
	}
	if !aOk && bOk {
		return false
	}
	return a < b
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

	names := make([]string, 0, len(stats))
	for n := range stats {
		if listFilter == "" || strings.Contains(strings.ToLower(n), strings.ToLower(listFilter)) {
			names = append(names, n)
		}
	}

	switch listSort {
	case "size":
		sort.Slice(names, func(i, j int) bool {
			return stats[names[i]].total > stats[names[j]].total
		})
	default:
		sort.Strings(names)
	}

	if listJSON {
		dtos := make([]output.ListSummaryDTO, 0, len(names))
		for _, name := range names {
			s := stats[name]
			dtos = append(dtos, output.ListSummaryDTO{
				Name:     name,
				Total:    s.total,
				Disabled: s.disabled,
			})
		}
		return output.JSON(dtos)
	}

	if len(names) == 0 {
		output.Info("Списков не найдено.")
		return nil
	}

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

	sort.Slice(entries, func(i, j int) bool {
		return compareAddresses(entries[i].Address, entries[j].Address)
	})

	if listJSON {
		dtos := make([]output.EntryDTO, 0, len(entries))
		for _, e := range entries {
			dtos = append(dtos, output.EntryDTO{
				Address:  e.Address,
				Comment:  e.Comment,
				Disabled: e.Disabled.Bool(),
			})
		}
		return output.JSON(dtos)
	}

	if len(entries) == 0 {
		output.Info(fmt.Sprintf("Список %q пуст.", listName))
		return nil
	}

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
