package models

import "time"

type Sport string

const (
	NFL  Sport = "NFL"
	MLB  Sport = "MLB"
	NBA  Sport = "NBA"
	NHL  Sport = "NHL"
	MLS  Sport = "MLS"
	FIFA Sport = "FIFA"
)

type Team struct {
	ID        string
	Name      string
	City      string
	Abbr      string
	Sport     Sport
	Primary   string // hex color
	Secondary string // hex color
	LogoURL   string
}

type GameStatus string

const (
	StatusScheduled GameStatus = "Scheduled"
	StatusLive      GameStatus = "Live"
	StatusFinal     GameStatus = "Final"
	StatusDelayed   GameStatus = "Delayed"
	StatusPostponed GameStatus = "Postponed"
	StatusCancelled GameStatus = "Cancelled"
)

type Game struct {
	ID          string
	HomeTeam    Team
	AwayTeam    Team
	HomeScore   int
	AwayScore   int
	Status      GameStatus
	Period      string // "Q3", "4th", "7th Inning", "P2", "90'"
	TimeLeft    string // "04:12"
	Baseball    *BaseballState
	Soccer      *SoccerState
	Lineup      *BaseballLineup
	Probable    *BaseballProbablePitchers
	StartTime   time.Time
	Venue       string
	City        string
	Broadcast   []string // primary is TV, secondary is radio
	Sport       Sport
	IsPreseason bool
}

type BoxScore struct {
	AwayTeam Team
	HomeTeam Team
	Sections []BoxScoreSection
}

type BoxScoreSection struct {
	Title   string
	Team    Team
	Columns []string
	Rows    []BoxScoreRow
}

type BoxScoreRow struct {
	Label  string
	Values []string
}

type TeamSchedule struct {
	Team  Team
	Games []Game
}

type BaseballState struct {
	OnFirst           bool
	OnSecond          bool
	OnThird           bool
	Outs              int
	Balls             int
	Strikes           int
	Batter            string
	Pitcher           string
	PitcherStrikeouts string
	CurrentPlay       string
	Plays             []BaseballPlay
}

type BaseballPlay struct {
	Inning      int
	HalfInning  string
	Description string
	Score       string
}

type BaseballLineup struct {
	AwayTeam    Team
	HomeTeam    Team
	AwayPitcher BaseballLineupPitcher
	HomePitcher BaseballLineupPitcher
	Away        []BaseballLineupEntry
	Home        []BaseballLineupEntry
}

type BaseballLineupEntry struct {
	Order          int
	Name           string
	Position       string
	BattingAverage string
}

type BaseballLineupPitcher struct {
	Name       string
	Handedness string
	ERA        string
}

type BaseballProbablePitchers struct {
	Away BaseballLineupPitcher
	Home BaseballLineupPitcher
}

type SoccerState struct {
	AwayStats SoccerTeamStats
	HomeStats SoccerTeamStats
	Lineup    *BaseballLineup
	Goals     []SoccerGoal
}

type SoccerGoal struct {
	Team    Team
	Scorer  string
	Assist  string
	Minute  string
	OwnGoal bool
}

type SoccerTeamStats struct {
	Shots         string
	ShotsOnTarget string
	Possession    string
	YellowCards   string
	RedCards      string
}

type StandingsRow struct {
	Team      Team
	Record    string // overall e.g. "9-3" or "28-14-6" (NHL)
	Home      string // home W-L
	Away      string // away W-L
	Rank      string // provider rank/seed when available
	GamesBack string // games behind, when provided by standings source
	HomeDiff  int
	AwayDiff  int
}

type LeagueStandings struct {
	Sport Sport
	Views []StandingsView
}

type StandingsView struct {
	Key   string
	Label string
	Scope string
	Rows  []StandingsRow
}

type RecentResult struct {
	GameID            string
	Team              Team
	Opponent          Team
	Home              bool
	Result            string           // "W", "L", "T"
	Record            string           // display string e.g. "W 6-4"
	Summary           string           // brief recap from provider or score data
	Bullets           []string         // short recap bullets derived from Summary
	Highlights        []VideoHighlight // official provider video links for this game
	HighlightsPending bool             // true when a recent final game may still publish highlights
	GameDate          time.Time        // for sorting
}

type VideoHighlight struct {
	Title       string
	Description string
	Thumbnail   string
	URL         string
	Provider    string
	PublishedAt time.Time
}

type WorldCup struct {
	Live         []WorldCupMatch
	Recent       []WorldCupMatch
	Upcoming     []WorldCupMatch
	Groups       []WorldCupGroup
	Bracket      []WorldCupRound
	LeftBracket  []WorldCupRound
	RightBracket []WorldCupRound
	Final        WorldCupMatch
	Watch        []WorldCupWatch
	Leaders      []WorldCupLeaderCategory
}

type WorldCupMatch struct {
	ID                string
	Stage             string
	HomeTeam          Team
	AwayTeam          Team
	HomeScore         int
	AwayScore         int
	Status            GameStatus
	Period            string
	TimeLeft          string
	StartTime         time.Time
	Venue             string
	City              string
	Broadcast         []string
	Summary           string
	Bullets           []string
	Highlights        []VideoHighlight
	HighlightsPending bool
	Soccer            *SoccerState
	Scenarios         []string
}

type WorldCupGroup struct {
	Name string
	Rows []WorldCupStanding
}

type WorldCupStanding struct {
	Team     Team
	Played   string
	Wins     string
	Draws    string
	Losses   string
	For      string
	Against  string
	Diff     string
	Points   string
	Note     string
	Rank     int
	Advanced bool
}

type WorldCupRound struct {
	Name    string
	Matches []WorldCupMatch
}

type WorldCupWatch struct {
	Label       string
	Description string
	Networks    []string
}

type WorldCupLeaderCategory struct {
	Name    string
	Kind    string
	Leaders []WorldCupLeader
}

type WorldCupLeader struct {
	Player       string
	Team         Team
	Value        int
	DisplayValue string
	Rank         int
	Headshot     string
}
