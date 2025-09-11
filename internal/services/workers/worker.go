package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/jobs"
)

// JobProcessor defines the interface for processing different job types
type JobProcessor interface {
	ProcessJob(ctx context.Context, job *models.Job) error
	CanProcess(jobType models.JobType) bool
}

// Worker represents a background worker that processes jobs
type Worker struct {
	id           string
	jobService   jobs.Service
	processors   []JobProcessor
	stopChan     chan struct{}
	wg           sync.WaitGroup
	pollInterval time.Duration
}

// NewWorker creates a new worker instance
func NewWorker(id string, jobService jobs.Service, pollInterval time.Duration) *Worker {
	return &Worker{
		id:           id,
		jobService:   jobService,
		processors:   make([]JobProcessor, 0),
		stopChan:     make(chan struct{}),
		pollInterval: pollInterval,
	}
}

// RegisterProcessor registers a job processor
func (w *Worker) RegisterProcessor(processor JobProcessor) {
	w.processors = append(w.processors, processor)
}

// Start starts the worker in a goroutine
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop stops the worker gracefully
func (w *Worker) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}

// run is the main worker loop
func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	log.Printf("Worker %s starting", w.id)
	defer log.Printf("Worker %s stopped", w.id)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			if err := w.processNextJob(ctx); err != nil {
				log.Printf("Worker %s: error processing job: %v", w.id, err)
			}
		}
	}
}

// processNextJob claims and processes the next available job
func (w *Worker) processNextJob(ctx context.Context) error {
	// Get supported job types from processors
	var supportedTypes []models.JobType
	typeMap := make(map[models.JobType]bool)

	// Collect all unique job types from all processors
	allJobTypes := []models.JobType{
		models.JobTypeWaveformGeneration,
		models.JobTypeTranscriptionGeneration,
		models.JobTypePodcastSync,
	}

	for _, jobType := range allJobTypes {
		for _, p := range w.processors {
			if p.CanProcess(jobType) && !typeMap[jobType] {
				supportedTypes = append(supportedTypes, jobType)
				typeMap[jobType] = true
			}
		}
	}

	if len(supportedTypes) == 0 {
		return fmt.Errorf("no job processors registered")
	}

	// Claim next job
	job, err := w.jobService.ClaimNextJob(ctx, w.id, supportedTypes)
	if err != nil {
		// No jobs available is not an error
		return nil
	}
	if job == nil {
		// No jobs available
		return nil
	}

	log.Printf("Worker %s claimed job %d (type: %s)", w.id, job.ID, job.Type)

	// Find appropriate processor
	var processor JobProcessor
	for _, p := range w.processors {
		if p.CanProcess(job.Type) {
			processor = p
			break
		}
	}

	if processor == nil {
		return fmt.Errorf("no processor found for job type %s", job.Type)
	}

	// Process the job
	err = processor.ProcessJob(ctx, job)
	if err != nil {
		// Check if error is a structured error from waveform processor
		if structuredErr, ok := err.(*models.StructuredJobError); ok {
			// Use enhanced FailJob with error details
			failErr := w.jobService.FailJobWithDetails(ctx, job.ID, structuredErr.Type, structuredErr.Code, structuredErr.Message, structuredErr.Details)
			if failErr != nil {
				log.Printf("Worker %s: failed to mark job %d as failed: %v", w.id, job.ID, failErr)
			}
		} else {
			// Use standard FailJob for unstructured errors
			failErr := w.jobService.FailJob(ctx, job.ID, err)
			if failErr != nil {
				log.Printf("Worker %s: failed to mark job %d as failed: %v", w.id, job.ID, failErr)
			}
		}
		return fmt.Errorf("job processing failed: %w", err)
	}

	log.Printf("Worker %s completed job %d", w.id, job.ID)
	return nil
}

// WorkerPool manages multiple workers
type WorkerPool struct {
	workers    []*Worker
	jobService jobs.Service
	mu         sync.RWMutex
	started    bool
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(jobService jobs.Service, workerCount int, pollInterval time.Duration) *WorkerPool {
	pool := &WorkerPool{
		jobService: jobService,
		workers:    make([]*Worker, workerCount),
	}

	// Create workers
	for i := 0; i < workerCount; i++ {
		workerID := fmt.Sprintf("worker-%d", i+1)
		pool.workers[i] = NewWorker(workerID, jobService, pollInterval)
	}

	return pool
}

// RegisterProcessor registers a processor with all workers
func (p *WorkerPool) RegisterProcessor(processor JobProcessor) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, worker := range p.workers {
		worker.RegisterProcessor(processor)
	}
}

// Start starts all workers
func (p *WorkerPool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("worker pool already started")
	}

	log.Printf("Starting worker pool with %d workers", len(p.workers))

	for _, worker := range p.workers {
		worker.Start(ctx)
	}

	p.started = true
	return nil
}

// Stop stops all workers gracefully
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return
	}

	log.Printf("Stopping worker pool")

	for _, worker := range p.workers {
		worker.Stop()
	}

	p.started = false
}
