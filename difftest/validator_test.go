package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The Java engine the three-engine comparison measures is a pinned release of
// HL7's validator (#223). These run offline, with fake jars and pins of their
// own.

func fakeJar(t *testing.T, dir string, content []byte) (path string, pin jarPin) {
	t.Helper()
	path = filepath.Join(dir, "validator_cli.jar")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	return path, jarPin{version: "9.9.9", size: int64(len(content)), sha256: hex.EncodeToString(sum[:])}
}

func digestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestAPinnedJarIsSnapshottedAndOthersAreRefused(t *testing.T) {
	content := []byte("a validator, as far as this test knows")
	dir := t.TempDir()
	jar, pin := fakeJar(t, dir, content)

	snap, err := prepareJar(jar, pin, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if snap.path == jar || digestFile(t, snap.path) != pin.sha256 {
		t.Errorf("snapshot %s is not a verified copy of %s", snap.path, jar)
	}
	if snap.marker != "" || !strings.Contains(snap.describe, "9.9.9") {
		t.Errorf("a pinned jar is described %q, marked %q", snap.describe, snap.marker)
	}

	// a symlink is followed: its target's bytes are what is checked
	link := filepath.Join(dir, "link.jar")
	if err := os.Symlink(jar, link); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareJar(link, pin, "", t.TempDir()); err != nil {
		t.Errorf("a symlink to the pinned jar: %v", err)
	}

	other := append([]byte(nil), content...)
	other[0] ^= 1
	empty := filepath.Join(dir, "empty.jar")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	altered := filepath.Join(dir, "altered.jar")
	if err := os.WriteFile(altered, other, 0o600); err != nil {
		t.Fatal(err)
	}
	longer := filepath.Join(dir, "longer.jar")
	if err := os.WriteFile(longer, append(append([]byte(nil), content...), 'x'), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"one byte changed": altered,
		"another size":     longer,
		"empty":            empty,
		"a directory":      dir,
		"missing":          filepath.Join(dir, "missing.jar"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := prepareJar(path, pin, "", t.TempDir())
			if err == nil {
				t.Fatal("accepted")
			}
			if name != "missing" && name != "a directory" {
				if !strings.Contains(err.Error(), "9.9.9") || !strings.Contains(err.Error(), pin.sha256) {
					t.Errorf("the error should name the version and the pinned digest: %v", err)
				}
			}
		})
	}
}

