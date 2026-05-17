package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/fetcher"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var fetchProviders []string
var fetchOutput string
var fetchTimeout int
var fetchAll bool

var (
	fetchStylePending = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	fetchStyleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	fetchStyleName    = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Скачать актуальные IPv4 CIDR от крупных провайдеров и сохранить в файл",
	Long: `Загружает IPv4 CIDR-диапазоны из публичных источников и записывает их
в native .lst файл с секциями по провайдерам.

Доступные провайдеры: cloudflare, google, aws, azure, fastly, telegram, github, oracle

GitHub — выбор сервисов: github/hooks, github/web, github/api, github/git,
  github/packages, github/pages, github/actions, github/copilot и др.
Oracle — выбор регионов: oracle/us-ashburn-1, oracle/eu-frankfurt-1 и др.

Без флагов — интерактивный выбор провайдеров (TUI).
С --all    — скачать все провайдеры без вопросов.
С -p       — указать конкретные провайдеры.

Примеры:
  -p github              — все сервисы GitHub
  -p github/copilot      — только Copilot
  -p oracle              — все регионы Oracle
  -p oracle/eu-frankfurt-1,oracle/us-ashburn-1

Если провайдер недоступен — выводится предупреждение, остальные продолжают скачиваться.`,
	RunE: runFetch,
}

func init() {
	fetchCmd.Flags().StringArrayVarP(&fetchProviders, "provider", "p", nil,
		"Провайдеры: -p cloudflare,google или -p cloudflare -p google")
	fetchCmd.Flags().BoolVarP(&fetchAll, "all", "a", false, "Скачать все провайдеры без интерактивного выбора")
	fetchCmd.Flags().StringVarP(&fetchOutput, "output", "o", "", "Путь к выходному файлу (обязательный)")
	fetchCmd.Flags().IntVarP(&fetchTimeout, "timeout", "t", 30, "Таймаут HTTP-запроса в секундах")
	_ = fetchCmd.MarkFlagRequired("output")
}

func runFetch(cmd *cobra.Command, args []string) error {
	client := fetcher.NewClient(time.Duration(fetchTimeout) * time.Second)

	providers, err := resolveFetchProviders(fetchProviders, fetchAll, client)
	if err != nil {
		return err
	}
	if len(providers) == 0 {
		return nil
	}

	type result struct {
		provider fetcher.Provider
		cidrs    []string
		err      error
	}

	fmt.Println()
	results := make([]result, 0, len(providers))
	for _, p := range providers {
		fmt.Printf("  %s  %s\n",
			fetchStyleName.Render(fmt.Sprintf("%-32s", p.Name)),
			fetchStylePending.Render("загрузка..."),
		)
		cidrs, fetchErr := p.Fetch(client)
		fmt.Print("\033[1A\033[2K")
		if fetchErr != nil {
			output.Warn(fmt.Sprintf("%-32s %v", p.Name, fetchErr))
		} else {
			fmt.Printf("  %s  %s\n",
				fetchStyleName.Render(fmt.Sprintf("%-32s", p.Name)),
				fetchStyleOK.Render(fmt.Sprintf("%d CIDR", len(cidrs))),
			)
		}
		results = append(results, result{provider: p, cidrs: cidrs, err: fetchErr})
	}

	var sb strings.Builder
	total := 0
	for _, r := range results {
		if r.err != nil || len(r.cidrs) == 0 {
			continue
		}
		sb.WriteString(sectionHeader(r.provider.Name))
		sb.WriteByte('\n')
		tag := strings.ToUpper(r.provider.Name)
		for _, cidr := range r.cidrs {
			sb.WriteString(fmt.Sprintf("%-24s ## %s\n", cidr, tag))
		}
		sb.WriteByte('\n')
		total += len(r.cidrs)
	}

	if total == 0 {
		return fmt.Errorf("не удалось получить ни одного CIDR — файл не записан")
	}

	if err := os.WriteFile(fetchOutput, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("запись файла: %w", err)
	}

	output.Summary(total, 0, 0, false)
	fmt.Printf("\n  %s\n\n", fetchStylePending.Render("→ "+fetchOutput))
	return nil
}

// resolveFetchProviders returns the flat list of leaf providers to fetch.
// Priority: --provider > --all > interactive TUI.
func resolveFetchProviders(flagVals []string, all bool, client *fetcher.HTTPClient) ([]fetcher.Provider, error) {
	if len(flagVals) > 0 {
		return parseFetchProviderSlugs(flagVals, client)
	}
	if all {
		return expandAll(client)
	}
	return selectProvidersInteractive(client)
}

// expandAll fetches all providers, loading dynamic sub-providers (Oracle) via HTTP.
func expandAll(client *fetcher.HTTPClient) ([]fetcher.Provider, error) {
	var result []fetcher.Provider
	for _, p := range fetcher.All {
		if len(p.SubProviders) > 0 {
			result = append(result, p.SubProviders...)
		} else if p.LoadSubProviders != nil {
			output.Info(fmt.Sprintf("Загрузка регионов %s...", p.Name))
			subs, err := p.LoadSubProviders(client)
			if err != nil {
				output.Warn(fmt.Sprintf("%s: не удалось загрузить регионы: %v", p.Name, err))
				continue
			}
			result = append(result, subs...)
		} else {
			result = append(result, p)
		}
	}
	return result, nil
}

