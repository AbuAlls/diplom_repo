package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type SSEHandler struct {
	Broker ports.Broker
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/jobs/")
	jobID = strings.TrimSuffix(jobID, "/events")
	jobID = strings.Trim(jobID, "/")
	if jobID == "" {
		http.Error(w, "job id is required", http.StatusBadRequest)
		return
	}
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

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	errCh := make(chan error, 1)

	go func() {
		errCh <- h.Broker.Subscribe(ctx, subject, "", "", func(evt domain.Event) error {
			b, _ := json.Marshal(evt)
			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", b); err != nil {
				return err
			}
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
			_, _ = fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}
