package entry

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEntryNotFound  = errors.New("entry not found")
	ErrTopicNotFound  = errors.New("topic not found")
	ErrAuthorNotFound = errors.New("author not found")
	ErrInvalidEntryID = errors.New("invalid entry ID")
)

// RelatedReference represents a bibliographic or related citation reference in AFP.
type RelatedReference struct {
	HTML string `json:"html,omitempty"`
	DOI  string `json:"doi,omitempty"`
}

// Entry is the primary aggregate root representing a formal verification entry in AFP.
type Entry struct {
	Shortname  string             `json:"shortname"`
	Title      string             `json:"title"`
	Authors    []string           `json:"authors"`
	Date       string             `json:"date"`
	Year       string             `json:"year"`
	Topics     []string           `json:"topics"`
	TopicLinks []string           `json:"topic_links,omitempty"`
	Abstract   string             `json:"abstract"`
	Link       string             `json:"link"`
	Permalink  string             `json:"permalink"`
	Related    []RelatedReference `json:"related,omitempty"`
}

// ID returns the unique canonical shortname identifier.
func (e Entry) ID() string {
	return e.Shortname
}

// ReleaseTarballURL returns the direct download link for the entry's verified package archive.
func (e Entry) ReleaseTarballURL() string {
	return "https://isa-afp.org/release/afp-" + e.Shortname + "-current.tar.gz"
}

// ParsedDate parses the entry release date.
func (e Entry) ParsedDate() (time.Time, error) {
	return time.Parse("2006-01-02", e.Date)
}

// MatchesKeyword checks if entry matches a keyword in shortname, title, or topics (case-insensitive).
func (e Entry) MatchesKeyword(kw string) bool {
	q := strings.ToLower(kw)
	if strings.Contains(strings.ToLower(e.Shortname), q) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Title), q) {
		return true
	}
	for _, t := range e.Topics {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
