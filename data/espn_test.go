package data

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gametime/models"
)

func TestGetTodaysGamesRequestsPhiladelphiaDate(t *testing.T) {
	wantDate := NowPhilly().Format("20060102")
	gotDate := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDate <- r.URL.Query().Get("dates")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":[]}`))
	}))
	defer server.Close()

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLB,
			ScoreboardURL: server.URL,
			PhillyTeamIDs: []string{"22"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	store.GetTodaysGames()

	select {
	case got := <-gotDate:
		if got != wantDate {
			t.Fatalf("GetTodaysGames() requested dates=%q, want %q", got, wantDate)
		}
	default:
		t.Fatal("GetTodaysGames() did not request the scoreboard endpoint")
	}
}

func TestShouldPrefetchLineupWithinPregameWindow(t *testing.T) {
	now := DatePhilly(2026, time.June, 9, 17, 0, 0)
	game := models.Game{
		ID:        "mlb-game",
		Sport:     models.MLB,
		Status:    models.StatusScheduled,
		StartTime: now.Add(105 * time.Minute),
	}
	if !shouldPrefetchLineup(game, now) {
		t.Fatal("shouldPrefetchLineup() = false, want true at 1h45m before first pitch")
	}

	game.StartTime = now.Add(106 * time.Minute)
	if shouldPrefetchLineup(game, now) {
		t.Fatal("shouldPrefetchLineup() = true, want false before prefetch window")
	}

	game.StartTime = now.Add(30 * time.Minute)
	game.Sport = models.NBA
	if shouldPrefetchLineup(game, now) {
		t.Fatal("shouldPrefetchLineup() = true for non-MLB game")
	}
}

func TestFindMLBGamePkMatchesPhilliesOpponent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"dates": [{
				"games": [{
					"gamePk": 777001,
					"teams": {
						"away": {"team": {"id": 121, "name": "New York Mets"}},
						"home": {"team": {"id": 143, "name": "Philadelphia Phillies"}}
					}
				}]
			}]
		}`))
	}))
	defer server.Close()

	originalScheduleURL := mlbScheduleURL
	mlbScheduleURL = server.URL + "/schedule?date=%s"
	defer func() { mlbScheduleURL = originalScheduleURL }()

	game := models.Game{
		HomeTeam:  Phillies,
		AwayTeam:  Mets,
		Sport:     models.MLB,
		StartTime: DatePhilly(2026, time.May, 26, 18, 45, 0),
	}

	got := NewESPNStore().findMLBGamePk(game)
	if got != 777001 {
		t.Fatalf("findMLBGamePk() = %d, want 777001", got)
	}
}

func TestEnrichMLBGameAddsLivePlayByPlay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/schedule") {
			_, _ = w.Write([]byte(`{
				"dates": [{
					"games": [{
						"gamePk": 777002,
						"teams": {
							"away": {"team": {"id": 121, "name": "New York Mets"}},
							"home": {"team": {"id": 143, "name": "Philadelphia Phillies"}}
						}
					}]
				}]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"liveData": {
				"linescore": {
					"outs": 1,
					"balls": 2,
					"strikes": 1,
					"offense": {
						"first": {"id": 1, "fullName": "Trea Turner"},
						"second": {"id": 0, "fullName": ""},
						"third": {"id": 3, "fullName": "Bryce Harper"},
						"batter": {"id": 4, "fullName": "Kyle Schwarber"}
					},
					"defense": {
						"pitcher": {"id": 5, "fullName": "Kodai Senga"}
					}
				},
				"boxscore": {
					"teams": {
						"away": {
							"players": {
								"ID5": {
									"person": {"id": 5, "fullName": "Kodai Senga"},
									"stats": {"pitching": {"strikeOuts": 7}}
								}
							}
						},
						"home": {"players": {}}
					}
				},
				"plays": {
					"currentPlay": {
						"result": {"description": "Kyle Schwarber takes ball two."}
					},
					"allPlays": [
						{"about": {"inning": 1, "halfInning": "top"}, "result": {"description": "Mets go down in order."}},
						{"about": {"inning": 1, "halfInning": "bottom"}, "result": {"description": "Trea Turner singles on a line drive."}},
						{"about": {"inning": 1, "halfInning": "bottom"}, "result": {"description": "Bryce Harper walks."}}
					]
				}
			}
		}`))
	}))
	defer server.Close()

	originalScheduleURL := mlbScheduleURL
	originalLiveFeedURL := mlbLiveFeedURL
	mlbScheduleURL = server.URL + "/schedule?date=%s"
	mlbLiveFeedURL = server.URL + "/feed/%d"
	defer func() {
		mlbScheduleURL = originalScheduleURL
		mlbLiveFeedURL = originalLiveFeedURL
	}()

	game := models.Game{
		ID:        "espn-1",
		HomeTeam:  Phillies,
		AwayTeam:  Mets,
		Status:    models.StatusLive,
		Sport:     models.MLB,
		StartTime: DatePhilly(2026, time.May, 26, 18, 45, 0),
		Baseball:  &models.BaseballState{},
	}

	got := NewESPNStore().enrichMLBGame(game)
	if got.Baseball == nil {
		t.Fatal("enrichMLBGame() Baseball = nil")
	}
	if got.Baseball.CurrentPlay != "Kyle Schwarber takes ball two." {
		t.Fatalf("CurrentPlay = %q", got.Baseball.CurrentPlay)
	}
	if got.Baseball.Batter != "Kyle Schwarber" || got.Baseball.Pitcher != "Kodai Senga" {
		t.Fatalf("Batter/Pitcher = %q/%q", got.Baseball.Batter, got.Baseball.Pitcher)
	}
	if got.Baseball.PitcherStrikeouts != "7" {
		t.Fatalf("PitcherStrikeouts = %q, want 7", got.Baseball.PitcherStrikeouts)
	}
	if !got.Baseball.OnFirst || !got.Baseball.OnThird || got.Baseball.OnSecond {
		t.Fatalf("base occupancy = first:%v second:%v third:%v", got.Baseball.OnFirst, got.Baseball.OnSecond, got.Baseball.OnThird)
	}
	if len(got.Baseball.Plays) != 3 {
		t.Fatalf("Plays length = %d, want 3", len(got.Baseball.Plays))
	}
	if got.Baseball.Plays[0].Description != "Bryce Harper walks." || got.Baseball.Plays[0].HalfInning != "Bottom" {
		t.Fatalf("latest play = %#v", got.Baseball.Plays[0])
	}
}

func TestRecentResultSummaryUsesESPNDescription(t *testing.T) {
	ev := espnEvent{
		Competitions: []espnCompetition{
			{
				Headlines: []espnHeadline{
					{
						ShortLinkText: "Phillies win",
						Description:   "— Wheeler pitched six shutout innings as the Phillies beat Cleveland 3-0.",
					},
				},
			},
		},
	}

	got, hasProviderSummary := recentResultSummary(ev, Phillies, Mets, 3, 0)
	want := "Wheeler pitched six shutout innings as the Phillies beat Cleveland 3-0."

	if got != want {
		t.Fatalf("recentResultSummary() = %q, want %q", got, want)
	}
	if !hasProviderSummary {
		t.Fatal("recentResultSummary() provider flag = false, want true")
	}
}

func TestRecentResultSummaryFallsBackToScoreSentence(t *testing.T) {
	got, hasProviderSummary := recentResultSummary(espnEvent{}, Phillies, Mets, 3, 0)
	want := "Phillies beat the Mets 3-0."

	if got != want {
		t.Fatalf("recentResultSummary() = %q, want %q", got, want)
	}
	if hasProviderSummary {
		t.Fatal("recentResultSummary() provider flag = true, want false")
	}
}

func TestRecentResultBulletsExtractsHighlights(t *testing.T) {
	summary := "Zack Wheeler pitched six shutout innings and Bryson Stott hit a two-run single as the Philadelphia Phillies defeated Cleveland 3-0 on Saturday, ending the Guardians' seven-game winning streak."
	guardians := models.Team{Name: "Guardians", City: "Cleveland"}

	got := recentResultBullets(summary, Phillies, guardians)
	want := []string{
		"Zack Wheeler pitched six shutout innings.",
		"Bryson Stott hit a two-run single.",
		"The Phillies ended Cleveland's seven-game winning streak.",
	}

	if len(got) != len(want) {
		t.Fatalf("recentResultBullets() returned %d bullets, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recentResultBullets()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestESPNScoreParsesStringAndObjectShapes(t *testing.T) {
	var fromString espnScore
	if err := fromString.UnmarshalJSON([]byte(`"4"`)); err != nil {
		t.Fatalf("UnmarshalJSON string score error = %v", err)
	}
	if string(fromString) != "4" {
		t.Fatalf("string score = %q, want 4", fromString)
	}

	var fromObject espnScore
	if err := fromObject.UnmarshalJSON([]byte(`{"value":6.0,"displayValue":"6"}`)); err != nil {
		t.Fatalf("UnmarshalJSON object score error = %v", err)
	}
	if string(fromObject) != "6" {
		t.Fatalf("object score = %q, want 6", fromObject)
	}
}

func TestParseESPNEventRecognizesUnionPayload(t *testing.T) {
	ev := espnEvent{
		ID:   "761657",
		Date: espnTime{Time: DatePhilly(2026, time.May, 24, 19, 0, 0)},
		Competitions: []espnCompetition{
			{
				Competitors: []espnCompetitor{
					{
						HomeAway: "home",
						Team: espnTeam{
							ID:           "20232",
							Location:     "Inter Miami CF",
							DisplayName:  "Inter Miami CF",
							Abbreviation: "MIA",
						},
					},
					{
						HomeAway: "away",
						Team: espnTeam{
							ID:               "10739",
							Location:         "Philadelphia Union",
							Nickname:         "Union",
							DisplayName:      "Philadelphia Union",
							ShortDisplayName: "Philadelphia",
							Abbreviation:     "PHI",
						},
					},
				},
			},
		},
	}

	got, ok := parseESPNEvent(ev, models.MLS)
	if !ok {
		t.Fatal("parseESPNEvent() returned ok=false")
	}
	if !isPhillyGame(got) {
		t.Fatalf("isPhillyGame() = false for parsed Union game: %#v", got.AwayTeam)
	}
	if got.AwayTeam.ID != "10739" || got.AwayTeam.Name != "Union" || got.AwayTeam.City != "Philadelphia" {
		t.Fatalf("AwayTeam = %#v, want canonical Union with ESPN id 10739", got.AwayTeam)
	}
}

func TestMLSConfigUsesCurrentESPNUnionID(t *testing.T) {
	for _, cfg := range sportConfigs {
		if cfg.Sport != models.MLS {
			continue
		}
		if len(cfg.PhillyTeamIDs) != 1 || cfg.PhillyTeamIDs[0] != "10739" {
			t.Fatalf("MLS PhillyTeamIDs = %#v, want [10739]", cfg.PhillyTeamIDs)
		}
		return
	}
	t.Fatal("MLS sport config not found")
}

func TestGetUpcomingGamesFallsBackToScoreboardDateRange(t *testing.T) {
	startTime := NowPhilly().AddDate(0, 0, 30).UTC().Format(time.RFC3339)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.Query().Get("dates"), "-") {
			_, _ = w.Write([]byte(`{"events":[]}`))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"events": [{
				"id": "union-upcoming-range",
				"date": %q,
				"competitions": [{
					"venue": {"fullName": "Subaru Park", "address": {"city": "Chester", "state": "PA"}},
					"competitors": [
						{"homeAway": "home", "team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"}},
						{"homeAway": "away", "team": {"id": "1908", "location": "New York Red Bulls", "displayName": "New York Red Bulls", "abbreviation": "RBNY"}}
					],
					"status": {"type": {"name": "STATUS_SCHEDULED", "shortDetail": "Scheduled"}}
				}]
			}]
		}`, startTime)))
	}))
	defer server.Close()

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLS,
			ScoreboardURL: server.URL,
			PhillyTeamIDs: []string{"10739"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	got := store.GetUpcomingGames()
	if len(got) != 1 {
		t.Fatalf("GetUpcomingGames() returned %d games, want 1: %#v", len(got), got)
	}
	if got[0].HomeTeam.ID != "10739" || got[0].HomeTeam.Name != "Union" {
		t.Fatalf("HomeTeam = %#v, want Union from date-range scoreboard", got[0].HomeTeam)
	}
}

