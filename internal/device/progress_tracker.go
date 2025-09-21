package device

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/entro314-labs/ember/internal/logger"
	"github.com/entro314-labs/ember/internal/types"
)

// ProgressTracker manages progress reporting across multiple operations
type ProgressTracker struct {
	operations   map[string]*Operation
	subscribers  []ProgressSubscriber
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	totalOps     int
	completedOps int
}

// Operation represents a trackable operation
type Operation struct {
	ID          string
	Name        string
	Status      OperationStatus
	Progress    float64 // 0.0 to 100.0
	StartTime   time.Time
	EndTime     *time.Time
	CurrentFile string
	BytesTotal  int64
	BytesDone   int64
	Error       error
	SubOps      []*Operation
	Parent      *Operation
	mu          sync.RWMutex
}

// OperationStatus represents the status of an operation
type OperationStatus int

const (
	StatusPending OperationStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCanceled
)

// String returns string representation of operation status
func (s OperationStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// ProgressUpdate contains progress information
type ProgressUpdate struct {
	OperationID   string
	OperationName string
	Status        OperationStatus
	Progress      float64
	CurrentFile   string
	BytesTotal    int64
	BytesDone     int64
	EstimatedTime *time.Duration
	Message       string
	Error         error
	Timestamp     time.Time
}

// ProgressSubscriber defines the interface for progress updates
type ProgressSubscriber interface {
	OnProgressUpdate(update ProgressUpdate)
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProgressTracker{
		operations:  make(map[string]*Operation),
		subscribers: make([]ProgressSubscriber, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Subscribe adds a progress subscriber
func (pt *ProgressTracker) Subscribe(subscriber ProgressSubscriber) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.subscribers = append(pt.subscribers, subscriber)
}

// StartOperation creates and starts tracking a new operation
func (pt *ProgressTracker) StartOperation(id, name string) *Operation {
	op := &Operation{
		ID:        id,
		Name:      name,
		Status:    StatusRunning,
		Progress:  0.0,
		StartTime: time.Now(),
		SubOps:    make([]*Operation, 0),
	}

	pt.mu.Lock()
	pt.operations[id] = op
	pt.totalOps++
	pt.mu.Unlock()

	// Notify subscribers outside of the lock to avoid deadlock
	pt.notifySubscribers(ProgressUpdate{
		OperationID:   id,
		OperationName: name,
		Status:        StatusRunning,
		Progress:      0.0,
		Message:       fmt.Sprintf("Started %s", name),
		Timestamp:     time.Now(),
	})

	return op
}

// UpdateProgress updates the progress of an operation
func (pt *ProgressTracker) UpdateProgress(id string, progress float64, currentFile string, bytesDone, bytesTotal int64) {
	pt.mu.Lock()
	op, exists := pt.operations[id]
	pt.mu.Unlock()

	if !exists {
		return
	}

	op.mu.Lock()
	op.Progress = progress
	op.CurrentFile = currentFile
	op.BytesDone = bytesDone
	op.BytesTotal = bytesTotal
	op.mu.Unlock()

	// Calculate estimated time
	var estimatedTime *time.Duration
	if progress > 0 {
		elapsed := time.Since(op.StartTime)
		totalEstimated := time.Duration(float64(elapsed) * 100.0 / progress)
		remaining := totalEstimated - elapsed
		estimatedTime = &remaining
	}

	pt.notifySubscribers(ProgressUpdate{
		OperationID:   id,
		OperationName: op.Name,
		Status:        op.Status,
		Progress:      progress,
		CurrentFile:   currentFile,
		BytesTotal:    bytesTotal,
		BytesDone:     bytesDone,
		EstimatedTime: estimatedTime,
		Message:       fmt.Sprintf("Processing %s (%.1f%%)", currentFile, progress),
		Timestamp:     time.Now(),
	})
}

// CompleteOperation marks an operation as completed
func (pt *ProgressTracker) CompleteOperation(id string) {
	pt.mu.Lock()
	op, exists := pt.operations[id]
	if exists {
		pt.completedOps++
	}
	pt.mu.Unlock()

	if !exists {
		return
	}

	now := time.Now()
	op.mu.Lock()
	op.Status = StatusCompleted
	op.Progress = 100.0
	op.EndTime = &now
	opName := op.Name // Copy name to avoid accessing op after unlock
	op.mu.Unlock()

	pt.notifySubscribers(ProgressUpdate{
		OperationID:   id,
		OperationName: opName,
		Status:        StatusCompleted,
		Progress:      100.0,
		Message:       fmt.Sprintf("Completed %s", opName),
		Timestamp:     time.Now(),
	})
}

// FailOperation marks an operation as failed
func (pt *ProgressTracker) FailOperation(id string, err error) {
	pt.mu.Lock()
	op, exists := pt.operations[id]
	if exists {
		pt.completedOps++
	}
	pt.mu.Unlock()

	if !exists {
		return
	}

	now := time.Now()
	op.mu.Lock()
	op.Status = StatusFailed
	op.Error = err
	op.EndTime = &now
	opName := op.Name     // Copy name to avoid accessing op after unlock
	opProgress := op.Progress // Copy progress value
	op.mu.Unlock()

	pt.notifySubscribers(ProgressUpdate{
		OperationID:   id,
		OperationName: opName,
		Status:        StatusFailed,
		Progress:      opProgress,
		Error:         err,
		Message:       fmt.Sprintf("Failed %s: %v", opName, err),
		Timestamp:     time.Now(),
	})
}

// CancelOperation cancels an operation
func (pt *ProgressTracker) CancelOperation(id string) {
	pt.mu.Lock()
	op, exists := pt.operations[id]
	pt.mu.Unlock()

	if !exists {
		return
	}

	now := time.Now()
	op.mu.Lock()
	op.Status = StatusCanceled
	op.EndTime = &now
	opName := op.Name     // Copy name to avoid accessing op after unlock
	opProgress := op.Progress // Copy progress value
	op.mu.Unlock()

	pt.notifySubscribers(ProgressUpdate{
		OperationID:   id,
		OperationName: opName,
		Status:        StatusCanceled,
		Progress:      opProgress,
		Message:       fmt.Sprintf("Canceled %s", opName),
		Timestamp:     time.Now(),
	})
}

// AddSubOperation adds a sub-operation to an existing operation
func (pt *ProgressTracker) AddSubOperation(parentID, subID, subName string) *Operation {
	pt.mu.Lock()
	parentOp, exists := pt.operations[parentID]
	pt.mu.Unlock()

	if !exists {
		return nil
	}

	subOp := &Operation{
		ID:        subID,
		Name:      subName,
		Status:    StatusPending,
		Progress:  0.0,
		StartTime: time.Now(),
		SubOps:    make([]*Operation, 0),
		Parent:    parentOp,
	}

	parentOp.mu.Lock()
	parentOp.SubOps = append(parentOp.SubOps, subOp)
	parentOp.mu.Unlock()

	pt.mu.Lock()
	pt.operations[subID] = subOp
	pt.mu.Unlock()

	return subOp
}

// GetOperation returns an operation by ID
func (pt *ProgressTracker) GetOperation(id string) *Operation {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.operations[id]
}

// GetAllOperations returns all operations
func (pt *ProgressTracker) GetAllOperations() map[string]*Operation {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make(map[string]*Operation)
	for k, v := range pt.operations {
		result[k] = v
	}
	return result
}

// GetOverallProgress calculates overall progress across all operations
func (pt *ProgressTracker) GetOverallProgress() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.totalOps == 0 {
		return 0.0
	}

	var totalProgress float64
	for _, op := range pt.operations {
		op.mu.RLock()
		totalProgress += op.Progress
		op.mu.RUnlock()
	}

	return totalProgress / float64(pt.totalOps)
}

// notifySubscribers sends updates to all subscribers
func (pt *ProgressTracker) notifySubscribers(update ProgressUpdate) {
	pt.mu.RLock()
	subscribers := make([]ProgressSubscriber, len(pt.subscribers))
	copy(subscribers, pt.subscribers)
	pt.mu.RUnlock()

	for _, subscriber := range subscribers {
		go func(s ProgressSubscriber) {
			defer func() {
				if r := recover(); r != nil {
					logger.GetLogger().Warn("Progress subscriber panic", "error", r)
				}
			}()
			s.OnProgressUpdate(update)
		}(subscriber)
	}
}

// StartPeriodicUpdates starts sending periodic progress updates
func (pt *ProgressTracker) StartPeriodicUpdates(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-pt.ctx.Done():
				return
			case <-ticker.C:
				pt.sendPeriodicUpdate()
			}
		}
	}()
}

