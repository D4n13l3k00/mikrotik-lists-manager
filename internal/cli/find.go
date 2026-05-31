package cli

import (
	"fmt"
	"net"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var findFlags connFlags

var findStyleList = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
var findStyleMatch = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

var findCmd = &cobra.Command{
	Use:   "find <address>",
	Short: "Найти адрес или CIDR во всех address-list на роутере",
	Long: `Ищет точное совпадение адреса, а также проверяет попадание IP в CIDR-записи.

Примеры:
  mikrotik-lists-manager find 8.8.8.8 -H 192.168.1.1 -u admin
  mikrotik-lists-manager find 192.168.0.0/16 -H 192.168.1.1 -u admin`,
	Args: cobra.ExactArgs(1),
	RunE: runFind,
}

func init() {
	findCmd.Flags().StringVarP(&findFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	findCmd.Flags().StringVarP(&findFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	findCmd.Flags().StringVarP(&findFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	findCmd.Flags().BoolVarP(&findFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
}

func runFind(cmd *cobra.Command, args []string) error {
	needle := args[0]
	host := resolve(findFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(findFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	pass, err := resolvePassword(findFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(findFlags.skipTLSVerify))
	ctx := cmd.Context()

	entries, err := client.GetAllEntries(ctx)
	if err != nil {
		return fmt.Errorf("получение записей: %w", err)
	}

	needleIP := net.ParseIP(needle)
	_, needleNet, _ := net.ParseCIDR(needle)

	var found []mikrotik.AddressListEntry
	for _, e := range entries {
		if e.Address == needle {
			found = append(found, e)
			continue
		}
		// needle is an IP — check if it falls inside a CIDR entry
		if needleIP != nil {
			if _, entryNet, err := net.ParseCIDR(e.Address); err == nil {
				if entryNet.Contains(needleIP) {
					found = append(found, e)
					continue
				}
			}
		}
		// needle is a CIDR — check if entry IP falls inside it
		if needleNet != nil {
			if entryIP := net.ParseIP(e.Address); entryIP != nil {
				if needleNet.Contains(entryIP) {
					found = append(found, e)
					continue
				}
			}
		}
	}

	if len(found) == 0 {
		output.Info(fmt.Sprintf("Адрес %q не найден ни в одном списке.", needle))
		return nil
	}

	output.Header(fmt.Sprintf("Результаты поиска %q на %s", needle, host))
	for _, e := range found {
		list := findStyleList.Render(fmt.Sprintf("%-24s", e.List))
		addr := e.Address
		if e.Address == needle {
			addr = findStyleMatch.Render(addr)
		}
		line := fmt.Sprintf("  %s  %s", list, addr)
		if e.Comment != "" {
			line += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render("# "+e.Comment)
		}
		if e.Disabled.Bool() {
			line += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true).Render("[off]")
		}
		fmt.Println(line)
	}
	fmt.Println()
	output.Info(fmt.Sprintf("Найдено: %d", len(found)))
	return nil
}