func parseFetchProviderSlugs(flagVals []string, client *fetcher.HTTPClient) ([]fetcher.Provider, error) {
	var slugs []string
	for _, v := range flagVals {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" {
				slugs = append(slugs, part)
			}
		}
	}

	var providers []fetcher.Provider
	for _, slug := range slugs {
		switch {
		case slug == "github":
			providers = append(providers, fetcher.GitHubProvider().SubProviders...)
		case strings.HasPrefix(slug, "github/"):
			p, ok := fetcher.BySlug(slug)
			if !ok {
				return nil, fmt.Errorf("неизвестный GitHub сервис %q", slug)
			}
			providers = append(providers, p)
		case slug == "oracle":
			output.Info("Загрузка регионов Oracle Cloud...")
			subs, err := fetcher.OracleProvider().LoadSubProviders(client)
			if err != nil {
				return nil, fmt.Errorf("oracle: загрузка регионов: %w", err)
			}
			providers = append(providers, subs...)
		case strings.HasPrefix(slug, "oracle/"):
			// oracle/us-ashburn-1 — create on the fly since regions are dynamic
			region := strings.TrimPrefix(slug, "oracle/")
			providers = append(providers, fetcher.MakeOracleRegionProvider(region))
		default:
			p, ok := fetcher.BySlug(slug)
			if !ok {
				available := make([]string, 0, len(fetcher.All))
				for _, a := range fetcher.All {
					available = append(available, a.Slug)
				}
				return nil, fmt.Errorf("неизвестный провайдер %q. Доступные: %s", slug, strings.Join(available, ", "))
			}
			providers = append(providers, p)
		}
	}
	return providers, nil
}

func selectProvidersInteractive(client *fetcher.HTTPClient) ([]fetcher.Provider, error) {
	topOptions := make([]huh.Option[string], len(fetcher.All))
	for i, p := range fetcher.All {
		topOptions[i] = huh.NewOption(p.Name, p.Slug)
	}

	ghProvider := fetcher.GitHubProvider()
	ghSubOptions := make([]huh.Option[string], len(ghProvider.SubProviders))
	for i, sub := range ghProvider.SubProviders {
		ghSubOptions[i] = huh.NewOption(sub.Name, sub.Slug)
	}

	var selectedSlugs []string
	var selectedGHSubs []string
	var selectedOracleSubs []string

	// Oracle regions are loaded lazily — only when Oracle is selected.
	// We pre-load them before building the form so the TUI doesn't block mid-render.
	output.Info("Загрузка регионов Oracle Cloud...")
	oracleSubs, oracleErr := fetcher.OracleProvider().LoadSubProviders(client)
	// clear the info line
	fmt.Print("\033[1A\033[2K")
	if oracleErr != nil {
		output.Warn(fmt.Sprintf("Oracle Cloud: не удалось загрузить регионы: %v", oracleErr))
	}

	oracleSubOptions := make([]huh.Option[string], len(oracleSubs))
	for i, sub := range oracleSubs {
		oracleSubOptions[i] = huh.NewOption(sub.Name, sub.Slug)
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Выберите провайдеры").
				Description("Пробел — выбрать/снять, Enter — подтвердить").
				Options(topOptions...).
				Height(tuiHeight()).
				Value(&selectedSlugs),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("GitHub — выберите сервисы").
				Description("Пробел — выбрать/снять, Enter — подтвердить").
				Options(ghSubOptions...).
				Height(tuiHeight()).
				Value(&selectedGHSubs),
		).WithHideFunc(func() bool {
			for _, s := range selectedSlugs {
				if s == "github" {
					return false
				}
			}
			return true
		}),
	}

	if len(oracleSubOptions) > 0 {
		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Oracle Cloud — выберите регионы").
				Description("Пробел — выбрать/снять, Enter — подтвердить").
				Options(oracleSubOptions...).
				Height(tuiHeight()).
				Value(&selectedOracleSubs),
		).WithHideFunc(func() bool {
			for _, s := range selectedSlugs {
				if s == "oracle" {
					return false
				}
			}
			return true
		}))
	}

	if err := huh.NewForm(groups...).Run(); err != nil {
		if err == huh.ErrUserAborted {
			output.Info("Отменено")
			return nil, nil
		}
		return nil, fmt.Errorf("выбор провайдеров: %w", err)
	}

	var providers []fetcher.Provider
	for _, slug := range selectedSlugs {
		switch slug {
		case "github":
			for _, subSlug := range selectedGHSubs {
				sub, _ := fetcher.BySlug(subSlug)
				providers = append(providers, sub)
			}
		case "oracle":
			for _, subSlug := range selectedOracleSubs {
				sub, _ := fetcher.BySlug(subSlug)
				providers = append(providers, sub)
			}
		default:
			p, _ := fetcher.BySlug(slug)
			providers = append(providers, p)
		}
	}
	return providers, nil
}

// sectionHeader returns a native-format section comment line matching the style in list.lst.
func sectionHeader(name string) string {
	const width = 80
	prefix := "# ── " + name + " "
	dashes := width - len(prefix)
	if dashes < 2 {
		dashes = 2
	}
	return prefix + strings.Repeat("─", dashes)
}

// tuiHeight returns a comfortable list height based on terminal rows.
// Reserves ~6 rows for title, description, borders and prompt.
func tuiHeight() int {
	_, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || rows < 10 {
		return 10
	}
	h := rows - 6
	if h > 20 {
		h = 20
	}
	return h
}