func TestESPNGameStatusTreatsSoccerFullTimeAsFinal(t *testing.T) {
	got := espnGameStatus(espnStatus{
		Type: espnStatusType{
			Name:        "STATUS_FULL_TIME",
			ShortDetail: "FT",
		},
	})
	if got != models.StatusFinal {
		t.Fatalf("espnGameStatus(STATUS_FULL_TIME) = %q, want %q", got, models.StatusFinal)
	}
}

func TestGetRecentResultsIncludesUnionWithoutUpcomingGame(t *testing.T) {
	now := NowPhilly()
	wantDate := now.AddDate(0, 0, -1).Format("20060102")
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("dates") != wantDate {
			_, _ = w.Write([]byte(`{"events":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"events": [{
				"id": "union-recent",
				"date": "2026-05-24T23:00Z",
				"competitions": [{
					"competitors": [
						{"homeAway": "home", "score": "6", "team": {"id": "20232", "location": "Inter Miami CF", "displayName": "Inter Miami CF", "abbreviation": "MIA"}},
						{"homeAway": "away", "score": "4", "team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"}}
					],
					"status": {"type": {"name": "STATUS_FULL_TIME", "shortDetail": "FT"}}
				}]
			}]
		}`))
	}))
	defer server.Close()

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLS,
			ScoreboardURL: server.URL,
			PhillyTeamIDs: []string{"10739"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	got := store.GetRecentResults()
	if len(got) != 1 {
		t.Fatalf("GetRecentResults() returned %d results, want 1: %#v", len(got), got)
	}
	if got[0].Team.ID != "10739" || got[0].Team.Name != "Union" {
		t.Fatalf("Recent team = %#v, want Union", got[0].Team)
	}
	if len(got[0].Bullets) == 0 {
		t.Fatalf("Recent result did not include fallback recap bullets: %#v", got[0])
	}
}

