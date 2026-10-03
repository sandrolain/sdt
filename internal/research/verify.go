package research

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Outcome is the deterministic verdict for one source after verification.
type Outcome struct {
	CanonicalURL string `json:"canonical_url" yaml:"canonical_url"`
	Status       string `json:"status" yaml:"status"`
	Reason       string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// VerifyResult is the whole verification report.
type VerifyResult struct {
	RunID    string    `json:"run_id" yaml:"run_id"`
	Checked  int       `json:"checked" yaml:"checked"`
	Verified int       `json:"verified" yaml:"verified"`
	Rejected int       `json:"rejected" yaml:"rejected"`
	Outcomes []Outcome `json:"outcomes" yaml:"outcomes"`
}

// Failed reports whether any source was rejected.
func (v VerifyResult) Failed() bool { return v.Rejected > 0 }

// Verify checks the recorded facts of the run against the raw payloads and
// marks each source verified or rejected. It is deterministic and offline: it
// recomputes the SHA-256 from raw_path, checks the file exists and non-empty,
// and validates the manifest schema. It never upgrades a source it cannot
// check; a missing or mismatched payload is a rejection with a reason.
//
// Sources still in `discovered` (never fetched) are left untouched and are not
// counted as checked: they have no payload to verify yet.
func (r *Run) Verify(runDir string) VerifyResult {
	res := VerifyResult{RunID: r.RunID, Outcomes: []Outcome{}}
	for i := range r.Sources {
		s := &r.Sources[i]
		if s.Status == StatusDiscovered {
			continue
		}
		res.Checked++

		reason := verifySource(runDir, *s)
		if reason == "" {
			s.Status = StatusVerified
			s.Error = ""
			res.Verified++
			res.Outcomes = append(res.Outcomes, Outcome{CanonicalURL: s.CanonicalURL, Status: StatusVerified})
			continue
		}
		s.Status = StatusRejected
		s.Error = reason
		res.Rejected++
		res.Outcomes = append(res.Outcomes, Outcome{CanonicalURL: s.CanonicalURL, Status: StatusRejected, Reason: reason})
	}
	return res
}

// verifySource returns "" when the source is verifiable, or the rejection
// reason. It checks the manifest schema, the raw file and its hash.
func verifySource(runDir string, s Source) string {
	if strings.TrimSpace(s.CanonicalURL) == "" {
		return "manifest schema: source has no canonical_url"
	}
	if s.RawPath == "" {
		return "no raw payload recorded (raw_path missing)"
	}
	abs := filepath.Join(runDir, filepath.FromSlash(s.RawPath))
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Sprintf("raw payload missing: %s", s.RawPath)
	}
	if info.Size() == 0 {
		return "raw payload is empty"
	}
	if s.SHA256 == "" {
		return "no sha256 recorded"
	}
	sum, size, err := HashFile(abs)
	if err != nil {
		return fmt.Sprintf("cannot hash raw payload: %v", err)
	}
	if sum != s.SHA256 {
		return fmt.Sprintf("sha256 mismatch: manifest %s disk %s", shortHash(s.SHA256), shortHash(sum))
	}
	if s.Bytes != 0 && size != s.Bytes {
		return fmt.Sprintf("size mismatch: manifest %d disk %d", s.Bytes, size)
	}
	return ""
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
