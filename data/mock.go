package data

import (
	"gametime/models"
	"strconv"
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
	Braves = models.Team{
		ID: "braves", Name: "Braves", City: "Atlanta",
		Abbr: "ATL", Sport: models.MLB, Primary: "#13274F", Secondary: "#CE1141", LogoURL: "https://a.espncdn.com/i/teamlogos/mlb/500/atl.png",
	}
	Marlins = models.Team{
		ID: "marlins", Name: "Marlins", City: "Miami",
		Abbr: "MIA", Sport: models.MLB, Primary: "#00A3E0", Secondary: "#EF3340", LogoURL: "https://a.espncdn.com/i/teamlogos/mlb/500/mia.png",
	}
	Knicks = models.Team{
		ID: "knicks", Name: "Knicks", City: "New York",
		Abbr: "NYK", Sport: models.NBA, Primary: "#006BB6", Secondary: "#F58426", LogoURL: "https://a.espncdn.com/i/teamlogos/nba/500/ny.png",
	}
	Rangers = models.Team{
		ID: "rangers", Name: "Rangers", City: "New York",
		Abbr: "NYR", Sport: models.NHL, Primary: "#0038A8", Secondary: "#CE1126", LogoURL: "https://a.espncdn.com/i/teamlogos/nhl/500/nyr.png",
	}
	Cowboys = models.Team{
		ID: "cowboys", Name: "Cowboys", City: "Dallas",
		Abbr: "DAL", Sport: models.NFL, Primary: "#041E42", Secondary: "#869397", LogoURL: "https://a.espncdn.com/i/teamlogos/nfl/500/dal.png",
	}
)

// MockStore returns hardcoded data. Replace with ESPN/SportsData.io API calls.
type MockStore struct{}

func NewMockStore() *MockStore { return &MockStore{} }

func (s *MockStore) GetTeams() []models.Team {
	return []models.Team{Eagles, Flyers, Phillies, Sixers, Union}
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
			Lineup:    philliesMetsLineup(),
			StartTime: DatePhilly(now.Year(), now.Month(), now.Day(), 13, 5, 0),
			Venue:     "Citizens Bank Park",
			City:      "Philadelphia, PA",
			Broadcast: []string{"NBC Sports Philadelphia"},
			Sport:     models.MLB,
		},
		{
			ID:        "game-union-miami",
			HomeTeam:  Union,
			AwayTeam:  models.Team{ID: "20232", Name: "Inter Miami CF", City: "", Abbr: "MIA", Sport: models.MLS, Primary: "#231F20", Secondary: "#F7B5CD", LogoURL: "https://a.espncdn.com/i/teamlogos/soccer/500/20232.png"},
			HomeScore: 2,
			AwayScore: 1,
			Status:    models.StatusLive,
			Period:    "2nd Half",
			TimeLeft:  "68'",
			Soccer: &models.SoccerState{
				AwayStats: models.SoccerTeamStats{Shots: "9", ShotsOnTarget: "4", YellowCards: "2", RedCards: "0"},
				HomeStats: models.SoccerTeamStats{Shots: "13", ShotsOnTarget: "6", YellowCards: "1", RedCards: "0"},
				Lineup:    mockSoccerLineup(),
			},
			Lineup:    mockSoccerLineup(),
			StartTime: DatePhilly(now.Year(), now.Month(), now.Day(), 19, 30, 0),
			Venue:     "Subaru Park",
			City:      "Chester, PA",
			Broadcast: []string{"Apple TV+"},
			Sport:     models.MLS,
		},
	}
}

