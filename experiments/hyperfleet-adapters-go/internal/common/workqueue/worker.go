package workqueue

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"

	"github.com/openshift-hyperfleet/hyperfleet-adapters-go/internal/common"
)

// Run starts n worker goroutines consuming from queue.
// Each worker dequeues items, calls reconcile, and handles requeue logic:
//   - On error: requeues via AddRateLimited.
//   - On Result.RequeueAfter > 0: requeues via AddAfter.
//   - Otherwise: item is not requeued.
//
// Run blocks until ctx is cancelled and all workers have exited.
func Run(
	ctx context.Context,
	n int,
	queue workqueue.RateLimitingInterface,
	reconcile func(ctx context.Context, id string) (common.Result, error),
	log *zap.SugaredLogger,
) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWorker(ctx, queue, reconcile, log)
		}()
	}

	<-ctx.Done()
	queue.ShutDown()
	wg.Wait()
}

func runWorker(
	ctx context.Context,
	queue workqueue.RateLimitingInterface,
	reconcile func(ctx context.Context, id string) (common.Result, error),
	log *zap.SugaredLogger,
) {
	for {
		item, shutdown := queue.Get()
		if shutdown {
			return
		}

		id, ok := item.(string)
		if !ok {
			log.Warnw("workqueue item is not a string, skipping", "item", item)
			queue.Done(item)
			continue
		}

		result, err := reconcile(ctx, id)
		if err != nil {
			log.Warnw("reconcile failed, requeuing with rate limit",
				"resourceID", id,
				"error", err,
			)
			queue.AddRateLimited(id)
			queue.Done(item)
			continue
		}

		if result.RequeueAfter > 0 {
			log.Debugw("reconcile requested requeue",
				"resourceID", id,
				"requeueAfter", result.RequeueAfter,
			)
			queue.AddAfter(id, result.RequeueAfter)
		}

		queue.Done(item)
	}
}