func TestGetRecentResultsIncludesTodaysFinalGames(t *testing.T) {
	now := NowPhilly()
	wantDate := now.Format("20060102")
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("dates") != wantDate {
			_, _ = w.Write([]byte(`{"events":[]}`))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"events": [{
				"id": "phillies-today-final",
				"date": %q,
				"competitions": [{
					"competitors": [
						{"homeAway": "home", "score": "3", "team": {"id": "22", "location": "Philadelphia", "name": "Phillies", "displayName": "Philadelphia Phillies", "abbreviation": "PHI"}},
						{"homeAway": "away", "score": "0", "team": {"id": "25", "location": "San Diego", "name": "Padres", "displayName": "San Diego Padres", "abbreviation": "SD"}}
					],
					"status": {"type": {"name": "STATUS_FINAL", "shortDetail": "Final"}}
				}]
			}]
		}`, now.UTC().Format(time.RFC3339))))
	}))
	defer server.Close()

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLB,
			ScoreboardURL: server.URL,
			PhillyTeamIDs: []string{"22"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	got := store.GetRecentResults()
	if len(got) != 1 {
		t.Fatalf("GetRecentResults() returned %d results, want 1: %#v", len(got), got)
	}
	if got[0].GameID != "phillies-today-final" {
		t.Fatalf("GameID = %q, want today's final game", got[0].GameID)
	}
}

func TestInvalidateRecentResultsClearsCache(t *testing.T) {
	store := NewESPNStore()
	store.mu.Lock()
	store.resultsCache = resultsCache{
		results:   []models.RecentResult{{GameID: "cached"}},
		expiresAt: time.Now().Add(time.Hour),
	}
	store.mu.Unlock()

	store.InvalidateRecentResults()

	store.mu.RLock()
	expiresAt := store.resultsCache.expiresAt
	store.mu.RUnlock()
	if !expiresAt.IsZero() {
		t.Fatalf("results cache expiresAt = %v, want zero after invalidation", expiresAt)
	}
}

func TestInvalidateStandingsClearsCache(t *testing.T) {
	store := NewESPNStore()
	store.mu.Lock()
	store.standingsCache = standingsCache{
		rows:      []models.StandingsRow{{Team: Phillies, Record: "43-38"}},
		expiresAt: time.Now().Add(time.Hour),
	}
	store.mu.Unlock()

	store.InvalidateStandings()

	store.mu.RLock()
	expiresAt := store.standingsCache.expiresAt
	store.mu.RUnlock()
	if !expiresAt.IsZero() {
		t.Fatalf("standings cache expiresAt = %v, want zero after invalidation", expiresAt)
	}
}

func TestGetRecentResultsEnhancesOnlyDisplayedMostRecentPerTeam(t *testing.T) {
	now := NowPhilly()
	newerDate := now.AddDate(0, 0, -1)
	olderDate := now.AddDate(0, 0, -2)
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))
	newerKey := newerDate.Format("20060102")
	olderKey := olderDate.Format("20060102")

	var openAICalls int32
	openAIDone := make(chan struct{}, 1)
	openAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&openAICalls, 1)
		defer func() { openAIDone <- struct{}{} }()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": [{
				"content": [{
					"type": "output_text",
					"text": "{\"bullets\":[\"Only the displayed result was cleaned\"]}"
				}]
			}]
		}`))
	}))
	defer openAIServer.Close()

	scoreboardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("dates") {
		case newerKey:
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"events": [{
					"id": "phillies-newer",
					"date": %q,
					"competitions": [{
						"competitors": [
							{"homeAway": "home", "score": "6", "team": {"id": "22", "location": "Philadelphia", "name": "Phillies", "displayName": "Philadelphia Phillies", "abbreviation": "PHI"}},
							{"homeAway": "away", "score": "4", "team": {"id": "21", "location": "New York", "name": "Mets", "displayName": "New York Mets", "abbreviation": "NYM"}}
						],
						"status": {"type": {"name": "STATUS_FINAL", "shortDetail": "Final"}},
						"headlines": [{"description": "The Phillies beat the Mets 6-4 after a late push from the lineup."}]
					}]
				}]
			}`, newerDate.UTC().Format(time.RFC3339))))
		case olderKey:
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"events": [{
					"id": "phillies-older",
					"date": %q,
					"competitions": [{
						"competitors": [
							{"homeAway": "home", "score": "2", "team": {"id": "22", "location": "Philadelphia", "name": "Phillies", "displayName": "Philadelphia Phillies", "abbreviation": "PHI"}},
							{"homeAway": "away", "score": "1", "team": {"id": "21", "location": "New York", "name": "Mets", "displayName": "New York Mets", "abbreviation": "NYM"}}
						],
						"status": {"type": {"name": "STATUS_FINAL", "shortDetail": "Final"}}
					}]
				}]
			}`, olderDate.UTC().Format(time.RFC3339))))
		default:
			_, _ = w.Write([]byte(`{"events":[]}`))
		}
	}))
	defer scoreboardServer.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", openAIServer.URL)
	t.Setenv("AI_RECAP_CACHE_PATH", filepath.Join(t.TempDir(), "ai-recaps.json"))

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLB,
			ScoreboardURL: scoreboardServer.URL,
			PhillyTeamIDs: []string{"22"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	got := store.GetRecentResults()
	if len(got) != 1 {
		t.Fatalf("GetRecentResults() returned %d results, want 1: %#v", len(got), got)
	}
	if got[0].Summary != "The Phillies beat the Mets 6-4 after a late push from the lineup." {
		t.Fatalf("Summary = %q, want ESPN/fallback summary preserved", got[0].Summary)
	}
	if len(got[0].Bullets) != 1 || got[0].Bullets[0] != "The Phillies beat the Mets 6-4 after a late push from the lineup." {
		t.Fatalf("Bullets = %#v, want immediate fallback bullets", got[0].Bullets)
	}
	select {
	case <-openAIDone:
	case <-time.After(2 * time.Second):
		t.Fatal("OpenAI background cleanup did not run")
	}
	if atomic.LoadInt32(&openAICalls) != 1 {
		t.Fatalf("OpenAI calls = %d, want 1", openAICalls)
	}
	var cached aiGameRecap
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.RLock()
		cached = store.aiRecapCache["phillies-newer"]
		store.mu.RUnlock()
		if len(cached.Bullets) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(cached.Bullets) != 1 || cached.Bullets[0] != "Only the displayed result was cleaned" {
		t.Fatalf("Cached AI bullets = %#v, want cleaned bullets", cached.Bullets)
	}

	got = store.GetRecentResults()
	if len(got) != 1 {
		t.Fatalf("GetRecentResults() after cache fill returned %d results, want 1: %#v", len(got), got)
	}
	if len(got[0].Bullets) != 1 || got[0].Bullets[0] != "Only the displayed result was cleaned." {
		t.Fatalf("Bullets after cache fill = %#v, want cached AI bullets", got[0].Bullets)
	}
}

