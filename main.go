package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gametime/data"
	"gametime/events"
	"gametime/handlers"
	"gametime/models"
)

func main() {
	bus := events.NewBus()
	var store data.Store = data.NewESPNStore()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("PHILLY_DATA")), "mock") {
		store = data.NewMockStore()
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Printf("OpenAI recap cleanup disabled: OPENAI_API_KEY is not set")
	} else {
		log.Printf("OpenAI recap cleanup enabled with model %q", openAIModelName())
	}
	h := handlers.New(store, bus)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /sw.js", serveServiceWorker)

	mux.HandleFunc("GET /", h.Home)
	mux.HandleFunc("GET /scores", h.Scores)
	mux.HandleFunc("GET /upcoming", h.Upcoming)
	mux.HandleFunc("GET /schedule", h.Schedule)
	mux.HandleFunc("GET /teams", h.Teams)
	mux.HandleFunc("GET /teams/{id}", h.TeamDetail)
	mux.HandleFunc("GET /stats", h.Stats)
	mux.HandleFunc("GET /tv", h.TV)
	mux.HandleFunc("GET /world-cup", h.WorldCup)

	mux.HandleFunc("GET /api/scores", h.APIScores)
	mux.HandleFunc("GET /api/upcoming", h.APIUpcoming)
	mux.HandleFunc("GET /api/world-cup", h.APIWorldCup)
	mux.HandleFunc("GET /api/games/{id}/lineup", h.APIGameLineup)
	mux.HandleFunc("GET /api/games/{id}/boxscore", h.APIGameBoxScore)
	mux.HandleFunc("GET /api/standings", h.APIStandings)
	mux.HandleFunc("GET /events", h.SSE)

	go publishScoreChanges(store, bus, 5*time.Second)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Philly Gametime -> http://localhost:%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

func serveServiceWorker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/")
	http.ServeFile(w, r, "static/sw.js")
}

func openAIModelName() string {
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		return model
	}
	return "gpt-5-nano"
}

func publishScoreChanges(store data.Store, bus *events.Bus, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	previous := map[string]models.Game{}
	for {
		for _, game := range store.GetTodaysGames() {
			if game.Status == models.StatusLive {
				if old, ok := previous[game.ID]; ok {
					changedClock := old.Period != game.Period || old.TimeLeft != game.TimeLeft
					changedScore := old.HomeScore != game.HomeScore || old.AwayScore != game.AwayScore
					if changedClock || changedScore {
						bus.Publish(events.Event{Type: events.EventScoreUpdate, Payload: game})
						if changedScore {
							bus.Publish(events.Event{Type: scoringEvent(game), Payload: game})
						}
					}
				} else {
					bus.Publish(events.Event{Type: events.EventGameStart, Payload: game})
				}
			}
			if old, ok := previous[game.ID]; ok && old.Status == models.StatusLive && game.Status == models.StatusFinal {
				if invalidator, ok := store.(interface{ InvalidateRecentResults() }); ok {
					invalidator.InvalidateRecentResults()
				}
				if invalidator, ok := store.(interface{ InvalidateStandings() }); ok {
					invalidator.InvalidateStandings()
				}
				bus.Publish(events.Event{Type: events.EventGameEnd, Payload: game})
			}
			previous[game.ID] = game
		}
		<-ticker.C
	}
}

func scoringEvent(game models.Game) events.EventType {
	switch game.Sport {
	case models.NFL:
		return events.EventTouchdown
	case models.MLB:
		return events.EventHomeRun
	case models.NHL, models.MLS:
		return events.EventGoalScored
	case models.NBA:
		return events.EventBasket
	default:
		return events.EventScoreUpdate
	}
}
