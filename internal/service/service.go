package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/adapters/azure_foundry"
	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/adapters/bedrock_agentcore"
	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/adapters/vertex_agent_engine"
	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/providers"
	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/schema"
)

var ErrAdapterNotFound = errors.New("adapter not found")

type HubService struct {
	mu          sync.Mutex
	adapters    map[string]providers.Adapter
	runs        map[string]providers.RunResponse
	idempotency map[string]providers.RunResponse
	inFlight    map[string]*idempotentRunCall
}

type idempotentRunCall struct {
	done chan struct{}
	resp providers.RunResponse
	err  error
}

func NewDefault() *HubService {
	s := &HubService{
		adapters:    map[string]providers.Adapter{},
		runs:        map[string]providers.RunResponse{},
		idempotency: map[string]providers.RunResponse{},
		inFlight:    map[string]*idempotentRunCall{},
	}
	s.Register(azure_foundry.New())
	s.Register(bedrock_agentcore.New())
	s.Register(vertex_agent_engine.New())
	return s
}

func (s *HubService) Register(adapter providers.Adapter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adapters[adapter.Name()] = adapter
}

func (s *HubService) Deploy(ctx context.Context, provider string, task schema.CanonicalTask) (providers.DeployResponse, error) {
	adapter, err := s.adapter(provider)
	if err != nil {
		return providers.DeployResponse{}, err
	}
	return adapter.Deploy(ctx, providers.DeployRequest{Task: task})
}

func (s *HubService) Run(ctx context.Context, provider string, task schema.CanonicalTask, input map[string]any, idempotencyKey string) (providers.RunResponse, bool, error) {
	adapter, err := s.adapter(provider)
	if err != nil {
		return providers.RunResponse{}, false, err
	}

	if idempotencyKey == "" {
		resp, runErr := adapter.Run(ctx, providers.RunRequest{
			Task:           task,
			Input:          input,
			IdempotencyKey: idempotencyKey,
		})
		if runErr != nil {
			return providers.RunResponse{}, false, runErr
		}

		s.mu.Lock()
		s.runs[resp.RunID] = resp
		s.mu.Unlock()
		return resp, true, nil
	}

	key := provider + "::" + idempotencyKey
	var call *idempotentRunCall

	for {
		s.mu.Lock()

		if existing, ok := s.idempotency[key]; ok {
			s.mu.Unlock()
			return existing, false, nil
		}

		inFlight, ok := s.inFlight[key]
		if !ok {
			call = &idempotentRunCall{done: make(chan struct{})}
			s.inFlight[key] = call
			s.mu.Unlock()
			break
		}

		s.mu.Unlock()
		<-inFlight.done
	}

	resp, runErr := adapter.Run(ctx, providers.RunRequest{
		Task:           task,
		Input:          input,
		IdempotencyKey: idempotencyKey,
	})

	s.mu.Lock()
	if runErr == nil {
		s.runs[resp.RunID] = resp
		s.idempotency[key] = resp
	}
	call.resp = resp
	call.err = runErr
	close(call.done)
	delete(s.inFlight, key)
	s.mu.Unlock()

	if runErr != nil {
		return providers.RunResponse{}, false, runErr
	}
	return resp, true, nil
}

func (s *HubService) Stream(ctx context.Context, provider string, task schema.CanonicalTask, input map[string]any) (<-chan providers.StreamChunk, error) {
	adapter, err := s.adapter(provider)
	if err != nil {
		return nil, err
	}
	return adapter.Stream(ctx, providers.StreamRequest{Task: task, Input: input})
}

func (s *HubService) Evaluate(ctx context.Context, provider string, task schema.CanonicalTask, expected, actual string) (providers.EvaluateResponse, error) {
	adapter, err := s.adapter(provider)
	if err != nil {
		return providers.EvaluateResponse{}, err
	}
	return adapter.Evaluate(ctx, providers.EvaluateRequest{Task: task, Expected: expected, Actual: actual})
}

func (s *HubService) Providers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	providersList := make([]string, 0, len(s.adapters))
	for name := range s.adapters {
		providersList = append(providersList, name)
	}
	return providersList
}

func (s *HubService) adapter(provider string) (providers.Adapter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	adapter, ok := s.adapters[provider]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAdapterNotFound, provider)
	}
	return adapter, nil
}

// Refinement.
