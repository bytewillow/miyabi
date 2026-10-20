package tasks

import (
	"fmt"
	"testing"
	"time"

	"github.com/ppxb/miyabi/internal/database"
	"github.com/ppxb/miyabi/internal/domain"
	"github.com/ppxb/miyabi/internal/ent/task"
)

// A rate-limited attempt is the account asking for quiet, not the task
// failing: the task must requeue on the limit's backoff with its retry
// budget untouched, however many times the episode repeats.
func TestRateLimitedTasksRequeueWithoutConsumingRetryBudget(t *testing.T) {
	store, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	registry := NewRegistry()
	registry.Register(NewHandler(KindScrape, nil, nil).WithRetry(domain.RetryDelay).WithFreeRetry(domain.IsRateLimited))
	svc := NewService(store.Client, registry)

	limited := fmt.Errorf("ensure 115 movie directory: %w",
		domain.E(domain.KindRateLimited, "115 error 770004: 已达到当前访问上限，请稍后再试", nil))
	job := store.Client.Task.Create().SetType(string(KindScrape)).SetStatus(task.StatusRunning).SaveX(t.Context())
	for range 5 {
		if err := svc.Queue().Finish(t.Context(), job.ID, limited); err != nil {
			t.Fatal(err)
		}
		got := store.Client.Task.GetX(t.Context(), job.ID)
		if got.Status != task.StatusQueued || got.RetryCount != 0 {
			t.Fatalf("rate-limited task consumed its budget: %+v", got)
		}
		if got.RetryAt == nil || time.Until(*got.RetryAt) < time.Minute {
			t.Fatalf("rate-limited task retry_at = %v, want at least a minute out", got.RetryAt)
		}
		// The pool would claim it again once due; simulate the next attempt.
		store.Client.Task.UpdateOneID(job.ID).SetStatus(task.StatusRunning).ClearRetryAt().ExecX(t.Context())
	}
}
