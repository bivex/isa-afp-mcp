package entry

import "strings"

// Topic represents an entry in the AFP hierarchical classification tree.
type Topic struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

// Category returns the top-level domain (e.g. "Computer science", "Mathematics", "Logic").
func (t Topic) Category() string {
	parts := strings.Split(t.Name, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return t.Name
}

// Subcategory returns the specific sub-topic if available.
func (t Topic) Subcategory() string {
	parts := strings.Split(t.Name, "/")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "/")
	}
	return ""
}
