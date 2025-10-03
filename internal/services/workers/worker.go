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

type JobProcessor interface {
	ProcessJob(ctx context.Context, job *models.Job) error
	CanProcess(jobType models.JobType) bool
}

type Worker struct {
	id           string
	jobService   jobs.Service
	processors   []JobProcessor
	stopChan     chan struct{}
	wg           sync.WaitGroup
	pollInterval time.Duration
}

func NewWorker(id string, jobService jobs.Service, pollInterval time.Duration) *Worker {
	return &Worker{
		id:           id,
		jobService:   jobService,
		processors:   make([]JobProcessor, 0),
		stopChan:     make(chan struct{}),
		pollInterval: pollInterval,
	}
}

func (w *Worker) RegisterProcessor(processor JobProcessor) {
	w.processors = append(w.processors, processor)
}

func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}

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

func (w *Worker) processNextJob(ctx context.Context) error {
	var supportedTypes []models.JobType
	typeMap := make(map[models.JobType]bool)

	allJobTypes := []models.JobType{
		models.JobTypeWaveformGeneration,
		models.JobTypeTranscriptionGeneration,
		models.JobTypePodcastSync,
		models.JobTypeClipExtraction,
		models.JobTypeAutoLabel,
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

	job, err := w.jobService.ClaimNextJob(ctx, w.id, supportedTypes)
	if err != nil {
		// No jobs available is not an error
		return nil
	}
	if job == nil {
		return nil
	}

	log.Printf("Worker %s claimed job %d (type: %s)", w.id, job.ID, job.Type)

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

	err = processor.ProcessJob(ctx, job)
	if err != nil {
		if structuredErr, ok := err.(*models.StructuredJobError); ok {
			failErr := w.jobService.FailJobWithDetails(ctx, job.ID, structuredErr.Type, structuredErr.Code, structuredErr.Message, structuredErr.Details)
			if failErr != nil {
				log.Printf("Worker %s: failed to mark job %d as failed: %v", w.id, job.ID, failErr)
			}
		} else {
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

type WorkerPool struct {
	workers    []*Worker
	jobService jobs.Service
	mu         sync.RWMutex
	started    bool
}

func NewWorkerPool(jobService jobs.Service, workerCount int, pollInterval time.Duration) *WorkerPool {
	pool := &WorkerPool{
		jobService: jobService,
		workers:    make([]*Worker, workerCount),
	}

	for i := 0; i < workerCount; i++ {
		workerID := fmt.Sprintf("worker-%d", i+1)
		pool.workers[i] = NewWorker(workerID, jobService, pollInterval)
	}

	return pool
}

func (p *WorkerPool) RegisterProcessor(processor JobProcessor) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, worker := range p.workers {
		worker.RegisterProcessor(processor)
	}
}

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
