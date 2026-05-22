package data

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gametime/models"
)

// ── ESPN date type ────────────────────────────────────────────────────────────
// ESPN returns dates in multiple formats; handle them all gracefully.

type espnTime struct{ time.Time }

func (t *espnTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	} {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	t.Time = time.Time{}
	return nil
}

// ── ESPN JSON response types ──────────────────────────────────────────────────

type espnScoreboard struct {
	Events []espnEvent `json:"events"`
}

type espnScheduleResp struct {
	Events []espnEvent `json:"events"`
}

type espnEvent struct {
	ID           string            `json:"id"`
	Date         espnTime          `json:"date"`
	Competitions []espnCompetition `json:"competitions"`
}

type espnCompetition struct {
	Venue       espnVenue        `json:"venue"`
	Competitors []espnCompetitor `json:"competitors"`
	Broadcasts  []espnBroadcast  `json:"broadcasts"`
	Status      espnStatus       `json:"status"`
}

type espnCompetitor struct {
	HomeAway string   `json:"homeAway"`
	Score    string   `json:"score"`
	Team     espnTeam `json:"team"`
}

type espnTeam struct {
	ID             string `json:"id"`
	Location       string `json:"location"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Abbreviation   string `json:"abbreviation"`
	Color          string `json:"color"`
	AlternateColor string `json:"alternateColor"`
	Logos          []struct {
		Href string `json:"href"`
	} `json:"logos"`
}

type espnBroadcast struct {
	Names []string `json:"names"`
}

type espnVenue struct {
	FullName string      `json:"fullName"`
	Address  espnAddress `json:"address"`
}

type espnAddress struct {
	City  string `json:"city"`
	State string `json:"state"`
}

type espnStatus struct {
	DisplayClock string         `json:"displayClock"`
	Period       int            `json:"period"`
	Type         espnStatusType `json:"type"`
}

type espnStatusType struct {
	Name        string `json:"name"`
	ShortDetail string `json:"shortDetail"`
}

// ── Standings JSON types ──────────────────────────────────────────────────────

type espnStandingsResp struct {
	Children  []espnStandingsGroup `json:"children"`
	Standings espnStandingsData    `json:"standings"`
}

type espnStandingsGroup struct {
	Name      string               `json:"name"`
	Children  []espnStandingsGroup `json:"children"`
	Standings espnStandingsData    `json:"standings"`
}

type espnStandingsData struct {
	Entries []espnStandingsEntry `json:"entries"`
}

type espnStandingsEntry struct {
	Team  espnTeam   `json:"team"`
	Stats []espnStat `json:"stats"`
}

type espnStat struct {
	Name         string  `json:"name"`
	Value        float64 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}

// ── Sport config ──────────────────────────────────────────────────────────────

type sportCfg struct {
	Sport         models.Sport
	ScoreboardURL string
	ScheduleBase  string
	StandingsURL  string
	PhillyTeamIDs []string
}

var sportConfigs = []sportCfg{
	{
		Sport:         models.NFL,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/football/nfl/standings",
		PhillyTeamIDs: []string{"21"},
	},
	{
		Sport:         models.MLB,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/baseball/mlb/scoreboard",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/baseball/mlb/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/baseball/mlb/standings",
		PhillyTeamIDs: []string{"22"},
	},
	{
		Sport:         models.NBA,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/basketball/nba/scoreboard",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/basketball/nba/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/basketball/nba/standings",
		PhillyTeamIDs: []string{"20"},
	},
	{
		Sport:         models.NHL,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/hockey/nhl/scoreboard",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/hockey/nhl/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/hockey/nhl/standings",
		PhillyTeamIDs: []string{"4"},
	},
	{
		Sport:         models.MLS,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/soccer/usa.1/scoreboard",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/soccer/usa.1/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/soccer/usa.1/standings",
		PhillyTeamIDs: []string{"16"},
	},
}

var phillyKeywords = map[string]bool{
	"philadelphia": true,
	"eagles":       true,
	"phillies":     true,
	"76ers":        true,
	"flyers":       true,
	"union":        true,
	"chester":      true,
}

func isPhillyESPN(t espnTeam) bool {
	lc := func(s string) bool { return phillyKeywords[strings.ToLower(s)] }
	return lc(t.Location) || lc(t.Name) || lc(t.DisplayName)
}

func isPhillyGame(g models.Game) bool {
	lc := func(s string) bool { return phillyKeywords[strings.ToLower(s)] }
	check := func(t models.Team) bool { return lc(t.City) || lc(t.Name) }
	return check(g.HomeTeam) || check(g.AwayTeam)
}

// ── ESPN Store ────────────────────────────────────────────────────────────────

type gameCache struct {
	games     []models.Game
	expiresAt time.Time
}

type standingsCache struct {
	rows      []models.StandingsRow
	expiresAt time.Time
}

type resultsCache struct {
	results   []models.RecentResult
	expiresAt time.Time
}

type ESPNStore struct {
	client         *http.Client
	mu             sync.RWMutex
	todayCache     gameCache
	upcomingCache  gameCache
	standingsCache standingsCache
	resultsCache   resultsCache
}

func NewESPNStore() *ESPNStore {
	return &ESPNStore{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *ESPNStore) fetchJSON(url string, v interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 PhillyGametime/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// ── Today's games ─────────────────────────────────────────────────────────────

func (s *ESPNStore) GetTodaysGames() []models.Game {
	s.mu.RLock()
	if time.Now().Before(s.todayCache.expiresAt) {
		games := s.todayCache.games
		s.mu.RUnlock()
		return games
	}
	s.mu.RUnlock()

	var mu sync.Mutex
	games := make([]models.Game, 0)
	var wg sync.WaitGroup

	for _, cfg := range sportConfigs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			var sb espnScoreboard
			if err := s.fetchJSON(cfg.ScoreboardURL, &sb); err != nil {
				return
			}
			todayY, todayM, todayD := NowPhilly().Date()
			for _, ev := range sb.Events {
				g, ok := parseESPNEvent(ev, cfg.Sport)
				if !ok || !isPhillyGame(g) {
					continue
				}
				gy, gm, gd := PhillyTime(g.StartTime).Date()
				if gy != todayY || gm != todayM || gd != todayD {
					continue // ESPN scoreboards can include the full week's slate
				}
				mu.Lock()
				games = append(games, g)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(games, func(i, j int) bool {
		li := games[i].Status == models.StatusLive
		lj := games[j].Status == models.StatusLive
		if li != lj {
			return li
		}
		return games[i].StartTime.Before(games[j].StartTime)
	})

	ttl := 60 * time.Second
	for _, g := range games {
		if g.Status == models.StatusLive {
			ttl = 30 * time.Second
			break
		}
	}

	s.mu.Lock()
	s.todayCache = gameCache{games: games, expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	return games
}

// ── Upcoming games ────────────────────────────────────────────────────────────

// GetUpcomingGames queries each sport's scoreboard for the next 7 days and
// returns the soonest upcoming game per Philly team, sorted by date.
// Uses the same scoreboard endpoint that powers today's games — more reliable
// than team schedule endpoints which have inconsistent JSON structures.
func (s *ESPNStore) GetUpcomingGames() []models.Game {
	s.mu.RLock()
	if time.Now().Before(s.upcomingCache.expiresAt) {
		games := s.upcomingCache.games
		s.mu.RUnlock()
		return games
	}
	s.mu.RUnlock()

	var mu sync.Mutex
	nextByKey := map[string]*models.Game{} // keyed by "sport:teamID"
	var wg sync.WaitGroup
	now := NowPhilly()

	for _, cfg := range sportConfigs {
		cfg := cfg
		for daysAhead := 1; daysAhead <= 7; daysAhead++ {
			date := now.AddDate(0, 0, daysAhead).Format("20060102")
			wg.Add(1)
			go func() {
				defer wg.Done()
				url := cfg.ScoreboardURL + "?dates=" + date
				var sb espnScoreboard
				if err := s.fetchJSON(url, &sb); err != nil {
					return
				}
				for _, ev := range sb.Events {
					g, ok := parseESPNEvent(ev, cfg.Sport)
					if !ok || !isPhillyGame(g) {
						continue
					}
					key := phillyGameKey(g)
					mu.Lock()
					if nextByKey[key] == nil || g.StartTime.Before(nextByKey[key].StartTime) {
						gc := g
						nextByKey[key] = &gc
					}
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()

	// Phase 2: for teams with no game in the next 7 days, fall back to the
	// full team schedule so off-season teams (e.g. Eagles in May) still appear
	// once their schedule is published.
	year := now.Format("2006")
	for _, cfg := range sportConfigs {
		for _, teamID := range cfg.PhillyTeamIDs {
			key := string(cfg.Sport) + ":" + teamID
			mu.Lock()
			_, found := nextByKey[key]
			mu.Unlock()
			if found {
				continue
			}
			url := cfg.ScheduleBase + teamID + "/schedule?season=" + year
			var sched espnScheduleResp
			if err := s.fetchJSON(url, &sched); err != nil {
				continue
			}
			for _, ev := range sched.Events {
				g, ok := parseESPNEvent(ev, cfg.Sport)
				if !ok || !isPhillyGame(g) || !PhillyTime(g.StartTime).After(now) {
					continue
				}
				gc := g
				mu.Lock()
				if nextByKey[key] == nil || g.StartTime.Before(nextByKey[key].StartTime) {
					nextByKey[key] = &gc
				}
				mu.Unlock()
			}
		}
	}

	games := make([]models.Game, 0, len(nextByKey))
	for _, g := range nextByKey {
		if g != nil {
			games = append(games, *g)
		}
	}
	sort.Slice(games, func(i, j int) bool {
		return games[i].StartTime.Before(games[j].StartTime)
	})

	s.mu.Lock()
	s.upcomingCache = gameCache{games: games, expiresAt: time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	return games
}

// ── Standings ─────────────────────────────────────────────────────────────────

func (s *ESPNStore) GetStandings() []models.StandingsRow {
	s.mu.RLock()
	if time.Now().Before(s.standingsCache.expiresAt) {
		rows := s.standingsCache.rows
		s.mu.RUnlock()
		return rows
	}
	s.mu.RUnlock()

	// Only show standings for teams that have an upcoming game — this keeps
	// off-season teams hidden and eliminated teams out after season end.
	activeKeys := s.activePhillyTeamKeys()
	var mu sync.Mutex
	rows := make([]models.StandingsRow, 0)
	var wg sync.WaitGroup

	for _, cfg := range sportConfigs {
		cfg := cfg
		if !isInSeason(cfg.Sport) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			var resp espnStandingsResp
			if err := s.fetchJSON(cfg.StandingsURL, &resp); err != nil {
				return
			}
			for _, entry := range flattenStandingsEntries(resp) {
				if !isPhillyESPN(entry.Team) {
					continue
				}
				if !activeKeys[string(cfg.Sport)+":"+entry.Team.ID] {
					continue
				}
				row := standingsEntryToRow(entry, cfg.Sport)
				mu.Lock()
				rows = append(rows, row)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool {
		return string(rows[i].Team.Sport) < string(rows[j].Team.Sport)
	})

	s.mu.Lock()
	s.standingsCache = standingsCache{rows: rows, expiresAt: time.Now().Add(1 * time.Hour)}
	s.mu.Unlock()
	return rows
}

// flattenStandingsEntries walks ESPN's nested group/children structure.
func flattenStandingsEntries(resp espnStandingsResp) []espnStandingsEntry {
	var entries []espnStandingsEntry
	entries = append(entries, resp.Standings.Entries...)

	var walk func(g espnStandingsGroup)
	walk = func(g espnStandingsGroup) {
		entries = append(entries, g.Standings.Entries...)
		for _, child := range g.Children {
			walk(child)
		}
	}
	for _, child := range resp.Children {
		walk(child)
	}
	return entries
}

func standingsEntryToRow(entry espnStandingsEntry, sport models.Sport) models.StandingsRow {
	sm := make(map[string]espnStat, len(entry.Stats))
	for _, s := range entry.Stats {
		sm[s.Name] = s
	}

	intStat := func(names ...string) int {
		for _, n := range names {
			if s, ok := sm[n]; ok {
				return int(s.Value)
			}
		}
		return 0
	}

	w := intStat("wins")
	l := intStat("losses")
	hw := intStat("homeWins", "homeWin")
	hl := intStat("homeLosses", "homeLoss")
	rw := intStat("roadWins", "awayWins", "roadWin")
	rl := intStat("roadLosses", "awayLosses", "roadLoss")

	// NHL OT losses
	otl := intStat("otLosses", "overtimeLosses")
	hotl := intStat("homeOtLosses", "homeOTLoss")
	rotl := intStat("roadOtLosses", "roadOTLoss")

	var record, homeStr, awayStr string
	if sport == models.NHL {
		record = fmt.Sprintf("%d-%d-%d", w, l, otl)
		homeStr = fmt.Sprintf("%d-%d-%d", hw, hl, hotl)
		awayStr = fmt.Sprintf("%d-%d-%d", rw, rl, rotl)
	} else {
		record = fmt.Sprintf("%d-%d", w, l)
		homeStr = fmt.Sprintf("%d-%d", hw, hl)
		awayStr = fmt.Sprintf("%d-%d", rw, rl)
	}

	return models.StandingsRow{
		Team:     espnToTeam(entry.Team, sport),
		Record:   record,
		Home:     homeStr,
		Away:     awayStr,
		HomeDiff: hw - hl,
		AwayDiff: rw - rl,
	}
}

// ── Recent results ────────────────────────────────────────────────────────────

// GetRecentResults queries the past 14 days of scoreboards for completed
// Philly games. Uses the same scoreboard endpoint as today's games.
func (s *ESPNStore) GetRecentResults() []models.RecentResult {
	s.mu.RLock()
	if time.Now().Before(s.resultsCache.expiresAt) {
		results := s.resultsCache.results
		s.mu.RUnlock()
		return results
	}
	s.mu.RUnlock()

	activeKeys := s.activePhillyTeamKeys()
	var mu sync.Mutex
	results := make([]models.RecentResult, 0)
	seen := map[string]bool{}
	var wg sync.WaitGroup
	now := NowPhilly()

	for _, cfg := range sportConfigs {
		cfg := cfg
		if !isInSeason(cfg.Sport) {
			continue
		}
		for daysBack := 1; daysBack <= 14; daysBack++ {
			date := now.AddDate(0, 0, -daysBack).Format("20060102")
			wg.Add(1)
			go func() {
				defer wg.Done()
				url := cfg.ScoreboardURL + "?dates=" + date
				var sb espnScoreboard
				if err := s.fetchJSON(url, &sb); err != nil {
					return
				}
				for _, ev := range sb.Events {
					g, ok := parseESPNEvent(ev, cfg.Sport)
					if !ok || g.Status != models.StatusFinal || !isPhillyGame(g) {
						continue
					}

					var phillyTeam models.Team
					var phillyScore, oppScore int
					if phillyKeywords[strings.ToLower(g.HomeTeam.City)] || phillyKeywords[strings.ToLower(g.HomeTeam.Name)] {
						phillyTeam = g.HomeTeam
						phillyScore = g.HomeScore
						oppScore = g.AwayScore
					} else {
						phillyTeam = g.AwayTeam
						phillyScore = g.AwayScore
						oppScore = g.HomeScore
					}
					if !activeKeys[string(phillyTeam.Sport)+":"+phillyTeam.ID] {
						continue
					}

					result := "W"
					if phillyScore < oppScore {
						result = "L"
					} else if phillyScore == oppScore {
						result = "T"
					}

					mu.Lock()
					if !seen[g.ID] {
						seen[g.ID] = true
						results = append(results, models.RecentResult{
							Team:     phillyTeam,
							Result:   result,
							Record:   fmt.Sprintf("%s %d-%d", result, phillyScore, oppScore),
							GameDate: g.StartTime,
						})
					}
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].GameDate.After(results[j].GameDate)
	})

	// Keep only the most recent result per Philly team.
	byTeam := map[string]models.RecentResult{}
	for _, r := range results {
		key := string(r.Team.Sport) + ":" + r.Team.ID
		if _, exists := byTeam[key]; !exists {
			byTeam[key] = r
		}
	}
	results = results[:0]
	for _, r := range byTeam {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].GameDate.After(results[j].GameDate)
	})

	s.mu.Lock()
	s.resultsCache = resultsCache{results: results, expiresAt: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	return results
}

// ── Misc ──────────────────────────────────────────────────────────────────────

func (s *ESPNStore) GetTeams() []models.Team {
	return []models.Team{Eagles, Phillies, Sixers, Flyers, Union}
}

func (s *ESPNStore) GetGameByID(id string) (*models.Game, bool) {
	all := append(s.GetTodaysGames(), s.GetUpcomingGames()...)
	for i := range all {
		if all[i].ID == id {
			return &all[i], true
		}
	}
	return nil, false
}

// ── Parsing helpers ───────────────────────────────────────────────────────────

func parseESPNEvent(ev espnEvent, sport models.Sport) (models.Game, bool) {
	if len(ev.Competitions) == 0 || len(ev.Competitions[0].Competitors) < 2 {
		return models.Game{}, false
	}
	comp := ev.Competitions[0]

	var home, away espnCompetitor
	for _, c := range comp.Competitors {
		switch c.HomeAway {
		case "home":
			home = c
		case "away":
			away = c
		}
	}

	homeScore, _ := strconv.Atoi(home.Score)
	awayScore, _ := strconv.Atoi(away.Score)

	broadcasts := make([]string, 0)
	for _, b := range comp.Broadcasts {
		broadcasts = append(broadcasts, b.Names...)
	}
	sort.Slice(broadcasts, func(i, j int) bool {
		return broadcastRank(broadcasts[i]) < broadcastRank(broadcasts[j])
	})

	city := comp.Venue.Address.City
	if comp.Venue.Address.State != "" {
		city += ", " + comp.Venue.Address.State
	}

	period, timeLeft := espnPeriod(sport, comp.Status)

	return models.Game{
		ID:        ev.ID,
		HomeTeam:  espnToTeam(home.Team, sport),
		AwayTeam:  espnToTeam(away.Team, sport),
		HomeScore: homeScore,
		AwayScore: awayScore,
		Status:    espnGameStatus(comp.Status),
		Period:    period,
		TimeLeft:  timeLeft,
		StartTime: ev.Date.Time,
		Venue:     comp.Venue.FullName,
		City:      city,
		Broadcast: broadcasts,
		Sport:     sport,
	}, true
}

func espnToTeam(t espnTeam, sport models.Sport) models.Team {
	primary := "#" + t.Color
	if t.Color == "" {
		primary = "#333333"
	}
	secondary := "#" + t.AlternateColor
	if t.AlternateColor == "" {
		secondary = "#ffffff"
	}
	name := t.Name
	if name == "" {
		name = strings.TrimSpace(strings.TrimPrefix(t.DisplayName, t.Location))
	}
	if name == "" {
		name = t.Abbreviation
	}

	team := models.Team{
		ID:        t.ID,
		Name:      name,
		City:      t.Location,
		Abbr:      t.Abbreviation,
		Sport:     sport,
		Primary:   primary,
		Secondary: secondary,
	}
	if len(t.Logos) > 0 {
		team.LogoURL = t.Logos[0].Href
	}
	if team.LogoURL == "" {
		team.LogoURL = fallbackLogoURL(team)
	}
	return canonicalPhillyTeam(team)
}

func espnGameStatus(s espnStatus) models.GameStatus {
	n := s.Type.Name
	switch {
	case strings.HasPrefix(n, "STATUS_FINAL"):
		return models.StatusFinal
	case n == "STATUS_IN_PROGRESS", n == "STATUS_HALFTIME", n == "STATUS_END_PERIOD":
		return models.StatusLive
	case n == "STATUS_POSTPONED":
		return models.StatusPostponed
	case n == "STATUS_CANCELED", n == "STATUS_CANCELLED":
		return models.StatusCancelled
	default:
		return models.StatusScheduled
	}
}

func espnPeriod(sport models.Sport, s espnStatus) (period, timeLeft string) {
	if espnGameStatus(s) != models.StatusLive {
		return s.Type.ShortDetail, ""
	}
	p := s.Period
	clock := s.DisplayClock

	switch sport {
	case models.NBA:
		labels := map[int]string{1: "Q1", 2: "Q2", 3: "Q3", 4: "Q4"}
		if l, ok := labels[p]; ok {
			return l, clock
		}
		return fmt.Sprintf("OT%d", p-4), clock
	case models.NFL:
		labels := map[int]string{1: "1st", 2: "2nd", 3: "3rd", 4: "4th"}
		if l, ok := labels[p]; ok {
			return l, clock
		}
		return "OT", clock
	case models.NHL:
		labels := map[int]string{1: "P1", 2: "P2", 3: "P3"}
		if l, ok := labels[p]; ok {
			return l, clock
		}
		return "OT", clock
	case models.MLS:
		if p == 1 {
			return "1st Half", clock
		}
		return "2nd Half", clock
	case models.MLB:
		return s.Type.ShortDetail, ""
	default:
		return s.Type.ShortDetail, ""
	}
}

// phillyGameKey returns a stable key for deduplicating upcoming/result entries
// per Philly team across multiple day queries.
func phillyGameKey(g models.Game) string {
	if phillyKeywords[strings.ToLower(g.HomeTeam.City)] || phillyKeywords[strings.ToLower(g.HomeTeam.Name)] {
		return string(g.HomeTeam.Sport) + ":" + g.HomeTeam.ID
	}
	return string(g.AwayTeam.Sport) + ":" + g.AwayTeam.ID
}

// broadcastRank returns a priority for a channel name — lower = shown first.
// Philly/local channels rank highest so they surface before national ones.
func (s *ESPNStore) activePhillyTeamKeys() map[string]bool {
	keys := map[string]bool{}
	for _, game := range s.GetUpcomingGames() {
		if !isInSeason(game.Sport) {
			continue
		}
		keys[phillyGameKey(game)] = true
	}
	return keys
}

func broadcastRank(name string) int {
	ranks := map[string]int{
		"nbc sports philadelphia": 1,
		"nbc sports phil":         1,
		"nbcsp":                   1,
		"nbcsph":                  1,
		"nbcs philly":             1,
		"nbc10":                   2,
		"phl17":                   3,
		"6abc":                    4,
		"wphl":                    5,
		"fox 29":                  6,
		"fox":                     7,
		"abc":                     8,
		"espn":                    9,
		"espn2":                   10,
		"tnt":                     11,
		"tbs":                     12,
		"fs1":                     13,
		"nbc":                     14,
		"peacock":                 15,
		"apple tv+":               16,
		"apple tv":                16,
		"mlb network":             17,
		"nfl network":             18,
		"nba tv":                  19,
	}
	if r, ok := ranks[strings.ToLower(name)]; ok {
		return r
	}
	return 99
}

func canonicalPhillyTeam(team models.Team) models.Team {
	teams := map[models.Sport]map[string]models.Team{
		models.NFL: {"21": Eagles},
		models.MLB: {"22": Phillies},
		models.NBA: {"20": Sixers},
		models.NHL: {"4": Flyers, "15": Flyers},
		models.MLS: {"16": Union},
	}
	if byID, ok := teams[team.Sport]; ok {
		if canonical, ok := byID[team.ID]; ok {
			canonical.ID = team.ID
			return canonical
		}
	}
	return team
}

func fallbackLogoURL(team models.Team) string {
	abbr := strings.ToLower(team.Abbr)
	if abbr == "" {
		return ""
	}
	switch team.Sport {
	case models.NFL:
		return "https://a.espncdn.com/i/teamlogos/nfl/500/" + abbr + ".png"
	case models.MLB:
		return "https://a.espncdn.com/i/teamlogos/mlb/500/" + abbr + ".png"
	case models.NBA:
		return "https://a.espncdn.com/i/teamlogos/nba/500/" + abbr + ".png"
	case models.NHL:
		return "https://a.espncdn.com/i/teamlogos/nhl/500/" + abbr + ".png"
	case models.MLS:
		if team.ID != "" {
			return "https://a.espncdn.com/i/teamlogos/soccer/500/" + team.ID + ".png"
		}
	}
	return ""
}

// isInSeason returns true when the sport is actively playing regular/post-season.
func isInSeason(sport models.Sport) bool {
	m := NowPhilly().Month()
	switch sport {
	case models.NFL:
		// September – February
		return m >= time.September || m <= time.February
	case models.MLB:
		// April – October (including playoffs)
		return m >= time.April && m <= time.October
	case models.NBA:
		// October – June
		return m >= time.October || m <= time.June
	case models.NHL:
		// October – June
		return m >= time.October || m <= time.June
	case models.MLS:
		// March – November
		return m >= time.March && m <= time.November
	}
	return false
}
