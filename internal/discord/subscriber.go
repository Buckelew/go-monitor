package discord

import (
	"context"
	"log"

	"github.com/buckelew/go-monitor/internal/database"
	"github.com/buckelew/go-monitor/internal/hub"
	"github.com/buckelew/go-monitor/internal/monitor"
)

// HubSubscriber bridges hub events to Discord notifications.
// It reads from the hub, looks up subscriptions, and dispatches
// notifications using the existing notifier.
type HubSubscriber struct {
	hub      *hub.Hub
	notifier *DiscordNotifier
	queries  *database.Queries
}

func NewHubSubscriber(h *hub.Hub, notifier *DiscordNotifier, queries *database.Queries) *HubSubscriber {
	return &HubSubscriber{hub: h, notifier: notifier, queries: queries}
}

// Run subscribes to the hub and dispatches notifications until ctx is cancelled.
func (s *HubSubscriber) Run(ctx context.Context) {
	ch := s.hub.Subscribe(64)
	defer s.hub.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			s.handleEvent(ctx, evt)
		}
	}
}

func (s *HubSubscriber) handleEvent(ctx context.Context, evt hub.Event) {
	itemEvent, ok := evt.Data.(monitor.ItemEvent)
	if !ok {
		return
	}

	subs, err := s.queries.GetSubscriptionsByTask(ctx, evt.TaskID)
	if err != nil {
		log.Printf("[discord-sub] failed to get subscriptions for task %d: %v", evt.TaskID, err)
		return
	}

	for _, sub := range subs {
		if monitor.ShouldNotify(sub, itemEvent) {
			if err := s.notifier.Notify(sub.ChannelID, itemEvent); err != nil {
				log.Printf("[discord-sub] failed to notify %s: %v", sub.ChannelID, err)
			}
		}
	}
}
