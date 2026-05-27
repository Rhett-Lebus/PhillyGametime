package data

import (
	"gametime/models"
	"time"
)

// Philadelphia teams
var (
	Eagles = models.Team{
		ID: "eagles", Name: "Eagles", City: "Philadelphia",
		Abbr: "PHI", Sport: models.NFL, Primary: "#004C54", Secondary: "#A5ACAF", LogoURL: "https://a.espncdn.com/i/teamlogos/nfl/500/phi.png",
	}
	Phillies = models.Team{
		ID: "phillies", Name: "Phillies", City: "Philadelphia",
		Abbr: "PHI", Sport: models.MLB, Primary: "#E81828", Secondary: "#002D72", LogoURL: "https://a.espncdn.com/i/teamlogos/mlb/500/phi.png",
	}
	Sixers = models.Team{
		ID: "sixers", Name: "76ers", City: "Philadelphia",
		Abbr: "76s", Sport: models.NBA, Primary: "#006BB6", Secondary: "#ED174C", LogoURL: "https://a.espncdn.com/i/teamlogos/nba/500/phi.png",
	}
	Flyers = models.Team{
		ID: "flyers", Name: "Flyers", City: "Philadelphia",
		Abbr: "PHI", Sport: models.NHL, Primary: "#F74902", Secondary: "#000000", LogoURL: "https://a.espncdn.com/i/teamlogos/nhl/500/phi.png",
	}
	Union = models.Team{
		ID: "union", Name: "Union", City: "Philadelphia",
		Abbr: "PHU", Sport: models.MLS, Primary: "#071B2C", Secondary: "#B19B69", LogoURL: "https://a.espncdn.com/i/teamlogos/soccer/500/10739.png",
	}
)

// Opponent teams
var (
	Nets = models.Team{
		ID: "nets", Name: "Nets", City: "Brooklyn",
		Abbr: "BKN", Sport: models.NBA, Primary: "#000000", Secondary: "#FFFFFF", LogoURL: "https://a.espncdn.com/i/teamlogos/nba/500/bkn.png",
	}
	Mets = models.Team{
		ID: "mets", Name: "Mets", City: "New York",
		Abbr: "NYM", Sport: models.MLB, Primary: "#002D72", Secondary: "#FF5910", LogoURL: "https://a.espncdn.com/i/teamlogos/mlb/500/nym.png",
	}
	Giants = models.Team{
		ID: "giants", Name: "Giants", City: "New York",
		Abbr: "NYG", Sport: models.NFL, Primary: "#0B2265", Secondary: "#A71930", LogoURL: "https://a.espncdn.com/i/teamlogos/nfl/500/nyg.png",
	}
	Penguins = models.Team{
		ID: "penguins", Name: "Penguins", City: "Pittsburgh",
		Abbr: "PIT", Sport: models.NHL, Primary: "#000000", Secondary: "#FCB514", LogoURL: "https://a.espncdn.com/i/teamlogos/nhl/500/pit.png",
	}
	RedBulls = models.Team{
		ID: "rbny", Name: "Red Bulls", City: "New York",
		Abbr: "RBNY", Sport: models.MLS, Primary: "#ED1C24", Secondary: "#23A0D8", LogoURL: "https://a.espncdn.com/i/teamlogos/soccer/500/1908.png",
	}
	Bucks = models.Team{
		ID: "bucks", Name: "Bucks", City: "Milwaukee",
		Abbr: "MIL", Sport: models.NBA, Primary: "#00471B", Secondary: "#EEE1C6", LogoURL: "https://a.espncdn.com/i/teamlogos/nba/500/mil.png",
	}
)

// MockStore returns hardcoded data. Replace with ESPN/SportsData.io API calls.
type MockStore struct{}

func NewMockStore() *MockStore { return &MockStore{} }

func (s *MockStore) GetTeams() []models.Team {
	return []models.Team{Eagles, Phillies, Sixers, Flyers, Union}
}

