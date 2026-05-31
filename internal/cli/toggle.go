package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var disableFlags connFlags
var disableAll bool

var disableCmd = &cobra.Command{
	Use:   "disable [адрес...]",
	Short: "Отключить записи или весь список на роутере",
	Long: `Отключает указанные записи (disabled=true) или весь список целиком (--all).
Не изменяет файл — только состояние на роутере.

Примеры:
  mikrotik-lists-manager disable 8.8.8.8 1.1.1.1 -H 192.168.1.1 -u admin -l VPN_LIST
  mikrotik-lists-manager disable --all -H 192.168.1.1 -u admin -l list1,list2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetDisabled(cmd.Context(), args, disableFlags, disableAll, true)
	},
}

var enableFlags connFlags
var enableAll bool

var enableCmd = &cobra.Command{
	Use:   "enable [адрес...]",
	Short: "Включить записи или весь список на роутере",
	Long: `Включает указанные записи (disabled=false) или весь список целиком (--all).
Не изменяет файл — только состояние на роутере.

Примеры:
  mikrotik-lists-manager enable 8.8.8.8 1.1.1.1 -H 192.168.1.1 -u admin -l VPN_LIST
  mikrotik-lists-manager enable --all -H 192.168.1.1 -u admin -l list1,list2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetDisabled(cmd.Context(), args, enableFlags, enableAll, false)
	},
}

func init() {
	for _, cmd := range []*cobra.Command{disableCmd, enableCmd} {
		var f *connFlags
		var all *bool
		if cmd == disableCmd {
			f, all = &disableFlags, &disableAll
		} else {
			f, all = &enableFlags, &enableAll
		}
		cmd.Flags().StringVarP(&f.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
		cmd.Flags().StringVarP(&f.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
		cmd.Flags().StringVarP(&f.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
		cmd.Flags().StringArrayVarP(&f.listNames, "list", "l", nil, "Имя address-list, можно несколько [$MT_LIST]")
		cmd.Flags().BoolVarP(&f.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
		cmd.Flags().BoolVarP(all, "all", "a", false, "Применить ко всему списку")
	}
}

func runSetDisabled(ctx context.Context, args []string, flags connFlags, all, disabled bool) error {
	if !all && len(args) == 0 {
		return fmt.Errorf("укажите адреса или используйте --all")
	}

	host := resolve(flags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(flags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	listNames, err := resolveListNames(flags.listNames, loadedConfig.List)
	if err != nil {
		return err
	}

	pass, err := resolvePassword(flags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(flags.skipTLSVerify))

	targets := map[string]bool{}
	if !all {
		for _, a := range args {
			targets[a] = true
		}
	}

	action := "Отключение"
	if !disabled {
		action = "Включение"
	}

	for _, listName := range listNames {
		entries, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		if all {
			targets = map[string]bool{}
			for _, e := range entries {
				targets[e.Address] = true
			}
		}

		output.Header(fmt.Sprintf("%s записей в %q", action, listName))

		count := 0
		for _, e := range entries {
			if !targets[e.Address] {
				continue
			}
			if e.Disabled.Bool() == disabled {
				continue
			}
			if disabled {
				output.Disable(e.Address, e.Comment)
			} else {
				output.Enable(e.Address, e.Comment)
			}
			if err := client.SetDisabled(ctx, e.ID, disabled); err != nil {
				return err
			}
			count++
		}

		entrySet := map[string]bool{}
		for _, e := range entries {
			entrySet[e.Address] = true
		}
		for addr := range targets {
			if !entrySet[addr] {
				output.Warn(fmt.Sprintf("%s не найден в списке %q", addr, listName))
			}
		}

		if count == 0 {
			output.Info("Все записи уже в нужном состоянии.")
		} else {
			output.Info(fmt.Sprintf("Готово. Изменено %d записей.", count))
		}
	}
	return nil
}
