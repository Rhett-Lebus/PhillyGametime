package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"gametime/data"
	"gametime/events"
	"gametime/models"
)

type Handler struct {
	store   data.Store
	bus     *events.Bus
	funcMap template.FuncMap
}

func New(store data.Store, bus *events.Bus) *Handler {
	h := &Handler{store: store, bus: bus}
	h.funcMap = h.buildFuncMap()
	return h
}

func (h *Handler) buildFuncMap() template.FuncMap {
	phillyCity := map[string]bool{"Philadelphia": true, "Chester": true}

	return template.FuncMap{
		"upper": func(v interface{}) string { return strings.ToUpper(fmt.Sprintf("%v", v)) },
		"lower": func(v interface{}) string { return strings.ToLower(fmt.Sprintf("%v", v)) },
		"add":   func(a, b int) int { return a + b },
		"isPhilly": func(t models.Team) bool {
			return phillyCity[t.City]
		},
		"vsAt": func(g models.Game) string {
			if phillyCity[g.HomeTeam.City] {
				return "vs"
			}
			return "at"
		},
		"phillyTeam": func(g models.Game) models.Team {
			if phillyCity[g.HomeTeam.City] {
				return g.HomeTeam
			}
			return g.AwayTeam
		},
		"dayLabel": func(t time.Time) string {
			now := data.NowPhilly()
			gameTime := data.PhillyTime(t)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			gameDay := time.Date(gameTime.Year(), gameTime.Month(), gameTime.Day(), 0, 0, 0, 0, now.Location())
			switch int(gameDay.Sub(today).Hours() / 24) {
			case 0:
				return "Today"
			case 1:
				return "Tomorrow"
			default:
				return gameTime.Format("Monday")
			}
		},
		"formatDateTime": func(t time.Time) string {
			return data.PhillyTime(t).Format("Monday, Jan 2 - 3:04 PM MST")
		},
		"broadcastShort": func(network string) string {
			switch strings.ToLower(network) {
			case "nbc sports philadelphia", "nbc sports phil":
				return "NBCSP"
			case "apple tv+":
				return "Apple TV"
			default:
				return network
			}
		},
		"tvClass": func(network string) string {
			switch strings.ToLower(network) {
			case "fox", "fox 29":
				return "fox"
			case "espn", "espn2":
				return "espn"
			case "nbc sports philadelphia", "nbc sports phil", "nbcsp", "nbcsph", "nbcs philly":
				return "nbcsp"
			case "tnt":
				return "tnt"
			case "apple tv+", "apple tv":
				return "apple"
			case "nbc", "nbc10":
				return "nbc"
			case "abc", "6abc":
				return "abc"
			case "phl17", "wphl":
				return "phl"
			default:
				return ""
			}
		},
	}
}

func (h *Handler) render(w http.ResponseWriter, page string, data interface{}) {
	tmpl, err := template.New("").Funcs(h.funcMap).ParseFiles(
		"templates/layout/base.html",
		"templates/layout/header.html",
		"templates/layout/footer.html",
		"templates/pages/"+page+".html",
	)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
	}
}

type HomeData struct {
	NavActive     string
	Title         string
	TodaysGames   []models.Game
	UpcomingGames []models.Game
	Standings     []models.StandingsRow
	Recent        []models.RecentResult
}

type ScoresData struct {
	NavActive string
	Title     string
	Games     []models.Game
}

type UpcomingData struct {
	NavActive string
	Title     string
	Games     []models.Game
}

type TeamsData struct {
	NavActive string
	Title     string
	Teams     []models.Team
}

type StatsData struct {
	NavActive string
	Title     string
	Standings []models.StandingsRow
	Recent    []models.RecentResult
}

type TVData struct {
	NavActive string
	Title     string
	Games     []models.Game
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.render(w, "home", HomeData{
		NavActive:     "home",
		Title:         "Home",
		TodaysGames:   h.store.GetTodaysGames(),
		UpcomingGames: h.store.GetUpcomingGames(),
		Standings:     h.store.GetStandings(),
		Recent:        h.store.GetRecentResults(),
	})
}

func (h *Handler) Scores(w http.ResponseWriter, r *http.Request) {
	h.render(w, "scores", ScoresData{
		NavActive: "scores",
		Title:     "Live Scores",
		Games:     h.store.GetTodaysGames(),
	})
}

func (h *Handler) Upcoming(w http.ResponseWriter, r *http.Request) {
	h.render(w, "upcoming", UpcomingData{
		NavActive: "upcoming",
		Title:     "Upcoming Games",
		Games:     h.store.GetUpcomingGames(),
	})
}

func (h *Handler) Teams(w http.ResponseWriter, r *http.Request) {
	h.render(w, "teams", TeamsData{
		NavActive: "teams",
		Title:     "Teams",
		Teams:     h.store.GetTeams(),
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	h.render(w, "stats", StatsData{
		NavActive: "stats",
		Title:     "Stats",
		Standings: h.store.GetStandings(),
		Recent:    h.store.GetRecentResults(),
	})
}

func (h *Handler) TV(w http.ResponseWriter, r *http.Request) {
	h.render(w, "tv", TVData{
		NavActive: "tv",
		Title:     "TV / Stream",
		Games:     h.store.GetUpcomingGames(),
	})
}

func (h *Handler) APIScores(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.store.GetTodaysGames())
}

func (h *Handler) APIUpcoming(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.store.GetUpcomingGames())
}

func (h *Handler) APIStandings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.store.GetStandings())
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) SSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan events.Event, 16)
	for _, eventType := range []events.EventType{
		events.EventScoreUpdate,
		events.EventGameStart,
		events.EventGameEnd,
		events.EventGoalScored,
		events.EventTouchdown,
		events.EventHomeRun,
		events.EventBasket,
	} {
		eventType := eventType
		h.bus.Subscribe(eventType, func(e events.Event) {
			select {
			case ch <- e:
			case <-r.Context().Done():
			}
		})
	}

	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case e := <-ch:
			data, _ := json.Marshal(e.Payload)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
