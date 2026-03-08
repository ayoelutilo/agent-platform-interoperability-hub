package schema

import "time"

type CanonicalTask struct {
	TaskID        string            `json:"task_id"`
	Name          string            `json:"name"`
	Instructions  string            `json:"instructions"`
	Input         map[string]any    `json:"input,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	RequestedBy   string            `json:"requested_by,omitempty"`
	CorrelationID string            `json:"correlation_id,omitempty"`
}

type Trace struct {
	TraceID   string      `json:"trace_id"`
	Provider  string      `json:"provider"`
	Operation string      `json:"operation"`
	Spans     []TraceSpan `json:"spans"`
}

type TraceSpan struct {
	SpanID     string            `json:"span_id"`
	Name       string            `json:"name"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at"`
	Attributes map[string]string `json:"attributes,omitempty"`
}
