package theory

import (
	"errors"
)

var (
	ErrTheoryNotFound  = errors.New("theory file not found")
	ErrRootNotFound    = errors.New("ROOT file not found in entry package")
	ErrInvalidTheory   = errors.New("invalid or empty theory file")
	ErrArchiveDownload = errors.New("failed to fetch package archive")
)

// Session represents an Isabelle session defined in a ROOT file.
type Session struct {
	Name         string   `json:"name"`
	Parent       string   `json:"parent,omitempty"`
	Sessions     []string `json:"sessions,omitempty"` // AFP/Isabelle session dependencies
	Theories     []string `json:"theories,omitempty"` // Exported theories
	Directories  []string `json:"directories,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// RootFile contains the parsed content of an entry's ROOT specification.
type RootFile struct {
	EntryShortname string    `json:"entry_shortname"`
	Chapter        string    `json:"chapter,omitempty"`
	Sessions       []Session `json:"sessions"`
	RawContent     string    `json:"raw_content,omitempty"`
}

// AllTheories returns deduplicated theory names declared in all sessions of this ROOT file.
func (r RootFile) AllTheories() []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range r.Sessions {
		for _, th := range s.Theories {
			if _, exists := seen[th]; !exists {
				seen[th] = struct{}{}
				result = append(result, th)
			}
		}
	}
	return result
}

// AllDependencies returns all session dependencies declared across sessions.
func (r RootFile) AllDependencies() []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range r.Sessions {
		if s.Parent != "" {
			if _, exists := seen[s.Parent]; !exists {
				seen[s.Parent] = struct{}{}
				result = append(result, s.Parent)
			}
		}
		for _, dep := range s.Sessions {
			if _, exists := seen[dep]; !exists {
				seen[dep] = struct{}{}
				result = append(result, dep)
			}
		}
	}
	return result
}
