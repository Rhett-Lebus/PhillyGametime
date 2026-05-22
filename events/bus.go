// Package events provides a simple pub/sub bus for game events.
// Subscribe to events here to integrate external systems (e.g., DMX lighting).
//
// Example DMX hook:
//
//	bus.Subscribe(EventGoalScored, func(e Event) {
//	    dmx.Flash(ColorBlue, 500*time.Millisecond)
//	})
package events

import "sync"

type EventType string

const (
	EventScoreUpdate EventType = "score_update"
	EventGameStart   EventType = "game_start"
	EventGameEnd     EventType = "game_end"
	EventGoalScored  EventType = "goal_scored"  // NHL/MLS
	EventTouchdown   EventType = "touchdown"     // NFL
	EventHomeRun     EventType = "home_run"      // MLB
	EventBasket      EventType = "basket"        // NBA (significant plays)
)

type Event struct {
	Type    EventType
	Payload interface{}
}

type Handler func(Event)

type Bus struct {
	mu       sync.RWMutex
	handlers map[EventType][]Handler
}

func NewBus() *Bus {
	return &Bus{
		handlers: make(map[EventType][]Handler),
	}
}

func (b *Bus) Subscribe(eventType EventType, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers[e.Type]))
	copy(handlers, b.handlers[e.Type])
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(e)
	}
}
