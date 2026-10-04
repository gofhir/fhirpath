package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// extract writes a package's JSON — its top level and a guide's examples —
// under the directory it is given and nowhere else, and skips the rest.
func TestExtractWritesUnderTheDirectoryOnly(t *testing.T) {
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for name, content := range map[string]string{
		"package/package.json":           `{"name":"p"}`,
		"package/example/Patient-a.json": `{"resourceType":"Patient"}`,
		"package/other/skip.json":        `{}`,
		"package/readme.md":              `x`,
		"../escape.json":                 `{}`,
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	work := t.TempDir()
	t.Chdir(work)
	dest := filepath.Join(work, "dest")

	n, err := extract(&archive, dest)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("extracted %d files, want 2", n)
	}
	for _, want := range []string{"package/package.json", "package/example/Patient-a.json"} {
		if _, err := os.Stat(filepath.Join(dest, want)); err != nil {
			t.Errorf("%s was not written under the destination: %v", want, err)
		}
	}
	entries, readErr := os.ReadDir(work)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, e := range entries {
		if e.Name() != "dest" {
			t.Errorf("extract wrote %s outside the destination", e.Name())
		}
	}
}
