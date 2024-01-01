package providers

import (
	"context"

	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/schema"
)

type Adapter interface {
	Name() string
	Deploy(ctx context.Context, req DeployRequest) (DeployResponse, error)
	Run(ctx context.Context, req RunRequest) (RunResponse, error)
	Stream(ctx context.Context, req StreamRequest) (<-chan StreamChunk, error)
	Evaluate(ctx context.Context, req EvaluateRequest) (EvaluateResponse, error)
}

type DeployRequest struct {
	Task schema.CanonicalTask `json:"task"`
}

type DeployResponse struct {
	Provider     string       `json:"provider"`
	DeploymentID string       `json:"deployment_id"`
	Accepted     bool         `json:"accepted"`
	Trace        schema.Trace `json:"trace"`
}

type RunRequest struct {
	Task           schema.CanonicalTask `json:"task"`
	Input          map[string]any       `json:"input,omitempty"`
	IdempotencyKey string               `json:"idempotency_key,omitempty"`
}

type RunResponse struct {
	Provider string       `json:"provider"`
	RunID    string       `json:"run_id"`
	Status   string       `json:"status"`
	Output   string       `json:"output"`
	Trace    schema.Trace `json:"trace"`
}

type StreamRequest struct {
	Task  schema.CanonicalTask `json:"task"`
	Input map[string]any       `json:"input,omitempty"`
}

type StreamChunk struct {
	Provider string       `json:"provider"`
	RunID    string       `json:"run_id"`
	Index    int          `json:"index"`
	Content  string       `json:"content"`
	Done     bool         `json:"done"`
	Trace    schema.Trace `json:"trace"`
}

type EvaluateRequest struct {
	Task     schema.CanonicalTask `json:"task"`
	Expected string               `json:"expected"`
	Actual   string               `json:"actual"`
}

type EvaluateResponse struct {
	Provider string       `json:"provider"`
	Score    float64      `json:"score"`
	Verdict  string       `json:"verdict"`
	Summary  string       `json:"summary"`
	Trace    schema.Trace `json:"trace"`
}
