package main

// Read-only learning surface: the course view and the lesson reader ask the CLI
// what it already composes instead of re-implementing it here. Decision 0043:
// `/api/learn/course` and `/api/learn/lesson` run `sdt learn … --format json`
// through the same delegation the document-state routes use (decision 0027), so
// the bundle's shape has one implementation and the viewer cannot drift from it.
//
// Nothing here writes, and nothing here is gated on a loopback bind: the verbs it
// runs are read-only, and a view over the corpus is the viewer's ordinary work.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// learnCoursePath and learnLessonPath are the two read-only learning routes.
	learnCoursePath = "/api/learn/course"
	learnLessonPath = "/api/learn/lesson"
	// learnTimeout bounds one bundle run; the CLI answers from the corpus in
	// milliseconds, and the budget only has to outlast a cold start.
	learnTimeout = 15 * time.Second
	// learnErrorKey is the JSON field a refusal uses; one spelling, so a client
	// reads one shape.
	learnErrorKey = "error"
)

// learnIDRe is the closed shape of a syllabus or concept id: a corpus id, never a
// path and never argv. The same rule the corpus registry uses, narrowed to what a
// route needs.
var learnIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9/_.-]*$`)

// learnRunner resolves the CLI once, on first use, and reports why it could not.
type learnRunner struct {
	once   sync.Once
	del    *delegator
	reason string
}

// runner returns the delegator, resolving the CLI on first call. A viewer without
// a CLI answers 503 with the reason instead of failing per request.
func (s *server) learnRunner() (*delegator, string) {
	s.learn.once.Do(func() {
		self := s.self
		if self == "" {
			if path, err := os.Executable(); err == nil {
				self = path
			}
		}
		del, reason := newDelegator(s.root, "", self)
		s.learn.del, s.learn.reason = del, reason
	})
	return s.learn.del, s.learn.reason
}

// handleLearnCourse answers the course view: one syllabus with its themes, its
// lessons' readiness and the objective-to-item matrix.
func (s *server) handleLearnCourse(w http.ResponseWriter, r *http.Request) {
	s.handleLearnBundle(w, r, "course", []string{"syllabus"})
}

// handleLearnLesson answers the lesson reader: objectives, pacing, the worked
// example with its span, the transfer case and the items.
func (s *server) handleLearnLesson(w http.ResponseWriter, r *http.Request) {
	s.handleLearnBundle(w, r, "lesson", []string{"concept"})
}

// handleLearnBundle validates the id, runs the CLI and answers its JSON. The course
// bundle is passed through: a proxy that reshaped it would be the second
// implementation decision 0043 avoids. The lesson bundle is redacted first — a
// reader surfaces prompts, and the grading key travels in the same payload.
func (s *server) handleLearnBundle(w http.ResponseWriter, r *http.Request, verb string, params []string) {
	args := []string{"learn", verb}
	for _, name := range params {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{learnErrorKey: name + " is required"})
			return
		}
		if !learnIDRe.MatchString(value) || strings.Contains(value, "..") {
			writeJSON(w, http.StatusBadRequest, map[string]string{learnErrorKey: name + " is not a corpus id"})
			return
		}
		args = append(args, value)
	}
	del, reason := s.learnRunner()
	if del == nil {
		slog.Warn("sdtviewer: no CLI for the learning surface", "reason", reason)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{learnErrorKey: reason})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), learnTimeout)
	defer cancel()
	out, err := del.run(ctx, del.bin, del.root, append(args, "--format", "json")...)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{learnErrorKey: learnFailure(out, err)})
		return
	}
	if !json.Valid([]byte(out)) {
		writeJSON(w, http.StatusBadGateway, map[string]string{learnErrorKey: "the CLI answered something that is not JSON"})
		return
	}
	if verb == "lesson" {
		redacted, rerr := redactLessonBundle([]byte(out))
		if rerr != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{learnErrorKey: "the lesson bundle could not be read: " + rerr.Error()})
			return
		}
		out = string(redacted)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write([]byte(out)); werr != nil {
		slog.Warn("sdtviewer: learn bundle write", "err", werr)
	}
}

// learnFailure is the CLI's own sentence when it has one, the exec error otherwise:
// a refusal names its cause, and a generic 502 hides the sentence a reader needs.
func learnFailure(out string, err error) string {
	if line := firstLine(out); line != "" {
		return line
	}
	return fmt.Sprintf("%v", err)
}

// lessonWire is what a reader may see of a lesson bundle: the teaching half, never
// the grading key. `expected`, `rubric`, `distractor` and `key_hash` are dropped on
// the way out — a key in the payload is a key handed to whoever opens devtools, and
// decision 0043's surface has no player and no answers.
type lessonWire struct {
	Concept       string   `json:"concept"`
	Title         string   `json:"title,omitempty"`
	Objectives    []string `json:"objectives"`
	Difficulty    string   `json:"difficulty,omitempty"`
	Minutes       int      `json:"minutes,omitempty"`
	Prerequisites []string `json:"prerequisites,omitempty"`
	Example       struct {
		SourceSpan string `json:"source_span,omitempty"`
		Text       string `json:"text"`
	} `json:"worked_example"`
	Items []struct {
		ID        string `json:"id"`
		Objective int    `json:"objective"`
		Type      string `json:"type"`
		Prompt    string `json:"prompt"`
		Claim     string `json:"claim,omitempty"`
	} `json:"items"`
	Claims []struct {
		ID     string `json:"id"`
		Digest string `json:"digest"`
		Text   string `json:"text,omitempty"`
	} `json:"claims,omitempty"`
	Transfer string   `json:"transfer,omitempty"`
	Missing  []string `json:"missing,omitempty"`
}

// redactLessonBundle decodes the bundle into the wire shape and drops the key.
func redactLessonBundle(raw []byte) ([]byte, error) {
	var wire lessonWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, err
	}
	return json.Marshal(wire)
}
