// Package output handles styled terminal output via lipgloss.
package output

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	green  = lipgloss.Color("10")
	red    = lipgloss.Color("9")
	yellow = lipgloss.Color("11")
	blue   = lipgloss.Color("12")
	cyan   = lipgloss.Color("14")
	gray   = lipgloss.Color("8")
	white  = lipgloss.Color("15")

	styleAdd = lipgloss.NewStyle().Foreground(green).Bold(true)
	styleDel = lipgloss.NewStyle().Foreground(red).Bold(true)
	styleUpd = lipgloss.NewStyle().Foreground(yellow).Bold(true)
	styleDis = lipgloss.NewStyle().Foreground(gray).Bold(true)

	styleAddr     = lipgloss.NewStyle().Foreground(white)
	styleAddrDis  = lipgloss.NewStyle().Foreground(gray).Strikethrough(true)
	styleComment  = lipgloss.NewStyle().Foreground(gray).Italic(true)
	styleOld      = lipgloss.NewStyle().Foreground(red).Strikethrough(true)
	styleNew      = lipgloss.NewStyle().Foreground(green)
	styleArrow    = lipgloss.NewStyle().Foreground(gray)

	styleHeader = lipgloss.NewStyle().
			Foreground(blue).Bold(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderBottom(true).
			BorderForeground(gray)

	styleInfo = lipgloss.NewStyle().Foreground(cyan)
	styleWarn = lipgloss.NewStyle().Foreground(yellow).Bold(true)

	styleErrBox = lipgloss.NewStyle().
			Foreground(red).Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(red).
			Padding(0, 1)

	styleSummaryOk = lipgloss.NewStyle().
			Foreground(green).Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(green).
			Padding(0, 1)

	styleSummaryDry = lipgloss.NewStyle().
			Foreground(yellow).Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(yellow).
			Padding(0, 1)

	styleKey = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	styleVal = lipgloss.NewStyle().Foreground(white)
	styleDim = lipgloss.NewStyle().Foreground(gray)
)

func addrStyle(disabled bool) lipgloss.Style {
	if disabled {
		return styleAddrDis
	}
	return styleAddr
}

func disabledTag(disabled bool) string {
	if disabled {
		return "  " + styleDis.Render("[off]")
	}
	return ""
}

func Add(address, comment string, disabled bool) {
	icon := styleAdd.Render("+")
	addr := addrStyle(disabled).Render(address)
	line := fmt.Sprintf("  %s  %s%s", icon, addr, disabledTag(disabled))
	if comment != "" {
		line += "  " + styleComment.Render("# "+comment)
	}
	fmt.Println(line)
}

// Normalize prints a "/32 → bare IP" conversion line.
func Normalize(from, to string) {
	icon := styleUpd.Render("~")
	line := fmt.Sprintf("  %s  %s%s",
		icon,
		styleOld.Render(from),
		styleArrow.Render(" → ")+styleNew.Render(to),
	)
	fmt.Println(line)
}

func Remove(address, comment string) {
	icon := styleDel.Render("−")
	addr := styleAddr.Render(address)
	line := fmt.Sprintf("  %s  %s", icon, addr)
	if comment != "" {
		line += "  " + styleComment.Render("# "+comment)
	}
	fmt.Println(line)
}

func Update(address, oldComment, newComment string, oldDisabled, newDisabled bool) {
	icon := styleUpd.Render("~")
	addr := addrStyle(newDisabled).Render(address)
	line := fmt.Sprintf("  %s  %s", icon, addr)

	// show disabled state change
	if oldDisabled != newDisabled {
		if newDisabled {
			line += "  " + styleDis.Render("enabled → [off]")
		} else {
			line += "  " + styleNew.Render("[off] → enabled")
		}
	}

	// show comment change
	if oldComment != newComment {
		line += "  " + styleOld.Render(oldComment) +
			styleArrow.Render(" → ") +
			styleNew.Render(newComment)
	}
	fmt.Println(line)
}

// Disable prints a "disabled" action line (for enable/disable commands).
func Disable(address, comment string) {
	icon := styleDis.Render("○")
	addr := styleAddrDis.Render(address)
	line := fmt.Sprintf("  %s  %s", icon, addr)
	if comment != "" {
		line += "  " + styleComment.Render("# "+comment)
	}
	fmt.Println(line)
}

// Enable prints an "enabled" action line.
func Enable(address, comment string) {
	icon := styleNew.Render("●")
	addr := styleAddr.Render(address)
	line := fmt.Sprintf("  %s  %s", icon, addr)
	if comment != "" {
		line += "  " + styleComment.Render("# "+comment)
	}
	fmt.Println(line)
}

func Header(msg string) {
	fmt.Println()
	fmt.Println(styleHeader.Render(msg))
}

func Info(msg string) {
	fmt.Println(styleInfo.Render("  " + msg))
}

func Warn(msg string) {
	fmt.Println(styleWarn.Render("  ⚠  " + msg))
}

func Error(msg string) {
	fmt.Fprintln(os.Stderr, styleErrBox.Render("✗  "+msg))
}

