package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/config"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/version"
)

type connFlags struct {
	host          string
	user          string
	pass          string
	listNames     []string // supports multiple via -l a,b or -l a -l b
	skipTLSVerify bool
}

var configFile string
var profileFlag string
var resolvedConfigFile string
var configGlobal bool
var proxyFlag string
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
	Use:          "mlm",
	Aliases:      []string{"mikrotik-lists-manager"},
	Short:        "Синхронизация address-list MikroTik из файла",
	SilenceUsage: true,
	Long: `Поддерживаемые форматы файлов:
  native    — IP/CIDR построчно, ## комментарий для MikroTik, # только локально
  mikrotik  — формат экспорта (/ip firewall address-list ... add address=...)

Конфиг (опционально): .mlm.yaml в текущей директории.
Создать шаблон: mlm config init

Приоритет: флаг > переменная окружения > конфиг файл
Переменные окружения: MT_HOST, MT_USER, MT_PASS, MT_LIST, MT_PROXY, MT_PROFILE`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Parent() != nil && cmd.Parent().Name() == "config" {
			return nil
		}
		resolvedPath, _, err := config.FindConfigFile(configFile)
		if err != nil {
			return err
		}
		resolvedConfigFile = resolvedPath
		cfg, err := config.Load(resolvedPath)
		if err != nil {
			return err
		}
		activeProfile := profileFlag
		if activeProfile == "" {
			activeProfile = os.Getenv("MT_PROFILE")
		}
		prof, err := cfg.EffectiveProfile(activeProfile)
		if err != nil {
			return err
		}
		loadedConfig = config.Config{
			Host:          prof.Host,
			User:          prof.User,
			Pass:          prof.Pass,
			List:          prof.List,
			SkipTLSVerify: prof.Insecure(false),
			DefaultFormat: prof.DefaultFormat,
			Proxy:         prof.Proxy,
		}
		return nil
	},
}

var errInterrupted = errors.New("прервано пользователем")

func isInterrupt(ctx context.Context, err error) bool {
	if ctx != nil && ctx.Err() != nil {
		return true
	}
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, errInterrupted) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "interrupt") ||
		strings.Contains(msg, "canceled") ||
		strings.Contains(msg, "cancelled") ||
		strings.Contains(msg, "deadline exceeded")
}

