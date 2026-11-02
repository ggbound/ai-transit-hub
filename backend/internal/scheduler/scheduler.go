package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/your-org/ai-transit-hub/internal/logger"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	jobs      []Job
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup // 跟踪 runJob 循环 goroutine
	jobWg     sync.WaitGroup // 跟踪在途的 job.Run goroutine，保证 Stop 时任务不被中途切断
	isRunning bool
	mu        sync.Mutex
}

// Job 定时任务接口
type Job interface {
	Run(ctx context.Context) error
	Name() string
	Interval() time.Duration
	Timeout() time.Duration
}

// New 创建新的调度器
func New() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		jobs:   make([]Job, 0),
		ctx:    ctx,
		cancel: cancel,
	}
}

// RegisterJob 注册定时任务
func (s *Scheduler) RegisterJob(job Job) {
	s.jobs = append(s.jobs, job)
	logger.Info("Registered job: %s (interval: %v, timeout: %v)", job.Name(), job.Interval(), job.Timeout())
}

// Start 启动所有定时任务
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		logger.Warn("Scheduler already started")
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	logger.Info("Starting scheduler with %d jobs", len(s.jobs))

	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.runJob(job)
	}
}

// Stop 优雅停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		logger.Warn("Scheduler not running")
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	logger.Info("Stopping scheduler...")
	s.cancel()
	s.wg.Wait()

	// 等待在途任务完成（最长 30s，避免卡死停机流程）
	stopped := make(chan struct{})
	go func() {
		s.jobWg.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(30 * time.Second):
		logger.Warn("Scheduler stop timed out, some tasks may still be running")
	}
	logger.Info("Scheduler stopped")
}

// runJob 运行单个定时任务
func (s *Scheduler) runJob(job Job) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Job %s panicked: %v", job.Name(), r)
		}
	}()

	logger.Info("Starting job: %s", job.Name())

	// 先运行一次
	s.runJobWithTimeout(job)

	// 定时运行
	ticker := time.NewTicker(job.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("Job %s received stop signal", job.Name())
			return
		case <-ticker.C:
			logger.Debug("Running job: %s", job.Name())
			s.runJobWithTimeout(job)
		}
	}
}

// runJobWithTimeout 带超时运行任务
func (s *Scheduler) runJobWithTimeout(job Job) {
	ctx, cancel := context.WithTimeout(s.ctx, job.Timeout())
	defer cancel()

	done := make(chan struct{})
	s.jobWg.Add(1)
	go func() {
		defer s.jobWg.Done()
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Job %s panicked: %v", job.Name(), r)
			}
		}()
		if err := job.Run(ctx); err != nil {
			logger.Error("Job %s failed: %v", job.Name(), err)
		}
	}()

	select {
	case <-done:
		// 任务在超时前正常完成
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			logger.Error("Job %s timed out after %v", job.Name(), job.Timeout())
			// 等待 job goroutine 退出：Interval < Timeout 时，
			// 防止下一次 ticker 触发在上一轮仍在运行的同名 job（重叠执行）
			<-done
		}
		// 父 ctx 取消（Stop）：直接返回，在途任务由 Stop 的 jobWg 兜底
	}
}
