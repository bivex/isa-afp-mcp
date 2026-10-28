package theoryarchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
)

const maxFileSize = 10 * 1024 * 1024 // 10MB safety cap per extracted theory file

// TarGzExtractor implements port.TheoryArchiveExtractor for streaming .tar.gz packages.
type TarGzExtractor struct{}

var _ port.TheoryArchiveExtractor = (*TarGzExtractor)(nil)

// New creates a new TarGzExtractor.
func New() *TarGzExtractor {
	return &TarGzExtractor{}
}

// ListFiles scans the archive headers and returns a list of all .thy and ROOT files.
func (e *TarGzExtractor) ListFiles(entryShortname string, r io.Reader) ([]theory.TheoryFileInfo, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var files []theory.TheoryFileInfo

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		base := filepath.Base(header.Name)
		isThy := strings.HasSuffix(base, ".thy")
		isRoot := base == "ROOT"

		if isThy || isRoot {
			name := base
			if isThy {
				name = strings.TrimSuffix(base, ".thy")
			}
			files = append(files, theory.TheoryFileInfo{
				EntryShortname: entryShortname,
				Path:           header.Name,
				Name:           name,
				SizeBytes:      header.Size,
				IsRoot:         isRoot,
			})
		}
	}

	return files, nil
}

// ExtractFile extracts the content of a specific file matching targetPath or filename.
func (e *TarGzExtractor) ExtractFile(entryShortname string, r io.Reader, targetPath string) ([]byte, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	targetClean := filepath.Clean(targetPath)
	targetBase := filepath.Base(targetClean)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		nameClean := filepath.Clean(header.Name)
		nameBase := filepath.Base(nameClean)

		matched := nameClean == targetClean ||
			strings.HasSuffix(nameClean, "/"+targetClean) ||
			strings.EqualFold(nameBase, targetBase) ||
			strings.EqualFold(nameBase, targetBase+".thy")

		if matched {
			if header.Size > maxFileSize {
				return nil, fmt.Errorf("file size %d exceeds safety limit of %d bytes", header.Size, maxFileSize)
			}
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, io.LimitReader(tr, maxFileSize)); err != nil {
				return nil, fmt.Errorf("copy file content: %w", err)
			}
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("%w: %s in entry %s", theory.ErrTheoryNotFound, targetPath, entryShortname)
}

// ExtractRoot extracts and parses the ROOT specification from the archive.
func (e *TarGzExtractor) ExtractRoot(entryShortname string, r io.Reader) (*theory.RootFile, error) {
	content, err := e.ExtractFile(entryShortname, r, "ROOT")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", theory.ErrRootNotFound, err)
	}
	return theory.ParseRoot(entryShortname, string(content))
}
