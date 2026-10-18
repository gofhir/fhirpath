package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// cleanups are what an interrupt runs before the program exits, since an
// interrupt skips deferred calls: a battery's 188 MB snapshot, a download's
// temporary file.
var (
	cleanupMu sync.Mutex
	cleanups  = map[int]func(){}
	nextID    int
)

// onInterrupt registers cleanup to run on an interrupt and returns what runs
// it now and unregisters it, to defer.
func onInterrupt(cleanup func()) (done func()) {
	cleanupMu.Lock()
	id := nextID
	nextID++
	cleanups[id] = cleanup
	cleanupMu.Unlock()
	return func() {
		// Run under the lock, so that an interrupt arriving meanwhile waits
		// for it rather than exiting halfway through.
		cleanupMu.Lock()
		defer cleanupMu.Unlock()
		if _, ok := cleanups[id]; ok {
			delete(cleanups, id)
			cleanup()
		}
	}
}

// runCleanups runs and unregisters every registered cleanup; the caller holds
// cleanupMu.
func runCleanups() {
	for id, cleanup := range cleanups {
		delete(cleanups, id)
		cleanup()
	}
}

// handleInterrupts runs the registered cleanups and exits on SIGINT or
// SIGTERM. A terminal's Ctrl-C reaches the JVMs a battery started too, being
// in its process group; a SIGTERM sent to difftest alone leaves a JVM already
// running to finish its case on the snapshot, unlinked by then.
func handleInterrupts() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-signals
		// A second signal now ends the program as it would have, should a
		// cleanup hang.
		signal.Stop(signals)
		cleanupMu.Lock() // held until exit: nothing registers afterwards
		runCleanups()
		code := 130 // 128 + SIGINT
		if sig == syscall.SIGTERM {
			code = 143
		}
		os.Exit(code)
	}()
}

// jarPin names a release of HL7's validator by the bytes of its jar.
type jarPin struct {
	version string
	url     string
	size    int64
	sha256  string
}

// pinnedValidator is the HL7 validator the three-engine comparison measures
// HL7's Java engine with (#223): the release asset of hapifhir/org.hl7.fhir.core,
// whose digest GitHub records as sha256:a3addadf…d7e4ccd6, the cross-check
// this pin was set from. The pin, not GitHub, is what a jar is checked
// against: an asset replaced under the same name has other bytes.
//
// Moving to another version is this one change, and a note in CONFORMANCE.md
// that the answers it cites from the Java engine were measured again.
var pinnedValidator = jarPin{
	version: "6.10.2",
	url:     "https://github.com/hapifhir/org.hl7.fhir.core/releases/download/6.10.2/validator_cli.jar",
	size:    187697077,
	sha256:  "a3addadfa18dfa23146a0a243b6ede68eaad92157a5407738c468bb3d7e4ccd6",
}

// jarSnapshot is the copy of a verified jar a battery launches every JVM
// from, so that the bytes checked are the bytes that answer whatever happens
// to the original meanwhile.
type jarSnapshot struct {
	path     string
	describe string // for the report's header
	marker   string // for every line an unpinned jar's answers reach; "" when pinned
}

// prepareJar copies the jar at src, through the file opened once and with
// symlinks followed, into dir, hashing it as it is copied, and checks the copy:
// against pin, or, for a run of a jar that is not the pinned one on purpose,
// against the digest unpinned gives ("sha256:<hex>").
func prepareJar(src string, pin jarPin, unpinned, dir string) (snap jarSnapshot, err error) {
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return snap, fmt.Errorf("the validator's jar: %w", err)
	}
	in, err := os.Open(resolved)
	if err != nil {
		return snap, fmt.Errorf("the validator's jar: %w", err)
	}
	defer in.Close()
	if info, statErr := in.Stat(); statErr != nil || info.IsDir() {
		return snap, fmt.Errorf("the validator's jar: %s is not a file", src)
	}

	out, err := os.CreateTemp(dir, "validator-*.jar")
	if err != nil {
		return snap, err
	}
	defer func() {
		if err != nil {
			os.Remove(out.Name())
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return snap, fmt.Errorf("copying the validator's jar: %w", err)
	}
	if chmodErr := os.Chmod(out.Name(), 0o400); chmodErr != nil {
		return snap, chmodErr
	}
	digest := hex.EncodeToString(h.Sum(nil))

	if unpinned == "" {
		if n != pin.size || digest != pin.sha256 {
			return snap, fmt.Errorf("%s is not HL7 validator %s: sha256 %s, %d bytes, where the pinned one is sha256 %s, %d bytes; "+
				"fetch it with make validator-jar, or measure this jar on purpose with -java-unpinned sha256:%s",
				src, pin.version, digest, n, pin.sha256, pin.size, digest)
		}
		return jarSnapshot{path: out.Name(), describe: fmt.Sprintf("HL7 validator %s (sha256 %s)", pin.version, pin.sha256)}, nil
	}

	want, ok := strings.CutPrefix(unpinned, "sha256:")
	if !ok || want != digest {
		return snap, fmt.Errorf("-java-unpinned %s: %s has sha256 %s", unpinned, src, digest)
	}
	return jarSnapshot{
		path:     out.Name(),
		describe: fmt.Sprintf("HL7 validator UNPINNED sha256:%s (pinned: %s sha256:%s)", digest, pin.version, pin.sha256),
		marker:   fmt.Sprintf("[UNPINNED sha256:%s]", digest[:12]),
	}, nil
}

