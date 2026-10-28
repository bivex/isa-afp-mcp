package port

import (
	"context"
	"io"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
)

// CatalogRepository manages storage and lookup of in-memory catalog data.
type CatalogRepository interface {
	SaveAll(entries []entry.Entry, topics []entry.Topic, authors []entry.Author, keywords []entry.Keyword) error
	GetEntry(id string) (*entry.Entry, bool)
	GetAllEntries() []entry.Entry
	GetTopics() []entry.Topic
	GetAuthors() []entry.Author
	GetKeywords() []entry.Keyword
	Count() int
	LastUpdated() time.Time
}

// AFPClient interacts with the upstream isa-afp.org HTTP endpoints.
type AFPClient interface {
	FetchEntries(ctx context.Context) ([]entry.Entry, error)
	FetchTopics(ctx context.Context) ([]entry.Topic, error)
	FetchAuthors(ctx context.Context) ([]entry.Author, error)
	FetchKeywords(ctx context.Context) ([]entry.Keyword, error)
	DownloadPackageArchive(ctx context.Context, entryShortname string) (io.ReadCloser, error)
}

// SearchIndex is the in-memory indexing engine for sub-millisecond full-text and filtered queries.
type SearchIndex interface {
	Build(entries []entry.Entry) error
	Search(q search.Query) (*search.Result, error)
}

// DiskCache manages persistent on-disk caching of catalog JSON and package archives.
type DiskCache interface {
	Get(key string) ([]byte, bool, error)
	Set(key string, data []byte) error
	Has(key string) bool
	GetArchivePath(entryShortname string) string
	SaveArchive(entryShortname string, r io.Reader) (string, error)
	OpenArchive(entryShortname string) (io.ReadCloser, error)
	Clear() error
}

// TheoryArchiveExtractor extracts ROOT files, theory file listings, and specific .thy files
// from .tar.gz packages.
type TheoryArchiveExtractor interface {
	ListFiles(entryShortname string, r io.Reader) ([]theory.TheoryFileInfo, error)
	ExtractFile(entryShortname string, r io.Reader, targetPath string) ([]byte, error)
	ExtractRoot(entryShortname string, r io.Reader) (*theory.RootFile, error)
}
