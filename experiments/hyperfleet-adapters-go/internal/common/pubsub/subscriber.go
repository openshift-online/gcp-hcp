package pubsub

import (
	"context"
	"encoding/json"
	"sync"

	gogopubsub "cloud.google.com/go/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"
)

const maxNackCount = 3

// Subscriber pulls messages from a Pub/Sub subscription, parses them as
// CloudEvents, and enqueues the resource ID onto a workqueue.
type Subscriber struct {
	sub      *gogopubsub.Subscriber
	queue    workqueue.RateLimitingInterface
	log      *zap.SugaredLogger
	nacksMu  sync.Mutex
	nackCount map[string]int
}

// New creates a new Subscriber.
func New(sub *gogopubsub.Subscriber, queue workqueue.RateLimitingInterface, log *zap.SugaredLogger) *Subscriber {
	return &Subscriber{
		sub:       sub,
		queue:     queue,
		log:       log,
		nackCount: make(map[string]int),
	}
}

// Run pulls messages in a loop until ctx is cancelled.
// For each message it parses a CloudEvent, extracts the event ID as the
// resource ID, and enqueues it. On parse failure it nacks up to maxNackCount
// times, then acks and logs the message as a poison pill.
func (s *Subscriber) Run(ctx context.Context) {
	err := s.sub.Receive(ctx, func(ctx context.Context, msg *gogopubsub.Message) {
		var ce cloudevents.Event
		if err := json.Unmarshal(msg.Data, &ce); err != nil {
			msgID := msg.ID
			s.nacksMu.Lock()
			count := s.nackCount[msgID] + 1
			s.nackCount[msgID] = count
			s.nacksMu.Unlock()

			if count < maxNackCount {
				s.log.Warnw("failed to parse CloudEvent, nacking message",
					"msgID", msgID,
					"attempt", count,
					"error", err,
				)
				msg.Nack()
				return
			}

			// Poison pill: ack and discard.
			s.log.Errorw("discarding unparseable message as poison pill",
				"msgID", msgID,
				"attempts", count,
				"error", err,
			)
			s.nacksMu.Lock()
			delete(s.nackCount, msgID)
			s.nacksMu.Unlock()
			msg.Ack()
			return
		}

		var data struct {
			ID string `json:"id"`
		}
		if err := ce.DataAs(&data); err != nil || data.ID == "" {
			s.log.Errorw("CloudEvent missing resource id in data, discarding",
				"msgID", msg.ID,
				"eventID", ce.ID(),
				"error", err,
			)
			msg.Ack()
			return
		}
		resourceID := data.ID
		s.log.Debugw("received CloudEvent, enqueuing resource",
			"resourceID", resourceID,
			"msgID", msg.ID,
		)
		s.queue.Add(resourceID)
		msg.Ack()
	})

	if err != nil && ctx.Err() == nil {
		s.log.Errorw("pubsub Receive returned unexpected error", "error", err)
	}
}
