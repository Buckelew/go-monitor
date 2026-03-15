package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/buckelew/go-monitor/internal/monitor"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsEvent struct {
	TaskID    int32             `json:"task_id"`
	EventType monitor.EventType `json:"event_type"`
	Item      monitor.Item      `json:"item"`
	ItemID    int32             `json:"item_id"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ch := s.hub.Subscribe(64)
	defer s.hub.Unsubscribe(ch)

	// Read loop: handle pong, detect close
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-done:
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			itemEvent, ok := evt.Data.(monitor.ItemEvent)
			if !ok {
				continue
			}
			msg := wsEvent{
				TaskID:    evt.TaskID,
				EventType: itemEvent.Type,
				Item:      itemEvent.Item,
				ItemID:    itemEvent.ItemID,
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
