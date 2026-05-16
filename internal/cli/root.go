package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/config"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

type connFlags struct {
	host          string
	user          string
	pass          string
	listNames     []string // supports multiple via -l a,b or -l a -l b
	skipTLSVerify bool
}

var configFile string
var loadedConfig config.Config

// ── help styles ──────────────────────────────────────────────────────────────

var (
	helpTitle   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	helpSection = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	helpCmd     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	helpFlag    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	helpDesc    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	helpDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpExample = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
)

func helpFunc(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println(helpTitle.Render("  " + cmd.CommandPath()))
	if cmd.Short != "" {
		fmt.Println(helpDesc.Render("  " + cmd.Short))
	}

	if cmd.Long != "" {
		fmt.Println()
		for _, line := range strings.Split(cmd.Long, "\n") {
			fmt.Println(helpDim.Render("  " + line))
		}
	}

	cmds := cmd.Commands()
	if len(cmds) > 0 {
		fmt.Println()
		fmt.Println(helpSection.Render("  Команды"))
		for _, sub := range cmds {
			if sub.Hidden {
				continue
			}
			name := helpCmd.Render(fmt.Sprintf("    %-18s", sub.Name()))
			fmt.Println(name + helpDesc.Render(sub.Short))
		}
	}

	// build set of inherited flag names to avoid duplication in local flags
	inheritedNames := map[string]bool{}
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		inheritedNames[f.Name] = true
	})

	printFlags := func(title string, fs *pflag.FlagSet) {
		if !fs.HasAvailableFlags() {
			return
		}
		hasVisible := false
		fs.VisitAll(func(f *pflag.Flag) {
			if !f.Hidden {
				hasVisible = true
			}
		})
		if !hasVisible {
			return
		}
		fmt.Println()
		fmt.Println(helpSection.Render("  " + title))
		fs.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			short := "  "
			if f.Shorthand != "" {
				short = helpFlag.Render("-"+f.Shorthand) + ","
			}
			def := ""
			if f.DefValue != "" && f.DefValue != "false" {
				def = helpDim.Render(fmt.Sprintf(" (по умолчанию: %s)", f.DefValue))
			}
			name := helpFlag.Render(fmt.Sprintf("--%-18s", f.Name))
			fmt.Println(fmt.Sprintf("    %s %s  %s%s", short, name, helpDesc.Render(f.Usage), def))
		})
	}

	// local flags — skip ones that are already in inherited (avoids duplication)
	localOnly := pflag.NewFlagSet("", pflag.ContinueOnError)
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !inheritedNames[f.Name] {
			localOnly.AddFlag(f)
		}
	})
	printFlags("Флаги", localOnly)
	printFlags("Глобальные флаги", cmd.InheritedFlags())

	fmt.Println()
	fmt.Println(helpDim.Render(fmt.Sprintf("  Использование: %s", cmd.UseLine())))
	fmt.Println()
}

var rootCmd = &cobra.Command{
	Use:   "mikrotik-lists-manager",
	Short: "Синхронизация address-list MikroTik из файла",
	Long: `Поддерживаемые форматы файлов:
  native    — IP/CIDR построчно, ## комментарий для MikroTik, # только локально
  mikrotik  — формат экспорта (/ip firewall address-list ... add address=...)

Конфиг (опционально): .mikrotik-lists-manager.yaml в текущей директории.
Создать шаблон: mikrotik-lists-manager config init

Приоритет: флаг > переменная окружения > конфиг файл
Переменные окружения: MT_HOST, MT_USER, MT_PASS, MT_LIST`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Parent() != nil && cmd.Parent().Name() == "config" {
			return nil
		}
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}
		loadedConfig = cfg
		return nil
	},
}

func Execute(version, commit string) {
	rootCmd.Version = version + " (" + commit + ")"
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", config.DefaultConfigFile, "Путь к конфиг файлу")
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(appendCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(configCmd)

	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	setHelp(rootCmd)
}

func setHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		helpFunc(c, args)
	})
	for _, sub := range cmd.Commands() {
		setHelp(sub)
	}
}

func resolve(flagVal, envKey, cfgVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return cfgVal
}

func resolvePassword(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if v := os.Getenv("MT_PASS"); v != "" {
		return v, nil
	}
	if loadedConfig.Pass != "" {
		return loadedConfig.Pass, nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func resolveSkipTLS(flagVal bool) bool {
	if flagVal {
		return true
	}
	return loadedConfig.SkipTLSVerify
}

// resolveListNames returns the list of address-list names from flags, env, or config.
// Supports comma-separated values and repeated flags: -l a,b or -l a -l b
func resolveListNames(flagVals []string, cfgVal string) ([]string, error) {
	var raw []string
	if len(flagVals) > 0 {
		raw = flagVals
	} else if v := os.Getenv("MT_LIST"); v != "" {
		raw = []string{v}
	} else if cfgVal != "" {
		raw = []string{cfgVal}
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("--list обязателен (или задайте MT_LIST / list в конфиге)")
	}

	// split comma-separated values
	var names []string
	for _, r := range raw {
		for _, part := range strings.Split(r, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				names = append(names, part)
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("--list не может быть пустым")
	}
	return names, nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Управление конфигурационным файлом",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Создать шаблон конфига в текущей директории",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configFile); err == nil {
			return fmt.Errorf("%s уже существует — удалите его или укажите другой путь через --config", configFile)
		}
		if err := os.WriteFile(configFile, []byte(config.Template()), 0o600); err != nil {
			return fmt.Errorf("запись конфига: %w", err)
		}
		output.Info(fmt.Sprintf("Создан %s", configFile))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Показать активную конфигурацию (файл + env)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}
		if v := os.Getenv("MT_HOST"); v != "" {
			cfg.Host = v
		}
		if v := os.Getenv("MT_USER"); v != "" {
			cfg.User = v
		}
		passHint := ""
		if os.Getenv("MT_PASS") != "" {
			cfg.Pass = "***"
			passHint = "из env"
		} else if cfg.Pass != "" {
			cfg.Pass = "***"
			passHint = "из конфига"
		}
		if v := os.Getenv("MT_LIST"); v != "" {
			cfg.List = v
		}

		output.Header("Конфигурация")
		output.KV("config", configFile, "")
		output.KV("host", orEmpty(cfg.Host), "")
		output.KV("user", orEmpty(cfg.User), "")
		output.KV("pass", orEmpty(cfg.Pass), passHint)
		output.KV("list", orEmpty(cfg.List), "")
		output.KV("insecure", fmt.Sprintf("%v", cfg.SkipTLSVerify), "")
		output.KV("format", orDefault(cfg.DefaultFormat, "auto"), "")
		fmt.Println()
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
}

func orEmpty(s string) string {
	if s == "" {
		return "(не задано)"
	}
	return s
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
