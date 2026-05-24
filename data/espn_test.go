package data

import (
	"net/http"
	"net/http/httptest"
	"testing"

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

	got := recentResultSummary(ev, Phillies, Mets, 3, 0)
	want := "Wheeler pitched six shutout innings as the Phillies beat Cleveland 3-0."

	if got != want {
		t.Fatalf("recentResultSummary() = %q, want %q", got, want)
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

func TestRecentResultSummaryFallsBackToScoreSentence(t *testing.T) {
	got := recentResultSummary(espnEvent{}, Phillies, Mets, 3, 0)
	want := "Phillies beat the Mets 3-0."

	if got != want {
		t.Fatalf("recentResultSummary() = %q, want %q", got, want)
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
