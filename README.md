# agent-platform-interoperability-hub

A Go interoperability hub exposing a canonical task API across multiple agent providers.

## Features

- Canonical task schema shared across providers.
- Adapter interface for deployment, execution, streaming, and evaluation.
- Three mock adapters:
  - `azure_foundry`
  - `bedrock_agentcore`
  - `vertex_agent_engine`
- REST endpoints:
  - `POST /v1/deploy`
  - `POST /v1/run`
  - `POST /v1/stream` (SSE)
  - `POST /v1/evaluate`
  - `GET /v1/providers`
- Simulated distributed traces returned by all operations.
- Run idempotency keyed per provider.

## Quick start

```bash
make run
```

Service listens on `:8090` by default (override with `PORT`).

## API examples

### Deploy

```bash
curl -X POST http://localhost:8090/v1/deploy \
  -H 'Content-Type: application/json' \
  -d '{
    "provider":"azure_foundry",
    "task":{"task_id":"task-001","name":"triage","instructions":"triage tickets"}
  }'
```

### Run

```bash
curl -X POST http://localhost:8090/v1/run \
  -H 'Content-Type: application/json' \
  -d '{
    "provider":"bedrock_agentcore",
    "task":{"task_id":"task-002","name":"summarize","instructions":"summarize incident"},
    "input":{"ticket":"db latency"},
    "idempotency_key":"run-42"
  }'
```

### Stream

```bash
curl -N -X POST http://localhost:8090/v1/stream \
  -H 'Content-Type: application/json' \
  -d '{
    "provider":"vertex_agent_engine",
    "task":{"task_id":"task-003","name":"chat","instructions":"respond"},
    "input":{"prompt":"hello"}
  }'
```

### Evaluate

```bash
curl -X POST http://localhost:8090/v1/evaluate \
  -H 'Content-Type: application/json' \
  -d '{
    "provider":"azure_foundry",
    "task":{"task_id":"task-004","name":"qa","instructions":"grade answer"},
    "expected":"resolved",
    "actual":"resolved"
  }'
```

## Testing

```bash
make test
```

- Changelog: minor updates.

- Changelog: minor updates.
