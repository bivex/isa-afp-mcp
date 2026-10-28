package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
)

const (
	cacheKeyEntries  = "entries.json"
	cacheKeyTopics   = "topics.json"
	cacheKeyAuthors  = "authors.json"
	cacheKeyKeywords = "keywords.json"
)

// CatalogService implements entry, topic, keyword, and sync use cases.
type CatalogService struct {
	repo          port.CatalogRepository
	afpClient     port.AFPClient
	searchIndex   port.SearchIndex
	diskCache     port.DiskCache
	theoryService port.TheoryUseCase
	logger        *slog.Logger
	mu            sync.Mutex
	ready         bool
	source        string
}

var (
	_ port.EntryUseCase       = (*CatalogService)(nil)
	_ port.TopicUseCase       = (*CatalogService)(nil)
	_ port.KeywordUseCase     = (*CatalogService)(nil)
	_ port.CatalogSyncUseCase = (*CatalogService)(nil)
)

// NewCatalogService creates a new CatalogService.
func NewCatalogService(
	repo port.CatalogRepository,
	afpClient port.AFPClient,
	searchIndex port.SearchIndex,
	diskCache port.DiskCache,
	theoryService port.TheoryUseCase,
	logger *slog.Logger,
) *CatalogService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CatalogService{
		repo:          repo,
		afpClient:     afpClient,
		searchIndex:   searchIndex,
		diskCache:     diskCache,
		theoryService: theoryService,
		logger:        logger,
	}
}

// Init attempts to load catalog data from disk cache first, falling back to upstream sync.
func (s *CatalogService) Init(ctx context.Context, forceSync bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !forceSync && s.tryLoadFromDisk() {
		s.ready = true
		s.source = "disk_cache"
		s.logger.Info("Catalog loaded from persistent disk cache",
			"entries", s.repo.Count(),
			"topics", len(s.repo.GetTopics()),
			"keywords", len(s.repo.GetKeywords()),
		)
		return nil
	}

	return s.syncLocked(ctx)
}

func (s *CatalogService) tryLoadFromDisk() bool {
	if s.diskCache == nil {
		return false
	}

	rawEntries, okE, errE := s.diskCache.Get(cacheKeyEntries)
	rawTopics, okT, errT := s.diskCache.Get(cacheKeyTopics)
	rawAuthors, okA, errA := s.diskCache.Get(cacheKeyAuthors)
	rawKeywords, okK, errK := s.diskCache.Get(cacheKeyKeywords)

	if !okE || !okT || !okA || !okK || errE != nil || errT != nil || errA != nil || errK != nil {
		return false
	}

	var entries []entry.Entry
	var topics []entry.Topic
	var authors []entry.Author
	var keywords []entry.Keyword

	if err := json.Unmarshal(rawEntries, &entries); err != nil {
		return false
	}
	if err := json.Unmarshal(rawTopics, &topics); err != nil {
		return false
	}
	if err := json.Unmarshal(rawAuthors, &authors); err != nil {
		return false
	}
	if err := json.Unmarshal(rawKeywords, &keywords); err != nil {
		return false
	}

	if len(entries) == 0 {
		return false
	}

	_ = s.repo.SaveAll(entries, topics, authors, keywords)
	if s.searchIndex != nil {
		_ = s.searchIndex.Build(entries)
	}

	return true
}

func (s *CatalogService) syncLocked(ctx context.Context) error {
	s.logger.Info("Syncing AFP catalog from upstream isa-afp.org...")

	type fetchResult struct {
		entries  []entry.Entry
		topics   []entry.Topic
		authors  []entry.Author
		keywords []entry.Keyword
		err      error
	}

	resCh := make(chan fetchResult, 1)

	go func() {
		entries, err := s.afpClient.FetchEntries(ctx)
		if err != nil {
			resCh <- fetchResult{err: fmt.Errorf("fetch entries: %w", err)}
			return
		}
		topics, err := s.afpClient.FetchTopics(ctx)
		if err != nil {
			resCh <- fetchResult{err: fmt.Errorf("fetch topics: %w", err)}
			return
		}
		authors, err := s.afpClient.FetchAuthors(ctx)
		if err != nil {
			resCh <- fetchResult{err: fmt.Errorf("fetch authors: %w", err)}
			return
		}
		keywords, err := s.afpClient.FetchKeywords(ctx)
		if err != nil {
			// Keywords failure is non-fatal: fallback to empty list
			keywords = nil
		}

		resCh <- fetchResult{
			entries:  entries,
			topics:   topics,
			authors:  authors,
			keywords: keywords,
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resCh:
		if res.err != nil {
			// If upstream fails but we have stale cache, fall back to it
			if s.tryLoadFromDisk() {
				s.ready = true
				s.source = "disk_cache_stale"
				s.logger.Warn("Upstream sync failed; using stale disk cache", "err", res.err)
				return nil
			}
			return res.err
		}

		if err := s.repo.SaveAll(res.entries, res.topics, res.authors, res.keywords); err != nil {
			return fmt.Errorf("save to repository: %w", err)
		}

		if s.searchIndex != nil {
			if err := s.searchIndex.Build(res.entries); err != nil {
				s.logger.Warn("Failed to build search index", "err", err)
			}
		}

		// Save to disk cache asynchronously
		if s.diskCache != nil {
			go func() {
				if bytesE, err := json.Marshal(res.entries); err == nil {
					_ = s.diskCache.Set(cacheKeyEntries, bytesE)
				}
				if bytesT, err := json.Marshal(res.topics); err == nil {
					_ = s.diskCache.Set(cacheKeyTopics, bytesT)
				}
				if bytesA, err := json.Marshal(res.authors); err == nil {
					_ = s.diskCache.Set(cacheKeyAuthors, bytesA)
				}
				if bytesK, err := json.Marshal(res.keywords); err == nil {
					_ = s.diskCache.Set(cacheKeyKeywords, bytesK)
				}
			}()
		}

		s.ready = true
		s.source = "upstream_http"
		s.logger.Info("Catalog synchronized successfully",
			"entries", len(res.entries),
			"topics", len(res.topics),
			"authors", len(res.authors),
			"keywords", len(res.keywords),
		)
		return nil
	}
}

// Sync forces synchronization of the catalog from upstream.
func (s *CatalogService) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked(ctx)
}

