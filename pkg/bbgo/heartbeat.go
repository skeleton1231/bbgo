package bbgo

import (
	"context"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// heartbeatInterval is how often the heartbeat file is refreshed once the
// writer is running. Picked to be well below the manager's stale threshold
// so a single missed tick doesn't trigger a false-positive recovery.
const heartbeatInterval = 60 * time.Second

// StartHeartbeat writes the current time to the file at path on a fixed
// cadence until ctx is canceled. Returns immediately; the writer runs in a
// goroutine. If path is empty, the call is a no-op — callers can
// unconditionally invoke it with the env-var value and let an empty value
// disable the feature.
//
// The SaaS deployment uses this to detect silent hangs: the container keeps
// reporting "running" to Docker, but bbgo itself is wedged and producing no
// trades or logs. A stale heartbeat file lets the manager's recovery loop
// distinguish "alive but quiet" from "dead but not yet reaped".
func StartHeartbeat(ctx context.Context, path string) {
	if path == "" {
		return
	}
	if err := writeHeartbeat(path); err != nil {
		log.WithError(err).Warn("heartbeat initial write failed")
	}
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writeHeartbeat(path); err != nil {
					log.WithError(err).Warn("heartbeat write failed")
				}
			}
		}
	}()
}

func writeHeartbeat(path string) error {
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339Nano)), 0o644)
}
