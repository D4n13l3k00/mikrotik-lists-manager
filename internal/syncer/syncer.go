package syncer

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"

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
	AddEntry(ctx context.Context, listName, address, comment string, disabled bool) error
	UpdateEntry(ctx context.Context, id, comment string, disabled bool) error
	DeleteEntry(ctx context.Context, id string) error
}

// normalizeAddr canonicalizes an IP/CIDR address for comparison.
// Bare IPs are expanded to /32 (IPv4) or /128 (IPv6) so that "8.8.8.8" and
// "8.8.8.8/32" resolve to the same key. Non-IP values (domains, MACs) are
// returned unchanged.
func normalizeAddr(s string) string {
	if strings.Contains(s, "/") {
		ip, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			return s
		}
		ones, _ := ipnet.Mask.Size()
		return fmt.Sprintf("%s/%d", ip.String(), ones)
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	if ip.To4() != nil {
		return s + "/32"
	}
	return s + "/128"
}

// Diff computes what needs to change to make MikroTik match desired.
// Returns changes and a list of duplicate addresses found in desired.
// Addresses are normalized before comparison so that "8.8.8.8" and "8.8.8.8/32"
// are treated as the same entry.
func Diff(desired []parser.Entry, current []mikrotik.AddressListEntry) ([]Change, []string) {
	currentMap := make(map[string]mikrotik.AddressListEntry, len(current))
	for _, e := range current {
		currentMap[normalizeAddr(e.Address)] = e
	}
	desiredMap := make(map[string]parser.Entry, len(desired))
	var duplicates []string
	for _, e := range desired {
		key := normalizeAddr(e.Address)
		if _, exists := desiredMap[key]; exists {
			duplicates = append(duplicates, e.Address)
		}
		desiredMap[key] = e
	}

	var changes []Change

	for _, want := range desired {
		if have, exists := currentMap[normalizeAddr(want.Address)]; exists {
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
		if _, wanted := desiredMap[normalizeAddr(have.Address)]; !wanted {
			changes = append(changes, Change{
				Action:  ActionDelete,
				Address: have.Address,
				ID:      have.ID,
			})
		}
	}

	return changes, duplicates
}

// Apply executes the changes against MikroTik. If dryRun is true, only prints.
// When len(changes) >= progressThreshold, shows a progress bar.
// If verbose is true, per-entry lines are printed above the bar without interleaving.
// concurrency controls how many API requests run in parallel (0 = sequential).
func Apply(ctx context.Context, client APIClient, listName string, changes []Change, dryRun, verbose bool, concurrency int) error {
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

	var mu sync.Mutex
	// printEntry clears the bar, prints the line, then redraws the bar so
	// text and bar never appear on the same line.
	printEntry := func(fn func()) {
		mu.Lock()
		defer mu.Unlock()
		if useProgress && verbose {
			bar.Clear()        //nolint:errcheck
			fn()
			bar.RenderBlank() //nolint:errcheck
		} else if !useProgress {
			fn()
		}
		// useProgress && !verbose: skip per-entry output entirely
	}

	var added, removed, updated atomic.Int64

	g, gctx := errgroup.WithContext(ctx)
	if concurrency > 0 {
		g.SetLimit(concurrency)
	}

	for _, ch := range changes {
		g.Go(func() error {
			switch ch.Action {
			case ActionAdd:
				printEntry(func() { output.Add(ch.Address, ch.NewComment, ch.NewDisabled) })
				if !dryRun {
					if err := client.AddEntry(gctx, listName, ch.Address, ch.NewComment, ch.NewDisabled); err != nil {
						return fmt.Errorf("add %s: %w", ch.Address, err)
					}
				}
				added.Add(1)
			case ActionDelete:
				printEntry(func() { output.Remove(ch.Address, "") })
				if !dryRun {
					if err := client.DeleteEntry(gctx, ch.ID); err != nil {
						return fmt.Errorf("delete %s: %w", ch.Address, err)
					}
				}
				removed.Add(1)
			case ActionUpdate:
				printEntry(func() {
					output.Update(ch.Address, ch.OldComment, ch.NewComment, ch.OldDisabled, ch.NewDisabled)
				})
				if !dryRun {
					if err := client.UpdateEntry(gctx, ch.ID, ch.NewComment, ch.NewDisabled); err != nil {
						return fmt.Errorf("update %s: %w", ch.Address, err)
					}
				}
				updated.Add(1)
			}
			if useProgress {
				bar.Add(1) //nolint:errcheck
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

	output.Summary(int(added.Load()), int(removed.Load()), int(updated.Load()), dryRun)
	return nil
}
