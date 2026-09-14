package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var infoFlags connFlags
var infoJSON bool

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Показать информацию о роутере",
	Long: `Подключается к MikroTik и выводит: модель, версию RouterOS,
версию прошивки и аптайм.

Примеры:
  mikrotik-lists-manager info -H 192.168.1.1 -u admin
  mikrotik-lists-manager info --json`,
	RunE: runInfo,
}

func init() {
	infoCmd.Flags().StringVarP(&infoFlags.host, "host", "H", "", "Адрес MikroTik [$MT_HOST]")
	infoCmd.Flags().StringVarP(&infoFlags.user, "user", "u", "", "Имя пользователя API [$MT_USER]")
	infoCmd.Flags().StringVarP(&infoFlags.pass, "pass", "p", "", "Пароль API [$MT_PASS]")
	infoCmd.Flags().BoolVarP(&infoFlags.skipTLSVerify, "insecure", "k", false, "Не проверять TLS сертификат")
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Вывод в формате JSON")
}

func runInfo(cmd *cobra.Command, args []string) error {
	host := resolve(infoFlags.host, "MT_HOST", loadedConfig.Host)
	user := resolve(infoFlags.user, "MT_USER", loadedConfig.User)

	if host == "" {
		return fmt.Errorf("--host обязателен")
	}
	if user == "" {
		return fmt.Errorf("--user обязателен")
	}

	pass, err := resolvePassword(infoFlags.pass)
	if err != nil {
		return err
	}

	client := mikrotik.NewClient(host, user, pass, resolveSkipTLS(infoFlags.skipTLSVerify))

	info, err := client.GetRouterInfo(cmd.Context())
	if err != nil {
		return fmt.Errorf("получение информации: %w", err)
	}

	if infoJSON {
		dto := output.RouterInfoDTO{
			BoardName:       info.BoardName,
			Version:         info.Version,
			Uptime:          info.Uptime,
			Architecture:    info.Architecture,
			CPU:             info.CPU,
			CPUCount:        info.CPUCount,
			TotalMemory:     info.TotalMemory,
			FreeMemory:      info.FreeMemory,
			Model:           info.Model,
			Revision:        info.Revision,
			SerialNumber:    info.SerialNumber,
			FirmwareType:    info.FirmwareType,
			FactoryFirmware: info.FactoryFirmware,
			CurrentFirmware: info.CurrentFirmware,
			UpgradeFirmware: info.UpgradeFirmware,
		}
		return output.JSON(dto)
	}

	output.RouterBanner(routerBannerInfo(info, host))
	return nil
}
