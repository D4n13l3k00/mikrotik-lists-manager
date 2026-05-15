package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/optimizer"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
)

var optimizeWrite bool

var optimizeCmd = &cobra.Command{
	Use:   "optimize [file]",
	Short: "Оптимизировать список: удалить дубли и поглощённые подсети",
	Long: `Читает native .list файл и выполняет:
  - удаление дублирующихся адресов и доменов
  - удаление IP/CIDR которые полностью покрываются более широкой подсетью в том же списке

По умолчанию выводит результат в stdout. С флагом --write перезаписывает файл.

Примеры:
  mikrotik-lists-manager optimize list.lst
  mikrotik-lists-manager optimize list.lst --write`,
	Args: cobra.ExactArgs(1),
	RunE: runOptimize,
}

func init() {
	optimizeCmd.Flags().BoolVarP(&optimizeWrite, "write", "w", false, "Перезаписать файл вместо вывода в stdout")
}

func runOptimize(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("чтение файла: %w", err)
	}

	result, err := optimizer.Optimize(string(content))
	if err != nil {
		return fmt.Errorf("оптимизация: %w", err)
	}

	if len(result.Removed) == 0 {
		output.Info("Список уже оптимален, изменений нет.")
		return nil
	}

	output.Header(fmt.Sprintf("Удалено %d записей", len(result.Removed)))
	for _, r := range result.Removed {
		output.Remove(r.Address, r.Reason)
	}

	optimized := optimizer.Render(result.Lines)

	if optimizeWrite {
		if err := os.WriteFile(filePath, []byte(optimized), 0o644); err != nil {
			return fmt.Errorf("запись файла: %w", err)
		}
		output.Info(fmt.Sprintf("Файл %s обновлён.", filePath))
	} else {
		fmt.Println()
		fmt.Print(optimized)
	}
	return nil
}
