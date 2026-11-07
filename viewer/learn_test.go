package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// learnServer builds a server whose learning routes run the given fake CLI.
func learnServer(t *testing.T, run func(ctx context.Context, bin, dir string, args ...string) (string, error)) *server {
	t.Helper()
	s := &server{root: t.TempDir(), self: "sdt"}

	s.learn.once.Do(func() {})
	s.learn.del = &delegator{bin: "sdt", root: s.root, run: run}
	return s
}

// TestLearnCourseProxiesTheCLI proves the route asks the CLI and passes its answer
// through: the bundle's shape has one implementation, and it is the CLI's.
func TestLearnCourseProxiesTheCLI(t *testing.T) {
	var got []string
	s := learnServer(t, func(_ context.Context, bin, dir string, args ...string) (string, error) {
		got = append([]string{bin}, args...)
		return `{"syllabus":"syllabi/software-architect","lessons":6}`, nil
	})

	rec := httptest.NewRecorder()
	s.handleLearnCourse(rec, httptest.NewRequest(http.MethodGet, "/api/learn/course?syllabus=syllabi/software-architect", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"lessons":6`) {
		t.Errorf("the CLI's answer was not passed through: %s", rec.Body.String())
	}
	want := []string{"sdt", "learn", "course", "syllabi/software-architect", "--format", "json"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}

// TestLearnLessonRefusesBadIDs proves the parameter is a corpus id, never a path:
// the route builds argv, so what it accepts is part of its surface.
func TestLearnLessonRefusesBadIDs(t *testing.T) {
	s := learnServer(t, func(context.Context, string, string, ...string) (string, error) {
		t.Fatal("the CLI must not run for a refused id")
		return "", nil
	})
	for _, tc := range []struct{ name, url string }{
		{"missing", "/api/learn/lesson"},
		{"traversal", "/api/learn/lesson?concept=../../etc/passwd"},
		{"flag", "/api/learn/lesson?concept=--format"},
		{"space", "/api/learn/lesson?concept=a%20b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.handleLearnLesson(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestLearnSurfacesTheCLIFailure proves a refusal names its cause: the CLI's own
// sentence reaches the reader instead of a generic gateway error.
func TestLearnSurfacesTheCLIFailure(t *testing.T) {
	s := learnServer(t, func(context.Context, string, string, ...string) (string, error) {
		return "no wiki page matches the concept \"architecture/nope\"", errors.New("exit 4")
	})
	rec := httptest.NewRecorder()
	s.handleLearnLesson(rec, httptest.NewRequest(http.MethodGet, "/api/learn/lesson?concept=architecture/nope", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no wiki page matches") {
		t.Errorf("the CLI's sentence is not in the answer: %s", rec.Body.String())
	}
}

// TestLearnRefusesANonJSONAnswer proves a proxy that cannot trust its source says so
// rather than forwarding bytes the SPA would fail to parse.
func TestLearnRefusesANonJSONAnswer(t *testing.T) {
	s := learnServer(t, func(context.Context, string, string, ...string) (string, error) {
		return "not json at all", nil
	})
	rec := httptest.NewRecorder()
	s.handleLearnCourse(rec, httptest.NewRequest(http.MethodGet, "/api/learn/course?syllabus=syllabi/x", nil))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

// TestLearnWithoutACLIAnswers503 proves a viewer with no CLI disables the surface
// with a reason instead of failing per request.
func TestLearnWithoutACLIAnswers503(t *testing.T) {
	s := &server{root: t.TempDir()}
	s.learn.once.Do(func() {})
	s.learn.reason = "no sdt CLI found for this root"
	rec := httptest.NewRecorder()
	s.handleLearnCourse(rec, httptest.NewRequest(http.MethodGet, "/api/learn/course?syllabus=syllabi/x", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no sdt CLI") {
		t.Errorf("the reason is missing: %s", rec.Body.String())
	}
}

// TestLearnLessonDropsTheGradingKey proves the reader's payload carries no answer:
// the prompt travels, the expected value, the rubric, the distractor and the key
// digest do not.
func TestLearnLessonDropsTheGradingKey(t *testing.T) {
	s := learnServer(t, func(context.Context, string, string, ...string) (string, error) {
		return `{"concept":"architecture/x","title":"X","objectives":["o1"],
			"worked_example":{"source_span":"refs/a.md@abcdef01:1-2","text":"example"},
			"items":[{"id":"x-1","objective":1,"type":"recall","prompt":"name it",
			"expected":"the key","rubric":"all","distractor":"wrong","claim":"architecture/x#c-aaaaaaaa",
			"key_hash":"deadbeef","hash":"1234567890ab"}],
			"transfer":"a case",
			"material":{"path":"context/renders/materials/s/x.it.md","title":"X — guida","lang":"it"},
			"practice":[{"path":"context/learning/practice/p.practice.md","title":"P","cases":[{"title":"A case","text":"the situation"}]}]}`, nil
	})
	rec := httptest.NewRecorder()
	s.handleLearnLesson(rec, httptest.NewRequest(http.MethodGet, "/api/learn/lesson?concept=architecture/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, leaked := range []string{"the key", "rubric", "distractor", "key_hash", "deadbeef"} {
		if strings.Contains(body, leaked) {
			t.Errorf("the answer leaked %q: %s", leaked, body)
		}
	}
	for _, kept := range []string{"name it", "recall", "example", "a case", "context/renders/materials/s/x.it.md", "context/learning/practice/p.practice.md", "the situation"} {
		if !strings.Contains(body, kept) {
			t.Errorf("the reader lost %q: %s", kept, body)
		}
	}
}
