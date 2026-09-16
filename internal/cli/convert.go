package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/dnsresolver"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/source"
)

var (
	convertFormat         string
	convertListName       string
	convertOutFile        string
	convertWrite          bool
	convertResolveDomains bool
	convertDNS            string
)

var convertCmd = &cobra.Command{
	Use:   "convert [файл|URL|-]",
	Short: "Конвертировать список между форматами native и mikrotik",
	Long: `Конвертирует список адресов между текстовым форматом native (.list/.lst)
и форматом команд RouterOS (.rsc) локально без подключения к роутеру.

Форматы:
  native    — IP/CIDR построчно, ## комментарий, ! отключённая запись
  mikrotik  — команды RouterOS (/ip firewall address-list add ...)

Поведение по умолчанию (если --format не указан):
  native вход    → mikrotik выход
  mikrotik вход  → native выход

Примеры:
  mlm convert blocked.list                           # вывод в stdout в формате mikrotik
  mlm convert blocked.list -o blocked.rsc            # сохранение в файл .rsc
  mlm convert export.rsc -f native -o export.list    # конвертация .rsc в .list
  mlm convert blocked.list -l vpn-routes             # явное имя списка для mikrotik
  mlm convert https://example.com/ips.txt -l mylist  # конвертация из URL
  cat list.txt | mlm convert - -l vpn               # конвертация из stdin
  mlm convert domains.list --resolve-domains         # резолв доменов в IP перед конвертацией
  mlm convert list.rsc -w                            # перезапись исходного файла результатом`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().StringVarP(&convertFormat, "format", "f", "auto", "Целевой формат: auto, native, mikrotik")
	convertCmd.Flags().StringVarP(&convertListName, "list", "l", "", "Имя address-list (для формата mikrotik) [$MT_LIST]")
	convertCmd.Flags().StringVarP(&convertOutFile, "output", "o", "", "Записать результат в файл вместо stdout")
	convertCmd.Flags().BoolVarP(&convertWrite, "write", "w", false, "Перезаписать исходный файл результатом")
	convertCmd.Flags().BoolVar(&convertResolveDomains, "resolve-domains", false, "Разрешить доменные имена в IP-адреса")
	convertCmd.Flags().StringVar(&convertDNS, "dns", "", "Пользовательский DNS-сервер (host:port, e.g. 8.8.8.8:53)")
}

func runConvert(cmd *cobra.Command, args []string) error {
	defer func() {
		convertFormat = "auto"
		convertListName = ""
		convertOutFile = ""
		convertWrite = false
		convertResolveDomains = false
		convertDNS = ""
	}()

	var target string
	if len(args) > 0 {
		target = args[0]
	} else {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("требуется указать файл, URL или \"-\" для чтения из stdin")
		}
		target = "-"
	}

	if convertWrite {
		if convertOutFile != "" {
			return fmt.Errorf("флаги --write (-w) и --output (-o) взаимоисключающие")
		}
		if target == "-" || source.IsURL(target) {
			return fmt.Errorf("флаг --write (-w) поддерживается только для локальных файлов")
		}
	}

	ctx := cmd.Context()
	proxyURL := resolveProxy(proxyFlag)

	// If writing to stdout, send output logs/warnings to stderr
	if convertOutFile == "" && !convertWrite {
		output.SetWriter(os.Stderr)
		defer output.ResetWriter()
	}

	content, err := source.ReadWithProxy(ctx, target, proxyURL)
	if err != nil {
		return fmt.Errorf("чтение источника %q: %w", target, err)
	}

	detected := parser.DetectFormat(string(content))
	entries, err := parseContent(content, detected)
	if err != nil {
		return fmt.Errorf("парсинг списка: %w", err)
	}

	targetFormat := strings.ToLower(convertFormat)
	switch targetFormat {
	case "", "auto":
		if detected == "mikrotik" {
			targetFormat = "native"
		} else {
			targetFormat = "mikrotik"
		}
	case "native", "list", "lst":
		targetFormat = "native"
	case "mikrotik", "rsc":
		targetFormat = "mikrotik"
	default:
		return fmt.Errorf("неизвестный формат: %q (поддерживаются: auto, native, mikrotik)", convertFormat)
	}

	if convertResolveDomains {
		res, _ := dnsresolver.ResolveEntries(ctx, entries, convertDNS, proxyURL)
		for _, w := range res.Warnings {
			output.Warn(w)
		}
		entries = res.Entries
	}

	var result string
	if targetFormat == "mikrotik" {
		listName := convertListName
		if listName == "" {
			if target != "-" && !source.IsURL(target) {
				base := filepath.Base(target)
				stem := strings.TrimSuffix(base, filepath.Ext(base))
				if stem != "" {
					listName = stem
				}
			}
		}
		if listName == "" {
			listName = os.Getenv("MT_LIST")
		}
		if listName == "" {
			listName = loadedConfig.List
		}
		if listName == "" {
			listName = "list"
		}

		result = formatMikrotikEntries(entries, listName)
	} else {
		result = formatNativeEntries(entries)
	}

	if convertWrite {
		if err := os.WriteFile(target, []byte(result), 0o644); err != nil {
			return fmt.Errorf("перезапись файла %s: %w", target, err)
		}
		output.Info(fmt.Sprintf("Файл %s успешно обновлен (%d записей, формат: %s)", target, len(entries), targetFormat))
		return nil
	}

	if convertOutFile != "" {
		if err := os.WriteFile(convertOutFile, []byte(result), 0o644); err != nil {
			return fmt.Errorf("запись файла %s: %w", convertOutFile, err)
		}
		output.Info(fmt.Sprintf("Записано %d записей в %s (формат: %s)", len(entries), convertOutFile, targetFormat))
		return nil
	}

	fmt.Print(result)
	return nil
}

func formatMikrotikEntries(entries []parser.Entry, listName string) string {
	var sb strings.Builder
	sb.WriteString("/ip firewall address-list\n")
	for _, e := range entries {
		line := fmt.Sprintf("add list=%s address=%s", listName, e.Address)
		if e.Comment != "" {
			line += fmt.Sprintf(" comment=%q", e.Comment)
		}
		if e.Disabled {
			line += " disabled=yes"
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func formatNativeEntries(entries []parser.Entry) string {
	var sb strings.Builder
	for _, e := range entries {
		prefix := ""
		if e.Disabled {
			prefix = "!"
		}
		if e.Comment != "" {
			sb.WriteString(fmt.Sprintf("%s%s  ## %s\n", prefix, e.Address, e.Comment))
		} else {
			sb.WriteString(prefix + e.Address + "\n")
		}
	}
	return sb.String()
}
