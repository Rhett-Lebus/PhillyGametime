package models

import "time"

type Sport string

const (
	NFL Sport = "NFL"
	MLB Sport = "MLB"
	NBA Sport = "NBA"
	NHL Sport = "NHL"
	MLS Sport = "MLS"
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
	ID        string
	HomeTeam  Team
	AwayTeam  Team
	HomeScore int
	AwayScore int
	Status    GameStatus
	Period    string // "Q3", "4th", "7th Inning", "P2", "90'"
	TimeLeft  string // "04:12"
	Baseball  *BaseballState
	StartTime time.Time
	Venue     string
	City      string
	Broadcast []string // primary is TV, secondary is radio
	Sport     Sport
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
}

type StandingsRow struct {
	Team     Team
	Record   string // overall e.g. "9-3" or "28-14-6" (NHL)
	Home     string // home W-L
	Away     string // away W-L
	HomeDiff int
	AwayDiff int
}

type RecentResult struct {
	Team     Team
	Opponent Team
	Home     bool
	Result   string    // "W", "L", "T"
	Record   string    // display string e.g. "W 6-4"
	Summary  string    // brief recap from provider or score data
	Bullets  []string  // short recap bullets derived from Summary
	GameDate time.Time // for sorting
}
