package data

import "gametime/models"

// Store is the data access interface. Swap MockStore for a real API-backed
// implementation without changing any handler code.
type Store interface {
	GetTodaysGames() []models.Game
	GetUpcomingGames() []models.Game
	GetStandings() []models.StandingsRow
	GetRecentResults() []models.RecentResult
	GetTeams() []models.Team
	GetGameByID(id string) (*models.Game, bool)
}