// GetStatus returns the current catalog health and metrics.
func (s *CatalogService) GetStatus(ctx context.Context) port.CatalogStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastUpdated := s.repo.LastUpdated()
	var lastSyncedStr string
	if !lastUpdated.IsZero() {
		lastSyncedStr = lastUpdated.Format(time.RFC3339)
	}

	return port.CatalogStatus{
		TotalEntries:  s.repo.Count(),
		TotalTopics:   len(s.repo.GetTopics()),
		TotalAuthors:  len(s.repo.GetAuthors()),
		TotalKeywords: len(s.repo.GetKeywords()),
		LastSyncedAt:  lastSyncedStr,
		Source:        s.source,
		Ready:         s.ready,
	}
}

// GetEntry retrieves an entry by ID/shortname.
func (s *CatalogService) GetEntry(ctx context.Context, id string) (*entry.Entry, error) {
	e, ok := s.repo.GetEntry(id)
	if !ok {
		return nil, fmt.Errorf("%w: %s", entry.ErrEntryNotFound, id)
	}
	return e, nil
}

// ListEntries returns a paginated slice of entries.
func (s *CatalogService) ListEntries(ctx context.Context, limit, offset int) ([]entry.Entry, int, error) {
	all := s.repo.GetAllEntries()
	total := len(all)

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}

	if offset >= total {
		return nil, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return all[offset:end], total, nil
}

// GetDependencies returns the parsed ROOT specification and dependencies for an entry.
func (s *CatalogService) GetDependencies(ctx context.Context, id string) (*theory.RootFile, error) {
	if s.theoryService == nil {
		return nil, fmt.Errorf("theory service is unavailable")
	}
	return s.theoryService.GetRootFile(ctx, id)
}

// ListTopics returns all classified topics.
func (s *CatalogService) ListTopics(ctx context.Context) ([]entry.Topic, error) {
	return s.repo.GetTopics(), nil
}

// GetEntriesByTopic returns entries filtered by topic name.
func (s *CatalogService) GetEntriesByTopic(ctx context.Context, topicName string, limit, offset int) ([]entry.Entry, int, error) {
	all := s.repo.GetAllEntries()
	target := strings.ToLower(topicName)

	var filtered []entry.Entry
	for _, e := range all {
		for _, t := range e.Topics {
			if strings.EqualFold(t, topicName) || strings.Contains(strings.ToLower(t), target) {
				filtered = append(filtered, e)
				break
			}
		}
	}

	total := len(filtered)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}

	if offset >= total {
		return nil, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total, nil
}

// SuggestKeywords finds autocomplete/search terms by prefix or substring.
func (s *CatalogService) SuggestKeywords(ctx context.Context, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	p := strings.ToLower(strings.TrimSpace(prefix))
	keywords := s.repo.GetKeywords()

	var matched []string
	// First pass: prefix matches
	for _, kw := range keywords {
		k := kw.Keyword
		kLC := strings.ToLower(k)
		if strings.HasPrefix(kLC, p) {
			matched = append(matched, k)
			if len(matched) >= limit {
				return matched, nil
			}
		}
	}

	// Second pass: contains matches
	if len(matched) < limit {
		for _, kw := range keywords {
			k := kw.Keyword
			kLC := strings.ToLower(k)
			if !strings.HasPrefix(kLC, p) && strings.Contains(kLC, p) {
				matched = append(matched, k)
				if len(matched) >= limit {
					return matched, nil
				}
			}
		}
	}

	return matched, nil
}
