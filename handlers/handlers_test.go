package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"gametime/data"
	"gametime/events"
	"gametime/models"
)

func TestFormatDateTimeUsesPhiladelphiaTime(t *testing.T) {
	h := New(data.NewMockStore(), events.NewBus())
	formatDateTime := h.funcMap["formatDateTime"].(func(time.Time) string)

	gameTime := time.Date(2026, time.May, 23, 20, 5, 0, 0, time.UTC)
	got := formatDateTime(gameTime)
	want := "Saturday, May 23 - 4:05 PM EDT"

	if got != want {
		t.Fatalf("formatDateTime() = %q, want %q", got, want)
	}
}

func TestShouldShowThemePickerInMockMode(t *testing.T) {
	t.Setenv("PHILLY_DATA", "mock")
	t.Setenv("PHILLY_ENV", "")
	t.Setenv("PORT", "8081")

	if !shouldShowThemePicker() {
		t.Fatal("shouldShowThemePicker() = false, want true in mock mode")
	}
}

func TestShouldHideThemePickerInProduction(t *testing.T) {
	t.Setenv("PHILLY_DATA", "mock")
	t.Setenv("PHILLY_ENV", "production")
	t.Setenv("PORT", "8080")

	if shouldShowThemePicker() {
		t.Fatal("shouldShowThemePicker() = true, want false in production")
	}
}

func TestShouldShowThemePickerOnDefaultLocalPort(t *testing.T) {
	t.Setenv("PHILLY_DATA", "")
	t.Setenv("PHILLY_ENV", "")
	if err := os.Unsetenv("PORT"); err != nil {
		t.Fatal(err)
	}

	if !shouldShowThemePicker() {
		t.Fatal("shouldShowThemePicker() = false, want true with default local port")
	}
}

func TestWorldCupPageRenders(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(".."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	h := New(data.NewMockStore(), events.NewBus())
	req := httptest.NewRequest(http.MethodGet, "/world-cup", nil)
	rec := httptest.NewRecorder()

	h.WorldCup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("WorldCup() status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"World Cup Match Center", "Live Scores", "How to Watch"} {
		if !strings.Contains(body, want) {
			t.Fatalf("WorldCup() body missing %q", want)
		}
	}
}

func TestDayLabelUsesPhiladelphiaTime(t *testing.T) {
	h := New(data.NewMockStore(), events.NewBus())
	dayLabel := h.funcMap["dayLabel"].(func(time.Time) string)

	now := data.NowPhilly()
	tomorrow := data.DatePhilly(now.Year(), now.Month(), now.Day()+1, 1, 0, 0)

	if got := dayLabel(tomorrow); got != "Tomorrow" {
		t.Fatalf("dayLabel() = %q, want Tomorrow", got)
	}
}

func TestStatsLeagueSportOrderPrefersInSeasonThenTeamOrder(t *testing.T) {
	sports := []models.Sport{models.NFL, models.NHL, models.MLB, models.NBA, models.MLS}
	activeSports := map[models.Sport]bool{
		models.MLB: true,
		models.MLS: true,
	}

	sort.SliceStable(sports, func(i, j int) bool {
		return statsLeagueSportLess(sports[i], sports[j], activeSports)
	})

	want := []models.Sport{models.MLB, models.MLS, models.NFL, models.NHL, models.NBA}
	for i := range want {
		if sports[i] != want[i] {
			t.Fatalf("active Phillies/Union stats order = %#v, want %#v", sports, want)
		}
	}
}

func TestStatsLeagueSportOrderKeepsMLBFirstWhenMultipleSportsActive(t *testing.T) {
	sports := []models.Sport{models.MLB, models.MLS, models.NFL, models.NHL, models.NBA}
	activeSports := map[models.Sport]bool{
		models.NFL: true,
		models.MLB: true,
		models.MLS: true,
	}

	sort.SliceStable(sports, func(i, j int) bool {
		return statsLeagueSportLess(sports[i], sports[j], activeSports)
	})

	want := []models.Sport{models.MLB, models.NFL, models.MLS, models.NHL, models.NBA}
	for i := range want {
		if sports[i] != want[i] {
			t.Fatalf("active Phillies/Eagles/Union stats order = %#v, want %#v", sports, want)
		}
	}
}

func TestBuildLeagueStandingsViewsOrdersScopesBroadToSpecific(t *testing.T) {
	leagues := []models.LeagueStandings{
		{
			Sport: models.MLB,
			Views: []models.StandingsView{
				{Key: "division", Label: "NL East", Scope: "Division", Rows: []models.StandingsRow{{Team: models.Team{Name: "Phillies", Sport: models.MLB}}}},
				{Key: "conference", Label: "National League", Scope: "Conference", Rows: []models.StandingsRow{{Team: models.Team{Name: "Phillies", Sport: models.MLB}}}},
				{Key: "overall", Label: "MLB", Scope: "Overall", Rows: []models.StandingsRow{{Team: models.Team{Name: "Phillies", Sport: models.MLB}}}},
			},
		},
	}

	got := buildLeagueStandingsViews(leagues, map[models.Sport]bool{models.MLB: true})
	if len(got) != 1 || len(got[0].Views) != 3 {
		t.Fatalf("buildLeagueStandingsViews() = %#v, want one league with three views", got)
	}
	wantKeys := []string{"overall", "conference", "division"}
	for i, want := range wantKeys {
		if got[0].Views[i].Key != want {
			t.Fatalf("view order = %#v, want keys %#v", got[0].Views, wantKeys)
		}
	}
	if !got[0].Views[0].Active {
		t.Fatalf("overall view Active = false, want true")
	}
}
