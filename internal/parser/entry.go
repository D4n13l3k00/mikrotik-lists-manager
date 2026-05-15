package parser

// Entry represents a single address-list entry.
// Comment is the MikroTik-visible comment (from ## prefix or inline ##).
// HumanNote is a local-only annotation (from # prefix or inline #), never sent to MikroTik.
// Disabled maps to the ! prefix — entry is synced as disabled=true on MikroTik.
type Entry struct {
	Address   string
	Comment   string // sent to MikroTik
	HumanNote string // local only
	Disabled  bool   // ! prefix → disabled=true on MikroTik
}