func TestGetRecentResultsDoesNotCallOpenAIWithoutProviderSummary(t *testing.T) {
	now := NowPhilly()
	wantDate := now.Format("20060102")
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	var openAICalls int32
	openAIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&openAICalls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer openAIServer.Close()

	scoreboardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("dates") != wantDate {
			_, _ = w.Write([]byte(`{"events":[]}`))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{
			"events": [{
				"id": "phillies-no-provider-summary",
				"date": %q,
				"competitions": [{
					"competitors": [
						{"homeAway": "home", "score": "3", "team": {"id": "22", "location": "Philadelphia", "name": "Phillies", "displayName": "Philadelphia Phillies", "abbreviation": "PHI"}},
						{"homeAway": "away", "score": "0", "team": {"id": "25", "location": "San Diego", "name": "Padres", "displayName": "San Diego Padres", "abbreviation": "SD"}}
					],
					"status": {"type": {"name": "STATUS_FINAL", "shortDetail": "Final"}}
				}]
			}]
		}`, now.UTC().Format(time.RFC3339))))
	}))
	defer scoreboardServer.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", openAIServer.URL)
	t.Setenv("AI_RECAP_CACHE_PATH", filepath.Join(t.TempDir(), "ai-recaps.json"))

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLB,
			ScoreboardURL: scoreboardServer.URL,
			PhillyTeamIDs: []string{"22"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	got := store.GetRecentResults()
	if len(got) != 1 {
		t.Fatalf("GetRecentResults() returned %d results, want 1: %#v", len(got), got)
	}
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&openAICalls) != 0 {
		t.Fatalf("OpenAI calls = %d, want 0 without provider summary", openAICalls)
	}
}

func TestGetRecentResultsAddsMLBHighlights(t *testing.T) {
	now := NowPhilly()
	wantDate := now.Format("20060102")
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/scoreboard":
			if r.URL.Query().Get("dates") != wantDate {
				_, _ = w.Write([]byte(`{"events":[]}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"events": [{
					"id": "espn-phillies-game",
					"date": %q,
					"competitions": [{
						"competitors": [
							{"homeAway": "home", "score": "6", "team": {"id": "22", "location": "Philadelphia", "name": "Phillies", "displayName": "Philadelphia Phillies", "abbreviation": "PHI"}},
							{"homeAway": "away", "score": "4", "team": {"id": "21", "location": "New York", "name": "Mets", "displayName": "New York Mets", "abbreviation": "NYM"}}
						],
						"status": {"type": {"name": "STATUS_FINAL", "shortDetail": "Final"}}
					}]
				}]
			}`, now.UTC().Format(time.RFC3339))))
		case "/mlb/schedule":
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"dates": [{"games": [{
					"gamePk": 123456,
					"gameDate": %q,
					"teams": {
						"home": {"team": {"id": 143, "name": "Philadelphia Phillies"}},
						"away": {"team": {"id": 121, "name": "New York Mets"}}
					}
				}]}]
			}`, now.UTC().Format(time.RFC3339))))
		case "/mlb/game/123456/content":
			_, _ = w.Write([]byte(`{
				"highlights": {"highlights": {"items": [{
					"title": "Phillies top Mets in series opener",
					"description": "Game recap",
					"duration": "00:03:15",
					"date": "2026-05-31T22:30:00Z",
					"image": {"cuts": [{"src": "https://img.example/thumb-small.jpg"}, {"src": "https://img.example/thumb.jpg"}]},
					"playbacks": [{"name": "HTTP_CLOUD_WIRED_WEB", "url": "https://mlb.example/highlight.mp4"}]
				}, {
					"title": "Condensed Game: NYM@PHI - 5/31/26",
					"description": "Condensed game",
					"duration": "00:11:49",
					"date": "2026-05-31T23:30:00Z",
					"image": {"cuts": [{"src": "https://img.example/condensed-small.jpg"}, {"src": "https://img.example/condensed.jpg"}]},
					"playbacks": [{"name": "mp4Avc", "url": "https://mlb.example/condensed.mp4"}]
				}, {
					"title": "Bryson Stott's solo homer",
					"duration": "00:00:27",
					"playbacks": [{"name": "mp4Avc", "url": "https://mlb.example/stott.mp4"}]
				}]}}
			}`))
		default:
			_, _ = w.Write([]byte(`{"events":[]}`))
		}
	}))
	defer server.Close()

	originalConfigs := sportConfigs
	originalScheduleURL := mlbScheduleURL
	originalContentURL := mlbContentURL
	sportConfigs = []sportCfg{{
		Sport:         models.MLB,
		ScoreboardURL: server.URL + "/scoreboard",
		PhillyTeamIDs: []string{"22"},
	}}
	mlbScheduleURL = server.URL + "/mlb/schedule?date=%s"
	mlbContentURL = server.URL + "/mlb/game/%d/content"
	defer func() {
		sportConfigs = originalConfigs
		mlbScheduleURL = originalScheduleURL
		mlbContentURL = originalContentURL
	}()

	store := NewESPNStore()
	got := store.GetRecentResults()
	if len(got) != 1 {
		t.Fatalf("GetRecentResults() returned %d results, want 1: %#v", len(got), got)
	}
	if got[0].HighlightsPending {
		t.Fatal("HighlightsPending = true, want false when a highlight is available")
	}
	if len(got[0].Highlights) != 1 {
		t.Fatalf("Highlights = %#v, want 1 MLB highlight", got[0].Highlights)
	}
	if got[0].Highlights[0].Provider != "MLB" || got[0].Highlights[0].URL != "https://mlb.example/highlight.mp4" {
		t.Fatalf("Highlight = %#v, want short MLB recap playback URL", got[0].Highlights[0])
	}
	if got[0].Highlights[0].Title != "Phillies top Mets in series opener" {
		t.Fatalf("Title = %q, want short game recap", got[0].Highlights[0].Title)
	}
	if got[0].Highlights[0].Thumbnail != "https://img.example/thumb.jpg" {
		t.Fatalf("Thumbnail = %q, want largest provided image", got[0].Highlights[0].Thumbnail)
	}
}

func TestPendingHighlightsRetryAfterNextFetch(t *testing.T) {
	now := NowPhilly().Add(-24 * time.Hour)
	var contentCalls int32
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mlb/schedule":
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"dates": [{"games": [{
					"gamePk": 123456,
					"gameDate": %q,
					"teams": {
						"home": {"team": {"id": 143, "name": "Philadelphia Phillies"}},
						"away": {"team": {"id": 121, "name": "New York Mets"}}
					}
				}]}]
			}`, now.UTC().Format(time.RFC3339))))
		case "/mlb/game/123456/content":
			call := atomic.AddInt32(&contentCalls, 1)
			if call == 1 {
				_, _ = w.Write([]byte(`{"highlights":{"highlights":{"items":[]}}}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"highlights": {"highlights": {"items": [{
					"title": "Game recap",
					"playbacks": [{"name": "HTTP_CLOUD_WIRED_WEB", "url": "https://mlb.example/recap.mp4"}]
				}]}}
			}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	originalScheduleURL := mlbScheduleURL
	originalContentURL := mlbContentURL
	mlbScheduleURL = server.URL + "/mlb/schedule?date=%s"
	mlbContentURL = server.URL + "/mlb/game/%d/content"
	defer func() {
		mlbScheduleURL = originalScheduleURL
		mlbContentURL = originalContentURL
	}()

	store := NewESPNStore()
	game := models.Game{
		ID:        "espn-phillies-game",
		HomeTeam:  Phillies,
		AwayTeam:  Mets,
		StartTime: now,
		Sport:     models.MLB,
	}
	cfg := sportCfg{Sport: models.MLB}
	result := models.RecentResult{GameID: game.ID, GameDate: now}

	got := store.attachHighlights(cfg, game, result)
	if !got.HighlightsPending || len(got.Highlights) != 0 {
		t.Fatalf("First attach = pending %v highlights %#v, want pending with no highlights", got.HighlightsPending, got.Highlights)
	}

	got = store.attachHighlights(cfg, game, result)
	if atomic.LoadInt32(&contentCalls) != 1 {
		t.Fatalf("Content calls = %d, want 1 before retry time", contentCalls)
	}
	if !got.HighlightsPending {
		t.Fatal("HighlightsPending = false before retry time, want true")
	}

	store.mu.Lock()
	entry := store.highlights[game.ID]
	entry.NextFetchAt = time.Now().Add(-time.Minute)
	store.highlights[game.ID] = entry
	store.mu.Unlock()

	got = store.attachHighlights(cfg, game, result)
	if atomic.LoadInt32(&contentCalls) != 2 {
		t.Fatalf("Content calls = %d, want 2 after retry time", contentCalls)
	}
	if got.HighlightsPending || len(got.Highlights) != 1 || got.Highlights[0].URL != "https://mlb.example/recap.mp4" {
		t.Fatalf("Retried attach = pending %v highlights %#v, want found highlight", got.HighlightsPending, got.Highlights)
	}
}

func TestFoundHighlightsRefreshDuringUpgradeWindow(t *testing.T) {
	now := time.Date(2026, 6, 2, 22, 0, 0, 0, time.UTC)
	entry := newHighlightsCacheEntry([]models.VideoHighlight{{
		Title: "Game recap",
		URL:   "https://example.com/recap.mp4",
	}}, now.Add(-2*time.Hour), now)

	want := now.Add(highlightUpgradeRetry)
	if !entry.NextFetchAt.Equal(want) {
		t.Fatalf("NextFetchAt = %v, want %v during upgrade window", entry.NextFetchAt, want)
	}
}

func TestFoundHighlightsUseDailyCacheAfterUpgradeWindow(t *testing.T) {
	now := time.Date(2026, 6, 2, 22, 0, 0, 0, time.UTC)
	entry := newHighlightsCacheEntry([]models.VideoHighlight{{
		Title: "Game recap",
		URL:   "https://example.com/recap.mp4",
	}}, now.Add(-13*time.Hour), now)

	want := now.Add(highlightFoundTTL)
	if !entry.NextFetchAt.Equal(want) {
		t.Fatalf("NextFetchAt = %v, want %v after upgrade window", entry.NextFetchAt, want)
	}
}

