package bedrock_agentcore

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
	return "bedrock_agentcore"
}

func (a *Adapter) Deploy(ctx context.Context, req providers.DeployRequest) (providers.DeployResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.DeployResponse{}, err
	}

	trace := providers.NewTrace(a.Name(), "deploy", "manifest.parsed", "agent.allocated", "agent.warmed")
	return providers.DeployResponse{
		Provider:     a.Name(),
		DeploymentID: fmt.Sprintf("brdc-%s", req.Task.TaskID),
		Accepted:     true,
		Trace:        trace,
	}, nil
}

func (a *Adapter) Run(ctx context.Context, req providers.RunRequest) (providers.RunResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.RunResponse{}, err
	}

	toolCount := len(req.Task.Capabilities)
	trace := providers.NewTrace(a.Name(), "run", "policy.checked", "reasoning.graph.executed", "postprocess.serialized")
	return providers.RunResponse{
		Provider: a.Name(),
		RunID:    fmt.Sprintf("brrun-%s", req.Task.TaskID),
		Status:   "succeeded",
		Output:   fmt.Sprintf("bedrock_agentcore completed %q using %d capabilities", req.Task.Name, toolCount),
		Trace:    trace,
	}, nil
}

func (a *Adapter) Stream(ctx context.Context, req providers.StreamRequest) (<-chan providers.StreamChunk, error) {
	chunks := make(chan providers.StreamChunk, 5)
	runID := fmt.Sprintf("brstream-%s", req.Task.TaskID)
	parts := []string{"bedrock", "agentcore", "multi", "agent", "done"}

	go func() {
		defer close(chunks)
		for i, part := range parts {
			select {
			case <-ctx.Done():
				return
			default:
			}
			trace := providers.NewTrace(a.Name(), "stream", fmt.Sprintf("frame.%d", i+1))
			chunks <- providers.StreamChunk{
				Provider: a.Name(),
				RunID:    runID,
				Index:    i,
				Content:  strings.ToUpper(part),
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

	score := overlapScore(req.Expected, req.Actual)
	verdict := "fail"
	if score >= 0.9 {
		verdict = "pass"
	} else if score >= 0.5 {
		verdict = "partial"
	}

	trace := providers.NewTrace(a.Name(), "evaluate", "expected.tokenized", "actual.tokenized", "overlap.scored")
	return providers.EvaluateResponse{
		Provider: a.Name(),
		Score:    score,
		Verdict:  verdict,
		Summary:  fmt.Sprintf("token overlap %.2f", score),
		Trace:    trace,
	}, nil
}

func overlapScore(expected, actual string) float64 {
	expectedTokens := tokens(expected)
	actualTokens := tokens(actual)
	if len(expectedTokens) == 0 && len(actualTokens) == 0 {
		return 1
	}
	if len(expectedTokens) == 0 || len(actualTokens) == 0 {
		return 0
	}

	expectedSet := make(map[string]struct{}, len(expectedTokens))
	for _, token := range expectedTokens {
		expectedSet[token] = struct{}{}
	}

	intersections := 0
	union := len(expectedSet)
	seenActual := map[string]struct{}{}
	for _, token := range actualTokens {
		if _, seen := seenActual[token]; seen {
			continue
		}
		seenActual[token] = struct{}{}
		if _, ok := expectedSet[token]; ok {
			intersections++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersections) / float64(union)
}

func tokens(in string) []string {
	fields := strings.Fields(strings.ToLower(in))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.Trim(field, ".,!?;:'\"()[]{}")
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Refinement.
