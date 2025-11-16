package goverhaul

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/afero"
)

// Job represents a file to be linted
type Job struct {
	Path string
	Info os.FileInfo
}

// Result represents the violations found in a file
type Result struct {
	Violations []LintViolation
	Error      error
}

// ConcurrentLinter provides parallel linting capabilities
type ConcurrentLinter struct {
	*Goverhaul
	workerCount int
	bufferSize  int
	progress    ProgressReporter
	stats       *LintStats
}

// LintStats tracks performance metrics
type LintStats struct {
	filesProcessed atomic.Uint64
	totalFiles     atomic.Uint64
	startTime      time.Time
	endTime        time.Time
}

// ProgressReporter interface for progress updates
type ProgressReporter interface {
	StartFile(path string)
	CompleteFile(path string, violations int)
	UpdateProgress(current, total int)
	Complete(stats *LintStats)
}

// NoOpProgressReporter is a no-op implementation
type NoOpProgressReporter struct{}

func (n *NoOpProgressReporter) StartFile(path string)                    {}
func (n *NoOpProgressReporter) CompleteFile(path string, violations int) {}
func (n *NoOpProgressReporter) UpdateProgress(current, total int)         {}
func (n *NoOpProgressReporter) Complete(stats *LintStats)                 {}

// Option is a functional option for ConcurrentLinter
type Option func(*ConcurrentLinter) error

// WithWorkerCount sets the number of worker goroutines
func WithWorkerCount(count int) Option {
	return func(cl *ConcurrentLinter) error {
		if count < 1 {
			return fmt.Errorf("worker count must be at least 1, got %d", count)
		}
		cl.workerCount = count
		return nil
	}
}

// WithBufferSize sets the job buffer size
func WithBufferSize(size int) Option {
	return func(cl *ConcurrentLinter) error {
		if size < 1 {
			return fmt.Errorf("buffer size must be at least 1, got %d", size)
		}
		cl.bufferSize = size
		return nil
	}
}

// WithProgressReporter sets a progress reporter
func WithProgressReporter(reporter ProgressReporter) Option {
	return func(cl *ConcurrentLinter) error {
		cl.progress = reporter
		return nil
	}
}

// NewConcurrentLinter creates a new concurrent linter with options
func NewConcurrentLinter(cfg Config, logger *slog.Logger, fs afero.Fs, opts ...Option) (*ConcurrentLinter, error) {
	base, err := NewLinter(cfg, logger, fs)
	if err != nil {
		return nil, err
	}

	cl := &ConcurrentLinter{
		Goverhaul:   base,
		workerCount: runtime.NumCPU(),
		bufferSize:  100,
		progress:    &NoOpProgressReporter{},
		stats:       &LintStats{},
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(cl); err != nil {
			return nil, err
		}
	}

	return cl, nil
}

// LintWithContext performs concurrent linting with context support
func (cl *ConcurrentLinter) LintWithContext(ctx context.Context, path string) (*LintViolations, error) {
	cl.stats = &LintStats{startTime: time.Now()}

	// Collect all Go files first
	files, err := cl.collectFiles(ctx, path)
	if err != nil {
		return nil, err
	}

	cl.stats.totalFiles.Store(uint64(len(files)))
	cl.progress.UpdateProgress(0, len(files))

	// Process files concurrently
	violations, err := cl.processFilesConcurrently(ctx, files)
	if err != nil {
		return nil, err
	}

	cl.stats.endTime = time.Now()
	cl.progress.Complete(cl.stats)

	return violations, nil
}

// collectFiles walks the directory and collects all Go files
func (cl *ConcurrentLinter) collectFiles(ctx context.Context, path string) ([]Job, error) {
	var files []Job
	var mu sync.Mutex

	err := afero.Walk(cl.fs, path, func(filePath string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			cl.logger.Error("Failed walk", slog.String("file", filePath))
			return nil // Continue walking
		}

		if !isGoFileFs(info) {
			return nil
		}

		mu.Lock()
		files = append(files, Job{Path: filePath, Info: info})
		mu.Unlock()

		return nil
	})

	return files, err
}