func (s *MockStore) GetTodaysGames() []models.Game {
	now := NowPhilly()
	return []models.Game{
		{
			ID:        "game-sixers-nets",
			HomeTeam:  Sixers,
			AwayTeam:  Nets,
			HomeScore: 89,
			AwayScore: 81,
			Status:    models.StatusLive,
			Period:    "Q3",
			TimeLeft:  "04:12",
			StartTime: DatePhilly(now.Year(), now.Month(), now.Day(), 19, 30, 0),
			Venue:     "Wells Fargo Center",
			City:      "Philadelphia, PA",
			Broadcast: []string{"NBC Sports Philadelphia", "97.5 The Fanatic"},
			Sport:     models.NBA,
		},
		{
			ID:        "game-phillies-mets",
			HomeTeam:  Phillies,
			AwayTeam:  Mets,
			HomeScore: 6,
			AwayScore: 4,
			Status:    models.StatusLive,
			Period:    "Top 7th",
			Baseball: &models.BaseballState{
				OnFirst:           true,
				OnThird:           true,
				Outs:              1,
				Balls:             2,
				Strikes:           1,
				Batter:            "Bryce Harper",
				Pitcher:           "Kodai Senga",
				PitcherStrikeouts: "6",
				CurrentPlay:       "Bryce Harper takes ball two with runners on the corners.",
				Plays: []models.BaseballPlay{
					{Inning: 7, HalfInning: "Top", Description: "Bryce Harper takes ball two with runners on the corners."},
					{Inning: 7, HalfInning: "Top", Description: "Trea Turner singles on a line drive to center. Kyle Schwarber advances to third."},
					{Inning: 7, HalfInning: "Top", Description: "Kyle Schwarber walks."},
					{Inning: 6, HalfInning: "Bottom", Description: "Kodai Senga strikes out Brandon Marsh swinging."},
				},
			},
			StartTime: DatePhilly(now.Year(), now.Month(), now.Day(), 13, 5, 0),
			Venue:     "Citizens Bank Park",
			City:      "Philadelphia, PA",
			Broadcast: []string{"NBC Sports Philadelphia"},
			Sport:     models.MLB,
		},
	}
}

func (s *MockStore) GetUpcomingGames() []models.Game {
	base := NowPhilly()
	next := func(days int) time.Time { return base.AddDate(0, 0, days) }

	return []models.Game{
		{
			ID:          "upcoming-eagles-giants",
			HomeTeam:    Eagles,
			AwayTeam:    Giants,
			Status:      models.StatusScheduled,
			StartTime:   DatePhilly(next(1).Year(), next(1).Month(), next(1).Day(), 16, 25, 0),
			Venue:       "Lincoln Financial Field",
			City:        "Philadelphia, PA",
			Broadcast:   []string{"FOX 29", "94.1 WIP"},
			Sport:       models.NFL,
			IsPreseason: true,
		},
		{
			ID:        "upcoming-flyers-penguins",
			HomeTeam:  Flyers,
			AwayTeam:  Penguins,
			Status:    models.StatusScheduled,
			StartTime: DatePhilly(next(3).Year(), next(3).Month(), next(3).Day(), 19, 0, 0),
			Venue:     "Wells Fargo Center",
			City:      "Philadelphia, PA",
			Broadcast: []string{"NBCSP", "97.5 The Fanatic"},
			Sport:     models.NHL,
		},
		{
			ID:        "upcoming-union-rbny",
			HomeTeam:  RedBulls,
			AwayTeam:  Union,
			Status:    models.StatusScheduled,
			StartTime: DatePhilly(next(4).Year(), next(4).Month(), next(4).Day(), 19, 30, 0),
			Venue:     "Red Bull Arena",
			City:      "Harrison, NJ",
			Broadcast: []string{"Apple TV+"},
			Sport:     models.MLS,
		},
		{
			ID:        "upcoming-sixers-bucks",
			HomeTeam:  Bucks,
			AwayTeam:  Sixers,
			Status:    models.StatusScheduled,
			StartTime: DatePhilly(next(5).Year(), next(5).Month(), next(5).Day(), 20, 0, 0),
			Venue:     "Fiserv Forum",
			City:      "Milwaukee, WI",
			Broadcast: []string{"TNT"},
			Sport:     models.NBA,
		},
	}
}

