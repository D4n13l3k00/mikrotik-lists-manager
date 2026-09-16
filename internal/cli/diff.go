package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/dnsresolver"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/source"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/syncer"
)

var (
	diffFlags          connFlags
	diffFormat         string
	diffJSON           bool
	diffResolveDomains bool
	diffDNS            string
)

var diffCmd = &cobra.Command{
	Use:   "diff <source1> [source2]",
	Short: "Сравнить два списка оффлайн или локальный источник со списком на MikroTik",
	Long: `Сравнивает два источника данных или локальный источник с address-list на MikroTik:

1. Оффлайн-сравнение (2 аргумента):
   mlm diff list1.lst list2.lst
   mlm diff base.lst https://example.com/new.txt
   (list1 считается базовым состоянием, list2 — целевым)

2. Сравнение с роутером (1 аргумент):
   mlm diff list.lst -l vpn-routes -H 192.168.88.1 -u admin

Источниками могут быть локальные пути к файлам, HTTP/HTTPS URL или stdin ('-').`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVarP(&diffFlags.host, "host", "H", "", "Адрес MikroTik (host или host:port) [$MT_HOST]")
	diffCmd.Flags().StringVarP(&diffFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	diffCmd.Flags().StringVarP(&diffFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	diffCmd.Flags().StringArrayVarP(&diffFlags.listNames, "list", "l", nil, "Имя address-list на роутере [$MT_LIST]")
	diffCmd.Flags().BoolVarP(&diffFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	diffCmd.Flags().StringVarP(&diffFormat, "format", "f", "auto", "Формат файлов: auto, native, mikrotik")
	diffCmd.Flags().BoolVar(&diffResolveDomains, "resolve-domains", false, "Разрешать доменные имена в IP-адреса через DNS")
	diffCmd.Flags().StringVar(&diffDNS, "dns", "", "Пользовательский DNS-сервер для резолвинга (например: 1.1.1.1:53)")
	diffCmd.Flags().BoolVar(&diffJSON, "json", false, "Вывод результата в формате JSON")
}

func runDiff(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	effectiveFormat := diffFormat
	if effectiveFormat == "auto" && loadedConfig.DefaultFormat != "" {
		effectiveFormat = loadedConfig.DefaultFormat
	}
	proxyURL := resolveProxy(proxyFlag)

	if len(args) == 2 {
		src1 := args[0]
		src2 := args[1]

		data1, err := source.ReadWithProxy(ctx, src1, proxyURL)
		if err != nil {
			return fmt.Errorf("чтение первого источника %s: %w", src1, err)
		}
		entries1, err := parseContent(data1, effectiveFormat)
		if err != nil {
			return fmt.Errorf("парсинг первого источника %s: %w", src1, err)
		}

		data2, err := source.ReadWithProxy(ctx, src2, proxyURL)
		if err != nil {
			return fmt.Errorf("чтение второго источника %s: %w", src2, err)
		}
		entries2, err := parseContent(data2, effectiveFormat)
		if err != nil {
			return fmt.Errorf("парсинг второго источника %s: %w", src2, err)
		}

		if diffResolveDomains {
			res1, _ := dnsresolver.ResolveEntries(ctx, entries1, diffDNS, proxyURL)
			for _, w := range res1.Warnings {
				output.Warn(w)
			}
			entries1 = res1.Entries

			res2, _ := dnsresolver.ResolveEntries(ctx, entries2, diffDNS, proxyURL)
			for _, w := range res2.Warnings {
				output.Warn(w)
			}
			entries2 = res2.Entries
		}

		current := make([]mikrotik.AddressListEntry, 0, len(entries1))
		for _, e := range entries1 {
			current = append(current, mikrotik.AddressListEntry{
				Address:  e.Address,
				Comment:  e.Comment,
				Disabled: mikrotik.BoolString(e.Disabled),
			})
		}

		changes, duplicates := syncer.Diff(entries2, current)
		return renderDiff(changes, duplicates, fmt.Sprintf("%s ↔ %s", src1, src2), diffJSON)
	}

	// Single argument: compare source vs MikroTik router list
	src := args[0]
	host := resolve(diffFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(diffFlags.user, "MT_USER", loadedConfig.User)
	if host == "" {
		return fmt.Errorf("--host обязателен при сравнении со списком роутера")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен при сравнении со списком роутера")
	}

	listNames, err := resolveListNames(diffFlags.listNames, loadedConfig.List)
	if err != nil {
		return err
	}
	if len(listNames) == 0 {
		return fmt.Errorf("--list обязателен при сравнении со списком роутера")
	}
	listName := listNames[0]

	pass, err := resolvePassword(diffFlags.pass)
	if err != nil {
		return err
	}

	client := newClient(host, user, pass, resolveSkipTLS(diffFlags.skipTLSVerify))
	current, err := client.GetList(ctx, listName)
	if err != nil {
		return fmt.Errorf("получение списка %q с роутера: %w", listName, err)
	}

	data, err := source.ReadWithProxy(ctx, src, proxyURL)
	if err != nil {
		return fmt.Errorf("чтение источника %s: %w", src, err)
	}
	desired, err := parseContent(data, effectiveFormat)
	if err != nil {
		return fmt.Errorf("парсинг источника %s: %w", src, err)
	}

	if diffResolveDomains {
		res, _ := dnsresolver.ResolveEntries(ctx, desired, diffDNS, proxyURL)
		for _, w := range res.Warnings {
			output.Warn(w)
		}
		desired = res.Entries
	}

	changes, duplicates := syncer.Diff(desired, current)
	return renderDiff(changes, duplicates, fmt.Sprintf("%s ↔ %s на %s", src, listName, host), diffJSON)
}

func renderDiff(changes []syncer.Change, duplicates []string, title string, jsonOutput bool) error {
	var addCount, delCount, updCount int
	var dtos []output.DiffChangeDTO

	for _, ch := range changes {
		dto := output.DiffChangeDTO{
			Address: ch.Address,
		}
		switch ch.Action {
		case syncer.ActionAdd:
			addCount++
			dto.Action = "add"
			dto.NewComment = ch.NewComment
			newDis := ch.NewDisabled
			dto.NewDisabled = &newDis
		case syncer.ActionDelete:
			delCount++
			dto.Action = "delete"
			dto.OldComment = ch.OldComment
		case syncer.ActionUpdate:
			updCount++
			dto.Action = "update"
			dto.OldComment = ch.OldComment
			dto.NewComment = ch.NewComment
			oldDis := ch.OldDisabled
			newDis := ch.NewDisabled
			dto.OldDisabled = &oldDis
			dto.NewDisabled = &newDis
		}
		dtos = append(dtos, dto)
	}

	if jsonOutput {
		report := output.DiffReportDTO{
			Changes: dtos,
			Summary: output.DiffSummaryDTO{
				Total:  len(changes),
				Add:    addCount,
				Delete: delCount,
				Update: updCount,
			},
		}
		return output.JSON(report)
	}

	output.Header(fmt.Sprintf("Diff: %s", title))

	for _, addr := range duplicates {
		output.Warn(fmt.Sprintf("дубль в целевом списке: %s", addr))
	}

	if len(changes) == 0 {
		output.Info("Списки идентичны (изменений нет).")
		return nil
	}

	for _, ch := range changes {
		switch ch.Action {
		case syncer.ActionAdd:
			output.Add(ch.Address, ch.NewComment, ch.NewDisabled)
		case syncer.ActionDelete:
			output.Remove(ch.Address, ch.OldComment)
		case syncer.ActionUpdate:
			output.Update(ch.Address, ch.OldComment, ch.NewComment, ch.OldDisabled, ch.NewDisabled)
		}
	}

	output.Summary(addCount, delCount, updCount, true)
	return nil
}
