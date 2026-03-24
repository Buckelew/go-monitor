package monitor

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/hub"
)

const (
	minTickInterval  = 100 * time.Millisecond // floor regardless of proxy count
	defaultInterval  = 3 * time.Second        // when platform has 0 proxies
	minCycleTime     = 1 * time.Second        // no task runs more often than this
	proxyRefreshFreq = 30 * time.Second       // how often to re-query proxy count
)

type Scheduler struct {
	mu      sync.Mutex
	runners map[Platform]*platformRunner
	queries database.Queries
	hub     *hub.Hub
	wg      sync.WaitGroup
	addCh   chan Task
}

func NewScheduler(queries *database.Queries, hub *hub.Hub) *Scheduler {
	return &Scheduler{
		runners: make(map[Platform]*platformRunner),
		queries: *queries,
		hub:     hub,
		addCh:   make(chan Task, 64),
	}
}

// Start launches platform runners for initial tasks and blocks until ctx is
// cancelled. New tasks arriving via AddTask are routed to the correct runner.
func (s *Scheduler) Start(ctx context.Context, initialTasks []Task) {
	byPlatform := make(map[Platform][]Task)
	for _, t := range initialTasks {
		byPlatform[t.Platform()] = append(byPlatform[t.Platform()], t)
	}

	s.mu.Lock()
	for p, tasks := range byPlatform {
		r := s.newRunner(p, tasks)
		s.runners[p] = r
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			r.run(ctx)
		}()
	}
	s.mu.Unlock()

	// Retention: delete task_runs older than 3 days, every hour.
	go s.runRetention(ctx)

	for {
		select {
		case <-ctx.Done():
			s.wg.Wait()
			return
		case task := <-s.addCh:
			s.routeTask(ctx, task)
		}
	}
}

// AddTask adds a task to the running scheduler. Safe for concurrent use.
func (s *Scheduler) AddTask(ctx context.Context, task Task) {
	select {
	case s.addCh <- task:
	case <-ctx.Done():
	}
}

func (s *Scheduler) routeTask(ctx context.Context, task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := task.Platform()
	if r, ok := s.runners[p]; ok {
		r.addTask(task)
		return
	}

	r := s.newRunner(p, []Task{task})
	s.runners[p] = r
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		r.run(ctx)
	}()
}

func (s *Scheduler) newRunner(p Platform, tasks []Task) *platformRunner {
	return &platformRunner{
		platform: p,
		tasks:    tasks,
		lastRun:  make(map[int32]time.Time),
		notFirst: make(map[int32]bool),
		sem:      make(chan struct{}, 1), // sized once in run() after first proxy count
		queries:  s.queries,
		hub:      s.hub,
		wg:       &s.wg,
	}
}

// platformRunner drives round-robin execution for a single platform.
type platformRunner struct {
	platform   Platform
	mu         sync.RWMutex
	tasks      []Task
	cursor     int
	lastRun    map[int32]time.Time
	notFirst   map[int32]bool // cached: true once a task has completed at least one run
	proxyCount int64
	ticker     *time.Ticker
	sem        chan struct{}
	queries    database.Queries
	hub        *hub.Hub
	wg         *sync.WaitGroup
}

func (r *platformRunner) addTask(task Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tasks {
		if t.ID() == task.ID() {
			return
		}
	}
	r.tasks = append(r.tasks, task)
}

func (r *platformRunner) tickInterval() time.Duration {
	r.mu.RLock()
	pc := r.proxyCount
	r.mu.RUnlock()

	if pc <= 0 {
		return defaultInterval
	}
	d := time.Duration(float64(time.Second) / float64(pc))
	if d < minTickInterval {
		return minTickInterval
	}
	return d
}

func (r *platformRunner) refreshProxyCount(ctx context.Context) {
	count, err := r.queries.CountProxiesByPlatform(ctx, string(r.platform))
	if err != nil {
		log.Printf("[%s] failed to refresh proxy count: %v", r.platform, err)
		return
	}
	r.mu.Lock()
	r.proxyCount = count
	r.mu.Unlock()
}

func (r *platformRunner) run(ctx context.Context) {
	r.refreshProxyCount(ctx)

	// Size the semaphore once. Cap at the number of tasks to avoid running
	// more goroutines than tasks (each holds parsed product data in memory).
	pc := r.proxyCount
	if pc < 1 {
		pc = 1
	}
	maxConcurrent := int64(len(r.tasks))
	if maxConcurrent > 10 {
		maxConcurrent = 10
	}
	if pc > maxConcurrent {
		pc = maxConcurrent
	}
	r.sem = make(chan struct{}, pc)

	interval := r.tickInterval()
	r.ticker = time.NewTicker(interval)
	defer r.ticker.Stop()

	proxyRefresh := time.NewTicker(proxyRefreshFreq)
	defer proxyRefresh.Stop()

	log.Printf("[%s] runner started: %d tasks, %d proxies, tick=%v",
		r.platform, len(r.tasks), r.proxyCount, interval)

	// Immediate first execution
	r.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.ticker.C:
			r.tick(ctx)
		case <-proxyRefresh.C:
			old := r.tickInterval()
			r.refreshProxyCount(ctx)
			if nw := r.tickInterval(); nw != old {
				r.ticker.Reset(nw)
				log.Printf("[%s] tick interval: %v -> %v (%d proxies)",
					r.platform, old, nw, r.proxyCount)
			}
		}
	}
}

