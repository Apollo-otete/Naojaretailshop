package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/port"
)

// The health endpoints are deliberately outside /api/v1: they are operations surface, not part
// of the public contract, and versioning them would imply a compatibility promise to clients
// that are not supposed to call them at all.

func registerHealth(r *gin.Engine, d Deps) {
	r.GET("/healthz", handleLive(d))
	r.GET("/readyz", handleReady(d))
}

// handleLive answers only "is this process able to answer HTTP". It touches no dependency,
// because an orchestrator restarts a pod for a live failure, and restarting a process because
// the database is slow replaces a temporary outage with a permanent one.
func handleLive(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "live",
			"version":        d.Version,
			"uptime_seconds": int64(time.Since(d.StartedAt).Seconds()),
		})
	}
}

type dependencyStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// handleReady reports every dependency individually rather than one boolean. "unhealthy" tells
// an operator nothing; "postgresql: ping failed after 3s" tells them which thing to go fix —
// and it is why readiness takes a list of port.Checker instead of calling the pool directly.
func handleReady(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			wg    sync.WaitGroup
			mu    sync.Mutex
			items = make([]dependencyStatus, 0, len(d.Checkers))
			ready = true
		)

		for _, chk := range d.Checkers {
			wg.Add(1)
			go func(chk port.Checker) {
				defer wg.Done()
				err := chk.Check(c.Request.Context())
				st := dependencyStatus{Name: chk.Name(), Status: "ok"}
				if err != nil {
					st.Status = "error"
					// Error text is a debugging aid, not a report to the internet: a driver
					// message can quote a host or a user, so details stay out of production.
					if !d.Cfg.IsProduction() {
						st.Detail = err.Error()
					}
					logFrom(c, d).Warn("readiness check failed",
						slog.String("dependency", chk.Name()),
						slog.Any("error", err),
					)
				}
				mu.Lock()
				items = append(items, st)
				mu.Unlock()
			}(chk)
		}
		wg.Wait()

		for _, it := range items {
			if it.Status != "ok" {
				ready = false
			}
		}

		status := http.StatusOK
		outcome := "ready"
		if !ready {
			status = http.StatusServiceUnavailable
			outcome = "unready"
		}

		c.JSON(status, gin.H{
			"status":       outcome,
			"version":      d.Version,
			"dependencies": items,
		})
	}
}