func TestFoundHighlightsEmptyRefreshKeepsExistingVideo(t *testing.T) {
	now := NowPhilly()
	var contentCalls int32
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mlb/schedule":
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"dates": [{"games": [{
					"gamePk": 123456,
					"gameDate": %q,
					"teams": {
						"home": {"team": {"id": 143, "name": "Philadelphia Phillies"}},
						"away": {"team": {"id": 121, "name": "New York Mets"}}
					}
				}]}]
			}`, now.UTC().Format(time.RFC3339))))
		case "/mlb/game/123456/content":
			call := atomic.AddInt32(&contentCalls, 1)
			if call == 1 {
				_, _ = w.Write([]byte(`{
					"highlights": {"highlights": {"items": [{
						"title": "Game recap",
						"playbacks": [{"name": "HTTP_CLOUD_WIRED_WEB", "url": "https://mlb.example/recap.mp4"}]
					}]}}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{"highlights":{"highlights":{"items":[]}}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	originalScheduleURL := mlbScheduleURL
	originalContentURL := mlbContentURL
	mlbScheduleURL = server.URL + "/mlb/schedule?date=%s"
	mlbContentURL = server.URL + "/mlb/game/%d/content"
	defer func() {
		mlbScheduleURL = originalScheduleURL
		mlbContentURL = originalContentURL
	}()

	store := NewESPNStore()
	game := models.Game{
		ID:        "espn-phillies-game",
		HomeTeam:  Phillies,
		AwayTeam:  Mets,
		StartTime: now.Add(-2 * time.Hour),
		Sport:     models.MLB,
	}
	cfg := sportCfg{Sport: models.MLB}
	result := models.RecentResult{GameID: game.ID, GameDate: game.StartTime}

	got := store.attachHighlights(cfg, game, result)
	if got.HighlightsPending || len(got.Highlights) != 1 || got.Highlights[0].URL != "https://mlb.example/recap.mp4" {
		t.Fatalf("First attach = pending %v highlights %#v, want found highlight", got.HighlightsPending, got.Highlights)
	}

	store.mu.Lock()
	entry := store.highlights[game.ID]
	entry.NextFetchAt = time.Now().Add(-time.Minute)
	store.highlights[game.ID] = entry
	store.mu.Unlock()

	got = store.attachHighlights(cfg, game, result)
	if atomic.LoadInt32(&contentCalls) != 2 {
		t.Fatalf("Content calls = %d, want refresh attempt", contentCalls)
	}
	if got.HighlightsPending || len(got.Highlights) != 1 || got.Highlights[0].URL != "https://mlb.example/recap.mp4" {
		t.Fatalf("Empty refresh = pending %v highlights %#v, want existing highlight kept", got.HighlightsPending, got.Highlights)
	}
}

func TestPreferredMLBHighlightUsesDuration(t *testing.T) {
	items := []mlbContentItem{
		{Title: "Bryson Stott's solo homer", Duration: "00:00:27"},
		{Title: "Condensed Game: NYM@PHI - 5/31/26", Duration: "00:11:49"},
		{Title: "Phillies-Mets Game Highlights", Duration: "00:03:15"},
	}

	got := preferredMLBHighlightItems(items)
	if len(got) != 1 || got[0].Title != "Phillies-Mets Game Highlights" {
		t.Fatalf("preferredMLBHighlightItems() = %#v, want short game highlights", got)
	}
}

func TestPreferredMLBHighlightAvoidsTinyRecapClip(t *testing.T) {
	items := []mlbContentItem{
		{Title: "Phillies recap", Duration: "00:00:31"},
		{Title: "Condensed Game: NYM@PHI - 5/31/26", Duration: "00:11:49"},
	}

	got := preferredMLBHighlightItems(items)
	if len(got) != 1 || got[0].Title != "Condensed Game: NYM@PHI - 5/31/26" {
		t.Fatalf("preferredMLBHighlightItems() = %#v, want condensed fallback when only recap is tiny", got)
	}
}

func TestHighlightCachePersistsToDisk(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "highlights.json")
	t.Setenv("HIGHLIGHT_CACHE_PATH", cachePath)

	store := NewESPNStore()
	store.mu.Lock()
	store.highlights["game-1"] = highlightsCacheEntry{
		Highlights: []models.VideoHighlight{{
			Title:    "Game recap",
			URL:      "https://example.com/recap.mp4",
			Provider: "MLB",
		}},
		CachedAt:    time.Now().UTC(),
		NextFetchAt: time.Now().Add(24 * time.Hour).UTC(),
		StopAfter:   time.Now().Add(48 * time.Hour).UTC(),
	}
	if err := store.saveHighlightCacheLocked(); err != nil {
		store.mu.Unlock()
		t.Fatalf("saveHighlightCacheLocked() error = %v", err)
	}
	store.mu.Unlock()

	reloaded := NewESPNStore()
	reloaded.mu.RLock()
	entry, ok := reloaded.highlights["game-1"]
	reloaded.mu.RUnlock()
	if !ok {
		t.Fatal("highlight cache did not load persisted game")
	}
	if len(entry.Highlights) != 1 || entry.Highlights[0].URL != "https://example.com/recap.mp4" {
		t.Fatalf("Highlights = %#v, want persisted recap", entry.Highlights)
	}
}

