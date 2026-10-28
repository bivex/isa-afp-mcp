package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
)

// TheoryService implements port.TheoryUseCase.
type TheoryService struct {
	afpClient port.AFPClient
	diskCache port.DiskCache
	extractor port.TheoryArchiveExtractor
	logger    *slog.Logger
}

var _ port.TheoryUseCase = (*TheoryService)(nil)

// NewTheoryService creates a new TheoryService.
func NewTheoryService(
	afpClient port.AFPClient,
	diskCache port.DiskCache,
	extractor port.TheoryArchiveExtractor,
	logger *slog.Logger,
) *TheoryService {
	if logger == nil {
		logger = slog.Default()
	}
	return &TheoryService{
		afpClient: afpClient,
		diskCache: diskCache,
		extractor: extractor,
		logger:    logger,
	}
}

// openOrDownloadArchive opens the cached package archive or downloads and caches it on demand.
func (s *TheoryService) openOrDownloadArchive(ctx context.Context, entryID string) (io.ReadCloser, error) {
	entryShortname := strings.TrimSpace(entryID)

	// Check if already in disk cache
	if s.diskCache != nil {
		rc, err := s.diskCache.OpenArchive(entryShortname)
		if err == nil {
			return rc, nil
		}
	}

	// Download from upstream
	s.logger.Info("Downloading entry package archive from upstream...", "entry", entryShortname)
	upstreamBody, err := s.afpClient.DownloadPackageArchive(ctx, entryShortname)
	if err != nil {
		return nil, fmt.Errorf("%w for entry %s: %v", theory.ErrArchiveDownload, entryShortname, err)
	}
	defer upstreamBody.Close()

	if s.diskCache != nil {
		// Stream to cache file
		if _, err := s.diskCache.SaveArchive(entryShortname, upstreamBody); err != nil {
			s.logger.Warn("Failed to persist archive to disk cache", "err", err)
		}
		// Re-open from disk cache
		return s.diskCache.OpenArchive(entryShortname)
	}

	// If no disk cache available, read into buffer
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, upstreamBody); err != nil {
		return nil, fmt.Errorf("buffer archive download: %w", err)
	}
	return io.NopCloser(buf), nil
}

// ListTheories scans the package archive and lists all .thy files and ROOT.
func (s *TheoryService) ListTheories(ctx context.Context, entryID string) ([]theory.TheoryFileInfo, error) {
	rc, err := s.openOrDownloadArchive(ctx, entryID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return s.extractor.ListFiles(entryID, rc)
}

// GetRootFile extracts and parses the ROOT specification for the entry.
func (s *TheoryService) GetRootFile(ctx context.Context, entryID string) (*theory.RootFile, error) {
	rc, err := s.openOrDownloadArchive(ctx, entryID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return s.extractor.ExtractRoot(entryID, rc)
}

// GetTheoryOutline extracts a theory file and generates an agent-optimized outline
// containing types, definitions, and theorems without bulky proof tactics.
func (s *TheoryService) GetTheoryOutline(ctx context.Context, entryID, theoryName string) (*theory.TheoryOutline, error) {
	rc, err := s.openOrDownloadArchive(ctx, entryID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	rawBytes, err := s.extractor.ExtractFile(entryID, rc, theoryName)
	if err != nil {
		return nil, err
	}

	return theory.ParseTheoryOutline(entryID, string(rawBytes))
}

// GetTheoryContent retrieves the raw text content of a theory or ROOT file, with optional line range pagination.
func (s *TheoryService) GetTheoryContent(ctx context.Context, entryID, theoryPath string, startLine, maxLines int) (*theory.TheoryContent, error) {
	rc, err := s.openOrDownloadArchive(ctx, entryID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	rawBytes, err := s.extractor.ExtractFile(entryID, rc, theoryPath)
	if err != nil {
		return nil, err
	}

	if startLine <= 0 {
		startLine = 1
	}
	if maxLines <= 0 {
		maxLines = 200 // Default 200 lines to protect agent context window
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(rawBytes))
	totalLines := 0

	for scanner.Scan() {
		totalLines++
		currLine := totalLines
		if currLine >= startLine && len(lines) < maxLines {
			lines = append(lines, scanner.Text())
		}
	}

	lineEnd := startLine + len(lines) - 1
	if lineEnd > totalLines {
		lineEnd = totalLines
	}

	truncated := totalLines > lineEnd

	return &theory.TheoryContent{
		EntryShortname: entryID,
		Path:           theoryPath,
		TotalLines:     totalLines,
		LineStart:      startLine,
		LineEnd:        lineEnd,
		Content:        strings.Join(lines, "\n"),
		Truncated:      truncated,
	}, nil
}
