package builder

import (
	"context"
	"sync"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/pkg/errors"
)

// ProgressHandler handles BuildKit progress events and converts them to our format
type ProgressHandler struct {
	ch       chan<- *ProgressEvent
	mu       sync.RWMutex
	vertexes map[string]*ProgressVertex
	logs     map[string][]*ProgressLog
}

// ProgressVertex represents a BuildKit vertex (build step)
type ProgressVertex struct {
	ID        string
	Name      string
	Started   *time.Time
	Completed *time.Time
	Error     string
	Cached    bool
}

// ProgressLog represents a log entry from a build step
type ProgressLog struct {
	Vertex    string
	Stream    int
	Data      []byte
	Timestamp time.Time
}

// NewProgressHandler creates a new progress handler
func NewProgressHandler(ch chan<- *ProgressEvent) *ProgressHandler {
	return &ProgressHandler{
		ch:       ch,
		vertexes: make(map[string]*ProgressVertex),
		logs:     make(map[string][]*ProgressLog),
	}
}

// HandleProgress processes BuildKit progress events
func (ph *ProgressHandler) HandleProgress(ctx context.Context, ch chan *client.SolveStatus) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case status, ok := <-ch:
			if !ok {
				return nil
			}
			if err := ph.processStatus(status); err != nil {
				return err
			}
		}
	}
}

// processStatus processes a single BuildKit status update
func (ph *ProgressHandler) processStatus(status *client.SolveStatus) error {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	// Process vertex updates (build steps)
	for _, vertex := range status.Vertexes {
		ph.processVertex(vertex)
	}

	// Process status updates
	for _, st := range status.Statuses {
		ph.processStatusUpdate(st)
	}

	// Process logs
	for _, log := range status.Logs {
		ph.processLog(log)
	}

	return nil
}

// processVertex processes a vertex (build step) update
func (ph *ProgressHandler) processVertex(vertex *client.Vertex) {
	v, exists := ph.vertexes[vertex.Digest.String()]
	if !exists {
		v = &ProgressVertex{
			ID:   vertex.Digest.String(),
			Name: vertex.Name,
		}
		ph.vertexes[vertex.Digest.String()] = v
	}

	// Update vertex state
	if vertex.Started != nil && v.Started == nil {
		v.Started = vertex.Started
		ph.sendProgress(&ProgressEvent{
			ID:        v.ID,
			Name:      v.Name,
			Status:    StatusStarted,
			Timestamp: *vertex.Started,
		})
	}

	if vertex.Completed != nil && v.Completed == nil {
		v.Completed = vertex.Completed
		status := StatusCompleted
		if vertex.Error != "" {
			status = StatusError
			v.Error = vertex.Error
		}
		
		ph.sendProgress(&ProgressEvent{
			ID:        v.ID,
			Name:      v.Name,
			Status:    status,
			Error:     vertex.Error,
			Timestamp: *vertex.Completed,
		})
	}

	if vertex.Cached {
		v.Cached = true
	}
}

// processStatusUpdate processes a status update for a vertex
func (ph *ProgressHandler) processStatusUpdate(status *client.VertexStatus) {
	vertex, exists := ph.vertexes[status.Vertex.String()]
	if !exists {
		return
	}

	var progress *ProgressDetail
	if status.Total != 0 {
		progress = &ProgressDetail{
			Current: status.Current,
			Total:   status.Total,
		}
	}

	ph.sendProgress(&ProgressEvent{
		ID:        vertex.ID,
		Name:      status.Name,
		Status:    StatusRunning,
		Progress:  progress,
		Timestamp: status.Timestamp,
	})
}

// processLog processes a log entry
func (ph *ProgressHandler) processLog(log *client.VertexLog) {
	vertex, exists := ph.vertexes[log.Vertex.String()]
	if !exists {
		return
	}

	// Store log entry
	logEntry := &ProgressLog{
		Vertex:    log.Vertex.String(),
		Stream:    log.Stream,
		Data:      log.Data,
		Timestamp: log.Timestamp,
	}
	ph.logs[log.Vertex.String()] = append(ph.logs[log.Vertex.String()], logEntry)

	// Send progress event with log data
	ph.sendProgress(&ProgressEvent{
		ID:        vertex.ID,
		Name:      vertex.Name,
		Status:    StatusRunning,
		Stream:    string(log.Data),
		Timestamp: log.Timestamp,
	})
}

// sendProgress sends a progress event to the channel
func (ph *ProgressHandler) sendProgress(event *ProgressEvent) {
	if ph.ch != nil {
		select {
		case ph.ch <- event:
		default:
			// Channel is full or closed, skip this update
		}
	}
}

// GetVertexLogs returns all logs for a specific vertex
func (ph *ProgressHandler) GetVertexLogs(vertexID string) []*ProgressLog {
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	
	logs, exists := ph.logs[vertexID]
	if !exists {
		return nil
	}
	
	// Return a copy to avoid race conditions
	result := make([]*ProgressLog, len(logs))
	copy(result, logs)
	return result
}

// GetCompletedVertexes returns all completed vertexes
func (ph *ProgressHandler) GetCompletedVertexes() []*ProgressVertex {
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	
	var completed []*ProgressVertex
	for _, vertex := range ph.vertexes {
		if vertex.Completed != nil {
			completed = append(completed, vertex)
		}
	}
	return completed
}

// GetBuildStats returns build statistics
func (ph *ProgressHandler) GetBuildStats() BuildStats {
	ph.mu.RLock()
	defer ph.mu.RUnlock()
	
	stats := BuildStats{}
	for _, vertex := range ph.vertexes {
		stats.TotalSteps++
		if vertex.Completed != nil {
			stats.CompletedSteps++
			if vertex.Cached {
				stats.CachedSteps++
			}
		}
		if vertex.Error != "" {
			stats.ErroredSteps++
		}
	}
	return stats
}

// BuildStats contains statistics about the build process
type BuildStats struct {
	TotalSteps     int
	CompletedSteps int
	CachedSteps    int
	ErroredSteps   int
}

// EnhancedProgressReporter provides enhanced progress reporting with BuildKit integration
type EnhancedProgressReporter struct {
	handler *ProgressHandler
	stats   BuildStats
}

// NewEnhancedProgressReporter creates a new enhanced progress reporter
func NewEnhancedProgressReporter(ch chan<- *ProgressEvent) *EnhancedProgressReporter {
	return &EnhancedProgressReporter{
		handler: NewProgressHandler(ch),
	}
}

// HandleBuildKitProgress handles BuildKit progress and converts to our events
func (epr *EnhancedProgressReporter) HandleBuildKitProgress(ctx context.Context, ch chan *client.SolveStatus) error {
	return epr.handler.HandleProgress(ctx, ch)
}

// GetStats returns current build statistics
func (epr *EnhancedProgressReporter) GetStats() BuildStats {
	return epr.handler.GetBuildStats()
}

// Close finalizes the progress reporter
func (epr *EnhancedProgressReporter) Close() {
	// Cleanup if needed
}