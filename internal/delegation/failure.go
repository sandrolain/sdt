package delegation

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/coder/acp-go-sdk"
)

type FailureKind string

const AvailabilityScopeGroup = "group"

const (
	FailureQuota       FailureKind = "quota_exhausted"
	FailureUnavailable FailureKind = "unavailable"
	FailureAuth        FailureKind = "authentication_required"
	FailurePermission  FailureKind = "permission_denied"
	FailureUnknown     FailureKind = "unknown"
	FailureSuccess     FailureKind = "success"
)

type FailureSignal struct {
	Kind  FailureKind
	Scope string
	Until time.Time
}

// ClassifyACPFailure consumes only the SDK's structured RequestError fields.
// Unrecognized provider payloads remain unknown and never trigger failover.
func ClassifyACPFailure(err error) FailureSignal {
	signal := FailureSignal{Kind: FailureUnknown, Scope: "entry"}
	var request *acp.RequestError
	if !errors.As(err, &request) || request == nil {
		return signal
	}
	if request.Code == -32000 {
		signal.Kind = FailureAuth
		return signal
	}
	data := request.Data
	if raw, ok := data.(json.RawMessage); ok {
		var decoded map[string]any
		if json.Unmarshal(raw, &decoded) == nil {
			data = decoded
		}
	}
	fields, ok := data.(map[string]any)
	if !ok {
		return signal
	}
	marker := stringField(fields, "type")
	if marker == "" {
		marker = stringField(fields, "code")
	}
	switch marker {
	case "quota_exceeded", "rate_limit_exceeded":
		signal.Kind = FailureQuota
	case "service_unavailable", "provider_unavailable":
		signal.Kind = FailureUnavailable
	case "authentication_required", "auth_required":
		signal.Kind = FailureAuth
	case "permission_denied":
		signal.Kind = FailurePermission
	default:
		return signal
	}
	if scope := stringField(fields, "availability_scope"); scope == AvailabilityScopeGroup {
		signal.Scope = scope
	}
	if value := stringField(fields, "retry_at"); value != "" {
		until, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return FailureSignal{Kind: FailureUnknown, Scope: "entry"}
		}
		signal.Until = until.UTC()
	}
	return signal
}

func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string) //nolint:errcheck // A non-string provider field is treated as absent.
	return value
}

func (s FailureSignal) Recoverable() bool {
	return s.Kind == FailureQuota || s.Kind == FailureUnavailable
}

func (s FailureSignal) Validate() error {
	switch s.Kind {
	case FailureQuota, FailureUnavailable, FailureAuth, FailurePermission, FailureUnknown, FailureSuccess:
		return nil
	default:
		return fmt.Errorf("unknown failure class %q", s.Kind)
	}
}

func isACPRequestError(err error) bool {
	var request *acp.RequestError
	return errors.As(err, &request) && request != nil
}