// sendPeriodicUpdate sends a periodic update with overall progress
func (pt *ProgressTracker) sendPeriodicUpdate() {
	overallProgress := pt.GetOverallProgress()

	// Get operation counts outside of the notification
	pt.mu.RLock()
	totalOps := pt.totalOps
	completedOps := pt.completedOps
	pt.mu.RUnlock()

	pt.notifySubscribers(ProgressUpdate{
		OperationID:   "overall",
		OperationName: "Overall Progress",
		Status:        StatusRunning,
		Progress:      overallProgress,
		Message:       fmt.Sprintf("Overall progress: %.1f%% (%d/%d operations)", overallProgress, completedOps, totalOps),
		Timestamp:     time.Now(),
	})
}

// Stop stops the progress tracker
func (pt *ProgressTracker) Stop() {
	pt.cancel()
}

// ConsoleProgressSubscriber implements progress updates to console
type ConsoleProgressSubscriber struct {
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewConsoleProgressSubscriber creates a new console progress subscriber
func NewConsoleProgressSubscriber() *ConsoleProgressSubscriber {
	return &ConsoleProgressSubscriber{}
}

// OnProgressUpdate handles progress updates for console output
func (cps *ConsoleProgressSubscriber) OnProgressUpdate(update ProgressUpdate) {
	cps.mu.Lock()
	defer cps.mu.Unlock()

	// Throttle console updates to avoid spam
	if time.Since(cps.lastUpdate) < 500*time.Millisecond && update.Status == StatusRunning {
		return
	}
	cps.lastUpdate = time.Now()

	switch update.Status {
	case StatusRunning:
		if update.CurrentFile != "" {
			logger.GetLogger().Info("[%s] %.1f%% - %s", update.OperationName, update.Progress, update.CurrentFile)
		} else {
			logger.GetLogger().Info("[%s] %.1f%% - %s", update.OperationName, update.Progress, update.Message)
		}
	case StatusCompleted:
		logger.GetLogger().Info("[%s] ✓ Completed", update.OperationName)
	case StatusFailed:
		logger.GetLogger().Error("[%s] ✗ Failed: %v", update.OperationName, update.Error)
	case StatusCanceled:
		logger.GetLogger().Warn("[%s] ⚠ Canceled", update.OperationName)
	}
}

// FileProgressCallback creates a callback function for file operations
func (pt *ProgressTracker) FileProgressCallback(operationID string) types.FileProgressCallback {
	return func(current, total int64, currentFile string) {
		if total > 0 {
			progress := float64(current) / float64(total) * 100.0
			pt.UpdateProgress(operationID, progress, currentFile, current, total)
		}
	}
}

// TrackOperation is a helper function to track an operation with automatic completion/failure
func (pt *ProgressTracker) TrackOperation(id, name string, fn func() error) error {
	_ = pt.StartOperation(id, name)
	defer func() {
		if r := recover(); r != nil {
			pt.FailOperation(id, fmt.Errorf("panic: %v", r))
			panic(r)
		}
	}()

	err := fn()
	if err != nil {
		pt.FailOperation(id, err)
		return err
	}

	pt.CompleteOperation(id)
	return nil
}

// GetFormattedDuration returns a human-readable duration
func GetFormattedDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-60*d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) - 60*hours
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
}