func TestFetchESPNHighlightsPrefersGameHighlights(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"videos": [{
				"headline": "Luis Suarez leads Inter Miami with hat trick in win",
				"links": {"web": {"href": "https://espn.example/player-story"}}
			}, {
				"headline": "Inter Miami CF vs. Philadelphia Union - Game Highlights",
				"description": "Watch the Game Highlights from Inter Miami CF vs. Philadelphia Union",
				"thumbnail": "https://img.example/soccer.jpg",
				"links": {"web": {"href": "https://espn.example/game-highlights"}}
			}]
		}`))
	}))
	defer server.Close()

	store := NewESPNStore()
	got := store.fetchESPNHighlights(sportCfg{
		Sport:      models.MLS,
		SummaryURL: server.URL + "/summary?event=%s",
	}, "union-game")

	if len(got) != 1 {
		t.Fatalf("fetchESPNHighlights() returned %d highlights, want 1: %#v", len(got), got)
	}
	if got[0].URL != "https://espn.example/game-highlights" {
		t.Fatalf("URL = %q, want game highlights URL", got[0].URL)
	}
	if got[0].Provider != "ESPN" {
		t.Fatalf("Provider = %q, want ESPN", got[0].Provider)
	}
}

func TestGetStandingsIncludesUnionWithoutUpcomingGame(t *testing.T) {
	t.Setenv("HIGHLIGHT_CACHE_PATH", filepath.Join(t.TempDir(), "highlights.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/scoreboard":
			if r.URL.Query().Get("dates") != NowPhilly().AddDate(0, 0, -1).Format("20060102") {
				_, _ = w.Write([]byte(`{"events":[]}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"events": [{
					"id": "union-recent-for-standings",
					"date": %q,
					"competitions": [{
						"competitors": [
							{"homeAway": "home", "score": "1", "team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"}},
							{"homeAway": "away", "score": "0", "team": {"id": "1908", "location": "New York Red Bulls", "displayName": "New York Red Bulls", "abbreviation": "RBNY"}}
						],
						"status": {"type": {"name": "STATUS_FULL_TIME", "shortDetail": "FT"}}
					}]
				}]
			}`, NowPhilly().AddDate(0, 0, -1).UTC().Format(time.RFC3339))))
			return
		case "/teams/10739/schedule":
			_, _ = w.Write([]byte(`{
				"events": [
					{
						"id": "union-home-win",
						"date": "2026-05-16T23:30Z",
						"competitions": [{
							"competitors": [
								{"homeAway": "home", "score": "2", "team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"}},
								{"homeAway": "away", "score": "1", "team": {"id": "183", "location": "Columbus Crew", "displayName": "Columbus Crew", "abbreviation": "CLB"}}
							],
							"status": {"type": {"name": "STATUS_FULL_TIME", "shortDetail": "FT"}}
						}]
					},
					{
						"id": "union-home-tie",
						"date": "2026-05-09T23:30Z",
						"competitions": [{
							"competitors": [
								{"homeAway": "home", "score": "1", "team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"}},
								{"homeAway": "away", "score": "1", "team": {"id": "1908", "location": "New York Red Bulls", "displayName": "New York Red Bulls", "abbreviation": "RBNY"}}
							],
							"status": {"type": {"name": "STATUS_FULL_TIME", "shortDetail": "FT"}}
						}]
					},
					{
						"id": "union-away-loss",
						"date": "2026-05-24T23:00Z",
						"competitions": [{
							"competitors": [
								{"homeAway": "home", "score": "6", "team": {"id": "20232", "location": "Inter Miami CF", "displayName": "Inter Miami CF", "abbreviation": "MIA"}},
								{"homeAway": "away", "score": "4", "team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"}}
							],
							"status": {"type": {"name": "STATUS_FULL_TIME", "shortDetail": "FT"}}
						}]
					}
				]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"standings": {
				"entries": [
					{
						"team": {"id": "10739", "location": "Philadelphia Union", "nickname": "Union", "displayName": "Philadelphia Union", "abbreviation": "PHI"},
						"stats": [
							{"name": "wins", "value": 1},
							{"name": "losses", "value": 10},
							{"name": "ties", "value": 4}
						]
					},
					{
						"team": {"id": "20", "location": "Philadelphia", "name": "76ers", "displayName": "Philadelphia 76ers", "abbreviation": "PHI"},
						"stats": [
							{"name": "wins", "value": 24},
							{"name": "losses", "value": 58}
						]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	originalConfigs := sportConfigs
	sportConfigs = []sportCfg{
		{
			Sport:         models.MLS,
			ScoreboardURL: server.URL + "/scoreboard",
			ScheduleBase:  server.URL + "/teams/",
			StandingsURL:  server.URL + "/standings",
			PhillyTeamIDs: []string{"10739"},
		},
	}
	defer func() { sportConfigs = originalConfigs }()

	store := NewESPNStore()
	got := store.GetStandings()
	if len(got) != 1 {
		t.Fatalf("GetStandings() returned %d rows, want 1: %#v", len(got), got)
	}
	if got[0].Team.ID != "10739" || got[0].Team.Name != "Union" {
		t.Fatalf("Standings team = %#v, want Union", got[0].Team)
	}
	if got[0].Record != "1-10-4" || got[0].Home != "1-0-1" || got[0].Away != "0-1-0" {
		t.Fatalf("Union standings = record %q home %q away %q, want 1-10-4 / 1-0-1 / 0-1-0", got[0].Record, got[0].Home, got[0].Away)
	}
}

func TestStandingsEntryToRowUsesDisplayHomeAwayRecords(t *testing.T) {
	entry := espnStandingsEntry{
		Team: espnTeam{ID: "20", Location: "Philadelphia", Name: "76ers", Abbreviation: "PHI"},
		Stats: []espnStat{
			{Name: "wins", Value: 37},
			{Name: "losses", Value: 28},
			{Name: "home", DisplayValue: "22-10"},
			{Name: "road", DisplayValue: "15-18"},
		},
	}

	got := standingsEntryToRow(entry, models.NBA)
	if got.Record != "37-28" || got.Home != "22-10" || got.Away != "15-18" {
		t.Fatalf("NBA row = record %q home %q away %q, want 37-28 / 22-10 / 15-18", got.Record, got.Home, got.Away)
	}
}

func TestStandingsEntryToRowIncludesProviderRank(t *testing.T) {
	entry := standingsEntryWithRank("20", "Philadelphia", "76ers", 37, 28, 7)

	got := standingsEntryToRow(entry, models.NBA)
	if got.Rank != "7" {
		t.Fatalf("Rank = %q, want 7", got.Rank)
	}
}

func TestStandingsEntryToRowUsesNHLDisplayHomeAwayRecords(t *testing.T) {
	entry := espnStandingsEntry{
		Team: espnTeam{ID: "15", Location: "Philadelphia", Name: "Flyers", Abbreviation: "PHI"},
		Stats: []espnStat{
			{Name: "wins", Value: 32},
			{Name: "losses", Value: 32},
			{Name: "otLosses", Value: 11},
			{Name: "home", DisplayValue: "18-14-6"},
			{Name: "away", DisplayValue: "14-18-5"},
		},
	}

	got := standingsEntryToRow(entry, models.NHL)
	if got.Record != "32-32-11" || got.Home != "18-14-6" || got.Away != "14-18-5" {
		t.Fatalf("NHL row = record %q home %q away %q, want 32-32-11 / 18-14-6 / 14-18-5", got.Record, got.Home, got.Away)
	}
}

func TestLeagueStandingsKeepsConferenceAndOverallViewsDistinct(t *testing.T) {
	resp := espnStandingsResp{
		Children: []espnStandingsGroup{
			{
				Name: "Eastern Conference",
				Standings: espnStandingsData{Entries: []espnStandingsEntry{
					standingsEntry("20", "Philadelphia", "76ers", 37, 28),
					standingsEntry("2", "Boston", "Celtics", 50, 15),
				}},
			},
			{
				Name: "Western Conference",
				Standings: espnStandingsData{Entries: []espnStandingsEntry{
					standingsEntry("13", "Los Angeles", "Lakers", 44, 21),
				}},
			},
		},
	}

	got := leagueStandingsFromResponse(sportCfg{Sport: models.NBA, PhillyTeamIDs: []string{"20"}}, resp)
	if len(got.Views) != 3 {
		t.Fatalf("views = %#v, want division, conference, and overall", got.Views)
	}
	if got.Views[0].Key != "division-atlantic" || got.Views[0].Label != "Atlantic" || len(got.Views[0].Rows) != 2 {
		t.Fatalf("division view = %#v, want Atlantic with 2 fixture rows", got.Views[0])
	}
	if got.Views[1].Key != "conference-eastern-conference" || got.Views[1].Label != "Eastern Conference" || len(got.Views[1].Rows) != 2 {
		t.Fatalf("conference view = %#v, want Eastern Conference with 2 rows", got.Views[1])
	}
	if got.Views[2].Key != "overall-nba" || got.Views[2].Label != "NBA" || len(got.Views[2].Rows) != 3 {
		t.Fatalf("overall view = %#v, want NBA overall with 3 rows", got.Views[2])
	}
}

func TestStandingsRowsFromEntriesPreservesProviderOrderWithoutRank(t *testing.T) {
	rows := standingsRowsFromEntries([]espnStandingsEntry{
		standingsEntry("1", "Team", "Middle", 40, 30),
		standingsEntry("2", "Team", "Top", 50, 20),
		standingsEntry("3", "Team", "Bottom", 25, 45),
	}, models.NBA, standingsSortGroup)

	if got := []string{rows[0].Team.Name, rows[1].Team.Name, rows[2].Team.Name}; got[0] != "Middle" || got[1] != "Top" || got[2] != "Bottom" {
		t.Fatalf("sorted teams = %#v, want provider order Middle, Top, Bottom", got)
	}
}

func TestStandingsRowsFromEntriesUsesProviderRankWhenAvailable(t *testing.T) {
	rows := standingsRowsFromEntries([]espnStandingsEntry{
		standingsEntryWithRank("1", "Team", "Second", 45, 20, 2),
		standingsEntryWithRank("2", "Team", "First", 43, 22, 1),
	}, models.NHL, standingsSortGroup)

	if got := []string{rows[0].Team.Name, rows[1].Team.Name}; got[0] != "First" || got[1] != "Second" {
		t.Fatalf("ranked teams = %#v, want First, Second", got)
	}
}

func TestOverallStandingsSortIgnoresConferenceSeed(t *testing.T) {
	rows := standingsRowsFromEntries([]espnStandingsEntry{
		standingsEntryWithRank("1", "East", "Low Seed Better Record", 60, 22, 8),
		standingsEntryWithRank("2", "West", "Top Seed Worse Record", 45, 37, 1),
	}, models.NBA, standingsSortOverall)

	if got := []string{rows[0].Team.Name, rows[1].Team.Name}; got[0] != "Low Seed Better Record" || got[1] != "Top Seed Worse Record" {
		t.Fatalf("overall teams = %#v, want league-wide record order independent of conference seed", got)
	}
	if rows[0].Rank != "1" || rows[1].Rank != "2" {
		t.Fatalf("overall ranks = %q/%q, want recalculated 1/2", rows[0].Rank, rows[1].Rank)
	}
}

func TestPhillyDivisionStandingsUsesFlyersID(t *testing.T) {
	division := phillyDivisionStandings(models.NHL, []espnStandingsEntry{
		standingsEntryWithPoints("4", "Chicago", "Blackhawks", 29, 39, 72),
		standingsEntryWithPoints("15", "Philadelphia", "Flyers", 43, 27, 98),
		standingsEntryWithPoints("16", "Pittsburgh", "Penguins", 41, 25, 98),
	})

	if division.label != "Metropolitan" {
		t.Fatalf("division label = %q, want Metropolitan", division.label)
	}
	if len(division.entries) != 2 {
		t.Fatalf("division entries = %#v, want Flyers and Penguins only", division.entries)
	}
	for _, entry := range division.entries {
		if entry.Team.ID == "4" {
			t.Fatal("Metropolitan division included Chicago id 4")
		}
	}
}

func TestCanonicalPhillyTeamDoesNotTreatChicagoAsFlyers(t *testing.T) {
	got := canonicalPhillyTeam(models.Team{
		ID:    "4",
		City:  "Chicago",
		Name:  "Blackhawks",
		Abbr:  "CHI",
		Sport: models.NHL,
	})
	if got.City == "Philadelphia" || got.Name == "Flyers" {
		t.Fatalf("canonicalPhillyTeam mapped NHL id 4 to Flyers: %#v", got)
	}

	flyers := canonicalPhillyTeam(models.Team{
		ID:    "15",
		City:  "Philadelphia",
		Name:  "Flyers",
		Abbr:  "PHI",
		Sport: models.NHL,
	})
	if flyers.City != "Philadelphia" || flyers.Name != "Flyers" {
		t.Fatalf("canonicalPhillyTeam did not map NHL id 15 to Flyers: %#v", flyers)
	}
}

func TestSportOrderMatchesStandardTeamOrder(t *testing.T) {
	sports := []models.Sport{models.NFL, models.NHL, models.MLB, models.NBA, models.MLS}
	for i, sport := range sports {
		if got := sportOrder(sport); got != i {
			t.Fatalf("sportOrder(%s) = %d, want %d", sport, got, i)
		}
	}
}

func standingsEntry(id, city, name string, wins, losses float64) espnStandingsEntry {
	return espnStandingsEntry{
		Team: espnTeam{ID: id, Location: city, Name: name, Abbreviation: strings.ToUpper(id)},
		Stats: []espnStat{
			{Name: "wins", Value: wins},
			{Name: "losses", Value: losses},
			{Name: "winPercent", Value: wins / (wins + losses)},
		},
	}
}

func standingsEntryWithRank(id, city, name string, wins, losses, rank float64) espnStandingsEntry {
	entry := standingsEntry(id, city, name, wins, losses)
	entry.Stats = append(entry.Stats, espnStat{Name: "playoffSeed", Value: rank})
	return entry
}

func standingsEntryWithPoints(id, city, name string, wins, losses, points float64) espnStandingsEntry {
	entry := standingsEntry(id, city, name, wins, losses)
	entry.Stats = append(entry.Stats, espnStat{Name: "points", Value: points})
	return entry
}

func TestGenerateAIRecapParsesStructuredResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("OpenAI request path = %q, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": [{
				"content": [{
					"type": "output_text",
					"text": "{\"bullets\":[\"Philadelphia won 6-4 at home\",\"The lineup created separation late\"]}"
				}]
			}]
		}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	t.Setenv("OPENAI_MODEL", "gpt-5-nano")

	store := NewESPNStore()
	got, err := store.generateAIRecap(context.Background(), gameRecapFacts{
		Sport:       models.MLB,
		PhillyTeam:  Phillies,
		Opponent:    Mets,
		Home:        true,
		PhillyScore: 6,
		OppScore:    4,
		Result:      "W",
		GameDate:    DatePhilly(2026, time.May, 24, 13, 5, 0),
		Venue:       "Citizens Bank Park",
		City:        "Philadelphia, PA",
		RawSummary:  "Phillies beat the Mets 6-4 behind a late push from the lineup.",
	})
	if err != nil {
		t.Fatalf("generateAIRecap() error = %v", err)
	}
	if len(got.Bullets) != 2 || got.Bullets[0] != "Philadelphia won 6-4 at home" {
		t.Fatalf("Bullets = %#v", got.Bullets)
	}
}

func TestEnhanceRecentResultFallsBackWithoutAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	store := NewESPNStore()
	result := models.RecentResult{
		Team:    Phillies,
		Summary: "Phillies beat the Mets 6-4.",
	}

	got := store.enhanceRecentResult("game-1", result, gameRecapFacts{})
	if got.Summary != result.Summary || len(got.Bullets) != 0 {
		t.Fatalf("enhanceRecentResult() changed fallback result: %#v", got)
	}
}

