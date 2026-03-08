package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/providers"
	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/schema"
)

func TestRunIdempotencyIsScopedToProvider(t *testing.T) {
	hub := NewDefault()
	task := schema.CanonicalTask{
		TaskID:       "task-01",
		Name:         "Summarize ticket",
		Instructions: "Create concise summary",
	}

	first, created, err := hub.Run(context.Background(), "azure_foundry", task, map[string]any{"text": "hello"}, "idem-1")
	if err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	if !created {
		t.Fatalf("expected first run to be created")
	}

	second, created, err := hub.Run(context.Background(), "azure_foundry", task, map[string]any{"text": "different"}, "idem-1")
	if err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	if created {
		t.Fatalf("expected second run to be idempotent reuse")
	}
	if first.RunID != second.RunID {
		t.Fatalf("expected same run id for deduped calls, got %s and %s", first.RunID, second.RunID)
	}

	third, created, err := hub.Run(context.Background(), "bedrock_agentcore", task, map[string]any{"text": "hello"}, "idem-1")
	if err != nil {
		t.Fatalf("third Run failed: %v", err)
	}
	if !created {
		t.Fatalf("expected different provider to create its own run")
	}
	if third.RunID == first.RunID {
		t.Fatalf("expected provider-scoped run ids to differ")
	}
}

func TestUnknownProviderReturnsNotFoundError(t *testing.T) {
	hub := NewDefault()
	_, _, err := hub.Run(context.Background(), "unknown_provider", schema.CanonicalTask{TaskID: "x"}, nil, "")
	if !errors.Is(err, ErrAdapterNotFound) {
		t.Fatalf("expected ErrAdapterNotFound, got %v", err)
	}
}

func TestRunIdempotencyPreservesResponseByKey(t *testing.T) {
	hub := NewDefault()
	task := schema.CanonicalTask{
		TaskID:       "task-same",
		Name:         "Same task id",
		Instructions: "exercise idempotency cache",
	}

	first, created, err := hub.Run(context.Background(), "azure_foundry", task, map[string]any{"text": "A"}, "key-a")
	if err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	if !created {
		t.Fatalf("expected first run to be newly created")
	}

	second, created, err := hub.Run(context.Background(), "azure_foundry", task, map[string]any{"text": "B"}, "key-b")
	if err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	if !created {
		t.Fatalf("expected second run (different idempotency key) to be newly created")
	}

	again, created, err := hub.Run(context.Background(), "azure_foundry", task, map[string]any{"text": "A-again"}, "key-a")
	if err != nil {
		t.Fatalf("repeat Run failed: %v", err)
	}
	if created {
		t.Fatalf("expected repeat run for key-a to return cached response")
	}

	if first.Output != again.Output {
		t.Fatalf("expected key-a response to remain stable; first=%q repeat=%q", first.Output, again.Output)
	}
	if second.Output == again.Output {
		t.Fatalf("expected key-a response to differ from key-b response")
	}
}

func TestRunIdempotencyConcurrentCallsExecuteOnce(t *testing.T) {
	adapter := &countingAdapter{
		name:  "counting",
		delay: 25 * time.Millisecond,
	}

	hub := &HubService{
		adapters: map[string]providers.Adapter{
			adapter.Name(): adapter,
		},
		runs:        map[string]providers.RunResponse{},
		idempotency: map[string]providers.RunResponse{},
		inFlight:    map[string]*idempotentRunCall{},
	}

	task := schema.CanonicalTask{
		TaskID:       "task-race",
		Name:         "Concurrency",
		Instructions: "run once",
	}

	const workers = 12
	var wg sync.WaitGroup
	wg.Add(workers)

	var createdCount atomic.Int32
	var errCount atomic.Int32

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, created, err := hub.Run(context.Background(), adapter.Name(), task, map[string]any{"value": "x"}, "same-key")
			if err != nil {
				errCount.Add(1)
				return
			}
			if created {
				createdCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if errCount.Load() != 0 {
		t.Fatalf("expected no errors, got %d", errCount.Load())
	}
	if createdCount.Load() != 1 {
		t.Fatalf("expected exactly one creator, got %d", createdCount.Load())
	}
	if adapter.runCalls.Load() != 1 {
		t.Fatalf("expected adapter run to execute once, got %d", adapter.runCalls.Load())
	}
}

type countingAdapter struct {
	name     string
	delay    time.Duration
	runCalls atomic.Int32
}

func (a *countingAdapter) Name() string {
	return a.name
}

func (a *countingAdapter) Deploy(_ context.Context, req providers.DeployRequest) (providers.DeployResponse, error) {
	return providers.DeployResponse{
		Provider:     a.name,
		DeploymentID: fmt.Sprintf("dep-%s", req.Task.TaskID),
		Accepted:     true,
	}, nil
}

func (a *countingAdapter) Run(ctx context.Context, req providers.RunRequest) (providers.RunResponse, error) {
	if a.delay > 0 {
		select {
		case <-ctx.Done():
			return providers.RunResponse{}, ctx.Err()
		case <-time.After(a.delay):
		}
	}

	a.runCalls.Add(1)
	return providers.RunResponse{
		Provider: a.name,
		RunID:    fmt.Sprintf("run-%s", req.Task.TaskID),
		Status:   "succeeded",
		Output:   "ok",
	}, nil
}

func (a *countingAdapter) Stream(_ context.Context, _ providers.StreamRequest) (<-chan providers.StreamChunk, error) {
	ch := make(chan providers.StreamChunk)
	close(ch)
	return ch, nil
}

func (a *countingAdapter) Evaluate(_ context.Context, _ providers.EvaluateRequest) (providers.EvaluateResponse, error) {
	return providers.EvaluateResponse{
		Provider: a.name,
		Score:    1,
		Verdict:  "pass",
		Summary:  "ok",
	}, nil
}

// Refinement.
