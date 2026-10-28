package entry

// Author represents a contributor to Archive of Formal Proofs.
type Author struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Emails    []string `json:"emails,omitempty"`
	Homepages []string `json:"homepages,omitempty"`
	Link      string   `json:"link"`
}
