package theoryarchive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func createTestTarGz(t *testing.T) []byte {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	files := []struct {
		name string
		body string
	}{
		{
			name: "Formal_SSA/ROOT",
			body: "chapter AFP\nsession Formal_SSA = Collections +\n  theories Construct_SSA\n",
		},
		{
			name: "Formal_SSA/Construct_SSA.thy",
			body: "theory Construct_SSA imports Main begin\nlemma test: True by simp\nend\n",
		},
		{
			name: "Formal_SSA/README.md",
			body: "# Formal SSA\n",
		},
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.name,
			Mode: 0644,
			Size: int64(len(f.body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return buf.Bytes()
}

func TestTarGzExtractor(t *testing.T) {
	archiveData := createTestTarGz(t)
	extractor := New()

	// 1. ListFiles
	files, err := extractor.ListFiles("Formal_SSA", bytes.NewReader(archiveData))
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	// Should list ROOT and Construct_SSA.thy (ignoring README.md)
	if len(files) != 2 {
		t.Fatalf("expected 2 files (ROOT and .thy), got %d (%v)", len(files), files)
	}

	// 2. ExtractFile
	thyBytes, err := extractor.ExtractFile("Formal_SSA", bytes.NewReader(archiveData), "Construct_SSA.thy")
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}
	if !bytes.Contains(thyBytes, []byte("lemma test: True")) {
		t.Errorf("unexpected content: %s", string(thyBytes))
	}

	// 3. ExtractRoot
	root, err := extractor.ExtractRoot("Formal_SSA", bytes.NewReader(archiveData))
	if err != nil {
		t.Fatalf("ExtractRoot failed: %v", err)
	}
	if len(root.Sessions) != 1 || root.Sessions[0].Name != "Formal_SSA" {
		t.Errorf("unexpected root: %+v", root)
	}
}