// GetOperationSummary returns a summary of the operation status
func (pt *ProgressTracker) GetOperationSummary() map[string]interface{} {
	pt.mu.RLock()

	// Calculate overall progress without calling GetOverallProgress to avoid recursive lock
	var totalProgress float64
	totalOps := pt.totalOps
	if totalOps > 0 {
		for _, op := range pt.operations {
			op.mu.RLock()
			totalProgress += op.Progress
			op.mu.RUnlock()
		}
		totalProgress = totalProgress / float64(totalOps)
	}

	summary := map[string]interface{}{
		"total_operations":     pt.totalOps,
		"completed_operations": pt.completedOps,
		"overall_progress":     totalProgress,
		"operations":           make([]map[string]interface{}, 0),
	}

	for _, op := range pt.operations {
		op.mu.RLock()
		opSummary := map[string]interface{}{
			"id":           op.ID,
			"name":         op.Name,
			"status":       op.Status.String(),
			"progress":     op.Progress,
			"current_file": op.CurrentFile,
			"start_time":   op.StartTime,
		}
		if op.EndTime != nil {
			opSummary["end_time"] = *op.EndTime
			opSummary["duration"] = GetFormattedDuration(op.EndTime.Sub(op.StartTime))
		}
		if op.Error != nil {
			opSummary["error"] = op.Error.Error()
		}
		op.mu.RUnlock()

		summary["operations"] = append(summary["operations"].([]map[string]interface{}), opSummary)
	}

	pt.mu.RUnlock()
	return summary
}