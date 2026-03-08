package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/schema"
	"github.com/oss-showcase/agent-platform-interoperability-hub/internal/service"
)

type Server struct {
	hub *service.HubService
}

const maxJSONBodyBytes = 1 << 20

type deployRequest struct {
	Provider string               `json:"provider"`
	Task     schema.CanonicalTask `json:"task"`
}

type runRequest struct {
	Provider       string               `json:"provider"`
	Task           schema.CanonicalTask `json:"task"`
	Input          map[string]any       `json:"input,omitempty"`
	IdempotencyKey string               `json:"idempotency_key,omitempty"`
}

type streamRequest struct {
	Provider string               `json:"provider"`
	Task     schema.CanonicalTask `json:"task"`
	Input    map[string]any       `json:"input,omitempty"`
}

type evaluateRequest struct {
	Provider string               `json:"provider"`
	Task     schema.CanonicalTask `json:"task"`
	Expected string               `json:"expected"`
	Actual   string               `json:"actual"`
}

func NewHandler(hub *service.HubService) http.Handler {
	s := &Server{hub: hub}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/providers", s.handleProviders)
	mux.HandleFunc("/v1/deploy", s.handleDeploy)
	mux.HandleFunc("/v1/run", s.handleRun)
	mux.HandleFunc("/v1/stream", s.handleStream)
	mux.HandleFunc("/v1/evaluate", s.handleEvaluate)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.hub.Providers()})
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req deployRequest
	if err := decodeJSON(limitBody(w, r), &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.hub.Deploy(r.Context(), strings.TrimSpace(req.Provider), req.Task)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req runRequest
	if err := decodeJSON(limitBody(w, r), &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, created, err := s.hub.Run(r.Context(), strings.TrimSpace(req.Provider), req.Task, req.Input, strings.TrimSpace(req.IdempotencyKey))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	statusCode := http.StatusAccepted
	if !created {
		statusCode = http.StatusOK
	}
	writeJSON(w, statusCode, map[string]any{
		"created": created,
		"run":     response,
	})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	var req streamRequest
	if err := decodeJSON(limitBody(w, r), &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	chunks, err := s.hub.Stream(r.Context(), strings.TrimSpace(req.Provider), req.Task, req.Input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")

	for chunk := range chunks {
		payload, marshalErr := json.Marshal(chunk)
		if marshalErr != nil {
			writeError(w, http.StatusInternalServerError, marshalErr.Error())
			return
		}
		if _, writeErr := fmt.Fprintf(w, "event: chunk\ndata: %s\n\n", payload); writeErr != nil {
			return
		}
		flusher.Flush()
	}
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req evaluateRequest
	if err := decodeJSON(limitBody(w, r), &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := s.hub.Evaluate(r.Context(), strings.TrimSpace(req.Provider), req.Task, req.Expected, req.Actual)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func decodeJSON(body io.ReadCloser, out any) error {
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("invalid request body: multiple JSON values")
	}
	return nil
}

func limitBody(w http.ResponseWriter, r *http.Request) io.ReadCloser {
	return http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
}

func writeServiceError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	if errors.Is(err, service.ErrAdapterNotFound) {
		statusCode = http.StatusNotFound
	} else if errors.Is(err, context.DeadlineExceeded) {
		statusCode = http.StatusGatewayTimeout
	} else if errors.Is(err, context.Canceled) {
		statusCode = http.StatusRequestTimeout
	}
	writeError(w, statusCode, err.Error())
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]any{"error": message})
}

// Refinement.

// Refinement.
