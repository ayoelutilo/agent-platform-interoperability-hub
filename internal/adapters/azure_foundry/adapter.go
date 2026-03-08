package azure_foundry

import (
	"context"
	"fmt"
	"strings"

	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/providers"
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Name() string {
	return "azure_foundry"
}

func (a *Adapter) Deploy(ctx context.Context, req providers.DeployRequest) (providers.DeployResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.DeployResponse{}, err
	}

	deploymentID := fmt.Sprintf("azfd-%s", req.Task.TaskID)
	trace := providers.NewTrace(a.Name(), "deploy", "request.accepted", "plan.generated", "deployment.ready")

	return providers.DeployResponse{
		Provider:     a.Name(),
		DeploymentID: deploymentID,
		Accepted:     true,
		Trace:        trace,
	}, nil
}

func (a *Adapter) Run(ctx context.Context, req providers.RunRequest) (providers.RunResponse, error) {
	if err := ctx.Err(); err != nil {
		return providers.RunResponse{}, err
	}

	runID := fmt.Sprintf("azrun-%s", req.Task.TaskID)
	output := fmt.Sprintf("azure_foundry executed task %q with %d input fields", req.Task.Name, len(req.Input))
	trace := providers.NewTrace(a.Name(), "run", "request.validated", "toolchain.executed", "response.normalized")

	return providers.RunResponse{
		Provider: a.Name(),
		RunID:    runID,
		Status:   "succeeded",
		Output:   output,
		Trace:    trace,
	}, nil
}

func (a *Adapter) Stream(ctx context.Context, req providers.StreamRequest) (<-chan providers.StreamChunk, error) {
	chunks := make(chan providers.StreamChunk, 4)
	runID := fmt.Sprintf("azstream-%s", req.Task.TaskID)
	parts := []string{"azure", "foundry", "stream", "completed"}

	go func() {
		defer close(chunks)
		for i, part := range parts {
			select {
			case <-ctx.Done():
				return
			default:
			}
			trace := providers.NewTrace(a.Name(), "stream", fmt.Sprintf("token.chunk.%d", i+1))
			chunks <- providers.StreamChunk{
				Provider: a.Name(),
				RunID:    runID,
				Index:    i,
				Content:  part,
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

	score := similarity(req.Expected, req.Actual)
	verdict := "needs_review"
	if score >= 0.95 {
		verdict = "pass"
	} else if score >= 0.6 {
		verdict = "partial"
	}
	trace := providers.NewTrace(a.Name(), "evaluate", "eval.input_parsed", "eval.scored", "eval.reported")

	return providers.EvaluateResponse{
		Provider: a.Name(),
		Score:    score,
		Verdict:  verdict,
		Summary:  fmt.Sprintf("azure heuristic similarity %.2f", score),
		Trace:    trace,
	}, nil
}

func similarity(expected, actual string) float64 {
	expected = strings.TrimSpace(strings.ToLower(expected))
	actual = strings.TrimSpace(strings.ToLower(actual))
	if expected == "" && actual == "" {
		return 1
	}
	if expected == actual {
		return 1
	}
	if expected == "" || actual == "" {
		return 0
	}

	expectedSet := map[rune]struct{}{}
	for _, r := range expected {
		expectedSet[r] = struct{}{}
	}

	intersections := 0
	union := len(expectedSet)
	seenActual := map[rune]struct{}{}
	for _, r := range actual {
		if _, seen := seenActual[r]; seen {
			continue
		}
		seenActual[r] = struct{}{}
		if _, ok := expectedSet[r]; ok {
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
