package output

// ListSummaryDTO describes summary information about an address list on the router.
type ListSummaryDTO struct {
	Name     string `json:"name"`
	Total    int    `json:"total"`
	Disabled int    `json:"disabled"`
}

// EntryDTO describes an individual entry in an address list.
type EntryDTO struct {
	Address  string `json:"address"`
	Comment  string `json:"comment,omitempty"`
	Disabled bool   `json:"disabled"`
}

// MatchType defines how a search target matched an entry.
type MatchType string

const (
	MatchExact  MatchType = "exact"
	MatchSubnet MatchType = "subnet"
)

// FindResultDTO describes an address-list match found on the router.
type FindResultDTO struct {
	List      string    `json:"list"`
	Address   string    `json:"address"`
	Comment   string    `json:"comment,omitempty"`
	Disabled  bool      `json:"disabled"`
	MatchType MatchType `json:"match_type"`
}

// RouterInfoDTO represents structured router hardware and system information.
type RouterInfoDTO struct {
	BoardName       string `json:"board_name"`
	Version         string `json:"version"`
	Uptime          string `json:"uptime"`
	Architecture    string `json:"architecture,omitempty"`
	CPU             string `json:"cpu,omitempty"`
	CPUCount        string `json:"cpu_count,omitempty"`
	TotalMemory     string `json:"total_memory,omitempty"`
	FreeMemory      string `json:"free_memory,omitempty"`
	Model           string `json:"model,omitempty"`
	Revision        string `json:"revision,omitempty"`
	SerialNumber    string `json:"serial_number,omitempty"`
	FirmwareType    string `json:"firmware_type,omitempty"`
	FactoryFirmware string `json:"factory_firmware,omitempty"`
	CurrentFirmware string `json:"current_firmware,omitempty"`
	UpgradeFirmware string `json:"upgrade_firmware,omitempty"`
}

// DiffChangeDTO describes an individual change in a diff.
type DiffChangeDTO struct {
	Action      string `json:"action"` // "add", "delete", "update"
	Address     string `json:"address"`
	OldComment  string `json:"old_comment,omitempty"`
	NewComment  string `json:"new_comment,omitempty"`
	OldDisabled *bool  `json:"old_disabled,omitempty"`
	NewDisabled *bool  `json:"new_disabled,omitempty"`
}

// DiffSummaryDTO describes aggregated counts of changes in a diff.
type DiffSummaryDTO struct {
	Total  int `json:"total"`
	Add    int `json:"add"`
	Delete int `json:"delete"`
	Update int `json:"update"`
}

// DiffReportDTO represents a full diff result in JSON format.
type DiffReportDTO struct {
	Changes []DiffChangeDTO `json:"changes"`
	Summary DiffSummaryDTO  `json:"summary"`
}

// SnapshotDTO represents snapshot metadata in JSON format.
type SnapshotDTO struct {
	ID        string `json:"id"`
	Host      string `json:"host"`
	ListName  string `json:"list_name"`
	CreatedAt string `json:"created_at"`
	Total     int    `json:"total"`
}