// tick finds the next eligible task and fires it in a goroutine.
func (r *platformRunner) tick(ctx context.Context) {
	task := r.nextEligible()
	if task == nil {
		return
	}

	select {
	case r.sem <- struct{}{}:
	default:
		return // all slots busy
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer func() { <-r.sem }()
		r.executeTask(ctx, task)
	}()
}

// nextEligible advances the cursor to find a task that hasn't run within
// minCycleTime. Returns nil if all tasks ran recently.
func (r *platformRunner) nextEligible() Task {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(r.tasks)
	for attempts := 0; attempts < n; attempts++ {
		idx := r.cursor % n
		task := r.tasks[idx]
		r.cursor = (idx + 1) % n

		if time.Since(r.lastRun[task.ID()]) >= minCycleTime {
			r.lastRun[task.ID()] = time.Now()
			return task
		}
	}
	return nil
}

func (r *platformRunner) executeTask(ctx context.Context, task Task) {
	isFirst, err := r.isFirstRun(ctx, task.ID())
	if err != nil {
		log.Printf("[task %d] failed to check first run: %v", task.ID(), err)
		return
	}

	startedAt := time.Now()
	result, err := task.Run(ctx)
	if err != nil {
		log.Printf("[task %d] run failed: %v", task.ID(), err)
		r.insertTaskRun(ctx, task.ID(), startedAt, "error", err.Error(), nil)
		return
	}

	r.insertTaskRun(ctx, task.ID(), startedAt, "completed", "", result.Meta)

	if !isFirst && len(result.Events) > 0 {
		for _, event := range result.Events {
			log.Printf("[task %d] %s: %s", task.ID(), event.Type, event.Item.Title)
			r.hub.Publish(hub.Event{TaskID: task.ID(), Data: event})
		}
	}
}

func (r *platformRunner) insertTaskRun(ctx context.Context, taskID int32, startedAt time.Time, status string, errMsg string, meta *FetchMeta) {
	params := database.InsertCompletedTaskRunParams{
		TaskID:    taskID,
		StartedAt: startedAt,
		Status:    status,
	}
	if errMsg != "" {
		params.ErrorMessage = sql.NullString{String: errMsg, Valid: true}
	}
	if meta != nil {
		params.StatusCode = sql.NullInt16{Int16: int16(meta.StatusCode), Valid: meta.StatusCode != 0}
		params.ResponseTimeMs = sql.NullInt32{Int32: int32(meta.Duration.Milliseconds()), Valid: true}
		if meta.CacheStatus != "" {
			params.CacheStatus = sql.NullString{String: meta.CacheStatus, Valid: true}
		}
	}
	if err := r.queries.InsertCompletedTaskRun(ctx, params); err != nil {
		log.Printf("[task %d] failed to insert task run: %v", taskID, err)
	}
}

func (r *platformRunner) isFirstRun(ctx context.Context, taskID int32) (bool, error) {
	if r.notFirst[taskID] {
		return false, nil
	}
	hasItems, err := r.queries.HasItemsByTask(ctx, taskID)
	if err != nil {
		return false, err
	}
	if hasItems {
		r.notFirst[taskID] = true
		return false, nil
	}
	return true, nil
}

const (
	retentionInterval = 1 * time.Hour
	retentionAge      = 3 * 24 * time.Hour // keep 3 days of task_runs
	retentionBatch    = 1000
)

// runRetention periodically deletes task_runs older than retentionAge.
func (s *Scheduler) runRetention(ctx context.Context) {
	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()

	// Run once at startup to clean up existing backlog.
	s.deleteOldTaskRuns(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.deleteOldTaskRuns(ctx)
		}
	}
}

func (s *Scheduler) deleteOldTaskRuns(ctx context.Context) {
	cutoff := time.Now().Add(-retentionAge)
	var total int64
	for {
		deleted, err := s.queries.DeleteTaskRunsBefore(ctx, database.DeleteTaskRunsBeforeParams{
			CompletedAt: sql.NullTime{Time: cutoff, Valid: true},
			Limit:       retentionBatch,
		})
		if err != nil {
			log.Printf("[retention] failed to delete old task runs: %v", err)
			return
		}
		total += deleted
		if deleted < retentionBatch {
			break
		}
	}
	if total > 0 {
		log.Printf("[retention] deleted %d task runs older than %v", total, retentionAge)
	}
}

// ShouldNotify determines whether a subscription should receive a given event.
//
// Event routing matrix:
//
//	Mode      | new_product | restock | delisted
//	----------+-------------+---------+---------
//	"all"     |     yes     |   yes   |   yes      (store-level, all events)
//	"new"     |     yes     |   no    |   no       (store-level, new listings only)
//	"restock" |     no      |   yes   |   yes      (product-level, item_id must match)
func ShouldNotify(sub database.TaskSubscription, event ItemEvent) bool {
	switch sub.Mode {
	case "all":
		return true
	case "new":
		return event.Type == EventNewProduct
	case "restock":
		return (event.Type == EventRestock || event.Type == EventDelisted) &&
			sub.ItemID.Valid && event.ItemID == sub.ItemID.Int32
	default:
		return false
	}
}
