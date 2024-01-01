# ADR 0001: Canonical Adapter Hub Architecture

## Status

Accepted

## Context

We need a hub that can expose a stable API while supporting multiple agent platform backends with differing execution semantics.

## Decision

Adopt a canonical schema + adapter architecture:

- `internal/schema`: canonical task and trace structures.
- `internal/providers`: adapter contract and shared provider request/response models.
- `internal/adapters/*`: provider-specific implementations (`azure_foundry`, `bedrock_agentcore`, `vertex_agent_engine`).
- `internal/service`: provider registry, idempotent run orchestration, and routing logic.
- `internal/httpapi`: transport boundary exposing deploy/run/stream/evaluate REST APIs.

Streaming is represented as SSE at the HTTP layer, with adapter-level chunk channels.

## Consequences

Positive:

- Uniform API regardless of provider.
- Clear seam for plugging in real SDK-backed adapters later.
- Deterministic contract tests across adapters.

Tradeoffs:

- Current adapters are mocks and not production SDK integrations.
- In-memory idempotency is process-local and non-durable.
