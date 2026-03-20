package monitor_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/httpclient"
	"github.com/buckelew/go-monitor/internal/monitor"
	"github.com/buckelew/go-monitor/internal/monitor/platforms/shopify"
	"github.com/stretchr/testify/require"
)

// TestMemoryBaseline_GetCompletedTaskRuns directly measures the allocation cost
// of GetCompletedTaskRuns — the query called by isFirstRun every task cycle.
//
// With the bug: SELECT * FROM task_runs WHERE status='completed' AND task_id=$1
// loads ALL rows into Go structs. Each TaskRun is ~200 bytes, so 10K rows = ~2MB
// allocated per cycle, scaling linearly with uptime.
func TestMemoryBaseline_GetCompletedTaskRuns(t *testing.T) {
	if testDB == nil {
		t.Skip("POSTGRES_TEST_URL not set")
	}

	for _, count := range []int{100, 1_000, 10_000, 50_000} {
		t.Run(fmt.Sprintf("rows=%d", count), func(t *testing.T) {
			truncateTables(t)
			ctx := context.Background()
			queries := database.New(testDB)

			task := createTestTask(t, "shopify", "http://example.com")
			seedTaskRuns(t, task.ID, count)

			// Warm up
			queries.GetCompletedTaskRuns(ctx, task.ID)

			// Measure
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			const iterations = 20
			for range iterations {
				rows, err := queries.GetCompletedTaskRuns(ctx, task.ID)
				require.NoError(t, err)
				_ = len(rows)
			}

			runtime.GC()
			var after runtime.MemStats
			runtime.ReadMemStats(&after)

			totalAlloc := after.TotalAlloc - before.TotalAlloc
			perIter := totalAlloc / iterations

			t.Logf("rows=%d: %d KB/iter (%d KB total over %d calls)",
				count, perIter/1024, totalAlloc/1024, iterations)
		})
	}
}

// TestMemoryFixed_HasCompletedTaskRun measures the replacement query: EXISTS
// with LIMIT 1. Should be O(1) regardless of row count.
func TestMemoryFixed_HasCompletedTaskRun(t *testing.T) {
	if testDB == nil {
		t.Skip("POSTGRES_TEST_URL not set")
	}

	for _, count := range []int{100, 1_000, 10_000, 50_000} {
		t.Run(fmt.Sprintf("rows=%d", count), func(t *testing.T) {
			truncateTables(t)
			ctx := context.Background()
			queries := database.New(testDB)

			task := createTestTask(t, "shopify", "http://example.com")
			seedTaskRuns(t, task.ID, count)

			// Warm up
			queries.HasCompletedTaskRun(ctx, task.ID)

			// Measure
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			const iterations = 20
			for range iterations {
				exists, err := queries.HasCompletedTaskRun(ctx, task.ID)
				require.NoError(t, err)
				_ = exists
			}

			runtime.GC()
			var after runtime.MemStats
			runtime.ReadMemStats(&after)

			totalAlloc := after.TotalAlloc - before.TotalAlloc
			perIter := totalAlloc / iterations

			t.Logf("rows=%d: %d KB/iter (%d KB total over %d calls)",
				count, perIter/1024, totalAlloc/1024, iterations)
		})
	}
}

// TestMemoryBaseline_FullCycle measures end-to-end memory for the scheduler's
// executeTask path, which includes isFirstRun + task.Run + DB writes.
func TestMemoryBaseline_FullCycle(t *testing.T) {
	if testDB == nil {
		t.Skip("POSTGRES_TEST_URL not set")
	}

	for _, count := range []int{100, 1_000, 10_000} {
		t.Run(fmt.Sprintf("task_runs=%d", count), func(t *testing.T) {
			truncateTables(t)
			queries := database.New(testDB)

			store := &fakeStore{}
			server := httptest.NewServer(store)
			defer server.Close()

			task := createTestTask(t, "shopify", server.URL)

			client, err := httpclient.New(nil)
			require.NoError(t, err)

			scraper := shopify.NewSearchShopify(server.URL, client)
			searchTask := monitor.NewSearchTask(*queries, scraper, task, nil)

			products := []shopifyFixture{
				{Handle: "mem-a", Title: "Mem A", Available: true},
			}
			store.SetResponse(makeShopifyResponse(products...))

			// First run via scheduler (seeds DB items + first task_run)
			notifier := &mockNotifier{}
			runSchedulerOnce(t, searchTask, queries, notifier)

			// Seed historical runs to simulate long uptime
			seedTaskRuns(t, task.ID, count)

			// Warm up
			runSchedulerOnce(t, searchTask, queries, notifier)

			// Measure: run 5 scheduler cycles, each calls executeTask which
			// calls isFirstRun → GetCompletedTaskRuns
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			const iterations = 5
			for range iterations {
				runSchedulerOnce(t, searchTask, queries, notifier)
			}

			runtime.GC()
			var after runtime.MemStats
			runtime.ReadMemStats(&after)

			totalAlloc := after.TotalAlloc - before.TotalAlloc
			perIter := totalAlloc / iterations

			t.Logf("task_runs=%d: %d KB/iter (%d KB total over %d scheduler cycles)",
				count, perIter/1024, totalAlloc/1024, iterations)
		})
	}
}

// --- helpers ---

func seedTaskRuns(tb testing.TB, taskID int32, count int) {
	tb.Helper()
	for i := 0; i < count; i += 1000 {
		batch := min(count-i, 1000)
		query := `INSERT INTO task_runs (task_id, started_at, status, completed_at)
			SELECT $1, NOW() - (n || ' seconds')::interval, 'completed', NOW() - (n || ' seconds')::interval
			FROM generate_series(1, $2) AS n`
		_, err := testDB.Exec(query, taskID, batch)
		if err != nil {
			tb.Fatalf("seed task_runs: %v", err)
		}
	}
}
