package snapshot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/D4n13l3k00/mikrotik-lists-manager/internal/mikrotik"
)

const (
	maxSnapshotsPerList = 10
)

// SnapshotMeta contains metadata about a saved snapshot.
type SnapshotMeta struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	ListName  string    `json:"list_name"`
	CreatedAt time.Time `json:"created_at"`
	Total     int       `json:"total"`
	Path      string    `json:"path,omitempty"`
}

// SnapshotData contains full snapshot information including entries.
type SnapshotData struct {
	SnapshotMeta
	Entries []mikrotik.AddressListEntry `json:"entries"`
}

// BaseDir returns the root directory for storing snapshots.
func BaseDir() (string, error) {
	if custom := os.Getenv("MT_SNAPSHOTS_DIR"); custom != "" {
		return custom, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", fmt.Errorf("определение директории конфигурации: %w", err)
		}
		newPath := filepath.Join(home, ".mlm", "snapshots")
		oldPath := filepath.Join(home, ".mikrotik-lists-manager", "snapshots")
		if _, statErr := os.Stat(oldPath); statErr == nil {
			if _, newStatErr := os.Stat(newPath); os.IsNotExist(newStatErr) {
				return oldPath, nil
			}
		}
		return newPath, nil
	}
	newDir := filepath.Join(configDir, "mlm", "snapshots")
	oldDir := filepath.Join(configDir, "mikrotik-lists-manager", "snapshots")
	if _, statErr := os.Stat(oldDir); statErr == nil {
		if _, newStatErr := os.Stat(newDir); os.IsNotExist(newStatErr) {
			return oldDir, nil
		}
	}
	return newDir, nil
}

// sanitizeFilename replaces characters forbidden in directory/file names.
func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(
		":", "_",
		"/", "_",
		"\\", "_",
		"?", "_",
		"*", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(s)
}

func listDir(host, listName string) (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, sanitizeFilename(host), sanitizeFilename(listName)), nil
}

// Save creates a new snapshot file for the given host and listName,
// and prunes older snapshots exceeding maxSnapshotsPerList.
func Save(host, listName string, entries []mikrotik.AddressListEntry) (*SnapshotMeta, error) {
	dir, err := listDir(host, listName)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("создание директории снэпшотов: %w", err)
	}

	now := time.Now()
	randBytes := make([]byte, 2)
	_, _ = rand.Read(randBytes)
	id := fmt.Sprintf("%s_%s", now.Format("20060102-150405"), hex.EncodeToString(randBytes))

	meta := SnapshotMeta{
		ID:        id,
		Host:      host,
		ListName:  listName,
		CreatedAt: now,
		Total:     len(entries),
	}

	data := SnapshotData{
		SnapshotMeta: meta,
		Entries:      entries,
	}

	filePath := filepath.Join(dir, id+".json")
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("маршалинг снэпшота: %w", err)
	}

	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return nil, fmt.Errorf("запись снэпшота: %w", err)
	}

	meta.Path = filePath

	// Auto-prune older snapshots
	_ = Prune(host, listName, maxSnapshotsPerList)

	return &meta, nil
}

// List returns all available snapshots for host and listName, ordered newest first.
func List(host, listName string) ([]SnapshotMeta, error) {
	dir, err := listDir(host, listName)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("чтение директории снэпшотов: %w", err)
	}

	var metas []SnapshotMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var snap SnapshotData
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		snap.SnapshotMeta.Path = filePath
		metas = append(metas, snap.SnapshotMeta)
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})

	return metas, nil
}

// Load loads snapshot data by ID (or latest if id is "" or "latest").
// ID can also be a prefix of the snapshot ID.
func Load(host, listName, id string) (*SnapshotData, error) {
	metas, err := List(host, listName)
	if err != nil {
		return nil, err
	}
	if len(metas) == 0 {
		return nil, fmt.Errorf("снэпшоты для списка %q на %s не найдены", listName, host)
	}

	var targetMeta *SnapshotMeta
	if id == "" || id == "latest" {
		targetMeta = &metas[0]
	} else {
		for i, m := range metas {
			if m.ID == id || strings.HasPrefix(m.ID, id) {
				targetMeta = &metas[i]
				break
			}
		}
	}

	if targetMeta == nil {
		return nil, fmt.Errorf("снэпшот с ID %q не найден", id)
	}

	data, err := os.ReadFile(targetMeta.Path)
	if err != nil {
		return nil, fmt.Errorf("чтение файла снэпшота: %w", err)
	}

	var snap SnapshotData
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("декодирование снэпшота: %w", err)
	}
	snap.SnapshotMeta.Path = targetMeta.Path
	return &snap, nil
}

// Delete removes a specific snapshot file.
func Delete(host, listName, id string) error {
	metas, err := List(host, listName)
	if err != nil {
		return err
	}
	for _, m := range metas {
		if m.ID == id || strings.HasPrefix(m.ID, id) {
			if err := os.Remove(m.Path); err != nil {
				return fmt.Errorf("удаление снэпшота: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("снэпшот %q не найден", id)
}

// Prune keeps only the most recent keepCount snapshots and removes the rest.
func Prune(host, listName string, keepCount int) error {
	metas, err := List(host, listName)
	if err != nil {
		return err
	}
	if len(metas) <= keepCount {
		return nil
	}

	for _, m := range metas[keepCount:] {
		_ = os.Remove(m.Path)
	}
	return nil
}
