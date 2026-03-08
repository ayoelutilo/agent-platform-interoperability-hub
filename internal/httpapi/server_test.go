package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/service"
)

func TestRunEndpointIdempotency(t *testing.T) {
	hub := service.NewDefault()
	ts := httptest.NewServer(NewHandler(hub))
	defer ts.Close()

	body := []byte(`{
		"provider":"azure_foundry",
		"task":{"task_id":"task-12","name":"triage","instructions":"triage incident"},
		"input":{"ticket":"db down"},
		"idempotency_key":"idem-xyz"
	}`)

	firstStatus, firstRunID, firstCreated := callRunEndpoint(t, ts.Client(), ts.URL+"/v1/run", body)
	if firstStatus != http.StatusAccepted {
		t.Fatalf("expected first status 202, got %d", firstStatus)
	}
	if !firstCreated {
		t.Fatalf("expected first run created=true")
	}

	secondStatus, secondRunID, secondCreated := callRunEndpoint(t, ts.Client(), ts.URL+"/v1/run", body)
	if secondStatus != http.StatusOK {
		t.Fatalf("expected second status 200, got %d", secondStatus)
	}
	if secondCreated {
		t.Fatalf("expected deduped run created=false")
	}
	if firstRunID != secondRunID {
		t.Fatalf("expected same run id, got %s and %s", firstRunID, secondRunID)
	}
}

func TestStreamEndpointEmitsChunks(t *testing.T) {
	hub := service.NewDefault()
	ts := httptest.NewServer(NewHandler(hub))
	defer ts.Close()

	reqBody := []byte(`{
		"provider":"vertex_agent_engine",
		"task":{"task_id":"task-stream","name":"stream","instructions":"stream response"},
		"input":{"prompt":"hello"}
	}`)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/stream", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to construct stream request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected stream status 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream content type")
	}

	lineCh := make(chan string, 1)
	reader := bufio.NewReader(resp.Body)
	go func() {
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			if strings.HasPrefix(line, "event: chunk") {
				lineCh <- line
				return
			}
		}
	}()

	select {
	case <-lineCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for streamed chunk event")
	}
}

func callRunEndpoint(t *testing.T, client *http.Client, url string, body []byte) (status int, runID string, created bool) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build run request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("run request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Created bool `json:"created"`
		Run     struct {
			RunID string `json:"run_id"`
		} `json:"run"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode run response: %v", err)
	}

	return resp.StatusCode, payload.Run.RunID, payload.Created
}
