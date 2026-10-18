package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

// A package's bytes are pinned, so that every comparison over the official
// examples measures the same ones (#221). These run offline, against a test
// server, through fetchFrom with a pin of their own.

const testPackage = "test.package/1.0.0"

// testTarball is a package tarball holding package/package.json and one
// resource.
func testTarball(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{
		"package/package.json":      `{"name":"test.package","version":"1.0.0"}`,
		"package/Patient-p.json":    `{"resourceType":"Patient","id":"p"}`,
		"package/other/ignored.txt": "not a resource",
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// serve answers every request with body and status, counting the requests.
func serve(t *testing.T, status int, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/"+testPackage {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

// leftovers lists what the cache holds besides the package directories it
// was given, which a fetch must not leave behind.
func leftovers(t *testing.T, cache string, keep ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	var extra []string
	for _, e := range entries {
		kept := false
		for _, k := range keep {
			if e.Name() == k {
				kept = true
			}
		}
		if !kept {
			extra = append(extra, e.Name())
		}
	}
	return extra
}

func TestFetchExtractsAPackageWhoseBytesArePinned(t *testing.T) {
	tarball := testTarball(t)
	packageDir := packageDirName(testPackage, digestOf(tarball))
	srv, _ := serve(t, http.StatusOK, tarball)
	cache := t.TempDir()

	dir, err := fetchFrom(srv.URL+"/", cache, testPackage, digestOf(tarball), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(cache, packageDir, "package") {
		t.Errorf("dir = %s", dir)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Patient-p.json")); statErr != nil {
		t.Errorf("the resource was not extracted: %v", statErr)
	}
	marker, err := os.ReadFile(filepath.Join(cache, packageDir, "sha256"))
	if err != nil || strings.TrimSpace(string(marker)) != digestOf(tarball) {
		t.Errorf("marker = %q %v, want the pin", marker, err)
	}
	if extra := leftovers(t, cache, packageDir); len(extra) > 0 {
		t.Errorf("left behind: %v", extra)
	}
}

func TestFetchRefusesBytesThatAreNotThePinnedOnes(t *testing.T) {
	tarball := testTarball(t)
	pin := digestOf(tarball)
	altered := append([]byte(nil), tarball...)
	altered[len(altered)-1] ^= 1
	srv, _ := serve(t, http.StatusOK, altered)
	cache := t.TempDir()

	_, err := fetchFrom(srv.URL+"/", cache, testPackage, pin, 1<<20)
	if err == nil {
		t.Fatal("altered bytes were accepted")
	}
	if !strings.Contains(err.Error(), pin) || !strings.Contains(err.Error(), digestOf(altered)) {
		t.Errorf("the error should name both digests: %v", err)
	}
	if extra := leftovers(t, cache); len(extra) > 0 {
		t.Errorf("left behind: %v", extra)
	}
}

func TestFetchRefusesAFailedOrTruncatedDownload(t *testing.T) {
	tarball := testTarball(t)
	for name, tt := range map[string]struct {
		srv  func() *httptest.Server
		want string
	}{
		"not found": {func() *httptest.Server { s, _ := serve(t, http.StatusNotFound, nil); return s }, "404"},
		"truncated": {func() *httptest.Server { s, _ := serve(t, http.StatusOK, tarball[:len(tarball)/2]); return s }, "is not the pinned"},
	} {
		t.Run(name, func(t *testing.T) {
			cache := t.TempDir()
			_, err := fetchFrom(tt.srv().URL+"/", cache, testPackage, digestOf(tarball), 1<<20)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want one saying %q", err, tt.want)
			}
			if extra := leftovers(t, cache); len(extra) > 0 {
				t.Errorf("left behind: %v", extra)
			}
		})
	}
}

func TestFetchRefusesADownloadLargerThanTheCap(t *testing.T) {
	tarball := testTarball(t)
	srv, _ := serve(t, http.StatusOK, tarball)
	cache := t.TempDir()

	_, err := fetchFrom(srv.URL+"/", cache, testPackage, digestOf(tarball), int64(len(tarball)-1))
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("err = %v, want the cap refusing it", err)
	}
	if extra := leftovers(t, cache); len(extra) > 0 {
		t.Errorf("left behind: %v", extra)
	}
}

func TestFetchUsesACachedPackageOnlyWhenItIsThePinnedOne(t *testing.T) {
	tarball := testTarball(t)
	pin := digestOf(tarball)
	packageDir := packageDirName(testPackage, pin)

	stale := func(t *testing.T, cache, marker string) {
		t.Helper()
		pkg := filepath.Join(cache, packageDir, "package")
		if err := os.MkdirAll(pkg, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkg, "package.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if marker != "" {
			if err := os.WriteFile(filepath.Join(cache, packageDir, "sha256"), []byte(marker+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("the pin", func(t *testing.T) {
		srv, requests := serve(t, http.StatusOK, tarball)
		cache := t.TempDir()
		stale(t, cache, pin)
		if _, err := fetchFrom(srv.URL+"/", cache, testPackage, pin, 1<<20); err != nil {
			t.Fatal(err)
		}
		if got := requests.Load(); got != 0 {
			t.Errorf("requests = %d, want the cached package used", got)
		}
	})

	// A directory at the pinned name that is not the pinned package was
	// damaged by hand: it is reported, not removed, since another process
	// may be reading it.
	for name, marker := range map[string]string{"no marker": "", "other digest": strings.Repeat("0", 64)} {
		t.Run(name, func(t *testing.T) {
			srv, _ := serve(t, http.StatusOK, tarball)
			cache := t.TempDir()
			stale(t, cache, marker)
			_, err := fetchFrom(srv.URL+"/", cache, testPackage, pin, 1<<20)
			if err == nil || !strings.Contains(err.Error(), "remove it") {
				t.Fatalf("err = %v, want the damaged directory reported", err)
			}
			if _, statErr := os.Stat(filepath.Join(cache, packageDir, "package", "package.json")); statErr != nil {
				t.Errorf("the damaged directory was removed: %v", statErr)
			}
			if extra := leftovers(t, cache, packageDir); len(extra) > 0 {
				t.Errorf("left behind: %v", extra)
			}
		})
	}

	t.Run("refused refetch", func(t *testing.T) {
		srv, _ := serve(t, http.StatusOK, []byte("not the package"))
		cache := t.TempDir()
		stale(t, cache, "")
		if _, err := fetchFrom(srv.URL+"/", cache, testPackage, pin, 1<<20); err == nil || !strings.Contains(err.Error(), "is not the pinned") {
			t.Fatalf("err = %v, want the pin refusing it", err)
		}
		if _, err := os.Stat(filepath.Join(cache, packageDir, "package", "package.json")); err != nil {
			t.Errorf("a refused refetch removed the existing package: %v", err)
		}
		if extra := leftovers(t, cache, packageDir); len(extra) > 0 {
			t.Errorf("left behind: %v", extra)
		}
	})
}

func TestEveryPackageComparedIsPinned(t *testing.T) {
	hexDigits := regexp.MustCompile(`^[0-9a-f]{64}$`)
	seen := map[string]string{}
	for name, v := range versions {
		for _, pkg := range []string{v.core, v.examples} {
			pin, ok := packagePins[pkg]
			if !ok || !hexDigits.MatchString(pin) {
				t.Errorf("%s: %s has no pin of 64 lowercase hex digits: %q", name, pkg, pin)
				continue
			}
			if other, dup := seen[pin]; dup && other != pkg {
				t.Errorf("%s and %s share a pin", other, pkg)
			}
			seen[pin] = pkg
		}
	}
}

// fetchedOrSkip returns dir, skipping the test when the package could not be
// fetched, and failing it when what was fetched is not the pinned bytes.
func fetchedOrSkip(t interface {
	Helper()
	Fatal(args ...any)
	Skip(args ...any)
}, dir string, err error) string {
	t.Helper()
	if errors.Is(err, errNotPinned) {
		t.Fatal(err)
	}
	if err != nil {
		t.Skip("package unavailable:", err)
	}
	return dir
}

func TestABadPinFailsATestRatherThanSkippingIt(t *testing.T) {
	tarball := testTarball(t)
	srv, _ := serve(t, http.StatusOK, tarball)
	_, err := fetchFrom(srv.URL+"/", t.TempDir(), testPackage, strings.Repeat("0", 64), 1<<20)
	if !errors.Is(err, errNotPinned) {
		t.Fatalf("err = %v, want errNotPinned", err)
	}
}

// Two processes may fetch the same package at once — make check beside make
// corpusdiff, over one cache. Whichever installs it first wins; the others
// find the pinned package in place and use it rather than failing, and none
// removes the files another has already returned.
func TestConcurrentFetchesOfOnePackageAllSucceed(t *testing.T) {
	tarball := testTarball(t)
	srv, _ := serve(t, http.StatusOK, tarball)
	pin := digestOf(tarball)
	const fetchers = 12
	for round := range 100 {
		cache := t.TempDir()
		errs := make(chan error, fetchers)
		for range fetchers {
			go func() {
				dir, err := fetchFrom(srv.URL+"/", cache, testPackage, pin, 1<<20)
				if err == nil {
					for range 50 {
						if _, statErr := os.Stat(filepath.Join(dir, "Patient-p.json")); statErr != nil {
							err = statErr
							break
						}
					}
				}
				errs <- err
			}()
		}
		for range fetchers {
			if err := <-errs; err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if extra := leftovers(t, cache, packageDirName(testPackage, pin)); len(extra) > 0 {
			t.Fatalf("round %d: left behind: %v", round, extra)
		}
	}
}

// A pinned tarball that holds no resource is refused, and leaves nothing.
func TestFetchRefusesAPinnedTarballWithoutResources(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "package/readme.txt", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buf.Bytes()
	srv, _ := serve(t, http.StatusOK, tarball)
	cache := t.TempDir()
	_, err := fetchFrom(srv.URL+"/", cache, testPackage, digestOf(tarball), 1<<20)
	if err == nil || !strings.Contains(err.Error(), "no JSON") {
		t.Fatalf("err = %v, want a refusal for holding no JSON", err)
	}
	if extra := leftovers(t, cache); len(extra) > 0 {
		t.Errorf("left behind: %v", extra)
	}
}

// fetchedOrSkip fails a test on bytes that are not the pinned ones and skips
// it on any other failure to fetch.
func TestFetchedOrSkipFailsOnABadPinAndSkipsOtherwise(t *testing.T) {
	for name, tt := range map[string]struct {
		err   error
		calls string
	}{
		"fetched":     {nil, ""},
		"not pinned":  {fmt.Errorf("fetching x: %w", errNotPinned), "Fatal"},
		"unavailable": {errors.New("fetching x: dial tcp: no route"), "Skip"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := &recorder{}
			done := make(chan struct{})
			go func() {
				defer close(done)
				fetchedOrSkip(rec, "dir", tt.err)
			}()
			<-done
			if rec.calls != tt.calls {
				t.Errorf("called %q, want %q", rec.calls, tt.calls)
			}
		})
	}
}

// recorder stands in for a test, noting whether it was failed or skipped;
// both end the goroutine, as a test's do.
type recorder struct{ calls string }

func (r *recorder) Helper()           {}
func (r *recorder) Fatal(args ...any) { r.calls = "Fatal"; runtime.Goexit() }
func (r *recorder) Skip(args ...any)  { r.calls = "Skip"; runtime.Goexit() }
