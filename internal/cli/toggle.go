package cli

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

const toggleProgressThreshold = 10

var disableFlags connFlags
var disableAll bool
var disableConcurrency int

var disableCmd = &cobra.Command{
	Use:   "disable [адрес...]",
	Short: "Отключить записи или весь список на роутере",
	Long: `Отключает указанные записи (disabled=true) или весь список целиком (--all).
Не изменяет файл — только состояние на роутере.

Примеры:
  mlm disable 8.8.8.8 1.1.1.1 -H 192.168.1.1 -u admin -l VPN_LIST
  mlm disable --all -H 192.168.1.1 -u admin -l list1,list2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetDisabled(cmd.Context(), args, disableFlags, disableAll, true, disableConcurrency)
	},
}

var enableFlags connFlags
var enableAll bool
var enableConcurrency int

var enableCmd = &cobra.Command{
	Use:   "enable [адрес...]",
	Short: "Включить записи или весь список на роутере",
	Long: `Включает указанные записи (disabled=false) или весь список целиком (--all).
Не изменяет файл — только состояние на роутере.

Примеры:
  mlm enable 8.8.8.8 1.1.1.1 -H 192.168.1.1 -u admin -l VPN_LIST
  mlm enable --all -H 192.168.1.1 -u admin -l list1,list2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetDisabled(cmd.Context(), args, enableFlags, enableAll, false, enableConcurrency)
	},
}

func init() {
	for _, cmd := range []*cobra.Command{disableCmd, enableCmd} {
		var f *connFlags
		var all *bool
		var concurrency *int
		if cmd == disableCmd {
			f, all, concurrency = &disableFlags, &disableAll, &disableConcurrency
		} else {
			f, all, concurrency = &enableFlags, &enableAll, &enableConcurrency
		}
		cmd.Flags().StringVarP(&f.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
		cmd.Flags().StringVarP(&f.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
		cmd.Flags().StringVarP(&f.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
		cmd.Flags().StringArrayVarP(&f.listNames, "list", "l", nil, "Имя address-list, можно несколько [$MT_LIST]")
		cmd.Flags().BoolVarP(&f.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
		cmd.Flags().BoolVarP(all, "all", "a", false, "Применить ко всему списку")
		cmd.Flags().IntVarP(concurrency, "concurrency", "c", 5, "Число параллельных запросов к API (0 = последовательно)")
	}
}

func runSetDisabled(ctx context.Context, args []string, flags connFlags, all, disabled bool, concurrency int) error {
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

	client := newClient(host, user, pass, resolveSkipTLS(flags.skipTLSVerify))

	targets := map[string]bool{}
	if !all {
		for _, a := range args {
			targets[parser.NormalizeAddr(a)] = true
		}
	}

	action := "Отключение"
	if !disabled {
		action = "Включение"
	}

	for _, listName := range listNames {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		entries, err := client.GetList(ctx, listName)
		if err != nil {
			return fmt.Errorf("получение списка %q: %w", listName, err)
		}

		listTargets := targets
		if all {
			listTargets = map[string]bool{}
			for _, e := range entries {
				listTargets[parser.NormalizeAddr(e.Address)] = true
			}
		}

		output.Header(fmt.Sprintf("%s записей в %q", action, listName))

		var toChange []mikrotik.AddressListEntry
		for _, e := range entries {
			key := parser.NormalizeAddr(e.Address)
			if listTargets[key] && e.Disabled.Bool() != disabled {
				toChange = append(toChange, e)
			}
		}

		if len(toChange) == 0 {
			output.Info("Все записи уже в нужном состоянии.")
			checkMissingTargets(entries, listTargets, listName, args, all)
			continue
		}

		useProgress := len(toChange) >= toggleProgressThreshold
		var bar *progressbar.ProgressBar
		if useProgress {
			bar = newProgressBar(len(toChange), action+"...")
		}

		limit := concurrency
		if limit <= 0 {
			limit = 1
		}

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(limit)

		var mu sync.Mutex
		var modified atomic.Int64

		for _, e := range toChange {
			if gctx.Err() != nil {
				break
			}
			entry := e
			g.Go(func() error {
				if !useProgress {
					mu.Lock()
					if disabled {
						output.Disable(entry.Address, entry.Comment)
					} else {
						output.Enable(entry.Address, entry.Comment)
					}
					mu.Unlock()
				}

				if err := client.SetDisabled(gctx, entry.ID, disabled); err != nil {
					return fmt.Errorf("%s %s: %w", action, entry.Address, err)
				}

				modified.Add(1)
				if useProgress {
					_ = bar.Add(1)
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			if useProgress {
				fmt.Fprintln(os.Stderr)
			}
			return err
		}
		if useProgress {
			fmt.Fprintln(os.Stderr)
		}

		checkMissingTargets(entries, listTargets, listName, args, all)
		output.Info(fmt.Sprintf("Готово. Изменено %d записей.", modified.Load()))
	}
	return nil
}

func checkMissingTargets(entries []mikrotik.AddressListEntry, targets map[string]bool, listName string, args []string, all bool) {
	if all {
		return
	}
	entrySet := map[string]bool{}
	for _, e := range entries {
		entrySet[parser.NormalizeAddr(e.Address)] = true
	}
	for _, raw := range args {
		if !entrySet[parser.NormalizeAddr(raw)] {
			output.Warn(fmt.Sprintf("%s не найден в списке %q", raw, listName))
		}
	}
}