func (s *MockStore) GetFullSchedules() []models.TeamSchedule {
	upcoming := s.GetUpcomingGames()
	now := NowPhilly()
	return []models.TeamSchedule{
		{Team: Eagles, Games: []models.Game{upcoming[0]}},
		{Team: Phillies, Games: []models.Game{
			{
				ID:        "schedule-phillies-mets-final",
				HomeTeam:  Phillies,
				AwayTeam:  Mets,
				HomeScore: 5,
				AwayScore: 2,
				Status:    models.StatusFinal,
				StartTime: DatePhilly(now.Year(), now.Month(), now.Day()-3, 18, 40, 0),
				Venue:     "Citizens Bank Park",
				City:      "Philadelphia, PA",
				Broadcast: []string{"NBC Sports Philadelphia"},
				Sport:     models.MLB,
			},
			{
				ID:        "schedule-phillies-mets",
				HomeTeam:  Phillies,
				AwayTeam:  Mets,
				Status:    models.StatusScheduled,
				StartTime: DatePhilly(NowPhilly().Year(), time.June, 1, 18, 40, 0),
				Venue:     "Citizens Bank Park",
				City:      "Philadelphia, PA",
				Broadcast: []string{"NBC Sports Philadelphia"},
				Sport:     models.MLB,
			},
		}},
		{Team: Sixers, Games: []models.Game{upcoming[3]}},
		{Team: Flyers, Games: []models.Game{upcoming[1]}},
		{Team: Union, Games: []models.Game{upcoming[2]}},
	}
}

func (s *MockStore) GetRecentResults() []models.RecentResult {
	base := NowPhilly()
	recent := func(daysAgo int) time.Time { return base.AddDate(0, 0, -daysAgo) }

	results := []models.RecentResult{
		{Team: Eagles, Opponent: Giants, Home: true, Result: "L", Record: "L 17-24", Summary: "Eagles fell to the Giants 24-17.", Bullets: []string{"Eagles fell to the Giants 24-17."}, GameDate: recent(1)},
		{Team: Phillies, Opponent: Mets, Home: true, Result: "W", Record: "W 6-4", Summary: "Phillies beat the Mets 6-4 behind a late push from the lineup.", Bullets: []string{"Phillies beat the Mets 6-4.", "The lineup delivered a late push."}, GameDate: recent(2)},
		{Team: Sixers, Opponent: Nets, Home: true, Result: "W", Record: "W 112-103", Summary: "76ers beat the Nets 112-103 with a strong second-half finish.", Bullets: []string{"76ers beat the Nets 112-103.", "Philadelphia closed with a strong second half."}, GameDate: recent(3)},
		{Team: Flyers, Opponent: Penguins, Home: false, Result: "L", Record: "L 2-4", Summary: "Flyers fell to the Penguins 4-2 on the road.", Bullets: []string{"Flyers fell to the Penguins 4-2 on the road."}, GameDate: recent(4)},
		{Team: Union, Opponent: RedBulls, Home: false, Result: "L", Record: "L 0-1", Summary: "Union fell to the Red Bulls 1-0 in a tight road match.", Bullets: []string{"Union fell to the Red Bulls 1-0.", "The road match stayed tight throughout."}, GameDate: recent(5)},
	}
	filtered := make([]models.RecentResult, 0, len(results))
	for _, result := range results {
		if isInSeason(result.Team.Sport) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func (s *MockStore) GetStandings() []models.StandingsRow {
	rows := []models.StandingsRow{
		{Team: Eagles, Record: "14-3", Home: "8-2", Away: "6-1", HomeDiff: 6, AwayDiff: 5},
		{Team: Phillies, Record: "43-38", Home: "25-16", Away: "18-22", HomeDiff: 9, AwayDiff: -4},
		{Team: Sixers, Record: "37-28", Home: "22-10", Away: "15-18", HomeDiff: 12, AwayDiff: -3},
		{Team: Flyers, Record: "32-32-11", Home: "18-14-6", Away: "14-18-5", HomeDiff: 4, AwayDiff: -4},
		{Team: Union, Record: "10-8-5", Home: "6-3-2", Away: "4-5-3", HomeDiff: 3, AwayDiff: -1},
	}
	filtered := make([]models.StandingsRow, 0, len(rows))
	for _, row := range rows {
		if isInSeason(row.Team.Sport) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (s *MockStore) GetGameByID(id string) (*models.Game, bool) {
	all := append(s.GetTodaysGames(), s.GetUpcomingGames()...)
	for _, schedule := range s.GetFullSchedules() {
		all = append(all, schedule.Games...)
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], true
		}
	}
	return nil, false
}
