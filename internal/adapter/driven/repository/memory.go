package repository

import (
	"strings"
	"sync"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
)

// MemoryCatalogRepository provides fast in-memory access to catalog entries.
type MemoryCatalogRepository struct {
	entriesMap  map[string]entry.Entry
	entriesList []entry.Entry
	topics      []entry.Topic
	authors     []entry.Author
	keywords    []entry.Keyword
	lastUpdated time.Time
	mu          sync.RWMutex
}

var _ port.CatalogRepository = (*MemoryCatalogRepository)(nil)

// NewMemoryCatalogRepository creates an empty in-memory repository.
func NewMemoryCatalogRepository() *MemoryCatalogRepository {
	return &MemoryCatalogRepository{
		entriesMap: make(map[string]entry.Entry),
	}
}

// SaveAll replaces the repository content atomically.
func (r *MemoryCatalogRepository) SaveAll(
	entries []entry.Entry,
	topics []entry.Topic,
	authors []entry.Author,
	keywords []entry.Keyword,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entriesMap := make(map[string]entry.Entry, len(entries))
	for _, e := range entries {
		entriesMap[e.Shortname] = e
		entriesMap[strings.ToLower(e.Shortname)] = e
	}

	r.entriesMap = entriesMap
	r.entriesList = entries
	r.topics = topics
	r.authors = authors
	r.keywords = keywords
	r.lastUpdated = time.Now()

	return nil
}

// GetEntry retrieves an entry by case-insensitive shortname.
func (r *MemoryCatalogRepository) GetEntry(id string) (*entry.Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	e, ok := r.entriesMap[id]
	if !ok {
		e, ok = r.entriesMap[strings.ToLower(id)]
	}
	if !ok {
		return nil, false
	}
	return &e, true
}

// GetAllEntries returns a copy of all entries.
func (r *MemoryCatalogRepository) GetAllEntries() []entry.Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]entry.Entry, len(r.entriesList))
	copy(res, r.entriesList)
	return res
}

// GetTopics returns all topics.
func (r *MemoryCatalogRepository) GetTopics() []entry.Topic {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]entry.Topic, len(r.topics))
	copy(res, r.topics)
	return res
}

// GetAuthors returns all authors.
func (r *MemoryCatalogRepository) GetAuthors() []entry.Author {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]entry.Author, len(r.authors))
	copy(res, r.authors)
	return res
}

// GetKeywords returns all keywords.
func (r *MemoryCatalogRepository) GetKeywords() []entry.Keyword {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]entry.Keyword, len(r.keywords))
	copy(res, r.keywords)
	return res
}

// Count returns the number of indexed entries.
func (r *MemoryCatalogRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entriesList)
}

// LastUpdated returns the timestamp of the last catalog update.
func (r *MemoryCatalogRepository) LastUpdated() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastUpdated
}
