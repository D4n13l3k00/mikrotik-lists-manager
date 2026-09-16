package cli

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

var findFlags connFlags
var findJSON bool

var findStyleList = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
var findStyleMatch = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
var findStyleSubnet = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

var findCmd = &cobra.Command{
	Use:   "find <address>",
	Short: "Найти адрес или CIDR во всех address-list на роутере",
	Long: `Ищет точное совпадение адреса, а также проверяет попадание IP в CIDR-записи и вложенность подсетей.

Примеры:
  mlm find 8.8.8.8 -H 192.168.1.1 -u admin
  mlm find 192.168.0.0/16 -H 192.168.1.1 -u admin
  mlm find 1.1.1.1 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runFind,
}

func init() {
	findCmd.Flags().StringVarP(&findFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	findCmd.Flags().StringVarP(&findFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	findCmd.Flags().StringVarP(&findFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	findCmd.Flags().BoolVarP(&findFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	findCmd.Flags().BoolVar(&findJSON, "json", false, "Вывод в формате JSON")
}

func matchAddress(needle, entryAddr string) (bool, output.MatchType) {
	normNeedle := parser.NormalizeAddr(needle)
	normEntry := parser.NormalizeAddr(entryAddr)
	if strings.EqualFold(normNeedle, normEntry) {
		return true, output.MatchExact
	}

	needlePrefix, errNeedlePrefix := netip.ParsePrefix(needle)
	needleAddr, errNeedleAddr := netip.ParseAddr(needle)

	entryPrefix, errEntryPrefix := netip.ParsePrefix(entryAddr)
	entryAddrParsed, errEntryAddr := netip.ParseAddr(entryAddr)

	// Case 1: needle is Prefix, entry is Addr
	if errNeedlePrefix == nil && errEntryAddr == nil {
		if needlePrefix.Masked().Contains(entryAddrParsed) {
			return true, output.MatchSubnet
		}
	}
	// Case 2: needle is Addr, entry is Prefix
	if errNeedleAddr == nil && errEntryPrefix == nil {
		if entryPrefix.Masked().Contains(needleAddr) {
			return true, output.MatchSubnet
		}
	}
	// Case 3: both are Prefixes
	if errNeedlePrefix == nil && errEntryPrefix == nil {
		np := needlePrefix.Masked()
		ep := entryPrefix.Masked()
		// needle contains entry
		if np.Bits() <= ep.Bits() && np.Contains(ep.Addr()) {
			return true, output.MatchSubnet
		}
		// entry contains needle
		if ep.Bits() <= np.Bits() && ep.Contains(np.Addr()) {
			return true, output.MatchSubnet
		}
	}

	return false, ""
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

	client := newClient(host, user, pass, resolveSkipTLS(findFlags.skipTLSVerify))
	ctx := cmd.Context()

	entries, err := client.GetAllEntries(ctx)
	if err != nil {
		return fmt.Errorf("получение записей: %w", err)
	}

	var results []output.FindResultDTO
	for _, e := range entries {
		matched, matchType := matchAddress(needle, e.Address)
		if matched {
			results = append(results, output.FindResultDTO{
				List:      e.List,
				Address:   e.Address,
				Comment:   e.Comment,
				Disabled:  e.Disabled.Bool(),
				MatchType: matchType,
			})
		}
	}

	if findJSON {
		if results == nil {
			results = []output.FindResultDTO{}
		}
		return output.JSON(results)
	}

	if len(results) == 0 {
		output.Info(fmt.Sprintf("Адрес %q не найден ни в одном списке.", needle))
		return nil
	}

	output.Header(fmt.Sprintf("Результаты поиска %q на %s", needle, host))
	for _, res := range results {
		list := findStyleList.Render(fmt.Sprintf("%-24s", res.List))
		addr := res.Address
		if res.MatchType == output.MatchExact {
			addr = findStyleMatch.Render(addr)
		} else {
			addr = addr + " " + findStyleSubnet.Render("(subnet)")
		}
		line := fmt.Sprintf("  %s  %s", list, addr)
		if res.Comment != "" {
			line += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render("# "+res.Comment)
		}
		if res.Disabled {
			line += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true).Render("[off]")
		}
		fmt.Println(line)
	}
	fmt.Println()
	output.Info(fmt.Sprintf("Найдено: %d", len(results)))
	return nil
}
