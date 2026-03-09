package monitor

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/buckelew/go-monitor/internal/database"
)

type Scheduler struct {
	tasks    []Task
	queries  database.Queries
	notifier Notifier
}

func NewScheduler(tasks []Task, queries *database.Queries, notifier Notifier) *Scheduler {
	return &Scheduler{tasks: tasks, queries: *queries, notifier: notifier}
}

func (s *Scheduler) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for _, task := range s.tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.runTask(ctx, task)
		}()
	}
	wg.Wait()
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
			if s.notifier != nil && task.ChannelID() != "" {
				if err := s.notifier.Notify(task.ChannelID(), event); err != nil {
					log.Printf("[task %d] failed to send notification: %v", task.ID(), err)
				}
			}
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
