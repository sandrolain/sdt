//go:build unix

package delegation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
)

type TaskMessage struct {
	Sequence  uint64    `json:"sequence"`
	CreatedAt time.Time `json:"created_at"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Text      string    `json:"text"`
	Artifact  string    `json:"artifact,omitempty"`
}

const (
	maxTaskMessages     = 1000
	maxTaskMessageBytes = 5 << 20
)

// SendMessage appends coordination context to one task. Messages are untrusted
// data and do not authorize commands, permissions or task acceptance.
func (s *Store) SendMessage(taskID, from, to, text, artifact string) (TaskMessage, error) {
	if err := validateTaskMessage(from, to, text, artifact); err != nil {
		return TaskMessage{}, err
	}
	var message TaskMessage
	err := s.WithLock("task-"+taskID, func() error {
		task, err := s.Read(taskID)
		if err != nil {
			return err
		}
		participants := taskParticipants(task)
		if !participants[from] || !participants[to] {
			return fmt.Errorf("message sender and recipient must belong to task %q", taskID)
		}
		if artifact != "" && !taskRetainsArtifact(task, artifact) {
			return fmt.Errorf("artifact %q is not retained by task %q", artifact, taskID)
		}
		messages, err := s.readMessages(taskID)
		if err != nil {
			return err
		}
		if len(messages) >= maxTaskMessages {
			return fmt.Errorf("task message count limit reached")
		}
		messageBytes := len(text)
		for _, existing := range messages {
			messageBytes += len(existing.Text) + len(existing.Artifact)
		}
		if messageBytes > maxTaskMessageBytes {
			return fmt.Errorf("task message storage limit reached")
		}
		message = TaskMessage{Sequence: uint64(len(messages) + 1), CreatedAt: time.Now().UTC(), From: from, To: to, Text: text, Artifact: artifact}
		data, err := json.Marshal(message)
		if err != nil {
			return err
		}
		file, err := s.openTaskRelative(filepath.Join("tasks", taskID, "messages.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer file.Close() //nolint:errcheck
		if err := file.Chmod(0o600); err != nil {
			return err
		}
		_, err = file.Write(append(data, '\n'))
		return err
	})
	return message, err
}

func validateTaskMessage(from, to, text, artifact string) error {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" || strings.IndexFunc(from+to, unicode.IsControl) >= 0 || len(from) > 128 || len(to) > 128 || strings.TrimSpace(text) == "" {
		return fmt.Errorf("message sender, recipient and text are required")
	}
	if len(text) > 32<<10 || len(artifact) > 1024 || strings.IndexFunc(artifact, unicode.IsControl) >= 0 || artifact != "" && filepath.Base(artifact) != artifact {
		return fmt.Errorf("message exceeds the task message size limit")
	}
	return nil
}

func taskRetainsArtifact(task Task, artifact string) bool {
	for _, item := range task.Artifacts {
		if item.Name == artifact {
			return true
		}
	}
	return false
}

func (s *Store) ListMessages(taskID, participant string) ([]TaskMessage, error) {
	task, err := s.Read(taskID)
	if err != nil {
		return nil, err
	}
	if participant != "" && !taskParticipants(task)[participant] {
		return nil, fmt.Errorf("participant %q does not belong to task %q", participant, taskID)
	}
	messages, err := s.readMessages(taskID)
	if err != nil || participant == "" {
		return messages, err
	}
	filtered := messages[:0]
	for _, message := range messages {
		if message.To == participant || message.From == participant {
			filtered = append(filtered, message)
		}
	}
	return filtered, nil
}

func (s *Store) readMessages(taskID string) ([]TaskMessage, error) {
	file, err := s.openTaskRelative(filepath.Join("tasks", taskID, "messages.jsonl"), os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck
	var messages []TaskMessage
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		var message TaskMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return nil, fmt.Errorf("decode task message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, scanner.Err()
}

func (s *Store) openTaskRelative(name string, flags int, perm os.FileMode) (*os.File, error) {
	if filepath.IsAbs(name) || filepath.Clean(name) == ".." || strings.HasPrefix(filepath.Clean(name), ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("task path escapes delegation storage")
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck
	clean := filepath.Clean(name)
	parts := strings.Split(clean, string(filepath.Separator))
	for i := range parts {
		prefix := filepath.Join(parts[:i+1]...)
		info, err := root.Lstat(prefix)
		if os.IsNotExist(err) && i == len(parts)-1 && flags&os.O_CREATE != 0 {
			break
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || i < len(parts)-1 && !info.IsDir() || i == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("task path contains a non-regular or symbolic-link component")
		}
	}
	file, err := root.OpenFile(clean, flags|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func taskParticipants(task Task) map[string]bool {
	participants := map[string]bool{"principal": true}
	for _, attempt := range task.Attempts {
		if attempt.Agent != "" {
			participants[attempt.Agent] = true
		}
	}
	var snapshot struct {
		Agents []struct {
			Name         string   `json:"name"`
			Roles        []string `json:"roles"`
			Capabilities []string `json:"capabilities"`
		} `json:"agents"`
	}
	if json.Unmarshal(task.ConfigSnapshot, &snapshot) == nil {
		for _, agent := range snapshot.Agents {
			if agent.Name != "" && containsString(agent.Roles, task.Role) && containsAll(agent.Capabilities, task.RequiredCapabilities) {
				participants[agent.Name] = true
			}
		}
	}
	return participants
}
