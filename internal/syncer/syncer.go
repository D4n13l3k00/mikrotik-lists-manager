package syncer

import (
	"fmt"
	"os"

	"github.com/schollz/progressbar/v3"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/output"
	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/parser"
)

// progressThreshold is the minimum number of changes before a progress bar is shown.
const progressThreshold = 10

type Action int

const (
	ActionAdd Action = iota
	ActionDelete
	ActionUpdate // comment or disabled changed
)

// Change describes a single planned change.
type Change struct {
	Action      Action
	Address     string
	OldComment  string
	NewComment  string
	OldDisabled bool
	NewDisabled bool
	ID          string // MikroTik .id, set for Delete/Update
}

// APIClient is the subset of mikrotik.Client used by Apply.
type APIClient interface {
	AddEntry(listName, address, comment string, disabled bool) error
	UpdateEntry(id, comment string, disabled bool) error
	DeleteEntry(id string) error
}

// Diff computes what needs to change to make MikroTik match desired.
func Diff(desired []parser.Entry, current []mikrotik.AddressListEntry) []Change {
	currentMap := make(map[string]mikrotik.AddressListEntry, len(current))
	for _, e := range current {
		currentMap[e.Address] = e
	}
	desiredMap := make(map[string]parser.Entry, len(desired))
	for _, e := range desired {
		desiredMap[e.Address] = e
	}

	var changes []Change

	for _, want := range desired {
		if have, exists := currentMap[want.Address]; exists {
			if have.Comment != want.Comment || have.Disabled.Bool() != want.Disabled {
				changes = append(changes, Change{
					Action:      ActionUpdate,
					Address:     want.Address,
					OldComment:  have.Comment,
					NewComment:  want.Comment,
					OldDisabled: have.Disabled.Bool(),
					NewDisabled: want.Disabled,
					ID:          have.ID,
				})
			}
		} else {
			changes = append(changes, Change{
				Action:      ActionAdd,
				Address:     want.Address,
				NewComment:  want.Comment,
				NewDisabled: want.Disabled,
			})
		}
	}

	for _, have := range current {
		if _, wanted := desiredMap[have.Address]; !wanted {
			changes = append(changes, Change{
				Action:  ActionDelete,
				Address: have.Address,
				ID:      have.ID,
			})
		}
	}

	return changes
}

// Apply executes the changes against MikroTik. If dryRun is true, only prints.
// When len(changes) >= progressThreshold, shows a progress bar.
// If verbose is true, per-entry lines are printed above the bar without interleaving.
func Apply(client APIClient, listName string, changes []Change, dryRun, verbose bool) error {
	if len(changes) == 0 {
		output.Summary(0, 0, 0, dryRun)
		return nil
	}

	useProgress := len(changes) >= progressThreshold && !dryRun

	var bar *progressbar.ProgressBar
	if useProgress {
		bar = progressbar.NewOptions(len(changes),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionSetWidth(40),
			progressbar.OptionShowCount(),
			progressbar.OptionSetDescription("[cyan]Применение изменений...[reset]"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
	}

	// printEntry clears the bar, prints the line, then redraws the bar so
	// text and bar never appear on the same line.
	printEntry := func(fn func()) {
		if useProgress && verbose {
			bar.Clear()        //nolint:errcheck
			fn()
			bar.RenderBlank() //nolint:errcheck
		} else if !useProgress {
			fn()
		}
		// useProgress && !verbose: skip per-entry output entirely
	}

	var added, removed, updated int

	for _, ch := range changes {
		switch ch.Action {
		case ActionAdd:
			printEntry(func() { output.Add(ch.Address, ch.NewComment, ch.NewDisabled) })
			if !dryRun {
				if err := client.AddEntry(listName, ch.Address, ch.NewComment, ch.NewDisabled); err != nil {
					return fmt.Errorf("add %s: %w", ch.Address, err)
				}
			}
			added++
		case ActionDelete:
			printEntry(func() { output.Remove(ch.Address, "") })
			if !dryRun {
				if err := client.DeleteEntry(ch.ID); err != nil {
					return fmt.Errorf("delete %s: %w", ch.Address, err)
				}
			}
			removed++
		case ActionUpdate:
			printEntry(func() {
				output.Update(ch.Address, ch.OldComment, ch.NewComment, ch.OldDisabled, ch.NewDisabled)
			})
			if !dryRun {
				if err := client.UpdateEntry(ch.ID, ch.NewComment, ch.NewDisabled); err != nil {
					return fmt.Errorf("update %s: %w", ch.Address, err)
				}
			}
			updated++
		}
		if useProgress {
			bar.Add(1) //nolint:errcheck
		}
	}

	if useProgress {
		fmt.Fprintln(os.Stderr)
	}

	output.Summary(added, removed, updated, dryRun)
	return nil
}
