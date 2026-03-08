package providers

import (
	"fmt"
	"time"

	"github.com/ayoelutilo/agent-platform-interoperability-hub/internal/schema"
)

func NewTrace(provider, operation string, spanNames ...string) schema.Trace {
	now := time.Now().UTC()
	spans := make([]schema.TraceSpan, 0, len(spanNames))
	for i, spanName := range spanNames {
		start := now.Add(time.Duration(i) * time.Millisecond)
		finish := start.Add(1 * time.Millisecond)
		spans = append(spans, schema.TraceSpan{
			SpanID:     fmt.Sprintf("%s-%s-span-%d", provider, operation, i+1),
			Name:       spanName,
			StartedAt:  start,
			FinishedAt: finish,
			Attributes: map[string]string{"provider": provider, "operation": operation},
		})
	}

	return schema.Trace{
		TraceID:   fmt.Sprintf("%s-%s-%d", provider, operation, now.UnixNano()),
		Provider:  provider,
		Operation: operation,
		Spans:     spans,
	}
}

// Refinement.