func (s *MockStore) GetUpcomingGames() []models.Game {
	base := NowPhilly()
	next := func(days int) time.Time { return base.AddDate(0, 0, days) }

	return []models.Game{
		{
			ID:        "upcoming-phillies-mets",
			HomeTeam:  Phillies,
			AwayTeam:  Mets,
			Status:    models.StatusScheduled,
			StartTime: DatePhilly(next(1).Year(), next(1).Month(), next(1).Day(), 18, 40, 0),
			Venue:     "Citizens Bank Park",
			City:      "Philadelphia, PA",
			Broadcast: []string{"NBC Sports Philadelphia"},
			Sport:     models.MLB,
			Probable: &models.BaseballProbablePitchers{
				Away: models.BaseballLineupPitcher{Name: "Kodai Senga", Handedness: "R"},
				Home: models.BaseballLineupPitcher{Name: "Cristopher Sanchez", Handedness: "L"},
			},
		},
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
		{Team: Flyers, Games: []models.Game{upcoming[1]}},
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
		{Team: Union, Games: []models.Game{upcoming[2]}},
	}
}

func (s *MockStore) GetRecentResults() []models.RecentResult {
	base := NowPhilly()
	recent := func(daysAgo int) time.Time { return base.AddDate(0, 0, -daysAgo) }

	results := []models.RecentResult{
		{GameID: "recent-eagles-giants", Team: Eagles, Opponent: Giants, Home: true, Result: "L", Record: "L 17-24", Summary: "Eagles fell to the Giants 24-17.", Bullets: []string{"Eagles fell to the Giants 24-17."}, GameDate: recent(1)},
		{GameID: "recent-phillies-mets", Team: Phillies, Opponent: Mets, Home: true, Result: "W", Record: "W 6-4", Summary: "Phillies beat the Mets 6-4 behind a late push from the lineup.", Bullets: []string{"Phillies beat the Mets 6-4.", "The lineup delivered a late push."}, GameDate: recent(2)},
		{GameID: "recent-sixers-nets", Team: Sixers, Opponent: Nets, Home: true, Result: "W", Record: "W 112-103", Summary: "76ers beat the Nets 112-103 with a strong second-half finish.", Bullets: []string{"76ers beat the Nets 112-103.", "Philadelphia closed with a strong second half."}, GameDate: recent(3)},
		{GameID: "recent-flyers-penguins", Team: Flyers, Opponent: Penguins, Home: false, Result: "L", Record: "L 2-4", Summary: "Flyers fell to the Penguins 4-2 on the road.", Bullets: []string{"Flyers fell to the Penguins 4-2 on the road."}, GameDate: recent(4)},
		{GameID: "recent-union-red-bulls", Team: Union, Opponent: RedBulls, Home: false, Result: "L", Record: "L 0-1", Summary: "Union fell to the Red Bulls 1-0 in a tight road match.", Bullets: []string{"Union fell to the Red Bulls 1-0.", "The road match stayed tight throughout."}, GameDate: recent(5)},
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
		{Team: Flyers, Record: "32-32-11", Home: "18-14-6", Away: "14-18-5", HomeDiff: 4, AwayDiff: -4},
		{Team: Phillies, Record: "43-38", Home: "25-16", Away: "18-22", HomeDiff: 9, AwayDiff: -4},
		{Team: Sixers, Record: "37-28", Home: "22-10", Away: "15-18", HomeDiff: 12, AwayDiff: -3},
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

func (s *MockStore) GetLeagueStandings() []models.LeagueStandings {
	row := func(rank int, team models.Team, record, home, away, gamesBack string, homeDiff, awayDiff int) models.StandingsRow {
		return models.StandingsRow{Team: team, Record: record, Home: home, Away: away, Rank: strconv.Itoa(rank), GamesBack: gamesBack, HomeDiff: homeDiff, AwayDiff: awayDiff}
	}
	leagues := []models.LeagueStandings{
		{
			Sport: models.MLB,
			Views: []models.StandingsView{
				{Key: "division", Label: "NL East", Scope: "Division", Rows: []models.StandingsRow{
					row(1, Phillies, "43-38", "25-16", "18-22", "-", 9, -4),
					row(2, Braves, "41-40", "23-17", "18-23", "2", 6, -5),
					row(3, Mets, "39-42", "21-20", "18-22", "4", 1, -4),
					row(4, Marlins, "31-50", "17-23", "14-27", "12", -6, -13),
				}},
				{Key: "overall", Label: "MLB", Scope: "Overall", Rows: []models.StandingsRow{
					row(1, Phillies, "43-38", "25-16", "18-22", "-", 9, -4),
					row(2, Braves, "41-40", "23-17", "18-23", "2", 6, -5),
					row(3, Mets, "39-42", "21-20", "18-22", "4", 1, -4),
					row(4, Marlins, "31-50", "17-23", "14-27", "12", -6, -13),
				}},
			},
		},
		{
			Sport: models.NBA,
			Views: []models.StandingsView{
				{Key: "division", Label: "Atlantic", Scope: "Division", Rows: []models.StandingsRow{
					row(1, Knicks, "42-23", "24-9", "18-14", "", 15, 4),
					row(2, Sixers, "37-28", "22-10", "15-18", "", 12, -3),
					row(3, Nets, "24-41", "13-19", "11-22", "", -6, -11),
				}},
				{Key: "conference", Label: "Eastern Conference", Scope: "Conference", Rows: []models.StandingsRow{
					row(1, Knicks, "42-23", "24-9", "18-14", "", 15, 4),
					row(2, Sixers, "37-28", "22-10", "15-18", "", 12, -3),
					row(3, Bucks, "35-30", "20-13", "15-17", "", 7, -2),
					row(4, Nets, "24-41", "13-19", "11-22", "", -6, -11),
				}},
			},
		},
		{
			Sport: models.NFL,
			Views: []models.StandingsView{
				{Key: "division", Label: "NFC East", Scope: "Division", Rows: []models.StandingsRow{
					row(1, Eagles, "14-3", "8-2", "6-1", "", 6, 5),
					row(2, Cowboys, "10-7", "6-3", "4-4", "", 3, 0),
					row(3, Giants, "6-11", "3-5", "3-6", "", -2, -3),
				}},
			},
		},
		{
			Sport: models.NHL,
			Views: []models.StandingsView{
				{Key: "division", Label: "Metropolitan", Scope: "Division", Rows: []models.StandingsRow{
					row(1, Rangers, "44-24-8", "24-10-4", "20-14-4", "", 14, 6),
					row(2, Flyers, "32-32-11", "18-14-6", "14-18-5", "", 4, -4),
					row(3, Penguins, "31-34-10", "17-16-5", "14-18-5", "", 1, -4),
				}},
			},
		},
		{
			Sport: models.MLS,
			Views: []models.StandingsView{
				{Key: "conference", Label: "Eastern Conference", Scope: "Conference", Rows: []models.StandingsRow{
					row(1, Union, "10-8-5", "6-3-2", "4-5-3", "", 3, -1),
					row(2, RedBulls, "9-9-5", "6-4-2", "3-5-3", "", 2, -2),
				}},
			},
		},
	}
	return leagues
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

func (s *MockStore) GetWorldCup() models.WorldCup {
	wc := func(name, abbr, color string) models.Team {
		team := worldCupTeam("", name, abbr, color, "")
		team.LogoURL = worldCupFlagLogoURL(team)
		return team
	}
	germany := wc("Germany", "GER", "#000000")
	australia := wc("Australia", "AUS", "#ffcd00")
	france := wc("France", "FRA", "#1d4ed8")
	egypt := wc("Egypt", "EGY", "#ce1126")
	denmark := wc("Denmark", "DEN", "#c60c30")
	netherlands := wc("Netherlands", "NED", "#f36c21")
	morocco := wc("Morocco", "MAR", "#c1272d")
	colombia := wc("Colombia", "COL", "#fcd116")
	croatia := wc("Croatia", "CRO", "#171796")
	spain := wc("Spain", "ESP", "#aa151b")
	austria := wc("Austria", "AUT", "#ed2939")
	usa := wc("USA", "USA", "#3c3b6e")
	belgium := wc("Belgium", "BEL", "#fae042")
	brazil := wc("Brazil", "BRA", "#009b3a")
	japan := wc("Japan", "JPN", "#bc002d")
	ecuador := wc("Ecuador", "ECU", "#ffdd00")
	senegal := wc("Senegal", "SEN", "#00853f")
	ukraine := wc("Ukraine", "UKR", "#0057b7")
	england := wc("England", "ENG", "#ce1124")
	norway := wc("Norway", "NOR", "#ba0c2f")
	argentina := wc("Argentina", "ARG", "#75aadb")
	uruguay := wc("Uruguay", "URU", "#0038a8")
	turkey := wc("Turkey", "TUR", "#e30a17")
	iran := wc("Iran", "IRN", "#239f40")
	italy := wc("Italy", "ITA", "#008c45")
	algeria := wc("Algeria", "ALG", "#006233")
	portugal := wc("Portugal", "POR", "#006600")
	panama := wc("Panama", "PAN", "#005293")
	mexico := wc("Mexico", "MEX", "#006847")
	mexico.ID = "203"
	southAfrica := wc("South Africa", "RSA", "#087d5a")
	southAfrica.ID = "467"
	southKorea := wc("South Korea", "KOR", "#ce2028")
	southKorea.ID = "451"
	czechia := wc("Czechia", "CZE", "#d7141a")
	czechia.ID = "450"
	canada := wc("Canada", "CAN", "#d52b1e")
	canada.ID = "206"
	switzerland := wc("Switzerland", "SUI", "#ff0000")
	switzerland.ID = "475"

	now := NowPhilly()
	mockMatch := func(id, stage string, home, away models.Team, homeScore, awayScore int) models.WorldCupMatch {
		return models.WorldCupMatch{
			ID: id, Stage: stage, HomeTeam: home, AwayTeam: away,
			HomeScore: homeScore, AwayScore: awayScore, Status: models.StatusFinal,
			StartTime: DatePhilly(now.Year(), time.July, 19, 15, 0, 0),
			Venue:     "Mock Stadium", Broadcast: []string{"FOX"},
		}
	}
	cup := models.WorldCup{
		Live: []models.WorldCupMatch{
			{
				ID: "mock-wc-live", Stage: "Group Stage", HomeTeam: mexico, AwayTeam: southAfrica,
				HomeScore: 1, AwayScore: 0, Status: models.StatusLive, Period: "1st Half", TimeLeft: "34'",
				StartTime: DatePhilly(now.Year(), now.Month(), now.Day(), 15, 0, 0),
				Venue:     "Estadio Banorte", City: "Mexico City", Broadcast: []string{"FOX", "Tele", "Peacock"},
				Soccer: &models.SoccerState{
					AwayStats: models.SoccerTeamStats{Shots: "5", ShotsOnTarget: "2", YellowCards: "1"},
					HomeStats: models.SoccerTeamStats{Shots: "8", ShotsOnTarget: "4"},
					Lineup:    mockSoccerLineupFor(southAfrica, mexico),
					Goals: []models.SoccerGoal{
						{Team: mexico, Scorer: "Santiago Gimenez", Assist: "Edson Alvarez", Minute: "29'"},
					},
				},
			},
		},
		Recent: []models.WorldCupMatch{
			{
				ID: "mock-wc-recent-1", Stage: "Group Stage", HomeTeam: usa, AwayTeam: canada,
				HomeScore: 2, AwayScore: 1, Status: models.StatusFinal, StartTime: now.Add(-6 * time.Hour),
				Venue: "Lincoln Financial Field", City: "Philadelphia", Broadcast: []string{"FOX"},
				Summary: "USA beat Canada 2-1 behind a late winner in Philadelphia.",
				Bullets: []string{"USA found a late winner in Philadelphia.", "Canada stayed close in a one-goal finish."},
			},
		},
		Upcoming: []models.WorldCupMatch{
			{
				ID: "mock-wc-upcoming-1", Stage: "Group Stage", HomeTeam: southKorea, AwayTeam: czechia,
				Status: models.StatusScheduled, StartTime: now.AddDate(0, 0, 1),
				Venue: "Estadio Akron", City: "Guadalajara", Broadcast: []string{"FS1", "Tele", "Peacock"},
				Scenarios: []string{"South Korea qualifies with a win.", "Czechia is eliminated with a loss."},
			},
			{
				ID: "mock-wc-upcoming-2", Stage: "Group Stage", HomeTeam: canada, AwayTeam: switzerland,
				Status: models.StatusScheduled, StartTime: now.AddDate(0, 0, 2),
				Venue: "BMO Field", City: "Toronto", Broadcast: []string{"FOX", "Tele"},
			},
		},
		Groups: []models.WorldCupGroup{
			{Name: "Group A", Rows: []models.WorldCupStanding{
				{Team: mexico, Played: "2", Wins: "2", Draws: "0", Losses: "0", For: "3", Against: "0", Diff: "+3", Points: "6"},
				{Team: southAfrica, Played: "2", Wins: "0", Draws: "1", Losses: "1", For: "1", Against: "3", Diff: "-2", Points: "1"},
				{Team: southKorea, Played: "2", Wins: "1", Draws: "0", Losses: "1", For: "2", Against: "2", Diff: "0", Points: "3"},
				{Team: czechia, Played: "2", Wins: "0", Draws: "1", Losses: "1", For: "2", Against: "3", Diff: "-1", Points: "1"},
			}},
			{Name: "Group B", Rows: []models.WorldCupStanding{
				{Team: canada, Played: "0", Wins: "0", Draws: "0", Losses: "0", For: "0", Against: "0", Diff: "0", Points: "0", Note: "Advance to Round of 32"},
				{Team: switzerland, Played: "0", Wins: "0", Draws: "0", Losses: "0", For: "0", Against: "0", Diff: "0", Points: "0", Note: "Best 8 advance"},
			}},
		},
		Bracket: []models.WorldCupRound{
			{Name: "Round of 32", Matches: []models.WorldCupMatch{
				mockMatch("mock-r32-1", "Round of 32", germany, australia, 2, 0),
				mockMatch("mock-r32-2", "Round of 32", france, egypt, 3, 1),
				mockMatch("mock-r32-3", "Round of 32", denmark, switzerland, 1, 2),
				mockMatch("mock-r32-4", "Round of 32", netherlands, morocco, 2, 1),
				mockMatch("mock-r32-5", "Round of 32", colombia, croatia, 1, 2),
				mockMatch("mock-r32-6", "Round of 32", spain, austria, 2, 0),
				mockMatch("mock-r32-7", "Round of 32", usa, canada, 2, 1),
				mockMatch("mock-r32-8", "Round of 32", belgium, southKorea, 1, 0),
				mockMatch("mock-r32-9", "Round of 32", brazil, japan, 3, 1),
				mockMatch("mock-r32-10", "Round of 32", ecuador, senegal, 0, 1),
				mockMatch("mock-r32-11", "Round of 32", mexico, ukraine, 2, 1),
				mockMatch("mock-r32-12", "Round of 32", england, norway, 2, 0),
				mockMatch("mock-r32-13", "Round of 32", argentina, uruguay, 2, 1),
				mockMatch("mock-r32-14", "Round of 32", turkey, iran, 1, 0),
				mockMatch("mock-r32-15", "Round of 32", italy, algeria, 1, 2),
				mockMatch("mock-r32-16", "Round of 32", portugal, panama, 3, 0),
			}},
			{Name: "Round of 16", Matches: []models.WorldCupMatch{
				mockMatch("mock-r16-1", "Round of 16", germany, france, 1, 2),
				mockMatch("mock-r16-2", "Round of 16", switzerland, netherlands, 1, 2),
				mockMatch("mock-r16-3", "Round of 16", croatia, spain, 1, 3),
				mockMatch("mock-r16-4", "Round of 16", usa, belgium, 2, 1),
				mockMatch("mock-r16-5", "Round of 16", brazil, senegal, 2, 0),
				mockMatch("mock-r16-6", "Round of 16", mexico, england, 1, 2),
				mockMatch("mock-r16-7", "Round of 16", argentina, turkey, 3, 1),
				mockMatch("mock-r16-8", "Round of 16", algeria, portugal, 0, 2),
			}},
			{Name: "Quarterfinals", Matches: []models.WorldCupMatch{
				mockMatch("mock-qf-1", "Quarterfinals", france, netherlands, 2, 1),
				mockMatch("mock-qf-2", "Quarterfinals", spain, usa, 2, 0),
				mockMatch("mock-qf-3", "Quarterfinals", brazil, england, 1, 2),
				mockMatch("mock-qf-4", "Quarterfinals", argentina, portugal, 2, 1),
			}},
			{Name: "Semifinals", Matches: []models.WorldCupMatch{
				mockMatch("mock-sf-1", "Semifinals", france, spain, 1, 2),
				mockMatch("mock-sf-2", "Semifinals", england, argentina, 1, 2),
			}},
			{Name: "Final", Matches: []models.WorldCupMatch{
				mockMatch("mock-final", "Final", spain, argentina, 2, 1),
			}},
		},
		Watch: []models.WorldCupWatch{
			{Label: "English TV", Description: "National match windows on FOX or FS1.", Networks: []string{"FOX", "FS1"}},
			{Label: "Spanish TV", Description: "Spanish-language coverage listed by ESPN as Tele.", Networks: []string{"Tele"}},
			{Label: "Streaming", Description: "Streaming availability appears on match cards when listed.", Networks: []string{"Peacock"}},
		},
		Leaders: []models.WorldCupLeaderCategory{
			{Name: "Goals", Kind: "player", Leaders: []models.WorldCupLeader{
				{Player: "Mock Scorer", Team: mexico, Value: 3, Rank: 1},
			}},
			{Name: "Assists", Kind: "player", Leaders: []models.WorldCupLeader{
				{Player: "Mock Creator", Team: canada, Value: 2, Rank: 1},
			}},
		},
	}
	applyWorldCupBracketLayout(&cup)
	return cup
}

func worldCupTeam(id, name, abbr, color, logo string) models.Team {
	return models.Team{ID: id, Name: name, City: name, Abbr: abbr, Sport: models.FIFA, Primary: color, Secondary: "#ffffff", LogoURL: logo}
}

func (s *MockStore) GetGameLineup(id string) (*models.BaseballLineup, bool) {
	game, ok := s.GetGameByID(id)
	if ok && game.Lineup != nil && (game.Sport == models.MLB || game.Sport == models.MLS || game.Sport == models.FIFA) {
		return game.Lineup, true
	}
	for _, match := range s.GetWorldCup().Live {
		if match.ID == id && match.Soccer != nil && match.Soccer.Lineup != nil {
			return match.Soccer.Lineup, true
		}
	}
	return nil, false
}

func (s *MockStore) GetGameBoxScore(id string) (*models.BoxScore, bool) {
	game, ok := s.GetGameByID(id)
	if !ok {
		game, ok = recentResultGame(s.GetRecentResults(), id)
	}
	if !ok {
		game, ok = worldCupGameByID(s.GetWorldCup(), id)
	}
	if !ok || (game.Status != models.StatusLive && game.Status != models.StatusFinal) {
		return nil, false
	}
	return &models.BoxScore{
		AwayTeam: game.AwayTeam,
		HomeTeam: game.HomeTeam,
		Sections: []models.BoxScoreSection{
			{
				Title:   "Team Stats",
				Columns: []string{game.AwayTeam.Abbr, game.HomeTeam.Abbr},
				Rows: []models.BoxScoreRow{
					{Label: "Shots", Values: []string{"27", "31"}},
					{Label: "Turnovers", Values: []string{"9", "7"}},
				},
			},
			{
				Title:   "Player Stats",
				Team:    game.HomeTeam,
				Columns: []string{"MIN", "PTS", "REB", "AST"},
				Rows: []models.BoxScoreRow{
					{Label: "Philadelphia Player", Values: []string{"32", "24", "8", "6"}},
				},
			},
		},
	}, true
}

func mockSoccerLineup() *models.BaseballLineup {
	miami := models.Team{ID: "20232", Name: "Inter Miami CF", Abbr: "MIA", Sport: models.MLS, Primary: "#231F20", Secondary: "#F7B5CD", LogoURL: "https://a.espncdn.com/i/teamlogos/soccer/500/20232.png"}
	return mockSoccerLineupFor(miami, Union)
}

func mockSoccerLineupFor(away, home models.Team) *models.BaseballLineup {
	return &models.BaseballLineup{
		AwayTeam: away,
		HomeTeam: home,
		Away: []models.BaseballLineupEntry{
			{Order: 1, Name: "Oscar Ustari", Position: "G"},
			{Order: 2, Name: "Ian Fray", Position: "RB"},
			{Order: 5, Name: "Sergio Busquets", Position: "DM"},
			{Order: 10, Name: "Lionel Messi", Position: "FW"},
		},
		Home: []models.BaseballLineupEntry{
			{Order: 18, Name: "Andre Blake", Position: "G"},
			{Order: 5, Name: "Jakob Glesnes", Position: "CB"},
			{Order: 8, Name: "Jovan Lukic", Position: "CM"},
			{Order: 7, Name: "Mikael Uhre", Position: "FW"},
		},
	}
}

func philliesMetsLineup() *models.BaseballLineup {
	return &models.BaseballLineup{
		AwayTeam:    Mets,
		HomeTeam:    Phillies,
		AwayPitcher: models.BaseballLineupPitcher{Name: "Kodai Senga", Handedness: "R", ERA: "3.02"},
		HomePitcher: models.BaseballLineupPitcher{Name: "Cristopher Sanchez", Handedness: "L", ERA: "3.32"},
		Away: []models.BaseballLineupEntry{
			{Order: 1, Name: "Francisco Lindor", Position: "SS", BattingAverage: ".271"},
			{Order: 2, Name: "Brandon Nimmo", Position: "CF", BattingAverage: ".224"},
			{Order: 3, Name: "Pete Alonso", Position: "1B", BattingAverage: ".266"},
			{Order: 4, Name: "Juan Soto", Position: "RF", BattingAverage: ".251"},
			{Order: 5, Name: "Mark Vientos", Position: "3B", BattingAverage: ".243"},
			{Order: 6, Name: "Jeff McNeil", Position: "2B", BattingAverage: ".237"},
			{Order: 7, Name: "Starling Marte", Position: "LF", BattingAverage: ".270"},
			{Order: 8, Name: "Luis Torrens", Position: "C", BattingAverage: ".229"},
			{Order: 9, Name: "Kodai Senga", Position: "P", BattingAverage: ".000"},
		},
		Home: []models.BaseballLineupEntry{
			{Order: 1, Name: "Trea Turner", Position: "SS", BattingAverage: ".289"},
			{Order: 2, Name: "Kyle Schwarber", Position: "DH", BattingAverage: ".248"},
			{Order: 3, Name: "Bryce Harper", Position: "1B", BattingAverage: ".276"},
			{Order: 4, Name: "Alec Bohm", Position: "3B", BattingAverage: ".280"},
			{Order: 5, Name: "Nick Castellanos", Position: "RF", BattingAverage: ".254"},
			{Order: 6, Name: "Brandon Marsh", Position: "LF", BattingAverage: ".249"},
			{Order: 7, Name: "J.T. Realmuto", Position: "C", BattingAverage: ".266"},
			{Order: 8, Name: "Bryson Stott", Position: "2B", BattingAverage: ".257"},
			{Order: 9, Name: "Cristopher Sanchez", Position: "P", BattingAverage: ".000"},
		},
	}
}