func TestAnUnpinnedJarRunsOnlyWithItsDigestAndIsMarked(t *testing.T) {
	dir := t.TempDir()
	_, pin := fakeJar(t, dir, []byte("the pinned validator"))
	other := filepath.Join(dir, "newer.jar")
	if err := os.WriteFile(other, []byte("a newer validator"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := digestFile(t, other)

	snap, err := prepareJar(other, pin, "sha256:"+digest, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if snap.marker != "[UNPINNED sha256:"+digest[:12]+"]" {
		t.Errorf("marker %q", snap.marker)
	}
	if !strings.Contains(snap.describe, "UNPINNED") || !strings.Contains(snap.describe, digest) || !strings.Contains(snap.describe, pin.sha256) {
		t.Errorf("description %q should name the digest measured and the pin", snap.describe)
	}
	if strings.Contains(snap.describe, "9.9.9 ") && !strings.Contains(snap.describe, "pinned") {
		t.Errorf("an unverified version was printed: %q", snap.describe)
	}
	if _, err := prepareJar(other, pin, "sha256:"+strings.Repeat("0", 64), t.TempDir()); err == nil {
		t.Error("an unpinned run with the wrong digest was accepted")
	}
	if _, err := prepareJar(other, pin, digest, t.TempDir()); err == nil {
		t.Error("an unpinned digest without its sha256: prefix was accepted")
	}
}

// writeBattery writes a one-case battery and returns its path.
func writeBattery(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.json"), []byte(`{"resourceType":"Patient"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	battery := filepath.Join(dir, "b.tsv")
	if err := os.WriteFile(battery, []byte("p.json\tPatient.exists()\t4.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return battery
}

// stubEngines replaces the engines a battery runs with ones that count their
// calls and record the jar the Java engine is given.
type stubEngines struct {
	mu      sync.Mutex
	js      int
	java    int
	digests []string
	jar     string      // the path the Java engine was given
	mode    os.FileMode // and its mode
	before  func()      // run when fhirpath.js is called, before Java
}

func (s *stubEngines) install(t *testing.T) {
	t.Helper()
	oldJS, oldJava, oldVersion, oldJVM, oldLook := evaluateJS, evaluateJava, jsVersion, jvmVersion, lookJava
	t.Cleanup(func() {
		evaluateJS, evaluateJava, jsVersion, jvmVersion, lookJava = oldJS, oldJava, oldVersion, oldJVM, oldLook
	})
	lookJava = func() error { return nil }
	jsVersion = func() (string, error) { return "test", nil }
	jvmVersion = func() string { return "test JVM" }
	evaluateJS = func(cases []testCase) (map[string]answer, error) {
		s.mu.Lock()
		s.js++
		s.mu.Unlock()
		if s.before != nil {
			s.before()
		}
		out := map[string]answer{}
		for _, c := range cases {
			out[c.ID] = answer{ID: c.ID, Results: []string{"true"}}
		}
		return out, nil
	}
	evaluateJava = func(cases []batteryCase, jar string, _ int) (map[string]answer, error) {
		data, err := os.ReadFile(jar)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		info, err := os.Stat(jar)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.jar, s.mode = jar, info.Mode()
		s.java++
		s.digests = append(s.digests, hex.EncodeToString(sum[:]))
		s.mu.Unlock()
		out := map[string]answer{}
		for _, c := range cases {
			out[c.ID] = answer{ID: c.ID, Results: []string{"false"}, isJava: true}
		}
		return out, nil
	}
}

func captureOutput(t *testing.T, run func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan []byte)
	go func() {
		data, _ := io.ReadAll(r)
		done <- data
	}()
	runErr := run()
	w.Close()
	os.Stdout = old
	return string(<-done), runErr
}

func TestABatteryRefusesAJarThatIsNotPinnedBeforeEvaluatingAnything(t *testing.T) {
	engines := &stubEngines{}
	engines.install(t)
	dir := t.TempDir()
	_, pin := fakeJar(t, dir, []byte("pinned"))
	other := filepath.Join(dir, "other.jar")
	if err := os.WriteFile(other, []byte("not pinned"), 0o600); err != nil {
		t.Fatal(err)
	}
	withPin(t, pin)

	_, err := captureOutput(t, func() error { return runBattery(writeBattery(t), other, "", 1, false) })
	if err == nil || !strings.Contains(err.Error(), pin.sha256) {
		t.Fatalf("err = %v, want the jar refused with the pinned digest", err)
	}
	if engines.js != 0 || engines.java != 0 {
		t.Errorf("evaluated before refusing: fhirpath.js %d times, Java %d", engines.js, engines.java)
	}
}

func TestTheBytesVerifiedAreTheBytesThatRun(t *testing.T) {
	dir := t.TempDir()
	jar, pin := fakeJar(t, dir, []byte("the pinned validator"))
	withPin(t, pin)
	engines := &stubEngines{before: func() {
		// the original changes after it was verified, before Java runs
		if err := os.WriteFile(jar, []byte("something else"), 0o600); err != nil {
			t.Error(err)
		}
	}}
	engines.install(t)

	out, err := captureOutput(t, func() error { return runBattery(writeBattery(t), jar, "", 1, false) })
	if err != nil {
		t.Fatal(err)
	}
	if engines.java != 1 || engines.digests[0] != pin.sha256 {
		t.Errorf("Java ran %d times on %v, want the pinned bytes", engines.java, engines.digests)
	}
	if engines.mode&0o222 != 0 {
		t.Errorf("the snapshot Java ran was writable: %v", engines.mode)
	}
	if _, err := os.Stat(engines.jar); !os.IsNotExist(err) {
		t.Errorf("the snapshot %s outlived the battery: %v", engines.jar, err)
	}
	if !strings.Contains(out, "HL7 validator 9.9.9") || strings.Contains(out, "UNPINNED") {
		t.Errorf("the header should name the verified version:\n%s", out)
	}
}

func TestEveryJavaAnswerOfAnUnpinnedRunIsMarked(t *testing.T) {
	dir := t.TempDir()
	_, pin := fakeJar(t, dir, []byte("pinned"))
	withPin(t, pin)
	other := filepath.Join(dir, "newer.jar")
	if err := os.WriteFile(other, []byte("newer"), 0o600); err != nil {
		t.Fatal(err)
	}
	engines := &stubEngines{}
	engines.install(t)
	digest := digestFile(t, other)

	out, err := captureOutput(t, func() error {
		return runBattery(writeBattery(t), other, "sha256:"+digest, 1, true)
	})
	if err != nil {
		t.Fatal(err)
	}
	marker := "[UNPINNED sha256:" + digest[:12] + "]"
	header, _, _ := strings.Cut(out, "\n\n")
	for _, line := range strings.Split(header, "\n") {
		if !strings.HasPrefix(line, marker) {
			t.Errorf("unmarked header line: %q", line)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		carriesJava := strings.Contains(line, "Java") || strings.Contains(line, "agree") || strings.HasPrefix(line, "battery")
		if carriesJava && !strings.Contains(line, marker) {
			t.Errorf("unmarked line: %q", line)
		}
	}
}

// withPin makes pin the pinned validator for the test.
func withPin(t *testing.T, pin jarPin) {
	t.Helper()
	old := pinnedValidator
	pinnedValidator = pin
	t.Cleanup(func() { pinnedValidator = old })
}

func TestThePinIsWellFormed(t *testing.T) {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(pinnedValidator.sha256) {
		t.Errorf("sha256 %q", pinnedValidator.sha256)
	}
	if pinnedValidator.size <= 0 {
		t.Errorf("size %d", pinnedValidator.size)
	}
	if !strings.HasPrefix(pinnedValidator.url, "https://") || !strings.Contains(pinnedValidator.url, "/"+pinnedValidator.version+"/") {
		t.Errorf("url %q does not name version %s over https", pinnedValidator.url, pinnedValidator.version)
	}
}

// serveJar answers with body, a declared length (-1 for none, which makes the
// response chunked) and status.
func serveJar(t *testing.T, body []byte, length, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/jar", http.StatusFound)
			return
		}
		if length >= 0 {
			w.Header().Set("Content-Length", strconv.Itoa(length))
		}
		w.WriteHeader(status)
		if length < 0 {
			w.(http.Flusher).Flush()
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchingTheValidatorInstallsOnlyThePinnedBytes(t *testing.T) {
	content := []byte("the pinned validator, fetched")
	_, pin := fakeJar(t, t.TempDir(), content)
	pin.url = "/jar"
	name := "validator_cli-" + pin.version + ".jar"

	ok := func(t *testing.T, srv *httptest.Server, path string) {
		t.Helper()
		dir := t.TempDir()
		p := pin
		p.url = srv.URL + path
		got, err := fetchValidator(dir, p, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(dir, name) || digestFile(t, got) != pin.sha256 {
			t.Errorf("installed %s", got)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Errorf("left behind: %v", entries)
		}
	}
	t.Run("pinned", func(t *testing.T) { ok(t, serveJar(t, content, len(content), 200), "/jar") })
	t.Run("no length", func(t *testing.T) { ok(t, serveJar(t, content, -1, 200), "/jar") })
	t.Run("redirect", func(t *testing.T) { ok(t, serveJar(t, content, len(content), 200), "/redirect") })

	altered := append([]byte(nil), content...)
	altered[0] ^= 1
	for name, tt := range map[string]struct {
		body   []byte
		length int
		status int
	}{
		"altered":      {altered, len(altered), 200},
		"wrong length": {content, len(content) + 1, 200},
		"extra byte":   {append(append([]byte(nil), content...), 'x'), -1, 200},
		"truncated":    {content[:len(content)/2], -1, 200},
		"not found":    {nil, 0, 404},
	} {
		t.Run(name, func(t *testing.T) {
			srv := serveJar(t, tt.body, tt.length, tt.status)
			dir := t.TempDir()
			p := pin
			p.url = srv.URL + "/jar"
			if _, err := fetchValidator(dir, p, time.Minute); err == nil {
				t.Fatal("accepted")
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("left behind: %v", entries)
			}
		})
	}

	t.Run("deadline", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.WriteHeader(200)
			_, _ = w.Write(content[:3])
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		t.Cleanup(srv.Close)
		dir := t.TempDir()
		p := pin
		p.url = srv.URL + "/jar"
		_, err := fetchValidator(dir, p, 200*time.Millisecond)
		if err == nil {
			t.Fatal("a stalled download was accepted")
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("left behind: %v", entries)
		}
	})

	t.Run("wrong length, refused before the body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)+1))
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		t.Cleanup(srv.Close)
		p := pin
		p.url = srv.URL + "/jar"
		start := time.Now()
		_, err := fetchValidator(t.TempDir(), p, 10*time.Second)
		if err == nil || !strings.Contains(err.Error(), "announced") {
			t.Fatalf("err = %v, want the announced length refused", err)
		}
		if time.Since(start) > 5*time.Second {
			t.Errorf("waited %v for the body of a download already refused", time.Since(start))
		}
	})

	t.Run("redirect from https to http", func(t *testing.T) {
		plain := serveJar(t, content, len(content), 200)
		tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, plain.URL+"/jar", http.StatusFound)
		}))
		t.Cleanup(tlsSrv.Close)
		old := transport
		transport = tlsSrv.Client().Transport
		t.Cleanup(func() { transport = old })
		p := pin
		p.url = tlsSrv.URL + "/jar"
		_, err := fetchValidator(t.TempDir(), p, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "https to http") {
			t.Fatalf("err = %v, want the downgrade refused", err)
		}
	})

	t.Run("redirect loop", func(t *testing.T) {
		var hops atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hops.Add(1)
			http.Redirect(w, r, r.URL.Path, http.StatusFound)
		}))
		t.Cleanup(srv.Close)
		p := pin
		p.url = srv.URL + "/jar"
		_, err := fetchValidator(t.TempDir(), p, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "too many redirects") {
			t.Fatalf("err = %v, want the redirects bounded", err)
		}
		if n := hops.Load(); n > 5 {
			t.Errorf("followed %d requests, want at most 5", n)
		}
	})

	t.Run("an interrupt during the download removes it", func(t *testing.T) {
		started := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.WriteHeader(200)
			_, _ = w.Write(content[:3])
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
		}))
		t.Cleanup(srv.Close)
		dir := t.TempDir()
		p := pin
		p.url = srv.URL + "/jar"
		done := make(chan error)
		go func() {
			_, err := fetchValidator(dir, p, 2*time.Second)
			done <- err
		}()
		<-started
		time.Sleep(50 * time.Millisecond)
		// what an interrupt does before exiting
		cleanupMu.Lock()
		runCleanups()
		cleanupMu.Unlock()
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("an interrupt mid-download left %v", entries)
		}
		<-done
	})

	t.Run("no hard links", func(t *testing.T) {
		srv := serveJar(t, content, len(content), 200)
		dir := t.TempDir()
		p := pin
		p.url = srv.URL + "/jar"
		old := link
		link = func(string, string) error {
			return &os.LinkError{Op: "link", Err: errors.New("operation not supported")}
		}
		t.Cleanup(func() { link = old })
		_, err := fetchValidator(dir, p, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "hard links") {
			t.Fatalf("err = %v, want the filesystem named", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("left behind: %v", entries)
		}
	})

	t.Run("a wrong file appears before publication", func(t *testing.T) {
		srv := serveJar(t, content, len(content), 200)
		dir := t.TempDir()
		p := pin
		p.url = srv.URL + "/jar"
		old := beforePublish
		beforePublish = func() {
			if err := os.WriteFile(filepath.Join(dir, name), altered, 0o600); err != nil {
				t.Error(err)
			}
		}
		t.Cleanup(func() { beforePublish = old })
		_, err := fetchValidator(dir, p, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "remove") {
			t.Fatalf("err = %v, want the file found there refused", err)
		}
		if data, _ := os.ReadFile(filepath.Join(dir, name)); !bytes.Equal(data, altered) {
			t.Error("the file that appeared was replaced")
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Errorf("left behind: %v", entries)
		}
	})

	t.Run("stale downloads are removed", func(t *testing.T) {
		srv := serveJar(t, content, len(content), 200)
		dir := t.TempDir()
		stale, fresh := filepath.Join(dir, ".download-old"), filepath.Join(dir, ".download-running")
		for _, f := range []string{stale, fresh} {
			if err := os.WriteFile(f, []byte("partial"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		old := time.Now().Add(-48 * time.Hour)
		if err := os.Chtimes(stale, old, old); err != nil {
			t.Fatal(err)
		}
		p := pin
		p.url = srv.URL + "/jar"
		if _, err := fetchValidator(dir, p, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Error("a download left a day ago was kept")
		}
		if _, err := os.Stat(fresh); err != nil {
			t.Error("a download that may still be running was removed")
		}
	})

	t.Run("already there", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
		p := pin
		p.url = "http://127.0.0.1:1/unreachable"
		if _, err := fetchValidator(dir, p, time.Minute); err != nil {
			t.Errorf("a verified jar already there was not kept: %v", err)
		}
	})

	t.Run("wrong file there", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name), altered, 0o600); err != nil {
			t.Fatal(err)
		}
		p := pin
		p.url = "http://127.0.0.1:1/unreachable"
		_, err := fetchValidator(dir, p, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "remove") {
			t.Fatalf("err = %v, want a refusal saying to remove it", err)
		}
		if data, _ := os.ReadFile(filepath.Join(dir, name)); !bytes.Equal(data, altered) {
			t.Error("the wrong file was overwritten")
		}
	})

	t.Run("concurrent installers", func(t *testing.T) {
		srv := serveJar(t, content, len(content), 200)
		dir := t.TempDir()
		p := pin
		p.url = srv.URL + "/jar"
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := fetchValidator(dir, p, time.Minute)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Error(err)
			}
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 || digestFile(t, filepath.Join(dir, name)) != pin.sha256 {
			t.Errorf("after concurrent installs: %v", entries)
		}
	})
}
