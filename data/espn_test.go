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

func TestGetRecentResultsEnhancesOnlyDisplayedMostRecentPerTeam(t *testing.T) {
	now := NowPhilly()
	newerDate := now.AddDate(0, 0, -1)
	olderDate := now.AddDate(0, 0, -2)
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

func TestGetStandingsIncludesUnionWithoutUpcomingGame(t *testing.T) {
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
