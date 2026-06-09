package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"gametime/data"
	"gametime/events"
	"gametime/models"
)

type Handler struct {
	store           data.Store
	bus             *events.Bus
	funcMap         template.FuncMap
	showThemePicker bool
}

func New(store data.Store, bus *events.Bus) *Handler {
	h := &Handler{
		store:           store,
		bus:             bus,
		showThemePicker: shouldShowThemePicker(),
	}
	h.funcMap = h.buildFuncMap()
	return h
}

func shouldShowThemePicker() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("PHILLY_ENV")))
	if env == "production" || env == "prod" {
		return false
	}
	if os.Getenv("PHILLY_DATA") == "mock" || env == "local" || env == "dev" || env == "development" {
		return true
	}
	return os.Getenv("PORT") == "" || os.Getenv("PORT") == "8080"
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
		"opponentTeam": func(g models.Game) models.Team {
			if phillyCity[g.HomeTeam.City] {
				return g.AwayTeam
			}
			return g.HomeTeam
		},
		"scheduleResult": func(g models.Game) string {
			if g.Status != models.StatusFinal {
				return ""
			}
			phillyScore, oppScore := phillyGameScores(g, phillyCity)
			switch {
			case phillyScore > oppScore:
				return "W"
			case phillyScore < oppScore:
				return "L"
			default:
				return "T"
			}
		},
		"scheduleResultClass": func(g models.Game) string {
			switch {
			case g.Status != models.StatusFinal:
				return ""
			case phillyGameWon(g, phillyCity):
				return "schedule-result--win"
			case phillyGameLost(g, phillyCity):
				return "schedule-result--loss"
			default:
				return "schedule-result--tie"
			}
		},
		"scheduleScore": func(g models.Game) string {
			if g.Status != models.StatusFinal && g.Status != models.StatusLive {
				return ""
			}
			phillyScore, oppScore := phillyGameScores(g, phillyCity)
			return fmt.Sprintf("%d-%d", phillyScore, oppScore)
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
		"isTodayGame": func(g models.Game) bool {
			now := data.NowPhilly()
			gameTime := data.PhillyTime(g.StartTime)
			ny, nm, nd := now.Date()
			gy, gm, gd := gameTime.Date()
			return ny == gy && nm == gm && nd == gd
		},
		"formatDateTime": func(t time.Time) string {
			return data.PhillyTime(t).Format("Monday, Jan 2 - 3:04 PM MST")
		},
		"formatShortDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return data.PhillyTime(t).Format("Jan 2")
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return data.PhillyTime(t).Format("3:04 PM")
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

func phillyGameScores(g models.Game, phillyCity map[string]bool) (phillyScore, oppScore int) {
	if phillyCity[g.HomeTeam.City] {
		return g.HomeScore, g.AwayScore
	}
	return g.AwayScore, g.HomeScore
}

func phillyGameWon(g models.Game, phillyCity map[string]bool) bool {
	phillyScore, oppScore := phillyGameScores(g, phillyCity)
	return phillyScore > oppScore
}

func phillyGameLost(g models.Game, phillyCity map[string]bool) bool {
	phillyScore, oppScore := phillyGameScores(g, phillyCity)
	return phillyScore < oppScore
}

func (h *Handler) render(w http.ResponseWriter, page string, data interface{}) {
	tmpl, err := template.New("").Funcs(h.funcMap).ParseFiles(
		"templates/layout/base.html",
		"templates/layout/header.html",
		"templates/layout/footer.html",
		"templates/layout/recent_highlights.html",
		"templates/pages/"+page+".html",
	)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", h.withLayout(data)); err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) withLayout(data interface{}) interface{} {
	switch v := data.(type) {
	case HomeData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case ScoresData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case UpcomingData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case ScheduleData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case TeamsData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case TeamDetailData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case StatsData:
		v.ShowThemePicker = h.showThemePicker
		return v
	case TVData:
		v.ShowThemePicker = h.showThemePicker
		return v
	default:
		return data
	}
}

type LayoutData struct {
	ShowThemePicker bool
}

type HomeData struct {
	LayoutData
	NavActive     string
	Title         string
	TodaysGames   []models.Game
	UpcomingGames []models.Game
	Standings     []models.StandingsRow
	Recent        []models.RecentResult
}

type ScoresData struct {
	LayoutData
	NavActive string
	Title     string
	Games     []models.Game
}

type UpcomingData struct {
	LayoutData
	NavActive string
	Title     string
	Games     []models.Game
}

type ScheduleData struct {
	LayoutData
	NavActive string
	Title     string
	Schedules []ScheduleTeamView
}

type ScheduleTeamView struct {
	Team     models.Team
	Months   []ScheduleMonth
	HasGames bool
}

type ScheduleMonth struct {
	Key       string
	Label     string
	Lead      []int
	Days      []ScheduleDay
	IsCurrent bool
	HasGames  bool
}

type ScheduleDay struct {
	Date    time.Time
	Games   []models.Game
	IsToday bool
}

type TeamsData struct {
	LayoutData
	NavActive string
	Title     string
	Teams     []models.Team
}

type TeamDetailData struct {
	LayoutData
	NavActive string
	Title     string
	Team      models.Team
	LiveGame  *models.Game
	NextGame  *models.Game
	Upcoming  []models.Game
	Recent    *models.RecentResult
	Standing  *models.StandingsRow
}

type StatsData struct {
	LayoutData
	NavActive       string
	Title           string
	Standings       []models.StandingsRow
	LeagueStandings []LeagueStandingsView
	Recent          []models.RecentResult
}

type LeagueStandingsView struct {
	Sport  models.Sport
	Label  string
	Active bool
	Views  []LeagueScopeView
}

type LeagueScopeView struct {
	Key    string
	Label  string
	Scope  string
	Active bool
	Rows   []models.StandingsRow
}

type TVData struct {
	LayoutData
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

func (h *Handler) Schedule(w http.ResponseWriter, r *http.Request) {
	h.render(w, "schedule", ScheduleData{
		NavActive: "schedule",
		Title:     "Full Schedule",
		Schedules: buildScheduleViews(h.store.GetFullSchedules()),
	})
}

func buildScheduleViews(schedules []models.TeamSchedule) []ScheduleTeamView {
	today := data.NowPhilly()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	views := make([]ScheduleTeamView, 0, len(schedules)+1)
	allGames := make([]models.Game, 0)
	for _, schedule := range schedules {
		allGames = append(allGames, schedule.Games...)
	}
	views = append(views, buildScheduleView(models.Team{
		ID:      "all",
		Name:    "All Teams",
		City:    "Philadelphia",
		Abbr:    "ALL",
		Primary: "#0f172a",
	}, allGames, today))

	for _, schedule := range schedules {
		views = append(views, buildScheduleView(schedule.Team, schedule.Games, today))
	}
	return views
}

func buildScheduleView(team models.Team, teamGames []models.Game, today time.Time) ScheduleTeamView {
	games := append([]models.Game(nil), teamGames...)
	sort.Slice(games, func(i, j int) bool {
		return games[i].StartTime.Before(games[j].StartTime)
	})

	byMonth := map[string][]models.Game{}
	monthOrder := make([]time.Time, 0)
	if len(games) > 0 {
		currentMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		minMonth := currentMonth
		maxMonth := currentMonth
		for _, game := range games {
			gameDay := data.PhillyTime(game.StartTime)
			monthStart := time.Date(gameDay.Year(), gameDay.Month(), 1, 0, 0, 0, 0, gameDay.Location())
			key := monthStart.Format("2006-01")
			byMonth[key] = append(byMonth[key], game)
			if monthStart.Before(minMonth) {
				minMonth = monthStart
			}
			if monthStart.After(maxMonth) {
				maxMonth = monthStart
			}
		}
		for month := minMonth; !month.After(maxMonth); month = month.AddDate(0, 1, 0) {
			monthOrder = append(monthOrder, month)
		}
	}

	months := make([]ScheduleMonth, 0, len(monthOrder))
	for _, monthStart := range monthOrder {
		key := monthStart.Format("2006-01")
		months = append(months, buildScheduleMonth(monthStart, byMonth[key], today))
	}
	return ScheduleTeamView{Team: team, Months: months, HasGames: len(games) > 0}
}

func buildScheduleMonth(monthStart time.Time, games []models.Game, today time.Time) ScheduleMonth {
	gamesByDay := map[string][]models.Game{}
	for _, game := range games {
		day := data.PhillyTime(game.StartTime)
		key := day.Format("2006-01-02")
		gamesByDay[key] = append(gamesByDay[key], game)
	}
	for key := range gamesByDay {
		sort.Slice(gamesByDay[key], func(i, j int) bool {
			return gamesByDay[key][i].StartTime.Before(gamesByDay[key][j].StartTime)
		})
	}

	nextMonth := monthStart.AddDate(0, 1, 0)
	daysInMonth := nextMonth.AddDate(0, 0, -1).Day()
	days := make([]ScheduleDay, 0, daysInMonth)
	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(monthStart.Year(), monthStart.Month(), day, 0, 0, 0, 0, monthStart.Location())
		key := date.Format("2006-01-02")
		days = append(days, ScheduleDay{
			Date:    date,
			Games:   gamesByDay[key],
			IsToday: date.Equal(today),
		})
	}

	return ScheduleMonth{
		Key:       monthStart.Format("2006-01"),
		Label:     monthStart.Format("January 2006"),
		Lead:      make([]int, int(monthStart.Weekday())),
		Days:      days,
		IsCurrent: monthStart.Year() == today.Year() && monthStart.Month() == today.Month(),
		HasGames:  len(games) > 0,
	}
}

func (h *Handler) Teams(w http.ResponseWriter, r *http.Request) {
	h.render(w, "teams", TeamsData{
		NavActive: "teams",
		Title:     "Teams",
		Teams:     h.store.GetTeams(),
	})
}

func (h *Handler) TeamDetail(w http.ResponseWriter, r *http.Request) {
	teamID := strings.ToLower(strings.TrimSpace(r.PathValue("id")))
	team, ok := h.findTeam(teamID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	liveGame := firstMatchingGame(h.store.GetTodaysGames(), team, func(g models.Game) bool {
		return g.Status == models.StatusLive
	})
	upcoming := upcomingTeamGames(h.store.GetFullSchedules(), team, 5)
	nextGame := firstGame(upcoming)
	if nextGame == nil {
		nextGame = firstMatchingGame(h.store.GetTodaysGames(), team, func(g models.Game) bool {
			return g.Status != models.StatusFinal && g.Status != models.StatusCancelled && g.Status != models.StatusPostponed
		})
	}
	recent := firstRecentResult(h.store.GetRecentResults(), team)
	standing := firstStanding(h.store.GetStandings(), team)

	h.render(w, "team_detail", TeamDetailData{
		NavActive: "teams",
		Title:     team.City + " " + team.Name,
		Team:      team,
		LiveGame:  liveGame,
		NextGame:  nextGame,
		Upcoming:  upcoming,
		Recent:    recent,
		Standing:  standing,
	})
}

func (h *Handler) findTeam(teamID string) (models.Team, bool) {
	for _, team := range h.store.GetTeams() {
		if strings.EqualFold(team.ID, teamID) || strings.EqualFold(team.Name, teamID) {
			return team, true
		}
	}
	return models.Team{}, false
}

func firstMatchingGame(games []models.Game, team models.Team, keep func(models.Game) bool) *models.Game {
	for _, game := range games {
		if teamInGame(game, team) && keep(game) {
			g := game
			return &g
		}
	}
	return nil
}

func upcomingTeamGames(schedules []models.TeamSchedule, team models.Team, limit int) []models.Game {
	now := data.NowPhilly()
	games := make([]models.Game, 0)
	for _, schedule := range schedules {
		if !sameTeam(schedule.Team, team) {
			continue
		}
		for _, game := range schedule.Games {
			if data.PhillyTime(game.StartTime).Before(now) && game.Status != models.StatusLive {
				continue
			}
			if game.Status == models.StatusFinal || game.Status == models.StatusCancelled || game.Status == models.StatusPostponed {
				continue
			}
			games = append(games, game)
		}
		break
	}
	sort.Slice(games, func(i, j int) bool {
		return games[i].StartTime.Before(games[j].StartTime)
	})
	if limit > 0 && len(games) > limit {
		return games[:limit]
	}
	return games
}

func firstGame(games []models.Game) *models.Game {
	if len(games) == 0 {
		return nil
	}
	g := games[0]
	return &g
}

func firstRecentResult(results []models.RecentResult, team models.Team) *models.RecentResult {
	for _, result := range results {
		if sameTeam(result.Team, team) {
			r := result
			return &r
		}
	}
	return nil
}

func firstStanding(rows []models.StandingsRow, team models.Team) *models.StandingsRow {
	for _, row := range rows {
		if sameTeam(row.Team, team) {
			r := row
			return &r
		}
	}
	return nil
}

func teamInGame(game models.Game, team models.Team) bool {
	return sameTeam(game.HomeTeam, team) || sameTeam(game.AwayTeam, team)
}

func sameTeam(a, b models.Team) bool {
	if a.ID != "" && b.ID != "" && strings.EqualFold(a.ID, b.ID) {
		return true
	}
	return strings.EqualFold(a.Name, b.Name) && strings.EqualFold(a.City, b.City)
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	standings := h.store.GetStandings()
	h.render(w, "stats", StatsData{
		NavActive:       "stats",
		Title:           "Stats",
		Standings:       standings,
		LeagueStandings: buildLeagueStandingsViews(h.store.GetLeagueStandings(), activeStandingsSports(standings)),
		Recent:          h.store.GetRecentResults(),
	})
}

func buildLeagueStandingsViews(leagues []models.LeagueStandings, activeSports map[models.Sport]bool) []LeagueStandingsView {
	leagues = append([]models.LeagueStandings(nil), leagues...)
	sort.SliceStable(leagues, func(i, j int) bool {
		return statsLeagueSportLess(leagues[i].Sport, leagues[j].Sport, activeSports)
	})

	views := make([]LeagueStandingsView, 0, len(leagues))
	for _, league := range leagues {
		if len(league.Views) == 0 {
			continue
		}
		scopeViews := make([]LeagueScopeView, 0, len(league.Views))
		activeScope := preferredLeagueScope(league.Views)
		leagueViews := append([]models.StandingsView(nil), league.Views...)
		sort.SliceStable(leagueViews, func(i, j int) bool {
			return standingsScopeOrder(leagueViews[i].Key) < standingsScopeOrder(leagueViews[j].Key)
		})
		for _, view := range leagueViews {
			if len(view.Rows) == 0 {
				continue
			}
			scopeViews = append(scopeViews, LeagueScopeView{
				Key:    view.Key,
				Label:  view.Label,
				Scope:  view.Scope,
				Active: view.Key == activeScope,
				Rows:   view.Rows,
			})
		}
		if len(scopeViews) == 0 {
			continue
		}
		views = append(views, LeagueStandingsView{
			Sport:  league.Sport,
			Label:  teamLabelForSport(league.Sport),
			Active: len(views) == 0,
			Views:  scopeViews,
		})
	}
	return views
}

func activeStandingsSports(rows []models.StandingsRow) map[models.Sport]bool {
	active := make(map[models.Sport]bool, len(rows))
	for _, row := range rows {
		active[row.Team.Sport] = true
	}
	return active
}

func statsLeagueSportLess(a, b models.Sport, activeSports map[models.Sport]bool) bool {
	aActive := activeSports[a]
	bActive := activeSports[b]
	if aActive != bActive {
		return aActive
	}
	return statsSportOrder(a) < statsSportOrder(b)
}

func statsSportOrder(sport models.Sport) int {
	switch sport {
	case models.MLB:
		return 0
	case models.NFL:
		return 1
	case models.NHL:
		return 2
	case models.NBA:
		return 3
	case models.MLS:
		return 4
	default:
		return 99
	}
}

func preferredLeagueScope(views []models.StandingsView) string {
	for _, key := range []string{"overall", "league", "conference", "division"} {
		for _, view := range views {
			if strings.HasPrefix(view.Key, key) && len(view.Rows) > 0 {
				return view.Key
			}
		}
	}
	if len(views) == 0 {
		return ""
	}
	return views[0].Key
}

func standingsScopeOrder(key string) int {
	switch {
	case strings.HasPrefix(key, "overall"):
		return 0
	case strings.HasPrefix(key, "league"):
		return 1
	case strings.HasPrefix(key, "conference"):
		return 2
	case strings.HasPrefix(key, "division"):
		return 3
	default:
		return 9
	}
}

func teamLabelForSport(sport models.Sport) string {
	switch sport {
	case models.NFL:
		return "Eagles"
	case models.NHL:
		return "Flyers"
	case models.MLB:
		return "Phillies"
	case models.NBA:
		return "76ers"
	case models.MLS:
		return "Union"
	default:
		return string(sport)
	}
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

func (h *Handler) APIGameLineup(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	game, ok := h.store.GetGameByID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if game.Sport != models.MLB {
		http.Error(w, "lineups are available for baseball games only", http.StatusBadRequest)
		return
	}

	if game.Lineup != nil {
		writeJSON(w, map[string]interface{}{"Available": true, "Lineup": game.Lineup})
		return
	}

	lineupProvider, ok := h.store.(interface {
		GetGameLineup(string) (*models.BaseballLineup, bool)
	})
	if !ok {
		writeJSON(w, map[string]interface{}{"Available": false, "Message": "Lineup has not been posted yet."})
		return
	}
	lineup, available := lineupProvider.GetGameLineup(id)
	if !available {
		writeJSON(w, map[string]interface{}{"Available": false, "Message": "Lineup has not been posted yet."})
		return
	}
	writeJSON(w, map[string]interface{}{"Available": true, "Lineup": lineup})
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
