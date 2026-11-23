package main

// The course and lesson views ask the CLI for their composed bundles. Local QTI
// practice exposes only supported item XML, while attempt writes are explicit and
// regraded by the CLI. Both QTI routes share the viewer's loopback/origin gate.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// learnCoursePath and learnLessonPath are the two read-only learning routes.
	learnCoursePath = "/api/learn/course"
	learnLessonPath = "/api/learn/lesson"
	learnQTIPath    = "/api/learn/qti"
	learnRecordPath = "/api/learn/record"
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
var learnItemIDRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// learnRunner resolves the CLI once, on first use, and reports why it could not.
type learnRunner struct {
	once      sync.Once
	del       *delegator
	reason    string
	runBinary func(context.Context, string, string, ...string) ([]byte, string, error)
}

// runner returns the delegator, resolving the CLI on first call. A viewer without
// a CLI answers 503 with the reason instead of failing per request.
func (s *server) learnRunner() (*delegator, string) {
	s.learn.once.Do(func() {
		// Learning routes must use the same resolved CLI as the document-state
		// routes. Re-resolving here can select a different `sdt` from PATH than
		// the one explicitly configured with --sdt-bin.
		s.learn.del = s.cmd
		if s.cmd == nil {
			s.learn.reason = s.writeReason
			if s.learn.reason == "" {
				s.learn.reason = noCLIReason
			}
		}
		s.learn.runBinary = runLearnBinary
	})
	return s.learn.del, s.learn.reason
}

// runLearnBinary keeps the export's ZIP on stdout and diagnostics on stderr.
func runLearnBinary(ctx context.Context, bin, dir string, args ...string) ([]byte, string, error) {
	// #nosec G204,G702 -- bin is resolved by the viewer delegator; argv is a fixed
	// verb plus a validated corpus concept, and execution uses no shell.
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

// handleLearnQTI returns the current frozen item XML for local practice. QTI
// carries the answer key, so the route is available only through the same
// loopback/origin gate as other viewer capabilities.
func (s *server) handleLearnQTI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.cmd == nil || !sameOrigin(r) {
		writeErr(w, http.StatusForbidden, "QTI practice requires the local viewer origin")
		return
	}
	concept := strings.TrimSpace(r.URL.Query().Get("concept"))
	if !learnIDRe.MatchString(concept) || strings.Contains(concept, "..") {
		writeErr(w, http.StatusBadRequest, "concept is not a corpus id")
		return
	}
	del, reason := s.learnRunner()
	if del == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{learnErrorKey: reason})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), learnTimeout)
	defer cancel()
	archive, stderr, err := s.learn.runBinary(ctx, del.bin, del.root, "learn", "export-qti", concept, "--supported-only")
	if err != nil {
		if strings.Contains(stderr, "has no supported QTI items") {
			writeJSON(w, http.StatusOK, map[string]any{"items": map[string]string{}})
			return
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{learnErrorKey: learnFailure(stderr, err)})
		return
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "the CLI returned an invalid QTI package")
		return
	}
	items := make(map[string]string)
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "items/") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(file.Name, "items/"), ".xml")
		if !learnItemIDRe.MatchString(id) {
			continue
		}
		item, openErr := file.Open()
		if openErr != nil {
			writeErr(w, http.StatusBadGateway, "the QTI item could not be read")
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(item, 2<<20))
		closeErr := item.Close()
		if readErr != nil || closeErr != nil {
			writeErr(w, http.StatusBadGateway, "the QTI item could not be read")
			return
		}
		items[id] = string(data)
	}
	if len(items) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "the assessment has no supported QTI items")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type learnRecordRequest struct {
	Concept   string                     `json:"concept"`
	Item      string                     `json:"item"`
	Responses map[string]json.RawMessage `json:"responses"`
}

// handleLearnRecord stores only an explicitly submitted attempt. It derives the
// grade from the CLI and treats the browser's QTI outcomes as presentation only.
func (s *server) handleLearnRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.cmd == nil || !sameOrigin(r) {
		writeErr(w, http.StatusForbidden, "attempt recording requires the local viewer origin")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeErr(w, http.StatusBadRequest, "expected application/json")
		return
	}
	var req learnRecordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request body")
		return
	}
	req.Concept, req.Item = strings.TrimSpace(req.Concept), strings.TrimSpace(req.Item)
	if !learnIDRe.MatchString(req.Concept) || strings.Contains(req.Concept, "..") || !learnItemIDRe.MatchString(req.Item) {
		writeErr(w, http.StatusBadRequest, "concept or item is not a corpus id")
		return
	}
	selected, ok := learnSelectedOptions(req.Responses["RESPONSE"])
	if !ok || len(selected) == 0 {
		writeErr(w, http.StatusBadRequest, "a QTI RESPONSE selection is required")
		return
	}
	answer, err := json.Marshal(selected)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "responses must be JSON values")
		return
	}
	response, err := json.Marshal(req.Responses)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "responses must be JSON values")
		return
	}
	del, reason := s.learnRunner()
	if del == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{learnErrorKey: reason})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), learnTimeout)
	defer cancel()
	gradeOut, gradeErr := del.run(ctx, del.bin, del.root, "learn", "answer", "--concept", req.Concept, "--item", req.Item, "--answer", string(answer), "--format", "json")
	var grade struct {
		Grade string `json:"grade"`
		Rule  string `json:"rule"`
	}
	if json.Unmarshal([]byte(gradeOut), &grade) != nil || (gradeErr != nil && grade.Grade == "") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{learnErrorKey: learnFailure(gradeOut, gradeErr)})
		return
	}
	if grade.Grade != "correct" && grade.Grade != "partially_correct" && grade.Grade != "incorrect" {
		writeErr(w, http.StatusUnprocessableEntity, "the CLI could not grade this response")
		return
	}
	if grade.Rule != "exact-set" && grade.Rule != "partial-credit" {
		writeErr(w, http.StatusUnprocessableEntity, "the assessment item has no supported QTI interaction")
		return
	}
	recordOut, recordErr := del.run(ctx, del.bin, del.root, "learn", "record", "--concept", req.Concept, "--item", req.Item, "--grade", grade.Grade, "--response", string(response), "--format", "json")
	if recordErr != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{learnErrorKey: learnFailure(recordOut, recordErr)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grade": grade.Grade})
}

func learnSelectedOptions(raw json.RawMessage) ([]string, bool) {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}, learnItemIDRe.MatchString(one)
	}
	var many []string
	if json.Unmarshal(raw, &many) != nil {
		return nil, false
	}
	seen := make(map[string]bool, len(many))
	for _, id := range many {
		if !learnItemIDRe.MatchString(id) || seen[id] {
			return nil, false
		}
		seen[id] = true
	}
	return many, true
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
	Transfer string `json:"transfer,omitempty"`
	Material *struct {
		Path  string `json:"path"`
		Title string `json:"title,omitempty"`
		Lang  string `json:"lang,omitempty"`
	} `json:"material,omitempty"`
	Practice []struct {
		Path  string `json:"path"`
		Title string `json:"title,omitempty"`
		Cases []struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		} `json:"cases"`
	} `json:"practice,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// redactLessonBundle decodes the bundle into the wire shape and drops the key.
func redactLessonBundle(raw []byte) ([]byte, error) {
	var wire lessonWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, err
	}
	return json.Marshal(wire)
}
