package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/source"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/validator"
)

var (
	validateFormat string
	validateStrict bool
	validateJSON   bool
)

var validateCmd = &cobra.Command{
	Use:   "validate <file|url>",
	Short: "Проверить синтаксис, дубликаты и подсети в файле или URL",
	Long: `Выполняет глубокую проверку списка:
- Корректность синтаксиса IP, CIDR и доменных имен
- Обнаружение неканонических масок (ненулевые биты хоста)
- Поиск дубликатов адресов
- Поиск поглощенных подсетей (shadowed subnets)
- Подсчет статистики по типам записей

Примеры:
  mlm validate list.lst
  mlm validate list.lst --strict
  mlm validate https://example.com/ips.txt --json`,
	Args: cobra.ExactArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVarP(&validateFormat, "format", "f", "auto", "Формат файла: auto, native, mikrotik")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false, "Считать предупреждения ошибками (exit code 1)")
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "Вывод отчёта в формате JSON")
}

func runValidate(cmd *cobra.Command, args []string) error {
	target := args[0]
	ctx := cmd.Context()
	proxyURL := resolveProxy(proxyFlag)

	data, err := source.ReadWithProxy(ctx, target, proxyURL)
	if err != nil {
		return fmt.Errorf("чтение %s: %w", target, err)
	}

	effectiveFormat := validateFormat
	if effectiveFormat == "auto" && loadedConfig.DefaultFormat != "" {
		effectiveFormat = loadedConfig.DefaultFormat
	}

	entries, err := parseContent(data, effectiveFormat)
	if err != nil {
		return fmt.Errorf("парсинг %s: %w", target, err)
	}

	report := validator.Validate(entries)

	if validateJSON {
		if err := output.JSON(report); err != nil {
			return err
		}
		if !report.Passed || (validateStrict && report.Warnings > 0) {
			return fmt.Errorf("валидация завершилась с ошибками")
		}
		return nil
	}

	output.Header(fmt.Sprintf("Валидация %s", target))

	styleErr := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleWarn := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	if len(report.Issues) > 0 {
		for _, issue := range report.Issues {
			var badge string
			switch issue.Severity {
			case validator.SeverityError:
				badge = styleErr.Render("ERR ")
			case validator.SeverityWarning:
				badge = styleWarn.Render("WARN")
			case validator.SeverityInfo:
				badge = styleInfo.Render("INFO")
			}
			fmt.Printf("  [%s] строка %-4d  %-24s %s\n", badge, issue.Index, issue.Address, issue.Message)
		}
		fmt.Println()
	}

	output.KV("Всего записей", fmt.Sprintf("%d", report.Stats.Total), "")
	output.KV("Корректных", fmt.Sprintf("%d", report.Stats.Valid), "")
	output.KV("IPv4 хостов (/32)", fmt.Sprintf("%d", report.Stats.HostsIPv4), "")
	output.KV("IPv4 подсетей", fmt.Sprintf("%d", report.Stats.SubnetsIPv4), "")
	output.KV("IPv6 записей", fmt.Sprintf("%d", report.Stats.IPv6), "")
	output.KV("Доменных имен", fmt.Sprintf("%d", report.Stats.Domains), "")
	if report.Stats.Disabled > 0 {
		output.KV("Отключенных", fmt.Sprintf("%d", report.Stats.Disabled), "")
	}
	if report.Stats.Duplicates > 0 {
		output.KV("Дубликатов", fmt.Sprintf("%d", report.Stats.Duplicates), "")
	}
	if report.Stats.Shadowed > 0 {
		output.KV("Поглощенных подсетей", fmt.Sprintf("%d", report.Stats.Shadowed), "")
	}
	fmt.Println()

	if report.Errors > 0 {
		return fmt.Errorf("валидация не пройдена: найдено ошибок — %d, предупреждений — %d", report.Errors, report.Warnings)
	}

	if validateStrict && report.Warnings > 0 {
		return fmt.Errorf("валидация не пройдена (--strict): найдено предупреждений — %d", report.Warnings)
	}

	output.Info("Список валиден. Ошибок не обнаружено.")
	return nil
}
