package search

// Query represents a search and filter request across the AFP catalog.
type Query struct {
	Text     string `json:"text"`
	Topic    string `json:"topic,omitempty"`
	Author   string `json:"author,omitempty"`
	YearFrom int    `json:"year_from,omitempty"`
	YearTo   int    `json:"year_to,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	Format   string `json:"format,omitempty"` // "compact" (default for agents), "detailed", "json"
}

// Normalize ensures safe defaults for limit and offset.
func (q *Query) Normalize() {
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Format == "" {
		q.Format = "compact"
	}
}