func TestAIRecapCachePersistsToDisk(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "ai-recaps.json")
	t.Setenv("AI_RECAP_CACHE_PATH", cachePath)

	store := NewESPNStore()
	store.mu.Lock()
	store.aiRecapCache["game-1"] = aiGameRecap{
		Bullets:  []string{"Philadelphia won 6-4."},
		CachedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.saveAIRecapCacheLocked(); err != nil {
		store.mu.Unlock()
		t.Fatalf("saveAIRecapCacheLocked() error = %v", err)
	}
	store.mu.Unlock()

	reloaded := NewESPNStore()
	got, ok := reloaded.cachedAIRecap("game-1")
	if !ok {
		t.Fatal("cachedAIRecap() did not load persisted recap")
	}
	if len(got.Bullets) != 1 || got.Bullets[0] != "Philadelphia won 6-4." {
		t.Fatalf("Bullets = %#v", got.Bullets)
	}
}

func TestAIRecapCachePrunesOldEntries(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "ai-recaps.json")
	t.Setenv("AI_RECAP_CACHE_PATH", cachePath)
	t.Setenv("AI_RECAP_CACHE_MAX_ENTRIES", "2")

	store := NewESPNStore()
	store.mu.Lock()
	store.aiRecapCache["old"] = aiGameRecap{
		Bullets:  []string{"Old."},
		CachedAt: time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339),
	}
	store.aiRecapCache["middle"] = aiGameRecap{
		Bullets:  []string{"Middle."},
		CachedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
	}
	store.aiRecapCache["new"] = aiGameRecap{
		Bullets:  []string{"New."},
		CachedAt: time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
	}
	if err := store.saveAIRecapCacheLocked(); err != nil {
		store.mu.Unlock()
		t.Fatalf("saveAIRecapCacheLocked() error = %v", err)
	}
	store.mu.Unlock()

	reloaded := NewESPNStore()
	if _, ok := reloaded.cachedAIRecap("old"); ok {
		t.Fatal("old cache entry was not pruned")
	}
	if _, ok := reloaded.cachedAIRecap("middle"); !ok {
		t.Fatal("middle cache entry was pruned unexpectedly")
	}
	if _, ok := reloaded.cachedAIRecap("new"); !ok {
		t.Fatal("new cache entry was pruned unexpectedly")
	}
}

