package adapters_test

import (
	"context"
	"testing"
	"time"

	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/adapters/azure_foundry"
	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/adapters/bedrock_agentcore"
	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/adapters/vertex_agent_engine"
	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/providers"
	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/schema"
)

func TestAdapterContract(t *testing.T) {
	task := schema.CanonicalTask{
		TaskID:       "task-001",
		Name:         "Route support ticket",
		Instructions: "Classify and assign incoming support ticket",
		Capabilities: []string{"classify", "assign"},
		Input: map[string]any{
			"ticket": "Cannot login",
		},
	}

	adapters := []providers.Adapter{
		azure_foundry.New(),
		bedrock_agentcore.New(),
		vertex_agent_engine.New(),
	}

	for _, adapter := range adapters {
		adapter := adapter
		t.Run(adapter.Name(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			deployResp, err := adapter.Deploy(ctx, providers.DeployRequest{Task: task})
			if err != nil {
				t.Fatalf("Deploy failed: %v", err)
			}
			if deployResp.Provider != adapter.Name() {
				t.Fatalf("expected provider %q, got %q", adapter.Name(), deployResp.Provider)
			}
			if deployResp.DeploymentID == "" {
				t.Fatalf("expected deployment id")
			}
			if len(deployResp.Trace.Spans) == 0 {
				t.Fatalf("expected deploy trace spans")
			}

			runResp, err := adapter.Run(ctx, providers.RunRequest{Task: task, Input: task.Input})
			if err != nil {
				t.Fatalf("Run failed: %v", err)
			}
			if runResp.Provider != adapter.Name() {
				t.Fatalf("expected provider %q, got %q", adapter.Name(), runResp.Provider)
			}
			if runResp.RunID == "" || runResp.Output == "" {
				t.Fatalf("expected populated run response")
			}
			if len(runResp.Trace.Spans) == 0 {
				t.Fatalf("expected run trace spans")
			}

			stream, err := adapter.Stream(ctx, providers.StreamRequest{Task: task, Input: task.Input})
			if err != nil {
				t.Fatalf("Stream failed: %v", err)
			}

			chunks := make([]providers.StreamChunk, 0, 4)
		readLoop:
			for {
				select {
				case chunk, ok := <-stream:
					if !ok {
						break readLoop
					}
					chunks = append(chunks, chunk)
				case <-ctx.Done():
					t.Fatalf("timed out waiting for stream chunks")
				}
			}

			if len(chunks) == 0 {
				t.Fatalf("expected non-empty stream")
			}
			if !chunks[len(chunks)-1].Done {
				t.Fatalf("expected final chunk to be marked done")
			}
			for _, chunk := range chunks {
				if chunk.Provider != adapter.Name() {
					t.Fatalf("expected chunk provider %q, got %q", adapter.Name(), chunk.Provider)
				}
				if len(chunk.Trace.Spans) == 0 {
					t.Fatalf("expected trace span on each stream chunk")
				}
			}

			evalResp, err := adapter.Evaluate(ctx, providers.EvaluateRequest{Task: task, Expected: "login issue", Actual: "login issue"})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if evalResp.Provider != adapter.Name() {
				t.Fatalf("expected provider %q, got %q", adapter.Name(), evalResp.Provider)
			}
			if evalResp.Score < 0 || evalResp.Score > 1 {
				t.Fatalf("expected score in [0,1], got %.2f", evalResp.Score)
			}
			if len(evalResp.Trace.Spans) == 0 {
				t.Fatalf("expected evaluate trace spans")
			}
		})
	}
}

// Refinement.