// processFilesConcurrently processes files using a worker pool
func (cl *ConcurrentLinter) processFilesConcurrently(ctx context.Context, files []Job) (*LintViolations, error) {
	jobs := make(chan Job, cl.bufferSize)
	results := make(chan Result, cl.bufferSize)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < cl.workerCount; i++ {
		wg.Add(1)
		go cl.worker(ctx, &wg, jobs, results)
	}

	// Start result collector
	violations := NewLintViolations()
	var collectorDone = make(chan struct{})
	go cl.collectResults(violations, results, collectorDone)

	// Send jobs
	go func() {
		for _, file := range files {
			select {
			case <-ctx.Done():
				break
			case jobs <- file:
			}
		}
		close(jobs)
	}()

	// Wait for workers to complete
	wg.Wait()
	close(results)

	// Wait for collector to finish
	<-collectorDone

	// Check if context was cancelled
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return violations, nil
}

// worker processes jobs from the job channel
func (cl *ConcurrentLinter) worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan Job, results chan<- Result) {
	defer wg.Done()

	for job := range jobs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			results <- Result{Error: ctx.Err()}
			return
		default:
		}

		cl.progress.StartFile(job.Path)
		violations, err := cl.lintFile(job.Path)

		if err != nil {
			cl.logger.Error("Failed to lint file",
				slog.String("file", job.Path),
				slog.String("error", err.Error()))
			results <- Result{Error: err}
			continue
		}

		cl.progress.CompleteFile(job.Path, len(violations))
		cl.stats.filesProcessed.Add(1)

		current := cl.stats.filesProcessed.Load()
		total := cl.stats.totalFiles.Load()
		cl.progress.UpdateProgress(int(current), int(total))

		results <- Result{Violations: violations}
	}
}

// collectResults collects results from workers
func (cl *ConcurrentLinter) collectResults(violations *LintViolations, results <-chan Result, done chan<- struct{}) {
	var mu sync.Mutex

	for result := range results {
		if result.Error != nil {
			continue // Error already logged in worker
		}

		mu.Lock()
		for _, v := range result.Violations {
			violations.Add(v)
		}
		mu.Unlock()
	}

	close(done)
}

// lintFile processes a single file and returns violations
func (cl *ConcurrentLinter) lintFile(filePath string) ([]LintViolation, error) {
	imports, err := cl.getImports(filePath)
	if err != nil {
		return nil, err
	}

	var violations []LintViolation
	for _, rule := range cl.cfg.Rules {
		fileViolations := cl.checkImports(filePath, imports, rule, cl.cfg.Modfile)
		violations = append(violations, fileViolations...)
	}

	return violations, nil
}

// Duration returns the time taken for the last lint operation
func (s *LintStats) Duration() time.Duration {
	if s.endTime.IsZero() {
		return time.Since(s.startTime)
	}
	return s.endTime.Sub(s.startTime)
}

// FilesPerSecond returns the processing rate
func (s *LintStats) FilesPerSecond() float64 {
	duration := s.Duration().Seconds()
	if duration == 0 {
		return 0
	}
	return float64(s.filesProcessed.Load()) / duration
}

// BenchmarkComparison compares performance with sequential processing
func BenchmarkComparison(cfg Config, logger *slog.Logger, fs afero.Fs, path string) error {
	fmt.Println("Running performance comparison...")
	fmt.Println("==================================================")

	// Sequential processing
	sequential, err := NewLinter(cfg, logger, fs)
	if err != nil {
		return err
	}

	start := time.Now()
	seqViolations, err := sequential.Lint(path)
	if err != nil {
		return err
	}
	seqDuration := time.Since(start)

	// Concurrent processing
	concurrent, err := NewConcurrentLinter(cfg, logger, fs)
	if err != nil {
		return err
	}

	start = time.Now()
	concViolations, err := concurrent.LintWithContext(context.Background(), path)
	if err != nil {
		return err
	}
	concDuration := time.Since(start)

	// Display results
	fmt.Printf("Sequential Processing:\n")
	fmt.Printf("  Duration: %v\n", seqDuration)
	fmt.Printf("  Violations: %d\n", len(seqViolations.Violations))
	fmt.Println()

	fmt.Printf("Concurrent Processing (%d workers):\n", concurrent.workerCount)
	fmt.Printf("  Duration: %v\n", concDuration)
	fmt.Printf("  Violations: %d\n", len(concViolations.Violations))
	fmt.Printf("  Files/sec: %.2f\n", concurrent.stats.FilesPerSecond())
	fmt.Println()

	speedup := float64(seqDuration) / float64(concDuration)
	fmt.Printf("Speedup: %.2fx faster\n", speedup)

	if speedup > 1.5 {
		fmt.Println("✅ Concurrent processing is significantly faster!")
	}

	return nil
}