func TestPitcherStrikeoutsExtractsCurrentPitcherKColumn(t *testing.T) {
	boxscore := espnBoxscore{
		Players: []espnBoxscoreTeam{
			{
				Statistics: []espnBoxscoreStatGroup{
					{
						Names: []string{"IP", "H", "R", "ER", "BB", "K", "HR"},
						Athletes: []espnBoxscoreAthlete{
							{
								Athlete: espnPlayer{DisplayName: "Zack Wheeler"},
								Stats:   []string{"6.0", "4", "1", "1", "2", "8", "0"},
							},
							{
								Athlete: espnPlayer{DisplayName: "Orion Kerkering"},
								Stats:   []string{"1.0", "0", "0", "0", "0", "2", "0"},
							},
						},
					},
				},
			},
		},
	}

	got := pitcherStrikeouts(boxscore, "Orion Kerkering")
	if got != "2" {
		t.Fatalf("pitcherStrikeouts() = %q, want %q", got, "2")
	}
}

func TestMLBPitcherStrikeoutsUsesLiveFeedBoxscore(t *testing.T) {
	k := 4
	feed := mlbLiveFeedResp{}
	feed.LiveData.Boxscore.Teams.Away.Players = map[string]mlbBoxscorePlayer{
		"ID123": {
			Person: mlbPerson{ID: 123, FullName: "Ranger Suarez"},
			Stats: mlbBoxscorePlayerStats{Pitching: struct {
				ERA        string `json:"era"`
				StrikeOuts *int   `json:"strikeOuts"`
			}{StrikeOuts: &k}},
		},
	}

	got := mlbPitcherStrikeouts(feed, mlbPerson{ID: 123, FullName: "Ranger Suarez"})
	if got != "4" {
		t.Fatalf("mlbPitcherStrikeouts() = %q, want 4", got)
	}
}

func TestMLBLineupFromFeedIncludesSeasonStats(t *testing.T) {
	feed := mlbLiveFeedResp{}
	feed.LiveData.Boxscore.Teams.Away.BattingOrder = []int{11}
	feed.LiveData.Boxscore.Teams.Away.Pitchers = []int{22}
	feed.LiveData.Boxscore.Teams.Away.Players = map[string]mlbBoxscorePlayer{
		"ID11": {
			Person:       mlbPerson{ID: 11, FullName: "Trea Turner"},
			BattingOrder: "100",
			Position: struct {
				Abbreviation string `json:"abbreviation"`
				Name         string `json:"name"`
			}{Abbreviation: "SS"},
			SeasonStats: mlbBoxscorePlayerStats{Batting: struct {
				Avg string `json:"avg"`
			}{Avg: ".289"}},
		},
		"ID22": {
			Person: mlbPerson{ID: 22, FullName: "Zack Wheeler"},
			PitchHand: struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			}{Code: "R"},
			Position: struct {
				Abbreviation string `json:"abbreviation"`
				Name         string `json:"name"`
			}{Abbreviation: "P"},
			SeasonStats: mlbBoxscorePlayerStats{Pitching: struct {
				ERA        string `json:"era"`
				StrikeOuts *int   `json:"strikeOuts"`
			}{ERA: "2.86"}},
		},
	}

	got := mlbLineupFromFeed(feed, models.Game{})
	if got == nil || len(got.Away) != 1 {
		t.Fatalf("mlbLineupFromFeed() lineup = %#v, want away lineup", got)
	}
	if got.Away[0].BattingAverage != ".289" {
		t.Fatalf("BattingAverage = %q, want .289", got.Away[0].BattingAverage)
	}
	if got.AwayPitcher.ERA != "2.86" {
		t.Fatalf("AwayPitcher.ERA = %q, want 2.86", got.AwayPitcher.ERA)
	}
}

func TestHasCompleteLineupEntriesRequiresBothTeams(t *testing.T) {
	partial := &models.BaseballLineup{
		Home: []models.BaseballLineupEntry{{Name: "Trea Turner"}},
	}
	if !hasLineupEntries(partial) {
		t.Fatal("hasLineupEntries() = false, want true for partial lineup")
	}
	if hasCompleteLineupEntries(partial) {
		t.Fatal("hasCompleteLineupEntries() = true, want false for one-sided lineup")
	}

	complete := &models.BaseballLineup{
		Away: []models.BaseballLineupEntry{{Name: "Francisco Lindor"}},
		Home: []models.BaseballLineupEntry{{Name: "Trea Turner"}},
	}
	if !hasCompleteLineupEntries(complete) {
		t.Fatal("hasCompleteLineupEntries() = false, want true when both teams have lineups")
	}
}

func TestWorldCupFlagFallbackUsesAbbrAndTeamName(t *testing.T) {
	tests := []struct {
		name string
		team models.Team
		want string
	}{
		{name: "Curacao abbr", team: models.Team{Abbr: "CUW", Name: "Curaçao"}, want: "https://flagcdn.com/w80/cw.png"},
		{name: "Ivory Coast abbr", team: models.Team{Abbr: "CIV", Name: "Ivory Coast"}, want: "https://flagcdn.com/w80/ci.png"},
		{name: "Cape Verde abbr", team: models.Team{Abbr: "CPV", Name: "Cape Verde"}, want: "https://flagcdn.com/w80/cv.png"},
		{name: "Tunisia abbr", team: models.Team{Abbr: "TUN", Name: "Tunisia"}, want: "https://flagcdn.com/w80/tn.png"},
		{name: "Sweden abbr", team: models.Team{Abbr: "SWE", Name: "Sweden"}, want: "https://flagcdn.com/w80/se.png"},
		{name: "name fallback", team: models.Team{Name: "Curaçao"}, want: "https://flagcdn.com/w80/cw.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := worldCupFlagLogoURL(tt.team); got != tt.want {
				t.Fatalf("worldCupFlagLogoURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestESPNGameStatusUsesStateAndSoccerDetails(t *testing.T) {
	tests := []struct {
		name   string
		status espnStatus
		want   models.GameStatus
	}{
		{name: "state in", status: espnStatus{Type: espnStatusType{State: "in", Name: "STATUS_FIRST_HALF"}}, want: models.StatusLive},
		{name: "first half detail", status: espnStatus{Type: espnStatusType{Name: "STATUS_PERIOD", ShortDetail: "1st Half"}}, want: models.StatusLive},
		{name: "completed", status: espnStatus{Type: espnStatusType{Completed: true, Name: "STATUS_FULL_TIME"}}, want: models.StatusFinal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := espnGameStatus(tt.status); got != tt.want {
				t.Fatalf("espnGameStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasWorldCupMatchNearLiveWindow(t *testing.T) {
	now := DatePhilly(2026, time.June, 11, 15, 30, 0)
	matches := []models.WorldCupMatch{
		{StartTime: DatePhilly(2026, time.June, 11, 15, 0, 0)},
	}
	if !hasWorldCupMatchNearLiveWindow(matches, now) {
		t.Fatal("hasWorldCupMatchNearLiveWindow() = false, want true for in-window match")
	}

	matches = []models.WorldCupMatch{
		{StartTime: DatePhilly(2026, time.June, 12, 15, 0, 0)},
	}
	if hasWorldCupMatchNearLiveWindow(matches, now) {
		t.Fatal("hasWorldCupMatchNearLiveWindow() = true, want false for tomorrow's match")
	}
}

func TestHasCurrentOrFutureGame(t *testing.T) {
	today := DatePhilly(2026, time.May, 27, 0, 0, 0)
	games := []models.Game{
		{StartTime: DatePhilly(2026, time.May, 20, 19, 0, 0)},
		{StartTime: DatePhilly(2026, time.May, 27, 19, 0, 0)},
	}

	if !hasCurrentOrFutureGame(games, today) {
		t.Fatal("hasCurrentOrFutureGame() = false, want true for today's game")
	}

	games = []models.Game{{StartTime: DatePhilly(2026, time.May, 20, 19, 0, 0)}}
	if hasCurrentOrFutureGame(games, today) {
		t.Fatal("hasCurrentOrFutureGame() = true, want false for completed-only schedule")
	}
}