// KV prints a key-value pair, used in config show.
func KV(key, value, hint string) {
	k := styleKey.Render(fmt.Sprintf("%-12s", key))
	v := styleVal.Render(value)
	line := fmt.Sprintf("  %s  %s", k, v)
	if hint != "" {
		line += "  " + styleDim.Render(hint)
	}
	fmt.Println(line)
}

// Summary prints a final result box.
func Summary(added, removed, updated int, dryRun bool) {
	fmt.Println()
	if added+removed+updated == 0 {
		fmt.Println(styleSummaryOk.Render("  ✓  уже синхронизировано  "))
		return
	}

	parts := []string{}
	if added > 0 {
		parts = append(parts, styleAdd.Render(fmt.Sprintf("+%d добавлено", added)))
	}
	if removed > 0 {
		parts = append(parts, styleDel.Render(fmt.Sprintf("−%d удалено", removed)))
	}
	if updated > 0 {
		parts = append(parts, styleUpd.Render(fmt.Sprintf("~%d обновлено", updated)))
	}

	msg := strings.Join(parts, styleDim.Render("  ·  "))
	if dryRun {
		msg += styleDim.Render("  (dry run)")
		fmt.Println(styleSummaryDry.Render("  " + msg + "  "))
	} else {
		fmt.Println(styleSummaryOk.Render("  " + msg + "  "))
	}
}

// RouterBanner prints a styled box with router identity shown at command start.
// firmware may be empty for CHR/x86 devices.
// RouterBannerInfo is the display data for RouterBanner.
type RouterBannerInfo struct {
	Host            string
	BoardName       string
	Version         string
	Architecture    string
	CPU             string
	CPUCount        string
	TotalMemory     string
	FreeMemory      string
	Uptime          string
	// routerboard fields (empty for CHR/x86)
	Model           string
	Revision        string
	SerialNumber    string
	FirmwareType    string
	FactoryFirmware string
	CurrentFirmware string
	UpgradeFirmware string
}

func RouterBanner(r RouterBannerInfo) {
	kv := func(k, v string) string {
		return fmt.Sprintf("  %s  %s",
			styleKey.Render(fmt.Sprintf("%-12s", k)),
			styleVal.Render(v),
		)
	}
	kvDim := func(k, v string) string {
		return fmt.Sprintf("  %s  %s",
			styleKey.Render(fmt.Sprintf("%-12s", k)),
			styleDim.Render(v),
		)
	}
	sep := func() string {
		return styleDim.Render("  " + strings.Repeat("─", 40))
	}

	title := lipgloss.NewStyle().Foreground(white).Bold(true).Render(r.BoardName)
	if r.Host != "" {
		title += styleDim.Render("  ─  " + r.Host)
	}

	lines := []string{title}

	// system/resource section
	rosLine := r.Version
	if r.Architecture != "" {
		rosLine += styleDim.Render("  ·  " + r.Architecture)
	}
	lines = append(lines, kv("RouterOS", rosLine))

	if r.CPU != "" {
		cpu := r.CPU
		if r.CPUCount != "" && r.CPUCount != "1" {
			cpu += styleDim.Render(" ×" + r.CPUCount)
		}
		lines = append(lines, kv("CPU", cpu))
	}

	if r.TotalMemory != "" {
		lines = append(lines, kv("Memory", formatMemory(r.FreeMemory, r.TotalMemory)))
	}

	lines = append(lines, kv("Uptime", r.Uptime))

	// routerboard section
	if r.Model != "" || r.SerialNumber != "" {
		lines = append(lines, sep())
		if r.Model != "" {
			model := r.Model
			if r.Revision != "" {
				model += styleDim.Render("  rev " + r.Revision)
			}
			lines = append(lines, kv("Model", model))
		}
		if r.SerialNumber != "" {
			lines = append(lines, kvDim("Serial", r.SerialNumber))
		}
		if r.CurrentFirmware != "" {
			fw := r.CurrentFirmware
			if r.UpgradeFirmware != "" && r.UpgradeFirmware != r.CurrentFirmware {
				fw += styleWarn.Render("  → " + r.UpgradeFirmware + " available")
			}
			if r.FirmwareType != "" {
				fw += styleDim.Render("  (" + r.FirmwareType + ")")
			}
			lines = append(lines, kv("Firmware", fw))
		}
		if r.FactoryFirmware != "" {
			lines = append(lines, kvDim("Factory FW", r.FactoryFirmware))
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(blue).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))

	fmt.Println()
	fmt.Println(box)
	fmt.Println()
}

func formatMemory(free, total string) string {
	freeB, err1 := strconv.ParseInt(free, 10, 64)
	totalB, err2 := strconv.ParseInt(total, 10, 64)
	if err1 != nil || err2 != nil {
		if total != "" {
			return total + " B"
		}
		return ""
	}
	return fmt.Sprintf("%d MiB free / %d MiB", freeB/1024/1024, totalB/1024/1024)
}

// ListRow prints one row of the `list` command output.
func ListRow(name string, count int, disabled int) {
	n := styleAddr.Render(fmt.Sprintf("%-32s", name))
	c := styleNew.Render(fmt.Sprintf("%4d", count))
	line := fmt.Sprintf("  %s  %s записей", n, c)
	if disabled > 0 {
		line += "  " + styleDis.Render(fmt.Sprintf("(%d off)", disabled))
	}
	fmt.Println(line)
}
