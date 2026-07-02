package common

import "time"

// Result is returned by every adapter's Reconcile function.
type Result struct {
	// RequeueAfter > 0 means re-enqueue the item after this duration.
	// Zero means do not requeue.
	RequeueAfter time.Duration
}
