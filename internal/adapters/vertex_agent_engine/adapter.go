package vertex_agent_engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/providers"
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Name() string {
	return "vertex_agent_engine"
}

func (a *Adapter) Deploy(ctx context.Context, req providers.DeployRequest) (providers.DeployResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.DeployResponse{}, err
	}

	trace := providers.NewTrace(a.Name(), "deploy", "model.selected", "policy.bound", "endpoint.active")
	return providers.DeployResponse{
		Provider:     a.Name(),
		DeploymentID: fmt.Sprintf("vtx-%s", req.Task.TaskID),
		Accepted:     true,
		Trace:        trace,
	}, nil
}

func (a *Adapter) Run(ctx context.Context, req providers.RunRequest) (providers.RunResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.RunResponse{}, err
	}

	trace := providers.NewTrace(a.Name(), "run", "prompt.composed", "reasoning.executed", "result.grounded")
	return providers.RunResponse{
		Provider: a.Name(),
		RunID:    fmt.Sprintf("vtxrun-%s", req.Task.TaskID),
		Status:   "succeeded",
		Output:   fmt.Sprintf("vertex_agent_engine resolved task %q with instruction length %d", req.Task.Name, len(req.Task.Instructions)),
		Trace:    trace,
	}, nil
}

func (a *Adapter) Stream(ctx context.Context, req providers.StreamRequest) (<-chan providers.StreamChunk, error) {
	chunks := make(chan providers.StreamChunk, 4)
	runID := fmt.Sprintf("vtxstream-%s", req.Task.TaskID)
	parts := []string{"vertex", "agent", "engine", "ok"}

	go func() {
		defer close(chunks)
		for i, part := range parts {
			select {
			case <-ctx.Done():
				return
			default:
			}
			trace := providers.NewTrace(a.Name(), "stream", fmt.Sprintf("segment.%d", i+1))
			chunks <- providers.StreamChunk{
				Provider: a.Name(),
				RunID:    runID,
				Index:    i,
				Content:  strings.Repeat(part, 1),
				Done:     i == len(parts)-1,
				Trace:    trace,
			}
		}
	}()

	return chunks, nil
}

func (a *Adapter) Evaluate(ctx context.Context, req providers.EvaluateRequest) (providers.EvaluateResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.EvaluateResponse{}, err
	}

	score := prefixScore(req.Expected, req.Actual)
	verdict := "fail"
	if score >= 0.9 {
		verdict = "pass"
	} else if score >= 0.5 {
		verdict = "partial"
	}

	trace := providers.NewTrace(a.Name(), "evaluate", "sequence.compared", "score.normalized", "verdict.published")
	return providers.EvaluateResponse{
		Provider: a.Name(),
		Score:    score,
		Verdict:  verdict,
		Summary:  fmt.Sprintf("shared-prefix score %.2f", score),
		Trace:    trace,
	}, nil
}

func prefixScore(expected, actual string) float64 {
	expected = strings.TrimSpace(strings.ToLower(expected))
	actual = strings.TrimSpace(strings.ToLower(actual))
	if expected == "" && actual == "" {
		return 1
	}
	if expected == "" || actual == "" {
		return 0
	}

	expectedRunes := []rune(expected)
	actualRunes := []rune(actual)
	limit := len(expectedRunes)
	if len(actualRunes) < limit {
		limit = len(actualRunes)
	}

	commonPrefix := 0
	for i := 0; i < limit; i++ {
		if expectedRunes[i] != actualRunes[i] {
			break
		}
		commonPrefix++
	}

	denominator := len(expectedRunes)
	if len(actualRunes) > denominator {
		denominator = len(actualRunes)
	}
	if denominator == 0 {
		return 0
	}
	return float64(commonPrefix) / float64(denominator)
}

// Refinement.
