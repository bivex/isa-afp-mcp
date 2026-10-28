package cache

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func TestDiskCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "isa-afp-cache-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dc, err := NewDiskCache(tmpDir, 2*time.Second)
	if err != nil {
		t.Fatalf("NewDiskCache failed: %v", err)
	}

	key := "test_key.json"
	val := []byte(`{"hello": "afp"}`)

	// 1. Get non-existent
	data, ok, err := dc.Get(key)
	if err != nil || ok || data != nil {
		t.Errorf("expected miss, got ok=%v, data=%v, err=%v", ok, data, err)
	}

	// 2. Set & Get
	if err := dc.Set(key, val); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if !dc.Has(key) {
		t.Errorf("expected Has=true")
	}

	data, ok, err = dc.Get(key)
	if err != nil || !ok || string(data) != string(val) {
		t.Errorf("expected hit with %s, got ok=%v, data=%s, err=%v", string(val), ok, string(data), err)
	}

	// 3. SaveArchive & OpenArchive
	archiveContent := []byte("simulated tar.gz content")
	savedPath, err := dc.SaveArchive("Formal_SSA", bytes.NewReader(archiveContent))
	if err != nil {
		t.Fatalf("SaveArchive failed: %v", err)
	}
	if savedPath == "" {
		t.Errorf("saved path is empty")
	}

	rc, err := dc.OpenArchive("Formal_SSA")
	if err != nil {
		t.Fatalf("OpenArchive failed: %v", err)
	}
	defer rc.Close()

	readBytes, err := io.ReadAll(rc)
	if err != nil || string(readBytes) != string(archiveContent) {
		t.Errorf("expected archive content %s, got %s (err: %v)", string(archiveContent), string(readBytes), err)
	}

	// 4. Clear
	if err := dc.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}
	if dc.Has(key) {
		t.Errorf("expected Has=false after Clear")
	}
}
