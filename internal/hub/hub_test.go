package hub

import (
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	h := New()
	ch := h.Subscribe(8)

	evt := Event{TaskID: 1, Data: "test"}
	h.Publish(evt)

	select {
	case got := <-ch:
		if got.TaskID != 1 {
			t.Fatalf("unexpected task ID: %d", got.TaskID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	h := New()
	ch1 := h.Subscribe(8)
	ch2 := h.Subscribe(8)

	evt := Event{TaskID: 2}
	h.Publish(evt)

	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.TaskID != 2 {
				t.Fatalf("unexpected task ID: %d", got.TaskID)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
}

func TestUnsubscribe(t *testing.T) {
	h := New()
	ch := h.Subscribe(8)
	h.Unsubscribe(ch)

	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}

	// Publishing should not panic with no subscribers
	h.Publish(Event{TaskID: 3})
}

func TestNonBlockingPublish(t *testing.T) {
	h := New()
	ch := h.Subscribe(1)

	h.Publish(Event{TaskID: 1})
	h.Publish(Event{TaskID: 2})

	got := <-ch
	if got.TaskID != 1 {
		t.Fatalf("expected task ID 1, got %d", got.TaskID)
	}

	select {
	case <-ch:
		t.Fatal("expected channel to be empty after buffer overflow")
	default:
	}
}