func Execute(v, commit string) {
	if v != "" && v != "dev" {
		version.Version = v
	}
	if commit != "" && commit != "none" {
		version.Commit = commit
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(os.Args) > 0 {
		binName := filepath.Base(os.Args[0])
		binName = strings.TrimSuffix(strings.ToLower(binName), ".exe")
		if binName == "mikrotik-lists-manager" {
			rootCmd.Use = "mikrotik-lists-manager"
		}
	}

	rootCmd.Version = version.Version + " (" + version.Commit + ")"
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, errInterrupted) || isInterrupt(ctx, err) {
			os.Exit(130)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Путь к конфиг файлу (по умолчанию: автопоиск)")
	rootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "P", "", "Имя профиля роутера из конфига [$MT_PROFILE]")
	rootCmd.PersistentFlags().StringVar(&proxyFlag, "proxy", "", "URL прокси для внешних запросов и DNS (socks5://, socks5h://, http://, https://) [$MT_PROXY]")
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(appendCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(fetchCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(renameCmd)
	rootCmd.AddCommand(convertCmd)
	rootCmd.AddCommand(purgeCmd)
	rootCmd.AddCommand(completionCmd)

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

func resolveProxy(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("MT_PROXY"); v != "" {
		return v
	}
	return loadedConfig.Proxy
}

func newClient(host, user, pass string, skipTLS bool) *mikrotik.Client {
	return mikrotik.NewClient(host, user, pass, skipTLS)
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
	Short: "Создать шаблон конфига в текущей директории или глобально (--global)",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := configFile
		if configGlobal {
			globalPath, err := config.DefaultGlobalConfigPath()
			if err != nil {
				return err
			}
			targetPath = globalPath
		} else if targetPath == "" {
			targetPath = config.DefaultConfigFile
		}

		if _, err := os.Stat(targetPath); err == nil {
			return fmt.Errorf("%s уже существует — удалите его или укажите другой путь", targetPath)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("создание директории конфига: %w", err)
		}
		if err := os.WriteFile(targetPath, []byte(config.Template()), 0o600); err != nil {
			return fmt.Errorf("запись конфига: %w", err)
		}
		output.Info(fmt.Sprintf("Создан %s", targetPath))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Показать активную конфигурацию (файл + профиль + env)",
	RunE: func(cmd *cobra.Command, args []string) error {
		resolvedPath, found, err := config.FindConfigFile(configFile)
		if err != nil {
			return err
		}
		cfg, err := config.Load(resolvedPath)
		if err != nil {
			return err
		}
		activeProfile := profileFlag
		if activeProfile == "" {
			activeProfile = os.Getenv("MT_PROFILE")
		}
		prof, err := cfg.EffectiveProfile(activeProfile)
		if err != nil {
			return err
		}

		host := prof.Host
		if v := os.Getenv("MT_HOST"); v != "" {
			host = v
		}
		user := prof.User
		if v := os.Getenv("MT_USER"); v != "" {
			user = v
		}
		pass := prof.Pass
		passHint := ""
		if os.Getenv("MT_PASS") != "" {
			pass = "***"
			passHint = "из env"
		} else if pass != "" {
			pass = "***"
			passHint = "из конфига"
		}
		list := prof.List
		if v := os.Getenv("MT_LIST"); v != "" {
			list = v
		}

		output.Header("Конфигурация")
		if found {
			output.KV("config", resolvedPath, "")
		} else {
			output.KV("config", "(файл не найден, используются значения по умолчанию и env)", "")
		}
		if activeProfile != "" {
			output.KV("profile", activeProfile, "")
		} else if cfg.DefaultProfile != "" {
			output.KV("profile", fmt.Sprintf("%s (по умолчанию)", cfg.DefaultProfile), "")
		} else {
			output.KV("profile", "(не выбран)", "")
		}
		if len(cfg.Profiles) > 0 {
			var profNames []string
			for k := range cfg.Profiles {
				profNames = append(profNames, k)
			}
			sort.Strings(profNames)
			output.KV("available_profiles", strings.Join(profNames, ", "), "")
		}
		output.KV("host", orEmpty(host), "")
		output.KV("user", orEmpty(user), "")
		output.KV("pass", orEmpty(pass), passHint)
		output.KV("list", orEmpty(list), "")
		output.KV("insecure", fmt.Sprintf("%v", prof.Insecure(false)), "")
		output.KV("format", orDefault(prof.DefaultFormat, "auto"), "")
		proxyVal := prof.Proxy
		proxyHint := ""
		if v := os.Getenv("MT_PROXY"); v != "" {
			proxyVal = v
			proxyHint = "из env"
		} else if proxyVal != "" {
			proxyHint = "из конфига"
		}
		output.KV("proxy", orEmpty(proxyVal), proxyHint)
		fmt.Println()
		return nil
	},
}

func init() {
	configInitCmd.Flags().BoolVarP(&configGlobal, "global", "g", false, "Создать конфиг в глобальной директории пользователя")
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
}

func routerBannerInfo(info *mikrotik.RouterInfo, host string) output.RouterBannerInfo {
	return output.RouterBannerInfo{
		Host:            host,
		BoardName:       info.BoardName,
		Version:         info.Version,
		Architecture:    info.Architecture,
		CPU:             info.CPU,
		CPUCount:        info.CPUCount,
		TotalMemory:     info.TotalMemory,
		FreeMemory:      info.FreeMemory,
		Uptime:          info.Uptime,
		Model:           info.Model,
		Revision:        info.Revision,
		SerialNumber:    info.SerialNumber,
		FirmwareType:    info.FirmwareType,
		FactoryFirmware: info.FactoryFirmware,
		CurrentFirmware: info.CurrentFirmware,
		UpgradeFirmware: info.UpgradeFirmware,
	}
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
