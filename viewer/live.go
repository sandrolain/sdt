package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sandrolain/sdt/internal/corpus"
)

// watchDebounce coalesces a burst of corpus changes before rebuilding.
const watchDebounce = 500 * time.Millisecond

// eventPayload is the /api/events SSE data: project-root-relative changed paths.
type eventPayload struct {
	Paths []string `json:"paths"`
}

// broker fans out corpus change events to SSE subscribers, dropping for slow clients.
type broker struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newBroker() *broker {
	return &broker{subs: map[chan []byte]struct{}{}}
}

func (b *broker) subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 4)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
		close(ch)
	}
}

// publish marshals paths and fans out to subscribers; a full channel is skipped.
func (b *broker) publish(paths []string) {
	data, err := json.Marshal(eventPayload{Paths: paths})
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- data:
		default:
		}
	}
}

// handleEvents streams corpus changes as Server-Sent Events.
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, unsubscribe := s.broker.subscribe()
	defer unsubscribe()

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, open := <-ch:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "event: change\ndata: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// startWatching watches the corpus recursively (excluding excluded dirs),
// debounces bursts and rebuilds the search index before publishing the change.
func (s *server) startWatching() error {
	w, err := watchCorpus(s.root, s.corpus, watchDebounce, func(paths []string) {
		s.rebuildSearch(paths)
	})
	if err != nil {
		return err
	}
	s.watcher = w
	return nil
}

// stopWatching releases the fsnotify watcher, if any.
func (s *server) stopWatching() {
	if s.watcher == nil {
		return
	}
	if err := s.watcher.Close(); err != nil {
		slog.Warn("sdtviewer: watcher close", "err", err)
	}
	s.watcher = nil
}

// corpusWatcher debounces fsnotify events and reports changed corpus paths.
type corpusWatcher struct {
	w        *fsnotify.Watcher
	root     string
	debounce time.Duration
	onChange func([]string)
	pending  map[string]struct{}
	timer    *time.Timer
}

// watchCorpus adds a recursive watch over dir and calls onChange with the
// sorted project-root-relative changed paths after each debounce window.
func watchCorpus(root, dir string, debounce time.Duration, onChange func([]string)) (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addWatchDirs(w, dir); err != nil {
		if closeErr := w.Close(); closeErr != nil {
			slog.Warn("sdtviewer: watcher close", "err", closeErr)
		}
		return nil, err
	}
	cw := &corpusWatcher{w: w, root: root, debounce: debounce, onChange: onChange, pending: map[string]struct{}{}}
	go cw.run()
	return w, nil
}

func (cw *corpusWatcher) run() {
	for {
		select {
		case event, open := <-cw.w.Events:
			if !open {
				return
			}
			cw.handle(event)
		case watchErr, open := <-cw.w.Errors:
			if !open {
				return
			}
			slog.Warn("sdtviewer: watch error", "err", watchErr)
		}
	}
}

func (cw *corpusWatcher) handle(event fsnotify.Event) {
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := addWatchDirs(cw.w, event.Name); err != nil {
				slog.Warn("sdtviewer: watch dir failed", "path", event.Name, "err", err)
			}
		}
	}
	rel := relCorpus(cw.root, event.Name)
	if rel == "" || corpus.ExcludedPath(rel) {
		return
	}
	cw.pending[rel] = struct{}{}
	if cw.timer != nil {
		cw.timer.Stop()
	}
	cw.timer = time.AfterFunc(cw.debounce, cw.flush)
}

func (cw *corpusWatcher) flush() {
	if len(cw.pending) == 0 {
		return
	}
	paths := make([]string, 0, len(cw.pending))
	for p := range cw.pending {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	cw.pending = map[string]struct{}{}
	cw.onChange(paths)
}

// addWatchDirs adds dir and all non-excluded subdirectories to the watcher.
func addWatchDirs(w *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != dir && corpus.ExcludedDirName(d.Name()) {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

// relCorpus returns the project-root-relative slash path of name, or "" when outside.
func relCorpus(root, name string) string {
	rel, err := filepath.Rel(root, name)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	return rel
}
