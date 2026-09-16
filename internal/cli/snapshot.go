package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/snapshot"
)

var (
	snapshotFlags connFlags
	snapshotJSON  bool
	snapshotID    string
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Управление снимками (снэпшотами) списков адресов",
	Long: `Позволяет просматривать, создавать вручную и удалять снимки списков адресов.
Снимки сохраняются локально в конфигурационной директории пользователя
и могут быть использованы для мгновенного отката командой rollback.`,
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "Показать сохраненные снимки для списка",
	Long: `Выводит список всех доступных снимков для указанного роутера и address-list.

Примеры:
  mlm snapshot list -l vpn-routes -H 192.168.88.1
  mlm snapshot list -l vpn-routes --json`,
	RunE: runSnapshotList,
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создать снимок текущего состояния списка с роутера вручную",
	Long: `Подключается к роутеру, считывает текущий address-list и сохраняет локальный снимок.

Примеры:
  mlm snapshot create -l vpn-routes -H 192.168.88.1 -u admin`,
	RunE: runSnapshotCreate,
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete <snapshot-id>",
	Short: "Удалить снимок по ID",
	Long: `Удаляет локальный файл снимка по его ID (или префиксу ID).

Примеры:
  mlm snapshot delete 20260916-120000_abcd -l vpn-routes -H 192.168.88.1`,
	Args: cobra.ExactArgs(1),
	RunE: runSnapshotDelete,
}

func init() {
	snapshotCmd.PersistentFlags().StringVarP(&snapshotFlags.host, "host", "H", "", "Адрес MikroTik (host или host:port) [$MT_HOST]")
	snapshotCmd.PersistentFlags().StringVarP(&snapshotFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	snapshotCmd.PersistentFlags().StringVarP(&snapshotFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	snapshotCmd.PersistentFlags().StringArrayVarP(&snapshotFlags.listNames, "list", "l", nil, "Имя address-list [$MT_LIST]")
	snapshotCmd.PersistentFlags().BoolVarP(&snapshotFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")

	snapshotListCmd.Flags().BoolVar(&snapshotJSON, "json", false, "Вывод в формате JSON")

	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
}

func resolveSnapshotTarget() (string, string, string, error) {
	host := resolve(snapshotFlags.host, "MT_HOST", loadedConfig.Host)
	if host == "" {
		return "", "", "", fmt.Errorf("--host обязателен")
	}

	listNames, err := resolveListNames(snapshotFlags.listNames, loadedConfig.List)
	if err != nil {
		return "", "", "", err
	}
	if len(listNames) == 0 {
		return "", "", "", fmt.Errorf("--list обязателен")
	}

	user := resolve(snapshotFlags.user, "MT_USER", loadedConfig.User)
	return host, user, listNames[0], nil
}

func runSnapshotList(cmd *cobra.Command, args []string) error {
	host, _, listName, err := resolveSnapshotTarget()
	if err != nil {
		return err
	}

	metas, err := snapshot.List(host, listName)
	if err != nil {
		return fmt.Errorf("чтение снимков: %w", err)
	}

	if snapshotJSON {
		dtos := make([]output.SnapshotDTO, 0, len(metas))
		for _, m := range metas {
			dtos = append(dtos, output.SnapshotDTO{
				ID:        m.ID,
				Host:      m.Host,
				ListName:  m.ListName,
				CreatedAt: m.CreatedAt.Format(time.RFC3339),
				Total:     m.Total,
			})
		}
		return output.JSON(dtos)
	}

	output.Header(fmt.Sprintf("Снимки списка %q на %s", listName, host))

	if len(metas) == 0 {
		output.Info("Сохранённых снимков нет.")
		return nil
	}

	for _, m := range metas {
		timeStr := m.CreatedAt.Format("2006-01-02 15:04:05")
		output.KV(m.ID, fmt.Sprintf("%d записей (%s)", m.Total, timeStr), "")
	}

	fmt.Println()
	return nil
}

func runSnapshotCreate(cmd *cobra.Command, args []string) error {
	host, user, listName, err := resolveSnapshotTarget()
	if err != nil {
		return err
	}

	if user == "" {
		return fmt.Errorf("--user обязателен для создания снимка с роутера")
	}

	pass, err := resolvePassword(snapshotFlags.pass)
	if err != nil {
		return err
	}

	client := newClient(host, user, pass, resolveSkipTLS(snapshotFlags.skipTLSVerify))
	ctx := cmd.Context()

	output.Header(fmt.Sprintf("Создание снимка списка %q на %s", listName, host))

	current, err := client.GetList(ctx, listName)
	if err != nil {
		return fmt.Errorf("получение списка с роутера: %w", err)
	}

	meta, err := snapshot.Save(host, listName, current)
	if err != nil {
		return fmt.Errorf("сохранение снимка: %w", err)
	}

	output.Info(fmt.Sprintf("Снимок сохранён: %s (записей: %d)", meta.ID, meta.Total))
	return nil
}

func runSnapshotDelete(cmd *cobra.Command, args []string) error {
	host, _, listName, err := resolveSnapshotTarget()
	if err != nil {
		return err
	}

	id := args[0]
	if err := snapshot.Delete(host, listName, id); err != nil {
		return err
	}

	output.Info(fmt.Sprintf("Снимок %s удалён.", id))
	return nil
}