// verifyFile says whether path holds the pinned bytes, and what it holds.
func verifyFile(path string, pin jarPin) (ok bool, digest string, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err = io.Copy(h, f)
	if err != nil {
		return false, "", 0, err
	}
	digest = hex.EncodeToString(h.Sum(nil))
	return size == pin.size && digest == pin.sha256, digest, size, nil
}

// fetchValidator installs the pinned validator as dir/validator_cli-<version>.jar
// and returns its path. A file already there is kept when it is the pinned one
// and refused, untouched, when it is not. Otherwise the release is downloaded
// into a temporary file in dir, within deadline for the whole request: a
// declared length other than the pinned size is refused at once, an absent one
// accepted, and the body must be exactly the pinned bytes. It is published
// without overwriting anything, which needs a filesystem with hard links, so
// two installers at once both end with the pinned file; nothing is left
// behind on a failure or an interrupt. It needs no Java.
func fetchValidator(dir string, pin jarPin, deadline time.Duration) (string, error) {
	path := filepath.Join(dir, "validator_cli-"+pin.version+".jar")
	if done, err := installed(path, pin); done || err != nil {
		return path, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	removeStaleDownloads(dir)
	tmp, release, err := downloadPinned(dir, pin, deadline)
	if err != nil {
		return "", fmt.Errorf("fetching HL7 validator %s: %w", pin.version, err)
	}
	defer release()

	beforePublish()
	// A link fails where the name is taken, which a rename would replace: of
	// two installers, the second finds the first's file and checks it. A
	// filesystem without hard links cannot publish a file this way.
	if linkErr := link(tmp, path); linkErr != nil {
		if errors.Is(linkErr, os.ErrExist) {
			if done, verr := installed(path, pin); done || verr != nil {
				return path, verr
			}
		}
		return "", fmt.Errorf("installing HL7 validator %s in %s, which needs a filesystem with hard links: %w", pin.version, dir, linkErr)
	}
	return path, nil
}

// beforePublish runs between a verified download and its publication, link
// publishes it, and transport carries the request (nil: the default); tests
// replace them.
var (
	beforePublish                   = func() {}
	link                            = os.Link
	transport     http.RoundTripper = nil
)

// removeStaleDownloads removes the temporary files of downloads that ended
// without cleaning up, a killed process's: those over a day old, so that an
// installer still running beside this one keeps its own.
func removeStaleDownloads(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".download-") {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > 24*time.Hour {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// downloadPinned writes the pinned release to a temporary file in dir, synced
// and closed, and returns its name and what removes it; a download that is
// not the pinned bytes leaves no file, nor does an interrupt during it.
func downloadPinned(dir string, pin jarPin, deadline time.Duration) (name string, release func(), err error) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pin.url, http.NoBody)
	if err != nil {
		return "", nil, err
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if via[0].URL.Scheme == "https" && next.URL.Scheme != "https" {
			return fmt.Errorf("a redirect from https to %s", next.URL.Scheme)
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, errors.New(resp.Status)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != pin.size {
		return "", nil, fmt.Errorf("%d bytes announced, the pinned jar has %d", resp.ContentLength, pin.size)
	}

	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return "", nil, err
	}
	remove := onInterrupt(func() { os.Remove(tmp.Name()) })
	defer func() {
		tmp.Close()
		if err != nil {
			remove()
		}
	}()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(resp.Body, pin.size+1))
	if err != nil {
		return "", nil, err
	}
	if digest := hex.EncodeToString(h.Sum(nil)); n != pin.size || digest != pin.sha256 {
		return "", nil, fmt.Errorf("%d bytes with sha256 %s, where the pinned jar is %d bytes with sha256 %s", n, digest, pin.size, pin.sha256)
	}
	if err = tmp.Sync(); err != nil {
		return "", nil, err
	}
	if err = tmp.Close(); err != nil {
		return "", nil, err
	}
	return tmp.Name(), remove, nil
}

// installed says whether path already holds the pinned validator; a file
// there that is anything else is an error, and is left as it is.
func installed(path string, pin jarPin) (bool, error) {
	ok, digest, size, err := verifyFile(path, pin)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case !ok:
		return false, fmt.Errorf("%s is not HL7 validator %s (sha256 %s, %d bytes; the pinned one is sha256 %s, %d bytes); remove it",
			path, pin.version, digest, size, pin.sha256, pin.size)
	}
	return true, nil
}

// engineRevision is the revision of this engine the battery measures, the
// working tree difftest builds against: as git describes it, by the release
// tags release-please writes (lightweight, hence --tags), or "revision
// unknown" without git. A dirty tree is reproduced only with its patch.
func engineRevision() string {
	out, err := exec.CommandContext(context.Background(), "git", "-C", "..", "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		return "revision unknown"
	}
	rev := strings.TrimSpace(string(out))
	if strings.HasSuffix(rev, "-dirty") {
		rev += " (with uncommitted changes, which a run needs to be reproduced)"
	}
	return rev
}

// jvmVersion is the first line java -version writes, recorded beside the
// validator's answers since another JVM could in principle write them
// otherwise; it is not pinned.
var jvmVersion = func() string {
	out, err := exec.CommandContext(context.Background(), "java", "-version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	// JAVA_TOOL_OPTIONS makes the first line "Picked up …".
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "version") {
			return strings.TrimSpace(line)
		}
	}
	return "unknown"
}
