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

type Scheduler struct {
	mu      sync.Mutex
	tasks   map[int32]context.CancelFunc
	queries database.Queries
	hub     *hub.Hub
	wg      sync.WaitGroup
}

func NewScheduler(queries *database.Queries, hub *hub.Hub) *Scheduler {
	return &Scheduler{
		tasks:   make(map[int32]context.CancelFunc),
		queries: *queries,
		hub:     hub,
	}
}

// Start launches goroutines for each initial task and blocks until ctx is
// cancelled, then waits for all goroutines to finish.
func (s *Scheduler) Start(ctx context.Context, initialTasks []Task) {
	for _, task := range initialTasks {
		s.addTask(ctx, task)
	}
	<-ctx.Done()
	s.wg.Wait()
}

// AddTask adds a new task to the running scheduler. Safe for concurrent use.
func (s *Scheduler) AddTask(ctx context.Context, task Task) {
	s.addTask(ctx, task)
}

func (s *Scheduler) addTask(ctx context.Context, task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID()]; exists {
		return
	}

	taskCtx, cancel := context.WithCancel(ctx)
	s.tasks[task.ID()] = cancel

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runTask(taskCtx, task)
	}()
}

func (s *Scheduler) runTask(ctx context.Context, task Task) {
	ticker := time.NewTicker(time.Duration(task.Delay()) * time.Millisecond)
	defer ticker.Stop()

	// Immediately start task
	s.executeTask(ctx, task)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executeTask(ctx, task)
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task Task) {
	taskRun, err := s.queries.InsertTaskRun(ctx, task.ID())
	if err != nil {
		log.Printf("[task %d] failed to insert task run: %v", task.ID(), err)
		return
	}

	isFirst, err := s.isFirstRun(ctx, task.ID())
	if err != nil {
		log.Printf("[task %d] failed to check first run: %v", task.ID(), err)
		s.completeTaskRun(ctx, taskRun.ID, "error", err.Error())
		return
	}

	result, err := task.Run(ctx)
	if err != nil {
		log.Printf("[task %d] run failed: %v", task.ID(), err)
		s.completeTaskRun(ctx, taskRun.ID, "error", err.Error())
		return
	}

	s.completeTaskRun(ctx, taskRun.ID, "completed", "")

	if !isFirst && len(result.Events) > 0 {
		for _, event := range result.Events {
			log.Printf("[task %d] %s: %s", task.ID(), event.Type, event.Item.Title)
			s.hub.Publish(hub.Event{TaskID: task.ID(), Data: event})
		}
	}
}

func (s *Scheduler) completeTaskRun(ctx context.Context, runID int32, status string, errMsg string) {
	params := database.CompleteTaskRunParams{
		ID:     runID,
		Status: status,
	}
	if errMsg != "" {
		params.ErrorMessage = sql.NullString{String: errMsg, Valid: true}
	}
	if err := s.queries.CompleteTaskRun(ctx, params); err != nil {
		log.Printf("[run %d] failed to complete task run: %v", runID, err)
	}
}

// Checks task_runs to determine first run
func (s *Scheduler) isFirstRun(ctx context.Context, taskID int32) (bool, error) {
	taskRuns, err := s.queries.GetCompletedTaskRuns(ctx, taskID)
	if err != nil {
		return false, err
	}
	if len(taskRuns) == 0 {
		return true, nil
	}
	return false, nil
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
