package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"diplom.com/m/internal/ports"
)

type SSEHandler struct {
	Broker ports.Broker
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobID") // Go 1.22+ ServeMux patterns; иначе mux/chi
	subject := "ui.jobs." + jobID

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Heartbeat
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Простейший способ: broker.Subscribe вызывает handler на каждое событие (внутри NATS sub)
	errCh := make(chan error, 1)

	go func() {
		errCh <- h.Broker.Subscribe(ctx, subject, "", "", func(evt anyEvent) error {
			b, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
			flusher.Flush()
			return nil
		})
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				return
			}
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

// anyEvent — подставьте ваш domain.Event;
// здесь просто заглушка, чтобы показать поток.
type anyEvent struct{}
