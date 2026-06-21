package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
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
	Name         string            `json:"name"`
	ShortName    string            `json:"shortName"`
	Date         espnTime          `json:"date"`
	Season       espnSeason        `json:"season"`
	Competitions []espnCompetition `json:"competitions"`
}

type espnSeason struct {
	Type int    `json:"type"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type espnCompetition struct {
	Venue       espnVenue        `json:"venue"`
	Competitors []espnCompetitor `json:"competitors"`
	Broadcasts  []espnBroadcast  `json:"broadcasts"`
	Status      espnStatus       `json:"status"`
	Situation   espnSituation    `json:"situation"`
	Headlines   []espnHeadline   `json:"headlines"`
}

type espnHeadline struct {
	ShortLinkText string `json:"shortLinkText"`
	Description   string `json:"description"`
}

type espnSituation struct {
	OnFirst  bool       `json:"onFirst"`
	OnSecond bool       `json:"onSecond"`
	OnThird  bool       `json:"onThird"`
	Outs     int        `json:"outs"`
	Balls    int        `json:"balls"`
	Strikes  int        `json:"strikes"`
	Batter   espnPlayer `json:"batter"`
	Pitcher  espnPlayer `json:"pitcher"`
}

type espnPlayer struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	ShortName   string `json:"shortName"`
	FullName    string `json:"fullName"`
	Name        string `json:"name"`
	Headshot    struct {
		Href string `json:"href"`
	} `json:"headshot"`
	Team    espnTeam `json:"team"`
	Athlete struct {
		DisplayName string `json:"displayName"`
		ShortName   string `json:"shortName"`
		FullName    string `json:"fullName"`
		Name        string `json:"name"`
	} `json:"athlete"`
}

type espnCompetitor struct {
	HomeAway string    `json:"homeAway"`
	Score    espnScore `json:"score"`
	Team     espnTeam  `json:"team"`
}

type espnScore string

func (s *espnScore) UnmarshalJSON(b []byte) error {
	raw := strings.TrimSpace(string(b))
	if raw == "" || raw == "null" {
		*s = ""
		return nil
	}
	if strings.HasPrefix(raw, `"`) {
		var value string
		if err := json.Unmarshal(b, &value); err != nil {
			return err
		}
		*s = espnScore(value)
		return nil
	}
	var obj struct {
		DisplayValue string  `json:"displayValue"`
		Value        float64 `json:"value"`
	}
	if err := json.Unmarshal(b, &obj); err == nil {
		if obj.DisplayValue != "" {
			*s = espnScore(obj.DisplayValue)
		} else {
			*s = espnScore(strconv.Itoa(int(obj.Value)))
		}
		return nil
	}
	var value float64
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	*s = espnScore(strconv.Itoa(int(value)))
	return nil
}

type espnTeam struct {
	ID               string `json:"id"`
	Location         string `json:"location"`
	Name             string `json:"name"`
	Nickname         string `json:"nickname"`
	DisplayName      string `json:"displayName"`
	ShortDisplayName string `json:"shortDisplayName"`
	Abbreviation     string `json:"abbreviation"`
	Color            string `json:"color"`
	AlternateColor   string `json:"alternateColor"`
	Logo             string `json:"logo"`
	Logos            []struct {
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
	State       string `json:"state"`
	Completed   bool   `json:"completed"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Detail      string `json:"detail"`
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
	Team espnTeam `json:"team"`
	Note struct {
		Description string `json:"description"`
	} `json:"note"`
	Stats []espnStat `json:"stats"`
}

type espnStat struct {
	Name         string  `json:"name"`
	Abbreviation string  `json:"abbreviation"`
	DisplayName  string  `json:"displayName"`
	ShortName    string  `json:"shortName"`
	Value        float64 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}

type espnSummaryResp struct {
	Boxscore  espnBoxscore      `json:"boxscore"`
	Videos    []espnVideo       `json:"videos"`
	Rosters   []espnRoster      `json:"rosters"`
	KeyEvents []espnSoccerEvent `json:"keyEvents"`
}

type espnSoccerEvent struct {
	Type struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"type"`
	ShortText   string `json:"shortText"`
	ScoringPlay bool   `json:"scoringPlay"`
	Shootout    bool   `json:"shootout"`
	Clock       struct {
		DisplayValue string `json:"displayValue"`
	} `json:"clock"`
	Team         espnTeam `json:"team"`
	Participants []struct {
		Athlete espnPlayer `json:"athlete"`
	} `json:"participants"`
}

type espnStatisticsResp struct {
	Stats []struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Leaders     []struct {
			Value   float64    `json:"value"`
			Athlete espnPlayer `json:"athlete"`
		} `json:"leaders"`
	} `json:"stats"`
}

type espnBoxscore struct {
	Players []espnBoxscoreTeam      `json:"players"`
	Teams   []espnBoxscoreTeamStats `json:"teams"`
}

type espnBoxscoreTeamStats struct {
	Team       espnTeam   `json:"team"`
	Statistics []espnStat `json:"statistics"`
	HomeAway   string     `json:"homeAway"`
}

type espnVideo struct {
	Headline    string `json:"headline"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	Links       struct {
		Web struct {
			Href string `json:"href"`
		} `json:"web"`
		Source struct {
			Href string `json:"href"`
		} `json:"source"`
	} `json:"links"`
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
}

type espnBoxscoreTeam struct {
	Team       espnTeam                `json:"team"`
	Statistics []espnBoxscoreStatGroup `json:"statistics"`
}

type espnBoxscoreStatGroup struct {
	Name        string                `json:"name"`
	DisplayName string                `json:"displayName"`
	Labels      []string              `json:"labels"`
	Names       []string              `json:"names"`
	Athletes    []espnBoxscoreAthlete `json:"athletes"`
}

type espnBoxscoreAthlete struct {
	Athlete espnPlayer `json:"athlete"`
	Stats   []string   `json:"stats"`
}

type espnRoster struct {
	HomeAway  string              `json:"homeAway"`
	Team      espnTeam            `json:"team"`
	Roster    []espnRosterAthlete `json:"roster"`
	Formation string              `json:"formation"`
}

type espnRosterAthlete struct {
	Active   bool       `json:"active"`
	Starter  bool       `json:"starter"`
	Jersey   string     `json:"jersey"`
	Athlete  espnPlayer `json:"athlete"`
	Position struct {
		Name         string `json:"name"`
		DisplayName  string `json:"displayName"`
		Abbreviation string `json:"abbreviation"`
	} `json:"position"`
	SubbedIn  bool `json:"subbedIn"`
	SubbedOut bool `json:"subbedOut"`
}

type mlbScheduleResp struct {
	Dates []struct {
		Games []mlbScheduleGame `json:"games"`
	} `json:"dates"`
}

type mlbScheduleGame struct {
	GamePk   int      `json:"gamePk"`
	GameDate espnTime `json:"gameDate"`
	Teams    struct {
		Away mlbScheduleTeam `json:"away"`
		Home mlbScheduleTeam `json:"home"`
	} `json:"teams"`
}

type mlbScheduleTeam struct {
	Team            mlbTeamRef `json:"team"`
	ProbablePitcher mlbPerson  `json:"probablePitcher"`
}

type mlbTeamRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type mlbContentResp struct {
	Highlights struct {
		Highlights struct {
			Items []mlbContentItem `json:"items"`
		} `json:"highlights"`
	} `json:"highlights"`
}

type mlbContentItem struct {
	Title       string   `json:"title"`
	Headline    string   `json:"headline"`
	Blurb       string   `json:"blurb"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Duration    string   `json:"duration"`
	Date        espnTime `json:"date"`
	Image       struct {
		Cuts []struct {
			Src string `json:"src"`
		} `json:"cuts"`
	} `json:"image"`
	Playbacks []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"playbacks"`
}

type mlbLiveFeedResp struct {
	LiveData struct {
		Plays struct {
			AllPlays    []mlbPlay `json:"allPlays"`
			CurrentPlay mlbPlay   `json:"currentPlay"`
		} `json:"plays"`
		Linescore struct {
			Inning     int    `json:"currentInning"`
			InningHalf string `json:"inningHalf"`
			Outs       int    `json:"outs"`
			Balls      int    `json:"balls"`
			Strikes    int    `json:"strikes"`
			Offense    struct {
				First  mlbPerson `json:"first"`
				Second mlbPerson `json:"second"`
				Third  mlbPerson `json:"third"`
				Batter mlbPerson `json:"batter"`
			} `json:"offense"`
			Defense struct {
				Pitcher mlbPerson `json:"pitcher"`
			} `json:"defense"`
			Innings []struct {
				Num  int `json:"num"`
				Away struct {
					Runs *int `json:"runs"`
				} `json:"away"`
				Home struct {
					Runs *int `json:"runs"`
				} `json:"home"`
			} `json:"innings"`
			Teams struct {
				Away mlbLineScoreTeam `json:"away"`
				Home mlbLineScoreTeam `json:"home"`
			} `json:"teams"`
		} `json:"linescore"`
		Boxscore struct {
			Teams struct {
				Away mlbBoxscoreTeam `json:"away"`
				Home mlbBoxscoreTeam `json:"home"`
			} `json:"teams"`
		} `json:"boxscore"`
	} `json:"liveData"`
}

type mlbLineScoreTeam struct {
	Runs       int `json:"runs"`
	Hits       int `json:"hits"`
	Errors     int `json:"errors"`
	LeftOnBase int `json:"leftOnBase"`
}

type mlbBoxscoreTeam struct {
	Batters      []int                        `json:"batters"`
	BattingOrder []int                        `json:"battingOrder"`
	Pitchers     []int                        `json:"pitchers"`
	Players      map[string]mlbBoxscorePlayer `json:"players"`
}

type mlbBoxscorePlayer struct {
	Person       mlbPerson `json:"person"`
	JerseyNumber string    `json:"jerseyNumber"`
	Position     struct {
		Abbreviation string `json:"abbreviation"`
		Name         string `json:"name"`
	} `json:"position"`
	PitchHand struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"pitchHand"`
	BattingOrder string                 `json:"battingOrder"`
	Stats        mlbBoxscorePlayerStats `json:"stats"`
	SeasonStats  mlbBoxscorePlayerStats `json:"seasonStats"`
	CareerStats  mlbBoxscorePlayerStats `json:"careerStats"`
}

type mlbBoxscorePlayerStats struct {
	Batting struct {
		AtBats      int    `json:"atBats"`
		Runs        int    `json:"runs"`
		Hits        int    `json:"hits"`
		RBI         int    `json:"rbi"`
		BaseOnBalls int    `json:"baseOnBalls"`
		StrikeOuts  int    `json:"strikeOuts"`
		HomeRuns    int    `json:"homeRuns"`
		Avg         string `json:"avg"`
	} `json:"batting"`
	Pitching struct {
		InningsPitched  string `json:"inningsPitched"`
		Hits            int    `json:"hits"`
		Runs            int    `json:"runs"`
		EarnedRuns      int    `json:"earnedRuns"`
		BaseOnBalls     int    `json:"baseOnBalls"`
		StrikeOuts      *int   `json:"strikeOuts"`
		HomeRuns        int    `json:"homeRuns"`
		NumberOfPitches int    `json:"numberOfPitches"`
		ERA             string `json:"era"`
	} `json:"pitching"`
}

type mlbPlay struct {
	About struct {
		Inning     int    `json:"inning"`
		HalfInning string `json:"halfInning"`
	} `json:"about"`
	Result struct {
		Description string `json:"description"`
		Event       string `json:"event"`
	} `json:"result"`
	Matchup struct {
		Batter  mlbPerson `json:"batter"`
		Pitcher mlbPerson `json:"pitcher"`
	} `json:"matchup"`
	Count struct {
		Balls   int `json:"balls"`
		Strikes int `json:"strikes"`
		Outs    int `json:"outs"`
	} `json:"count"`
}

type mlbPerson struct {
	ID        int    `json:"id"`
	FullName  string `json:"fullName"`
	PitchHand struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"pitchHand"`
}

type aiGameRecap struct {
	Bullets  []string `json:"bullets"`
	CachedAt string   `json:"cachedAt,omitempty"`
}

type gameRecapFacts struct {
	Sport              models.Sport
	PhillyTeam         models.Team
	Opponent           models.Team
	Home               bool
	PhillyScore        int
	OppScore           int
	Result             string
	GameDate           time.Time
	Venue              string
	City               string
	RawSummary         string
	HasProviderSummary bool
	NeutralMatch       bool
}

type openAIResponse struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type aiRecapCacheFile struct {
	Version int                    `json:"version"`
	Recaps  map[string]aiGameRecap `json:"recaps"`
}

type highlightCacheFile struct {
	Version int                             `json:"version"`
	Games   map[string]highlightsCacheEntry `json:"games"`
}

// ── Sport config ──────────────────────────────────────────────────────────────

type sportCfg struct {
	Sport         models.Sport
	ScoreboardURL string
	SummaryURL    string
	ScheduleBase  string
	StandingsURL  string
	PhillyTeamIDs []string
}

var sportConfigs = []sportCfg{
	{
		Sport:         models.NFL,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard",
		SummaryURL:    "https://site.web.api.espn.com/apis/site/v2/sports/football/nfl/summary?event=%s",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/football/nfl/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/football/nfl/standings",
		PhillyTeamIDs: []string{"21"},
	},
	{
		Sport:         models.NHL,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/hockey/nhl/scoreboard",
		SummaryURL:    "https://site.web.api.espn.com/apis/site/v2/sports/hockey/nhl/summary?event=%s",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/hockey/nhl/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/hockey/nhl/standings",
		PhillyTeamIDs: []string{"15"},
	},
	{
		Sport:         models.MLB,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/baseball/mlb/scoreboard",
		SummaryURL:    "https://site.web.api.espn.com/apis/site/v2/sports/baseball/mlb/summary?event=%s",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/baseball/mlb/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/baseball/mlb/standings",
		PhillyTeamIDs: []string{"22"},
	},
	{
		Sport:         models.NBA,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/basketball/nba/scoreboard",
		SummaryURL:    "https://site.web.api.espn.com/apis/site/v2/sports/basketball/nba/summary?event=%s",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/basketball/nba/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/basketball/nba/standings",
		PhillyTeamIDs: []string{"20"},
	},
	{
		Sport:         models.MLS,
		ScoreboardURL: "https://site.api.espn.com/apis/site/v2/sports/soccer/usa.1/scoreboard",
		SummaryURL:    "https://site.web.api.espn.com/apis/site/v2/sports/soccer/usa.1/summary?event=%s",
		ScheduleBase:  "https://site.api.espn.com/apis/site/v2/sports/soccer/usa.1/teams/",
		StandingsURL:  "https://site.api.espn.com/apis/v2/sports/soccer/usa.1/standings",
		PhillyTeamIDs: []string{"10739"},
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
	return isPhillyText(t.Location) ||
		isPhillyText(t.Name) ||
		isPhillyText(t.Nickname) ||
		isPhillyText(t.DisplayName) ||
		isPhillyText(t.ShortDisplayName) ||
		strings.EqualFold(t.Abbreviation, "PHI") ||
		strings.EqualFold(t.Abbreviation, "PHU")
}

func isPhillyGame(g models.Game) bool {
	return isPhillyTeam(g.HomeTeam) || isPhillyTeam(g.AwayTeam)
}

func isPhillyTeam(t models.Team) bool {
	return isPhillyText(t.City) ||
		isPhillyText(t.Name) ||
		strings.EqualFold(t.Abbr, "PHI") ||
		strings.EqualFold(t.Abbr, "PHU")
}

func isPhillyText(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	if phillyKeywords[s] {
		return true
	}
	return strings.Contains(s, "philadelphia") ||
		strings.Contains(s, "philly") ||
		strings.Contains(s, "chester") ||
		strings.Contains(s, "union")
}

// ── ESPN Store ────────────────────────────────────────────────────────────────

type gameCache struct {
	games     []models.Game
	expiresAt time.Time
}

type scheduleCache struct {
	schedules []models.TeamSchedule
	expiresAt time.Time
}

type standingsCache struct {
	rows      []models.StandingsRow
	expiresAt time.Time
}

type leagueStandingsCache struct {
	leagues   []models.LeagueStandings
	expiresAt time.Time
}

type worldCupCache struct {
	cup       models.WorldCup
	expiresAt time.Time
}

type worldCupLeadersCache struct {
	leaders   []models.WorldCupLeaderCategory
	expiresAt time.Time
}

type resultsCache struct {
	results   []models.RecentResult
	expiresAt time.Time
}

type lineupCacheEntry struct {
	lineup    *models.BaseballLineup
	expiresAt time.Time
}

type soccerStateCacheEntry struct {
	state     *models.SoccerState
	expiresAt time.Time
}

type highlightsCacheEntry struct {
	Highlights  []models.VideoHighlight
	Pending     bool
	CachedAt    time.Time
	NextFetchAt time.Time
	StopAfter   time.Time
}

const (
	highlightPendingRetry  = 15 * time.Minute
	highlightUpgradeRetry  = 45 * time.Minute
	highlightFoundTTL      = 24 * time.Hour
	highlightUpgradeWindow = 12 * time.Hour
	highlightStopAfter     = 48 * time.Hour
	lineupPrefetchWindow   = 105 * time.Minute
	todayLiveTTL           = 5 * time.Second
	todayIdleTTL           = 60 * time.Second
	todayErrorRetryTTL     = 5 * time.Second
)

type ESPNStore struct {
	client           *http.Client
	mu               sync.RWMutex
	todayCache       map[models.Sport]gameCache
	todayInFlight    map[models.Sport]chan struct{}
	upcomingCache    gameCache
	schedulesCache   scheduleCache
	standingsCache   standingsCache
	leagueCache      leagueStandingsCache
	worldCupCache    worldCupCache
	worldCupLeaders  worldCupLeadersCache
	resultsCache     resultsCache
	lineupCache      map[string]lineupCacheEntry
	soccerStateCache map[string]soccerStateCacheEntry
	aiRecapCache     map[string]aiGameRecap
	highlights       map[string]highlightsCacheEntry
	aiInFlight       map[string]bool
	aiCachePath      string
	highlightPath    string
}

var (
	mlbScheduleURL        = "https://statsapi.mlb.com/api/v1/schedule?sportId=1&date=%s&teamId=143&hydrate=team,probablePitcher"
	mlbLiveFeedURL        = "https://statsapi.mlb.com/api/v1.1/game/%d/feed/live"
	mlbContentURL         = "https://statsapi.mlb.com/api/v1/game/%d/content?highlightLimit=8"
	worldCupScoreboardURL = "https://site.api.espn.com/apis/site/v2/sports/soccer/fifa.world/scoreboard"
	worldCupStandingsURL  = "https://site.api.espn.com/apis/v2/sports/soccer/fifa.world/standings"
	worldCupSummaryURL    = "https://site.web.api.espn.com/apis/site/v2/sports/soccer/fifa.world/summary?event=%s"
	worldCupStatisticsURL = "https://site.web.api.espn.com/apis/site/v2/sports/soccer/fifa.world/statistics"
)

func NewESPNStore() *ESPNStore {
	store := &ESPNStore{
		client:           &http.Client{Timeout: 8 * time.Second},
		todayCache:       map[models.Sport]gameCache{},
		todayInFlight:    map[models.Sport]chan struct{}{},
		lineupCache:      map[string]lineupCacheEntry{},
		soccerStateCache: map[string]soccerStateCacheEntry{},
		aiRecapCache:     map[string]aiGameRecap{},
		highlights:       map[string]highlightsCacheEntry{},
		aiInFlight:       map[string]bool{},
		aiCachePath:      aiRecapCachePath(),
		highlightPath:    highlightCachePath(),
	}
	store.loadAIRecapCache()
	store.loadHighlightCache()
	return store
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
	var mu sync.Mutex
	games := make([]models.Game, 0)
	var wg sync.WaitGroup

	for _, cfg := range sportConfigs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			sportGames := s.getTodaySportGames(cfg)
			mu.Lock()
			games = append(games, sportGames...)
			mu.Unlock()
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

	return games
}

func (s *ESPNStore) getTodaySportGames(cfg sportCfg) []models.Game {
	for {
		now := time.Now()
		s.mu.Lock()
		cached := s.todayCache[cfg.Sport]
		if now.Before(cached.expiresAt) {
			s.mu.Unlock()
			return cached.games
		}
		if inFlight := s.todayInFlight[cfg.Sport]; inFlight != nil {
			s.mu.Unlock()
			<-inFlight
			continue
		}

		inFlight := make(chan struct{})
		s.todayInFlight[cfg.Sport] = inFlight
		s.mu.Unlock()

		games, err := s.fetchTodaySportGames(cfg)

		s.mu.Lock()
		if err == nil {
			s.todayCache[cfg.Sport] = gameCache{
				games:     games,
				expiresAt: time.Now().Add(todaySportTTL(games)),
			}
			cached = s.todayCache[cfg.Sport]
		} else {
			cached.expiresAt = time.Now().Add(todayErrorRetryTTL)
			s.todayCache[cfg.Sport] = cached
		}
		close(inFlight)
		delete(s.todayInFlight, cfg.Sport)
		s.mu.Unlock()
		return cached.games
	}
}

func (s *ESPNStore) fetchTodaySportGames(cfg sportCfg) ([]models.Game, error) {
	var sb espnScoreboard
	now := NowPhilly()
	if err := s.fetchJSON(cfg.ScoreboardURL+"?dates="+now.Format("20060102"), &sb); err != nil {
		return nil, err
	}

	todayY, todayM, todayD := now.Date()
	games := make([]models.Game, 0)
	for _, ev := range sb.Events {
		g, ok := parseESPNEvent(ev, cfg.Sport)
		if !ok || !isPhillyGame(g) {
			continue
		}
		gy, gm, gd := PhillyTime(g.StartTime).Date()
		if gy != todayY || gm != todayM || gd != todayD {
			continue
		}
		g = s.enrichMLBGame(g)
		if g.Status == models.StatusLive && g.Sport == models.MLS {
			g = s.enrichSoccerGame(g, cfg.SummaryURL)
		}
		if shouldPrefetchLineup(g, now) {
			if lineup, ok := s.cachedMLBLineup(g); ok {
				g.Lineup = lineup
			}
		}
		if g.Status == models.StatusLive && g.Baseball != nil && g.Baseball.Pitcher != "" && g.Baseball.PitcherStrikeouts == "" {
			g.Baseball.PitcherStrikeouts = s.fetchPitcherStrikeouts(g.ID, g.Baseball.Pitcher)
		}
		games = append(games, g)
	}
	return games, nil
}

func todaySportTTL(games []models.Game) time.Duration {
	for _, game := range games {
		if game.Status == models.StatusLive {
			return todayLiveTTL
		}
	}
	return todayIdleTTL
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

	// Phase 2: for teams with no game in the next 7 days, ask the scoreboard
	// for a wider range. ESPN's soccer team schedule can omit future fixtures,
	// while the date-range scoreboard still includes them.
	for _, cfg := range sportConfigs {
		cfg := cfg
		if !missingPhillyTeam(cfg, nextByKey) {
			continue
		}
		start := now.AddDate(0, 0, 1).Format("20060102")
		end := now.AddDate(1, 0, 0).Format("20060102")
		url := cfg.ScoreboardURL + "?dates=" + start + "-" + end + "&limit=1000"
		var sb espnScoreboard
		if err := s.fetchJSON(url, &sb); err != nil {
			continue
		}
		for _, ev := range sb.Events {
			g, ok := parseESPNEvent(ev, cfg.Sport)
			if !ok || !isPhillyGame(g) || !PhillyTime(g.StartTime).After(now) {
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
	}

	// Phase 3: for teams still missing, fall back to the full team schedule so
	// off-season teams (e.g. Eagles in May) still appear once their schedule is
	// published.
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
	s.attachUpcomingProbablePitchers(games)
	s.prefetchUpcomingLineups(games, now)

	s.mu.Lock()
	s.upcomingCache = gameCache{games: games, expiresAt: time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	return games
}

func (s *ESPNStore) GetFullSchedules() []models.TeamSchedule {
	s.mu.RLock()
	if time.Now().Before(s.schedulesCache.expiresAt) {
		schedules := s.schedulesCache.schedules
		s.mu.RUnlock()
		return schedules
	}
	s.mu.RUnlock()

	type result struct {
		index    int
		schedule models.TeamSchedule
	}

	results := make(chan result, len(sportConfigs))
	var wg sync.WaitGroup
	for i, cfg := range sportConfigs {
		i, cfg := i, cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- result{index: i, schedule: s.fetchTeamSchedule(cfg)}
		}()
	}
	wg.Wait()
	close(results)

	byIndex := map[int]models.TeamSchedule{}
	for res := range results {
		byIndex[res.index] = res.schedule
	}

	schedules := make([]models.TeamSchedule, 0, len(sportConfigs))
	for i := range sportConfigs {
		if sched, ok := byIndex[i]; ok {
			schedules = append(schedules, sched)
		}
	}

	s.mu.Lock()
	s.schedulesCache = scheduleCache{schedules: schedules, expiresAt: time.Now().Add(30 * time.Minute)}
	s.mu.Unlock()
	return schedules
}

func (s *ESPNStore) fetchTeamSchedule(cfg sportCfg) models.TeamSchedule {
	team := canonicalTeamForSport(cfg.Sport)
	seen := map[string]bool{}
	games := make([]models.Game, 0)
	now := NowPhilly()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	addGame := func(ev espnEvent, includePast bool) {
		g, ok := parseESPNEvent(ev, cfg.Sport)
		if !ok || !isPhillyGame(g) || seen[g.ID] {
			return
		}
		if !includePast && PhillyTime(g.StartTime).Before(startOfToday) {
			return
		}
		seen[g.ID] = true
		games = append(games, g)
	}

	for i, year := range scheduleSeasonYears(cfg.Sport, now) {
		includePast := i == 0
		for _, teamID := range cfg.PhillyTeamIDs {
			url := cfg.ScheduleBase + teamID + "/schedule?season=" + year
			var sched espnScheduleResp
			if err := s.fetchJSON(url, &sched); err != nil {
				continue
			}
			for _, ev := range sched.Events {
				addGame(ev, includePast)
			}
		}
	}

	start := now.Format("20060102")
	end := now.AddDate(1, 0, 0).Format("20060102")
	url := cfg.ScoreboardURL + "?dates=" + start + "-" + end + "&limit=1000"
	var sb espnScoreboard
	if err := s.fetchJSON(url, &sb); err == nil {
		for _, ev := range sb.Events {
			addGame(ev, false)
		}
	}

	sort.Slice(games, func(i, j int) bool {
		return games[i].StartTime.Before(games[j].StartTime)
	})
	if !hasCurrentOrFutureGame(games, startOfToday) {
		games = nil
	}
	return models.TeamSchedule{Team: team, Games: games}
}

func hasCurrentOrFutureGame(games []models.Game, startOfToday time.Time) bool {
	for _, game := range games {
		if !PhillyTime(game.StartTime).Before(startOfToday) {
			return true
		}
	}
	return false
}

func scheduleSeasonYears(sport models.Sport, now time.Time) []string {
	year := now.Year()
	switch sport {
	case models.NBA, models.NHL:
		if now.Month() >= time.July {
			year++
		}
		return []string{strconv.Itoa(year), strconv.Itoa(year + 1)}
	default:
		return []string{strconv.Itoa(year), strconv.Itoa(year + 1)}
	}
}

func canonicalTeamForSport(sport models.Sport) models.Team {
	switch sport {
	case models.NFL:
		return Eagles
	case models.MLB:
		return Phillies
	case models.NBA:
		return Sixers
	case models.NHL:
		return Flyers
	case models.MLS:
		return Union
	default:
		return models.Team{Sport: sport}
	}
}

func missingPhillyTeam(cfg sportCfg, nextByKey map[string]*models.Game) bool {
	for _, teamID := range cfg.PhillyTeamIDs {
		if nextByKey[string(cfg.Sport)+":"+teamID] == nil {
			return true
		}
	}
	return false
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
				if cfg.Sport == models.MLS {
					row = s.withSoccerHomeAwaySplits(row, cfg, entry.Team.ID)
				}
				mu.Lock()
				rows = append(rows, row)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool {
		return sportOrder(rows[i].Team.Sport) < sportOrder(rows[j].Team.Sport)
	})

	s.mu.Lock()
	s.standingsCache = standingsCache{rows: rows, expiresAt: time.Now().Add(1 * time.Hour)}
	s.mu.Unlock()
	return rows
}

func (s *ESPNStore) GetLeagueStandings() []models.LeagueStandings {
	s.mu.RLock()
	if time.Now().Before(s.leagueCache.expiresAt) {
		leagues := s.leagueCache.leagues
		s.mu.RUnlock()
		return leagues
	}
	s.mu.RUnlock()

	var mu sync.Mutex
	leagues := make([]models.LeagueStandings, 0, len(sportConfigs))
	var wg sync.WaitGroup

	for _, cfg := range sportConfigs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			var resp espnStandingsResp
			if err := s.fetchJSON(cfg.StandingsURL, &resp); err != nil {
				return
			}
			league := leagueStandingsFromResponse(cfg, resp)
			if len(league.Views) == 0 {
				return
			}
			mu.Lock()
			leagues = append(leagues, league)
			mu.Unlock()
		}()
	}
	wg.Wait()

	sort.Slice(leagues, func(i, j int) bool {
		return sportOrder(leagues[i].Sport) < sportOrder(leagues[j].Sport)
	})

	s.mu.Lock()
	s.leagueCache = leagueStandingsCache{leagues: leagues, expiresAt: time.Now().Add(1 * time.Hour)}
	s.mu.Unlock()
	return leagues
}

func (s *ESPNStore) withSoccerHomeAwaySplits(row models.StandingsRow, cfg sportCfg, teamID string) models.StandingsRow {
	if teamID == "" || cfg.ScheduleBase == "" {
		return row
	}

	year := NowPhilly().Format("2006")
	url := cfg.ScheduleBase + teamID + "/schedule?season=" + year
	var sched espnScheduleResp
	if err := s.fetchJSON(url, &sched); err != nil {
		return row
	}

	hw, hl, ht := 0, 0, 0
	aw, al, at := 0, 0, 0
	for _, ev := range sched.Events {
		if len(ev.Competitions) == 0 {
			continue
		}
		comp := ev.Competitions[0]
		if espnGameStatus(comp.Status) != models.StatusFinal {
			continue
		}

		var team, opponent espnCompetitor
		found := false
		for _, competitor := range comp.Competitors {
			if competitor.Team.ID == teamID {
				team = competitor
				found = true
			} else {
				opponent = competitor
			}
		}
		if !found {
			continue
		}

		teamScore, _ := strconv.Atoi(string(team.Score))
		oppScore, _ := strconv.Atoi(string(opponent.Score))
		home := team.HomeAway == "home"
		switch {
		case teamScore > oppScore:
			if home {
				hw++
			} else {
				aw++
			}
		case teamScore < oppScore:
			if home {
				hl++
			} else {
				al++
			}
		default:
			if home {
				ht++
			} else {
				at++
			}
		}
	}

	if hw+hl+ht > 0 {
		row.Home = fmt.Sprintf("%d-%d-%d", hw, hl, ht)
		row.HomeDiff = hw - hl
	}
	if aw+al+at > 0 {
		row.Away = fmt.Sprintf("%d-%d-%d", aw, al, at)
		row.AwayDiff = aw - al
	}
	return row
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

func leagueStandingsFromResponse(cfg sportCfg, resp espnStandingsResp) models.LeagueStandings {
	allEntries := uniqueStandingsEntries(flattenStandingsEntries(resp))
	allRows := standingsRowsFromEntries(allEntries, cfg.Sport, standingsSortOverall)
	if len(allEntries) == 0 || len(allRows) == 0 {
		return models.LeagueStandings{}
	}

	phillyPath := phillyStandingsGroupPath(resp.Children, cfg.PhillyTeamIDs)
	views := make([]models.StandingsView, 0, 3)
	addView := func(key, scope, label string, entries []espnStandingsEntry) {
		rows := standingsRowsFromEntries(uniqueStandingsEntries(entries), cfg.Sport, standingsSortGroup)
		appendStandingsView(&views, key, scope, label, rows)
	}

	if division := phillyDivisionStandings(cfg.Sport, allEntries); len(division.entries) > 0 {
		rows := standingsRowsFromEntries(division.entries, cfg.Sport, standingsSortOverall)
		appendStandingsView(&views, "division", "Division", division.label, rows)
	}
	if len(phillyPath) > 0 {
		deepest := phillyPath[len(phillyPath)-1]
		deepestKey, deepestScope := standingsScopeForGroup(deepest.Name, len(phillyPath) > 1)
		addView(deepestKey, deepestScope, deepest.Name, collectStandingsGroupEntries(deepest))
	}
	if len(phillyPath) > 1 {
		parent := phillyPath[0]
		addView("conference", standingsScopeLabel(parent.Name, "Conference"), parent.Name, collectStandingsGroupEntries(parent))
	}
	appendStandingsView(&views, "overall", "Overall", string(cfg.Sport), allRows)

	return models.LeagueStandings{
		Sport: cfg.Sport,
		Views: views,
	}
}

func appendStandingsView(views *[]models.StandingsView, key, scope, label string, rows []models.StandingsRow) {
	if len(rows) == 0 || standingsViewExists(*views, key, label, rows) {
		return
	}
	*views = append(*views, models.StandingsView{
		Key:   standingsViewKey(key, label),
		Label: label,
		Scope: scope,
		Rows:  rows,
	})
}

type divisionStandingsEntries struct {
	label   string
	entries []espnStandingsEntry
}

func phillyDivisionStandings(sport models.Sport, entries []espnStandingsEntry) divisionStandingsEntries {
	label, teamIDs := phillyDivisionTeamIDs(sport)
	if label == "" || len(teamIDs) == 0 {
		return divisionStandingsEntries{}
	}
	keep := map[string]bool{}
	for _, id := range teamIDs {
		keep[id] = true
	}
	out := make([]espnStandingsEntry, 0, len(teamIDs))
	for _, entry := range entries {
		if keep[entry.Team.ID] {
			out = append(out, entry)
		}
	}
	return divisionStandingsEntries{label: label, entries: out}
}

func phillyDivisionTeamIDs(sport models.Sport) (string, []string) {
	switch sport {
	case models.NFL:
		return "NFC East", []string{"6", "19", "21", "28"}
	case models.MLB:
		return "NL East", []string{"15", "20", "21", "22", "28"}
	case models.NBA:
		return "Atlantic", []string{"2", "17", "18", "20", "28"}
	case models.NHL:
		return "Metropolitan", []string{"7", "11", "12", "13", "15", "16", "23", "29"}
	default:
		return "", nil
	}
}

func standingsViewKey(scope, label string) string {
	parts := []string{scope}
	for _, part := range strings.FieldsFunc(strings.ToLower(label), func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "-")
}

func phillyStandingsGroupPath(groups []espnStandingsGroup, phillyTeamIDs []string) []espnStandingsGroup {
	for _, group := range groups {
		if !standingsEntriesContainTeam(collectStandingsGroupEntries(group), phillyTeamIDs) {
			continue
		}
		childPath := phillyStandingsGroupPath(group.Children, phillyTeamIDs)
		return append([]espnStandingsGroup{group}, childPath...)
	}
	return nil
}

func collectStandingsGroupEntries(group espnStandingsGroup) []espnStandingsEntry {
	entries := append([]espnStandingsEntry(nil), group.Standings.Entries...)
	for _, child := range group.Children {
		entries = append(entries, collectStandingsGroupEntries(child)...)
	}
	return entries
}

func standingsEntriesContainTeam(entries []espnStandingsEntry, teamIDs []string) bool {
	for _, entry := range entries {
		for _, teamID := range teamIDs {
			if entry.Team.ID == teamID {
				return true
			}
		}
		if isPhillyESPN(entry.Team) {
			return true
		}
	}
	return false
}

type standingsSortMode int

const (
	standingsSortGroup standingsSortMode = iota
	standingsSortOverall
)

func standingsRowsFromEntries(entries []espnStandingsEntry, sport models.Sport, mode standingsSortMode) []models.StandingsRow {
	entries = append([]espnStandingsEntry(nil), entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if mode == standingsSortOverall {
			return standingsEntryOverallLess(entries[i], entries[j], sport)
		}
		return standingsEntryRankLess(entries[i], entries[j])
	})

	rows := make([]models.StandingsRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, standingsEntryToRow(entry, sport))
	}
	if mode == standingsSortOverall {
		for i := range rows {
			rows[i].Rank = strconv.Itoa(i + 1)
		}
	}
	return rows
}

func standingsEntryRankLess(a, b espnStandingsEntry) bool {
	av, aok := standingsRank(a)
	bv, bok := standingsRank(b)
	if aok && bok && av != bv {
		return av < bv
	}
	return false
}

func standingsRank(entry espnStandingsEntry) (float64, bool) {
	return standingsStat(entry, "playoffSeed", "rank", "seed", "position")
}

func standingsEntryOverallLess(a, b espnStandingsEntry, sport models.Sport) bool {
	if sport == models.NHL || sport == models.MLS {
		if av, bv := standingsValue(a, "points"), standingsValue(b, "points"); av != bv {
			return av > bv
		}
	}
	if av, bv := standingsWinPercent(a), standingsWinPercent(b); av != bv {
		return av > bv
	}
	if av, bv := standingsValue(a, "wins"), standingsValue(b, "wins"); av != bv {
		return av > bv
	}
	if av, bv := standingsValue(a, "losses"), standingsValue(b, "losses"); av != bv {
		return av < bv
	}
	if av, bv := standingsValue(a, "ties", "draws"), standingsValue(b, "ties", "draws"); av != bv {
		return av > bv
	}
	if av, bv := standingsValue(a, "pointDifferential", "differential", "pointsDifferential", "goalDifferential"), standingsValue(b, "pointDifferential", "differential", "pointsDifferential", "goalDifferential"); av != bv {
		return av > bv
	}
	return strings.Compare(standingsTeamSortName(a.Team), standingsTeamSortName(b.Team)) < 0
}

func standingsWinPercent(entry espnStandingsEntry) float64 {
	if value, ok := standingsStat(entry, "winPercent", "winningPercentage", "pct"); ok {
		return value
	}
	wins := standingsValue(entry, "wins")
	losses := standingsValue(entry, "losses")
	ties := standingsValue(entry, "ties", "draws")
	total := wins + losses + ties
	if total == 0 {
		return 0
	}
	return (wins + ties*0.5) / total
}

func standingsValue(entry espnStandingsEntry, names ...string) float64 {
	value, _ := standingsStat(entry, names...)
	return value
}

func standingsStat(entry espnStandingsEntry, names ...string) (float64, bool) {
	for _, name := range names {
		for _, stat := range entry.Stats {
			if strings.EqualFold(stat.Name, name) ||
				strings.EqualFold(stat.Abbreviation, name) ||
				strings.EqualFold(stat.DisplayName, name) ||
				strings.EqualFold(stat.ShortName, name) {
				return stat.Value, true
			}
		}
	}
	return 0, false
}

func standingsTeamSortName(team espnTeam) string {
	return strings.TrimSpace(team.Location + " " + firstNonEmpty(team.Name, team.Nickname, team.DisplayName))
}

func uniqueStandingsEntries(entries []espnStandingsEntry) []espnStandingsEntry {
	seen := map[string]bool{}
	out := make([]espnStandingsEntry, 0, len(entries))
	for _, entry := range entries {
		key := entry.Team.ID
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(entry.Team.Location + ":" + entry.Team.Name + ":" + entry.Team.Nickname))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out
}

func standingsScopeForGroup(name string, hasParent bool) (key, scope string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "conference"):
		return "conference", standingsScopeLabel(name, "Conference")
	case strings.Contains(lower, "league"):
		return "league", standingsScopeLabel(name, "League")
	case hasParent:
		return "division", standingsScopeLabel(name, "Division")
	default:
		return "division", standingsScopeLabel(name, "Division")
	}
}

func standingsScopeLabel(name, fallback string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "conference"):
		return "Conference"
	case strings.Contains(lower, "division"):
		return "Division"
	case strings.Contains(lower, "league"):
		return "League"
	default:
		return fallback
	}
}

func (s *ESPNStore) GetWorldCup() models.WorldCup {
	s.mu.RLock()
	if time.Now().Before(s.worldCupCache.expiresAt) {
		cup := s.worldCupCache.cup
		s.mu.RUnlock()
		return cup
	}
	s.mu.RUnlock()

	now := NowPhilly()
	cup := models.WorldCup{
		Groups:  s.fetchWorldCupGroups(),
		Bracket: s.fetchWorldCupBracket(),
		Watch:   worldCupWatchInfo(),
		Leaders: s.fetchWorldCupLeaders(),
	}
	applyWorldCupBracketLayout(&cup)

	recentCutoff := now.Add(-36 * time.Hour)
	start := recentCutoff.Format("20060102")
	end := now.AddDate(0, 0, 10).Format("20060102")
	matches := s.fetchWorldCupMatches(start + "-" + end)
	for _, match := range matches {
		if match.Status == models.StatusLive {
			match.Soccer = s.cachedSoccerState(match.ID, worldCupSummaryURL, match.AwayTeam, match.HomeTeam, 10*time.Second)
		} else if match.Status == models.StatusFinal && !PhillyTime(match.StartTime).Before(recentCutoff) {
			match.Soccer = s.cachedSoccerState(match.ID, worldCupSummaryURL, match.AwayTeam, match.HomeTeam, 12*time.Hour)
		} else if shouldPrefetchWorldCupLineup(match, now) {
			if lineup, ok := s.cachedSoccerLineup(match.ID, worldCupSummaryURL, match.AwayTeam, match.HomeTeam); ok {
				match.Soccer = &models.SoccerState{Lineup: lineup}
			}
		}
		switch {
		case match.Status == models.StatusLive:
			cup.Live = append(cup.Live, match)
		case match.Status == models.StatusFinal && !PhillyTime(match.StartTime).Before(recentCutoff):
			match = s.enhanceWorldCupRecentMatch(match)
			cup.Recent = append(cup.Recent, match)
		case match.Status == models.StatusScheduled && !PhillyTime(match.StartTime).Before(now):
			cup.Upcoming = append(cup.Upcoming, match)
		}
	}
	groupMatches := s.fetchWorldCupMatches("20260611-20260627")
	applyWorldCupMatchScenarios(cup.Live, cup.Groups, groupMatches)
	applyWorldCupMatchScenarios(cup.Upcoming, cup.Groups, groupMatches)
	sort.Slice(cup.Live, func(i, j int) bool {
		return cup.Live[i].StartTime.Before(cup.Live[j].StartTime)
	})
	sort.Slice(cup.Recent, func(i, j int) bool {
		return cup.Recent[i].StartTime.After(cup.Recent[j].StartTime)
	})
	sort.Slice(cup.Upcoming, func(i, j int) bool {
		return cup.Upcoming[i].StartTime.Before(cup.Upcoming[j].StartTime)
	})
	if len(cup.Recent) > 12 {
		cup.Recent = cup.Recent[:12]
	}
	if len(cup.Upcoming) > 12 {
		cup.Upcoming = cup.Upcoming[:12]
	}

	ttl := 2 * time.Minute
	if len(cup.Live) > 0 || hasWorldCupMatchNearLiveWindow(matches, now) {
		ttl = 15 * time.Second
	}
	s.mu.Lock()
	s.worldCupCache = worldCupCache{cup: cup, expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	return cup
}

func applyWorldCupMatchScenarios(matches []models.WorldCupMatch, groups []models.WorldCupGroup, groupMatches []models.WorldCupMatch) {
	for i := range matches {
		if !strings.EqualFold(worldCupStageLabel(matches[i].Stage), "Group Stage") {
			continue
		}
		group, ok := worldCupGroupForMatch(groups, matches[i])
		if !ok {
			continue
		}
		for _, team := range []models.Team{matches[i].AwayTeam, matches[i].HomeTeam} {
			row, ok := worldCupStandingForTeam(group.Rows, team)
			if !ok {
				continue
			}
			if !row.Advanced && !worldCupStandingEliminated(row) &&
				worldCupScenarioGuaranteed(group, groupMatches, matches[i], team, true, true) {
				matches[i].Scenarios = append(matches[i].Scenarios, fmt.Sprintf("%s qualifies with a win.", team.Name))
			}
			if !worldCupStandingEliminated(row) &&
				worldCupScenarioGuaranteed(group, groupMatches, matches[i], team, false, false) {
				matches[i].Scenarios = append(matches[i].Scenarios, fmt.Sprintf("%s is eliminated with a loss.", team.Name))
			}
		}
	}
}

func worldCupGroupForMatch(groups []models.WorldCupGroup, match models.WorldCupMatch) (models.WorldCupGroup, bool) {
	for _, group := range groups {
		_, awayOK := worldCupStandingForTeam(group.Rows, match.AwayTeam)
		_, homeOK := worldCupStandingForTeam(group.Rows, match.HomeTeam)
		if awayOK && homeOK {
			return group, true
		}
	}
	return models.WorldCupGroup{}, false
}

func worldCupStandingForTeam(rows []models.WorldCupStanding, team models.Team) (models.WorldCupStanding, bool) {
	key := worldCupTeamKey(team)
	for _, row := range rows {
		if worldCupTeamKey(row.Team) == key {
			return row, true
		}
	}
	return models.WorldCupStanding{}, false
}

func worldCupStandingEliminated(row models.WorldCupStanding) bool {
	return strings.Contains(strings.ToLower(row.Note), "eliminated")
}

func worldCupScenarioGuaranteed(
	group models.WorldCupGroup,
	allMatches []models.WorldCupMatch,
	target models.WorldCupMatch,
	team models.Team,
	forceWin bool,
	requireTopTwo bool,
) bool {
	points := make(map[string]int, len(group.Rows))
	played := make(map[string]int, len(group.Rows))
	groupTeams := make(map[string]bool, len(group.Rows))
	for _, row := range group.Rows {
		key := worldCupTeamKey(row.Team)
		groupTeams[key] = true
		points[key] = standingInt(row.Points)
		played[key] = standingInt(row.Played)
	}

	groupSchedule := make([]models.WorldCupMatch, 0, 6)
	finalCount := make(map[string]int, len(group.Rows))
	for _, match := range allMatches {
		awayKey := worldCupTeamKey(match.AwayTeam)
		homeKey := worldCupTeamKey(match.HomeTeam)
		if !groupTeams[awayKey] || !groupTeams[homeKey] ||
			!strings.EqualFold(worldCupStageLabel(match.Stage), "Group Stage") {
			continue
		}
		groupSchedule = append(groupSchedule, match)
		if match.Status == models.StatusFinal {
			finalCount[awayKey]++
			finalCount[homeKey]++
		}
	}

	// ESPN may include a live match's provisional result in its standings.
	// Remove it so every unfinished match can be simulated from a stable base.
	for _, match := range groupSchedule {
		if match.Status != models.StatusLive {
			continue
		}
		awayKey := worldCupTeamKey(match.AwayTeam)
		homeKey := worldCupTeamKey(match.HomeTeam)
		if played[awayKey] > finalCount[awayKey] && played[homeKey] > finalCount[homeKey] {
			awayPoints, homePoints := worldCupResultPoints(match.AwayScore, match.HomeScore)
			points[awayKey] -= awayPoints
			points[homeKey] -= homePoints
		}
	}

	remaining := make([]models.WorldCupMatch, 0, len(groupSchedule))
	for _, match := range groupSchedule {
		if match.Status != models.StatusFinal {
			remaining = append(remaining, match)
		}
	}
	if len(remaining) == 0 {
		return false
	}

	teamKey := worldCupTeamKey(team)
	targetFound := false
	allPass := true
	var simulate func(int)
	simulate = func(index int) {
		if !allPass {
			return
		}
		if index == len(remaining) {
			ahead := 0
			for otherKey, otherPoints := range points {
				if otherKey == teamKey {
					continue
				}
				if requireTopTwo {
					if otherPoints >= points[teamKey] {
						ahead++
					}
				} else if otherPoints > points[teamKey] {
					ahead++
				}
			}
			if (requireTopTwo && ahead > 1) || (!requireTopTwo && ahead < 3) {
				allPass = false
			}
			return
		}

		match := remaining[index]
		awayKey := worldCupTeamKey(match.AwayTeam)
		homeKey := worldCupTeamKey(match.HomeTeam)
		outcomes := [][2]int{{3, 0}, {1, 1}, {0, 3}}
		if match.ID == target.ID {
			targetFound = true
			teamIsAway := awayKey == teamKey
			if forceWin == teamIsAway {
				outcomes = [][2]int{{3, 0}}
			} else {
				outcomes = [][2]int{{0, 3}}
			}
		}
		for _, outcome := range outcomes {
			points[awayKey] += outcome[0]
			points[homeKey] += outcome[1]
			simulate(index + 1)
			points[awayKey] -= outcome[0]
			points[homeKey] -= outcome[1]
		}
	}
	simulate(0)
	return targetFound && allPass
}

func worldCupResultPoints(awayScore, homeScore int) (int, int) {
	switch {
	case awayScore > homeScore:
		return 3, 0
	case homeScore > awayScore:
		return 0, 3
	default:
		return 1, 1
	}
}

func worldCupTeamKey(team models.Team) string {
	if id := strings.TrimSpace(team.ID); id != "" {
		return "id:" + strings.ToLower(id)
	}
	return "name:" + worldCupStandingTeamName(team)
}

func hasWorldCupMatchNearLiveWindow(matches []models.WorldCupMatch, now time.Time) bool {
	for _, match := range matches {
		start := PhillyTime(match.StartTime)
		if now.After(start.Add(-30*time.Minute)) && now.Before(start.Add(3*time.Hour)) {
			return true
		}
	}
	return false
}

func (s *ESPNStore) fetchWorldCupMatches(dates string) []models.WorldCupMatch {
	var sb espnScoreboard
	url := worldCupScoreboardURL + "?dates=" + dates + "&limit=1000"
	if err := s.fetchJSON(url, &sb); err != nil {
		return nil
	}
	matches := make([]models.WorldCupMatch, 0, len(sb.Events))
	for _, ev := range sb.Events {
		if match, ok := worldCupMatchFromEvent(ev); ok {
			matches = append(matches, match)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].StartTime.Before(matches[j].StartTime)
	})
	return matches
}

func (s *ESPNStore) fetchWorldCupBracket() []models.WorldCupRound {
	matches := s.fetchWorldCupMatches("20260628-20260719")
	roundOrder := []string{"Round of 32", "Round of 16", "Quarterfinals", "Semifinals", "3rd-Place Match", "Final"}
	roundsByName := map[string][]models.WorldCupMatch{}
	for _, match := range matches {
		name := worldCupStageLabel(match.Stage)
		if strings.EqualFold(name, "Group Stage") || name == "" {
			continue
		}
		roundsByName[name] = append(roundsByName[name], match)
	}
	rounds := make([]models.WorldCupRound, 0, len(roundOrder))
	for _, name := range roundOrder {
		if matches := roundsByName[name]; len(matches) > 0 {
			rounds = append(rounds, models.WorldCupRound{Name: name, Matches: matches})
		}
	}
	return rounds
}

func (s *ESPNStore) fetchWorldCupGroups() []models.WorldCupGroup {
	var resp espnStandingsResp
	if err := s.fetchJSON(worldCupStandingsURL, &resp); err != nil {
		return nil
	}
	groups := make([]models.WorldCupGroup, 0, len(resp.Children))
	for _, child := range resp.Children {
		if len(child.Standings.Entries) == 0 {
			continue
		}
		rows := make([]models.WorldCupStanding, 0, len(child.Standings.Entries))
		for _, entry := range child.Standings.Entries {
			rows = append(rows, worldCupStandingFromEntry(entry))
		}
		sortWorldCupStandings(rows)
		groups = append(groups, models.WorldCupGroup{Name: child.Name, Rows: rows})
	}
	return groups
}

func (s *ESPNStore) fetchWorldCupLeaders() []models.WorldCupLeaderCategory {
	now := time.Now()
	s.mu.RLock()
	if now.Before(s.worldCupLeaders.expiresAt) {
		leaders := s.worldCupLeaders.leaders
		s.mu.RUnlock()
		return leaders
	}
	s.mu.RUnlock()

	var resp espnStatisticsResp
	if err := s.fetchJSON(worldCupStatisticsURL, &resp); err != nil {
		return nil
	}
	categories := make([]models.WorldCupLeaderCategory, 0, 10)
	playerCategories := make(map[string][]models.WorldCupLeader, 2)
	for _, category := range resp.Stats {
		if category.Name != "goalsLeaders" && category.Name != "assistsLeaders" {
			continue
		}
		leaders := make([]models.WorldCupLeader, 0, 5)
		for _, leader := range category.Leaders {
			if len(leaders) == 5 {
				break
			}
			player := espnPlayerName(leader.Athlete)
			if player == "" {
				continue
			}
			leaders = append(leaders, models.WorldCupLeader{
				Player:   player,
				Team:     espnToTeam(leader.Athlete.Team, models.FIFA),
				Value:    int(leader.Value),
				Headshot: strings.TrimSpace(leader.Athlete.Headshot.Href),
			})
		}
		applyWorldCupLeaderRanks(leaders)
		if len(leaders) > 0 {
			playerCategories[category.Name] = leaders
			categories = append(categories, models.WorldCupLeaderCategory{
				Name:    firstNonEmpty(category.DisplayName, category.Name),
				Kind:    "player",
				Leaders: leaders,
			})
		}
	}
	if contributions := worldCupGoalContributions(playerCategories["goalsLeaders"], playerCategories["assistsLeaders"]); len(contributions) > 0 {
		categories = append(categories, models.WorldCupLeaderCategory{
			Name:    "Goal Contributions",
			Kind:    "player",
			Leaders: contributions,
		})
	}
	matches := s.fetchWorldCupMatches("20260611-" + NowPhilly().Format("20060102"))
	categories = append(categories, s.fetchWorldCupTeamLeaderCategories(matches)...)
	s.mu.Lock()
	s.worldCupLeaders = worldCupLeadersCache{
		leaders:   categories,
		expiresAt: now.Add(30 * time.Minute),
	}
	s.mu.Unlock()
	return categories
}

func worldCupGoalContributions(goals, assists []models.WorldCupLeader) []models.WorldCupLeader {
	combined := map[string]models.WorldCupLeader{}
	add := func(leaders []models.WorldCupLeader) {
		for _, leader := range leaders {
			key := normalizePlayerName(leader.Player) + ":" + worldCupTeamKey(leader.Team)
			current := combined[key]
			if current.Player == "" {
				current = leader
				current.Rank = 0
				current.Value = 0
			}
			current.Value += leader.Value
			combined[key] = current
		}
	}
	add(goals)
	add(assists)

	leaders := make([]models.WorldCupLeader, 0, len(combined))
	for _, leader := range combined {
		leaders = append(leaders, leader)
	}
	sort.SliceStable(leaders, func(i, j int) bool {
		if leaders[i].Value != leaders[j].Value {
			return leaders[i].Value > leaders[j].Value
		}
		return leaders[i].Player < leaders[j].Player
	})
	if len(leaders) > 5 {
		leaders = leaders[:5]
	}
	applyWorldCupLeaderRanks(leaders)
	return leaders
}

type worldCupTeamTotals struct {
	team            models.Team
	games           int
	goals           int
	goalsConceded   int
	cleanSheets     int
	shotsOnTarget   int
	saves           int
	yellowCards     int
	redCards        int
	possessionTotal float64
	possessionGames int
	accuratePasses  float64
	totalPasses     float64
}

func (s *ESPNStore) fetchWorldCupTeamLeaderCategories(matches []models.WorldCupMatch) []models.WorldCupLeaderCategory {
	totals := map[string]*worldCupTeamTotals{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	limit := make(chan struct{}, 6)

	for _, match := range matches {
		if match.Status != models.StatusFinal && match.Status != models.StatusLive {
			continue
		}
		addWorldCupScoreTotals(totals, match)
	}
	for _, match := range matches {
		if match.Status != models.StatusFinal && match.Status != models.StatusLive {
			continue
		}
		match := match
		wg.Add(1)
		go func() {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()

			var summary espnSummaryResp
			if err := s.fetchJSON(fmt.Sprintf(worldCupSummaryURL, match.ID), &summary); err != nil {
				return
			}
			mu.Lock()
			addWorldCupSummaryTotals(totals, match, summary.Boxscore.Teams)
			mu.Unlock()
		}()
	}
	wg.Wait()

	discipline := worldCupTeamLeaderCategory("Most Yellow Cards", totals, func(total *worldCupTeamTotals) (int, string) {
		return total.yellowCards, strconv.Itoa(total.yellowCards)
	})
	discipline.Kind = "discipline"
	redCards := worldCupTeamLeaderCategory("Most Red Cards", totals, func(total *worldCupTeamTotals) (int, string) {
		return total.redCards, strconv.Itoa(total.redCards)
	})
	redCards.Kind = "discipline"
	all := []models.WorldCupLeaderCategory{
		worldCupTeamLeaderCategory("Team Goals", totals, func(total *worldCupTeamTotals) (int, string) {
			return total.goals, strconv.Itoa(total.goals)
		}),
		worldCupTeamLeaderCategory("Clean Sheets", totals, func(total *worldCupTeamTotals) (int, string) {
			return total.cleanSheets, strconv.Itoa(total.cleanSheets)
		}),
		worldCupTeamLeaderCategory("Shots on Target", totals, func(total *worldCupTeamTotals) (int, string) {
			return total.shotsOnTarget, strconv.Itoa(total.shotsOnTarget)
		}),
		worldCupTeamLeaderCategory("Average Possession", totals, func(total *worldCupTeamTotals) (int, string) {
			if total.possessionGames == 0 {
				return -1, ""
			}
			value := int(math.Round(total.possessionTotal / float64(total.possessionGames)))
			return value, strconv.Itoa(value) + "%"
		}),
		worldCupTeamLeaderCategory("Pass Accuracy", totals, func(total *worldCupTeamTotals) (int, string) {
			if total.totalPasses == 0 {
				return -1, ""
			}
			value := int(math.Round(total.accuratePasses / total.totalPasses * 100))
			return value, strconv.Itoa(value) + "%"
		}),
		worldCupTeamLeaderCategory("Saves", totals, func(total *worldCupTeamTotals) (int, string) {
			return total.saves, strconv.Itoa(total.saves)
		}),
		discipline,
		redCards,
	}
	categories := make([]models.WorldCupLeaderCategory, 0, len(all))
	for _, category := range all {
		if len(category.Leaders) > 0 {
			categories = append(categories, category)
		}
	}
	return categories
}

func addWorldCupScoreTotals(totals map[string]*worldCupTeamTotals, match models.WorldCupMatch) {
	away := worldCupTeamTotal(totals, match.AwayTeam)
	home := worldCupTeamTotal(totals, match.HomeTeam)
	away.games++
	home.games++
	away.goals += match.AwayScore
	away.goalsConceded += match.HomeScore
	home.goals += match.HomeScore
	home.goalsConceded += match.AwayScore
	if match.Status == models.StatusFinal {
		if match.HomeScore == 0 {
			away.cleanSheets++
		}
		if match.AwayScore == 0 {
			home.cleanSheets++
		}
	}
}

func addWorldCupSummaryTotals(totals map[string]*worldCupTeamTotals, match models.WorldCupMatch, teams []espnBoxscoreTeamStats) {
	for _, boxscoreTeam := range teams {
		team := match.AwayTeam
		if strings.EqualFold(boxscoreTeam.HomeAway, "home") || sameProviderTeam(boxscoreTeam.Team, match.HomeTeam) {
			team = match.HomeTeam
		}
		total := worldCupTeamTotal(totals, team)
		total.shotsOnTarget += int(espnStatNumber(boxscoreTeam.Statistics, "shotsOnTarget"))
		total.saves += int(espnStatNumber(boxscoreTeam.Statistics, "saves"))
		total.yellowCards += int(espnStatNumber(boxscoreTeam.Statistics, "yellowCards"))
		total.redCards += int(espnStatNumber(boxscoreTeam.Statistics, "redCards"))
		if possession := espnStatNumber(boxscoreTeam.Statistics, "possessionPct"); possession > 0 {
			total.possessionTotal += possession
			total.possessionGames++
		}
		total.accuratePasses += espnStatNumber(boxscoreTeam.Statistics, "accuratePasses")
		total.totalPasses += espnStatNumber(boxscoreTeam.Statistics, "totalPasses")
	}
}

func worldCupTeamTotal(totals map[string]*worldCupTeamTotals, team models.Team) *worldCupTeamTotals {
	key := worldCupTeamKey(team)
	if totals[key] == nil {
		totals[key] = &worldCupTeamTotals{team: team}
	}
	return totals[key]
}

func worldCupTeamLeaderCategory(
	name string,
	totals map[string]*worldCupTeamTotals,
	value func(*worldCupTeamTotals) (int, string),
) models.WorldCupLeaderCategory {
	leaders := make([]models.WorldCupLeader, 0, len(totals))
	for _, total := range totals {
		score, display := value(total)
		if score <= 0 || display == "" {
			continue
		}
		leaders = append(leaders, models.WorldCupLeader{
			Team:         total.team,
			Value:        score,
			DisplayValue: display,
		})
	}
	sort.SliceStable(leaders, func(i, j int) bool {
		if leaders[i].Value != leaders[j].Value {
			return leaders[i].Value > leaders[j].Value
		}
		return leaders[i].Team.Name < leaders[j].Team.Name
	})
	if len(leaders) > 5 {
		leaders = leaders[:5]
	}
	applyWorldCupLeaderRanks(leaders)
	return models.WorldCupLeaderCategory{Name: name, Kind: "team", Leaders: leaders}
}

func espnStatNumber(stats []espnStat, names ...string) float64 {
	for _, name := range names {
		for _, stat := range stats {
			if strings.EqualFold(stat.Name, name) ||
				strings.EqualFold(stat.Abbreviation, name) ||
				strings.EqualFold(stat.DisplayName, name) ||
				strings.EqualFold(stat.ShortName, name) {
				if stat.Value != 0 {
					return stat.Value
				}
				value, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(stat.DisplayValue), "%"), 64)
				return value
			}
		}
	}
	return 0
}

func applyWorldCupLeaderRanks(leaders []models.WorldCupLeader) {
	previousValue := -1
	rank := 0
	for i := range leaders {
		if i == 0 || leaders[i].Value != previousValue {
			rank = i + 1
			previousValue = leaders[i].Value
		}
		leaders[i].Rank = rank
	}
}

func applyWorldCupBracketLayout(cup *models.WorldCup) {
	if cup == nil || len(cup.Bracket) == 0 {
		return
	}
	cup.LeftBracket = nil
	cup.RightBracket = nil
	cup.Final = models.WorldCupMatch{}
	for _, round := range cup.Bracket {
		name := worldCupStageLabel(round.Name)
		if name == "Final" {
			if len(round.Matches) > 0 {
				cup.Final = round.Matches[0]
			}
			continue
		}
		if name == "3rd-Place Match" || len(round.Matches) == 0 {
			continue
		}
		left, right := splitWorldCupRound(round)
		if len(left.Matches) > 0 {
			cup.LeftBracket = append(cup.LeftBracket, left)
		}
		if len(right.Matches) > 0 {
			cup.RightBracket = append(cup.RightBracket, right)
		}
	}
	reverseWorldCupRounds(cup.RightBracket)
}

func splitWorldCupRound(round models.WorldCupRound) (models.WorldCupRound, models.WorldCupRound) {
	half := (len(round.Matches) + 1) / 2
	left := models.WorldCupRound{Name: round.Name, Matches: append([]models.WorldCupMatch(nil), round.Matches[:half]...)}
	right := models.WorldCupRound{Name: round.Name, Matches: append([]models.WorldCupMatch(nil), round.Matches[half:]...)}
	return left, right
}

func reverseWorldCupRounds(rounds []models.WorldCupRound) {
	for i, j := 0, len(rounds)-1; i < j; i, j = i+1, j-1 {
		rounds[i], rounds[j] = rounds[j], rounds[i]
	}
}

func worldCupMatchFromEvent(ev espnEvent) (models.WorldCupMatch, bool) {
	game, ok := parseESPNEvent(ev, models.FIFA)
	if !ok {
		return models.WorldCupMatch{}, false
	}
	summary := worldCupMatchSummary(ev, game)
	return models.WorldCupMatch{
		ID:        game.ID,
		Stage:     worldCupStageLabel(firstNonEmpty(ev.Season.Name, ev.Season.Slug)),
		HomeTeam:  game.HomeTeam,
		AwayTeam:  game.AwayTeam,
		HomeScore: game.HomeScore,
		AwayScore: game.AwayScore,
		Status:    game.Status,
		Period:    game.Period,
		TimeLeft:  game.TimeLeft,
		StartTime: game.StartTime,
		Venue:     game.Venue,
		City:      game.City,
		Broadcast: game.Broadcast,
		Summary:   summary,
		Bullets:   worldCupMatchBullets(summary),
	}, true
}

func (s *ESPNStore) enhanceWorldCupRecentMatch(match models.WorldCupMatch) models.WorldCupMatch {
	match = s.attachWorldCupHighlights(match)
	if match.ID == "" || strings.TrimSpace(match.Summary) == "" {
		return match
	}
	cacheKey := "world-cup:" + match.ID
	facts := worldCupRecapFacts(match)
	if len(match.Bullets) == 0 {
		match.Bullets = worldCupMatchBullets(match.Summary)
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		return match
	}
	if cached, ok := s.cachedAIRecap(cacheKey); ok {
		log.Printf("AI recap cache hit for World Cup match %s", match.ID)
		return applyAIRecapToWorldCupMatch(match, cached)
	}
	go s.generateAndCacheAIRecap(cacheKey, facts)
	return match
}

func (s *ESPNStore) attachWorldCupHighlights(match models.WorldCupMatch) models.WorldCupMatch {
	if match.ID == "" {
		return match
	}
	key := "world-cup:" + match.ID
	now := time.Now()
	s.mu.Lock()
	entry, ok := s.highlights[key]
	if ok && now.Before(entry.NextFetchAt) {
		match.Highlights = entry.Highlights
		match.HighlightsPending = entry.Pending
		s.mu.Unlock()
		return match
	}
	if ok && len(entry.Highlights) == 0 && !entry.Pending && !entry.StopAfter.IsZero() && now.After(entry.StopAfter) {
		s.mu.Unlock()
		return match
	}
	s.mu.Unlock()

	highlights := s.fetchESPNHighlights(sportCfg{Sport: models.FIFA, SummaryURL: worldCupSummaryURL}, match.ID)

	s.mu.Lock()
	if len(highlights) == 0 && ok && len(entry.Highlights) > 0 {
		entry = refreshedHighlightsCacheEntry(entry, match.StartTime, now)
	} else {
		entry = newHighlightsCacheEntry(highlights, match.StartTime, now)
	}
	s.highlights[key] = entry
	saveErr := s.saveHighlightCacheLocked()
	s.mu.Unlock()
	if saveErr != nil {
		log.Printf("World Cup highlight cache save failed for match %s: %v", match.ID, saveErr)
	}
	match.Highlights = entry.Highlights
	match.HighlightsPending = entry.Pending
	return match
}

func worldCupRecapFacts(match models.WorldCupMatch) gameRecapFacts {
	team, opponent := match.HomeTeam, match.AwayTeam
	teamScore, opponentScore := match.HomeScore, match.AwayScore
	result := "W"
	if match.AwayScore > match.HomeScore {
		team, opponent = match.AwayTeam, match.HomeTeam
		teamScore, opponentScore = match.AwayScore, match.HomeScore
	} else if match.HomeScore == match.AwayScore {
		result = "T"
	}
	return gameRecapFacts{
		Sport:              models.FIFA,
		PhillyTeam:         team,
		Opponent:           opponent,
		Home:               team.ID == match.HomeTeam.ID,
		PhillyScore:        teamScore,
		OppScore:           opponentScore,
		Result:             result,
		GameDate:           match.StartTime,
		Venue:              match.Venue,
		City:               match.City,
		RawSummary:         match.Summary,
		HasProviderSummary: strings.TrimSpace(match.Summary) != "",
		NeutralMatch:       true,
	}
}

func applyAIRecapToWorldCupMatch(match models.WorldCupMatch, recap aiGameRecap) models.WorldCupMatch {
	bullets := cleanAIBullets(recap.Bullets)
	if len(bullets) > 0 {
		match.Bullets = bullets
	}
	return match
}

func worldCupMatchSummary(ev espnEvent, game models.Game) string {
	if len(ev.Competitions) > 0 {
		for _, headline := range ev.Competitions[0].Headlines {
			if summary := cleanRecapText(headline.Description); summary != "" {
				return summary
			}
			if summary := cleanRecapText(headline.ShortLinkText); summary != "" {
				return summary
			}
		}
	}
	if game.Status != models.StatusFinal {
		return ""
	}

	winner, loser := game.HomeTeam, game.AwayTeam
	winnerScore, loserScore := game.HomeScore, game.AwayScore
	if game.AwayScore > game.HomeScore {
		winner, loser = game.AwayTeam, game.HomeTeam
		winnerScore, loserScore = game.AwayScore, game.HomeScore
	}
	if game.HomeScore == game.AwayScore {
		return fmt.Sprintf("%s and %s finished level %d-%d.", game.AwayTeam.Name, game.HomeTeam.Name, game.AwayScore, game.HomeScore)
	}
	return fmt.Sprintf("%s beat %s %d-%d.", winner.Name, loser.Name, winnerScore, loserScore)
}

func worldCupMatchBullets(summary string) []string {
	summary = cleanRecapText(summary)
	if summary == "" {
		return nil
	}
	return []string{ensurePeriod(summary)}
}

func worldCupStandingFromEntry(entry espnStandingsEntry) models.WorldCupStanding {
	return models.WorldCupStanding{
		Team:     espnToTeam(entry.Team, models.FIFA),
		Played:   worldCupStandingDisplay(entry, "gamesPlayed", "GP"),
		Wins:     worldCupStandingDisplay(entry, "wins", "W"),
		Draws:    worldCupStandingDisplay(entry, "ties", "draws", "D"),
		Losses:   worldCupStandingDisplay(entry, "losses", "L"),
		For:      worldCupStandingDisplay(entry, "pointsFor", "F"),
		Against:  worldCupStandingDisplay(entry, "pointsAgainst", "A"),
		Diff:     worldCupStandingDisplay(entry, "pointDifferential", "GD"),
		Points:   worldCupStandingDisplay(entry, "points", "P"),
		Note:     strings.TrimSpace(entry.Note.Description),
		Rank:     worldCupStandingInt(entry, "rank"),
		Advanced: worldCupStandingInt(entry, "advanced") > 0,
	}
}

func worldCupStandingInt(entry espnStandingsEntry, names ...string) int {
	for _, name := range names {
		for _, stat := range entry.Stats {
			if strings.EqualFold(stat.Name, name) ||
				strings.EqualFold(stat.Abbreviation, name) ||
				strings.EqualFold(stat.DisplayName, name) ||
				strings.EqualFold(stat.ShortName, name) {
				return int(stat.Value)
			}
		}
	}
	return 0
}

func worldCupStandingDisplay(entry espnStandingsEntry, names ...string) string {
	for _, name := range names {
		for _, stat := range entry.Stats {
			if strings.EqualFold(stat.Name, name) ||
				strings.EqualFold(stat.Abbreviation, name) ||
				strings.EqualFold(stat.DisplayName, name) ||
				strings.EqualFold(stat.ShortName, name) {
				if strings.TrimSpace(stat.DisplayValue) != "" {
					return stat.DisplayValue
				}
				return strconv.Itoa(int(stat.Value))
			}
		}
	}
	return "0"
}

func sortWorldCupStandings(rows []models.WorldCupStanding) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if av, bv := standingInt(a.Points), standingInt(b.Points); av != bv {
			return av > bv
		}
		if av, bv := standingInt(a.Diff), standingInt(b.Diff); av != bv {
			return av > bv
		}
		if av, bv := standingInt(a.For), standingInt(b.For); av != bv {
			return av > bv
		}
		if av, bv := standingInt(a.Wins), standingInt(b.Wins); av != bv {
			return av > bv
		}
		return strings.Compare(worldCupStandingTeamName(a.Team), worldCupStandingTeamName(b.Team)) < 0
	})
}

func standingInt(value string) int {
	value = strings.TrimSpace(strings.ReplaceAll(value, "+", ""))
	if value == "" {
		return 0
	}
	n, _ := strconv.Atoi(value)
	return n
}

func worldCupStandingTeamName(team models.Team) string {
	name := strings.TrimSpace(team.Name)
	if name == "" {
		name = strings.TrimSpace(team.City)
	}
	if name == "" {
		name = strings.TrimSpace(team.Abbr)
	}
	return strings.ToLower(name)
}

func worldCupStageLabel(stage string) string {
	stage = strings.TrimSpace(strings.ReplaceAll(stage, "-", " "))
	if stage == "" {
		return ""
	}
	lower := strings.ToLower(stage)
	switch {
	case strings.Contains(lower, "round of 32"):
		return "Round of 32"
	case strings.Contains(lower, "rd of 16"), strings.Contains(lower, "round of 16"):
		return "Round of 16"
	case strings.Contains(lower, "quarter"):
		return "Quarterfinals"
	case strings.Contains(lower, "semi"):
		return "Semifinals"
	case strings.Contains(lower, "3rd"), strings.Contains(lower, "third"):
		return "3rd-Place Match"
	case strings.Contains(lower, "final"):
		return "Final"
	case strings.Contains(lower, "group"):
		return "Group Stage"
	default:
		return strings.Title(stage)
	}
}

func worldCupWatchInfo() []models.WorldCupWatch {
	return []models.WorldCupWatch{
		{Label: "English TV", Description: "National match windows on FOX or FS1.", Networks: []string{"FOX", "FS1"}},
		{Label: "Spanish TV", Description: "Spanish-language coverage listed by ESPN as Tele.", Networks: []string{"Tele"}},
		{Label: "Streaming", Description: "Streaming availability appears on match cards when listed.", Networks: []string{"Peacock"}},
	}
}

func standingsViewExists(views []models.StandingsView, key, label string, rows []models.StandingsRow) bool {
	for _, view := range views {
		if view.Key == key || strings.EqualFold(view.Label, label) || sameStandingsRows(view.Rows, rows) {
			return true
		}
	}
	return false
}

func sameStandingsRows(a, b []models.StandingsRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameStandingsTeam(a[i].Team, b[i].Team) {
			return false
		}
	}
	return true
}

func sameStandingsTeam(a, b models.Team) bool {
	if a.ID != "" && b.ID != "" {
		return strings.EqualFold(a.ID, b.ID)
	}
	return strings.EqualFold(a.City, b.City) && strings.EqualFold(a.Name, b.Name)
}

func recordPartsOrStats(recordStat func(...string) (string, bool), intStat func(...string) int, recordNames, winNames, lossNames, thirdNames []string) (int, int, int) {
	if record, ok := recordStat(recordNames...); ok {
		return parseRecordParts(record)
	}
	return intStat(winNames...), intStat(lossNames...), intStat(thirdNames...)
}

func isRecordDisplayValue(value string) bool {
	if value == "" || !strings.Contains(value, "-") {
		return false
	}
	_, _, _, ok := parseRecordPartsOK(value)
	return ok
}

func parseRecordParts(record string) (int, int, int) {
	first, second, third, _ := parseRecordPartsOK(record)
	return first, second, third
}

func parseRecordPartsOK(record string) (int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(record), "-")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false
	}
	values := [3]int{}
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0, 0, 0, false
		}
		values[i] = value
	}
	return values[0], values[1], values[2], true
}

func sportOrder(sport models.Sport) int {
	switch sport {
	case models.NFL:
		return 0
	case models.NHL:
		return 1
	case models.MLB:
		return 2
	case models.NBA:
		return 3
	case models.MLS:
		return 4
	default:
		return 99
	}
}

func standingsEntryToRow(entry espnStandingsEntry, sport models.Sport) models.StandingsRow {
	sm := make(map[string]espnStat, len(entry.Stats))
	for _, s := range entry.Stats {
		sm[s.Name] = s
		sm[strings.ToLower(s.Name)] = s
		sm[strings.ToLower(s.Abbreviation)] = s
		sm[strings.ToLower(s.DisplayName)] = s
		sm[strings.ToLower(s.ShortName)] = s
	}

	intStat := func(names ...string) int {
		for _, n := range names {
			if s, ok := sm[strings.ToLower(n)]; ok {
				return int(s.Value)
			}
		}
		return 0
	}
	recordStat := func(names ...string) (string, bool) {
		for _, n := range names {
			if s, ok := sm[strings.ToLower(n)]; ok {
				value := strings.TrimSpace(s.DisplayValue)
				if isRecordDisplayValue(value) {
					return value, true
				}
			}
		}
		return "", false
	}
	displayStat := func(names ...string) string {
		for _, n := range names {
			if s, ok := sm[strings.ToLower(n)]; ok {
				value := strings.TrimSpace(s.DisplayValue)
				if value != "" && value != "-" {
					return value
				}
				if s.Value > 0 {
					return strconv.Itoa(int(s.Value))
				}
			}
		}
		return ""
	}

	w := intStat("wins")
	l := intStat("losses")
	t := intStat("ties", "draws")
	hw, hl, ht := recordPartsOrStats(recordStat, intStat, []string{"home", "homeRecord"}, []string{"homeWins", "homeWin"}, []string{"homeLosses", "homeLoss"}, []string{"homeTies", "homeDraws"})
	rw, rl, rt := recordPartsOrStats(recordStat, intStat, []string{"road", "away", "roadRecord", "awayRecord"}, []string{"roadWins", "awayWins", "roadWin"}, []string{"roadLosses", "awayLosses", "roadLoss"}, []string{"roadTies", "awayTies", "roadDraws", "awayDraws"})

	// NHL OT losses
	otl := intStat("otLosses", "overtimeLosses")
	hotl := intStat("homeOtLosses", "homeOTLoss", "homeOvertimeLosses")
	rotl := intStat("roadOtLosses", "roadOTLoss", "roadOvertimeLosses", "awayOtLosses", "awayOvertimeLosses")
	if sport == models.NHL {
		if homeRecord, ok := recordStat("home", "homeRecord"); ok {
			hw, hl, hotl = parseRecordParts(homeRecord)
		}
		if awayRecord, ok := recordStat("road", "away", "roadRecord", "awayRecord"); ok {
			rw, rl, rotl = parseRecordParts(awayRecord)
		}
	}

	var record, homeStr, awayStr string
	if sport == models.NHL {
		record = fmt.Sprintf("%d-%d-%d", w, l, otl)
		homeStr = fmt.Sprintf("%d-%d-%d", hw, hl, hotl)
		awayStr = fmt.Sprintf("%d-%d-%d", rw, rl, rotl)
	} else if sport == models.MLS {
		record = fmt.Sprintf("%d-%d-%d", w, l, t)
		homeStr = fmt.Sprintf("%d-%d-%d", hw, hl, ht)
		awayStr = fmt.Sprintf("%d-%d-%d", rw, rl, rt)
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
		Rank:     displayStat("playoffSeed", "rank", "seed", "position"),
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
		return s.applyCachedAIRecaps(results)
	}
	s.mu.RUnlock()

	type recentCandidate struct {
		gameID string
		result models.RecentResult
		facts  gameRecapFacts
	}

	var mu sync.Mutex
	candidates := make([]recentCandidate, 0)
	seen := map[string]bool{}
	var wg sync.WaitGroup
	now := NowPhilly()

	for _, cfg := range sportConfigs {
		cfg := cfg
		if !isInSeason(cfg.Sport) {
			continue
		}
		for daysBack := 0; daysBack <= 14; daysBack++ {
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

					var phillyTeam, opponent models.Team
					var phillyScore, oppScore int
					home := false
					if isPhillyTeam(g.HomeTeam) {
						phillyTeam = g.HomeTeam
						opponent = g.AwayTeam
						phillyScore = g.HomeScore
						oppScore = g.AwayScore
						home = true
					} else {
						phillyTeam = g.AwayTeam
						opponent = g.HomeTeam
						phillyScore = g.AwayScore
						oppScore = g.HomeScore
					}

					resultCode := "W"
					if phillyScore < oppScore {
						resultCode = "L"
					} else if phillyScore == oppScore {
						resultCode = "T"
					}

					mu.Lock()
					if seen[g.ID] {
						mu.Unlock()
						continue
					}
					seen[g.ID] = true
					mu.Unlock()

					summary, hasProviderSummary := recentResultSummary(ev, phillyTeam, opponent, phillyScore, oppScore)
					result := models.RecentResult{
						GameID:   g.ID,
						Team:     phillyTeam,
						Opponent: opponent,
						Home:     home,
						Result:   resultCode,
						Record:   fmt.Sprintf("%s %d-%d", resultCode, phillyScore, oppScore),
						Summary:  summary,
						Bullets:  recentResultBullets(summary, phillyTeam, opponent),
						GameDate: g.StartTime,
					}
					result = s.attachHighlights(cfg, g, result)
					facts := gameRecapFacts{
						Sport:              cfg.Sport,
						PhillyTeam:         phillyTeam,
						Opponent:           opponent,
						Home:               home,
						PhillyScore:        phillyScore,
						OppScore:           oppScore,
						Result:             resultCode,
						GameDate:           g.StartTime,
						Venue:              g.Venue,
						City:               g.City,
						RawSummary:         summary,
						HasProviderSummary: hasProviderSummary,
					}

					mu.Lock()
					candidates = append(candidates, recentCandidate{
						gameID: g.ID,
						result: result,
						facts:  facts,
					})
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].result.GameDate.After(candidates[j].result.GameDate)
	})

	// Keep only the most recent result per Philly team before doing optional
	// AI cleanup, so we only call OpenAI for results that are actually shown.
	byTeam := map[string]recentCandidate{}
	for _, candidate := range candidates {
		r := candidate.result
		key := string(r.Team.Sport) + ":" + r.Team.ID
		if _, exists := byTeam[key]; !exists {
			byTeam[key] = candidate
		}
	}
	results := make([]models.RecentResult, 0, len(byTeam))
	for _, candidate := range byTeam {
		result := s.applyCachedOrQueueAIRecap(candidate.gameID, candidate.result, candidate.facts)
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].GameDate.After(results[j].GameDate)
	})

	s.mu.Lock()
	s.resultsCache = resultsCache{results: results, expiresAt: time.Now().Add(recentResultsTTL(results))}
	s.mu.Unlock()
	return results
}

func (s *ESPNStore) InvalidateRecentResults() {
	s.mu.Lock()
	s.resultsCache = resultsCache{}
	s.mu.Unlock()
}

func (s *ESPNStore) attachHighlights(cfg sportCfg, g models.Game, result models.RecentResult) models.RecentResult {
	if result.GameID == "" {
		return result
	}
	now := time.Now()
	s.mu.RLock()
	entry, ok := s.highlights[result.GameID]
	s.mu.RUnlock()
	if ok && now.Before(entry.NextFetchAt) {
		result.Highlights = entry.Highlights
		result.HighlightsPending = entry.Pending
		return result
	}
	if ok && len(entry.Highlights) == 0 && !entry.Pending && !entry.StopAfter.IsZero() && now.After(entry.StopAfter) {
		return result
	}

	highlights := s.fetchGameHighlights(cfg, g)
	if len(highlights) == 0 && ok && len(entry.Highlights) > 0 {
		entry = refreshedHighlightsCacheEntry(entry, g.StartTime, now)
	} else {
		entry = newHighlightsCacheEntry(highlights, g.StartTime, now)
	}

	s.mu.Lock()
	s.highlights[result.GameID] = entry
	saveErr := s.saveHighlightCacheLocked()
	s.mu.Unlock()
	if saveErr != nil {
		log.Printf("Highlight cache save failed for game %s: %v", result.GameID, saveErr)
	}

	result.Highlights = entry.Highlights
	result.HighlightsPending = entry.Pending
	return result
}

func newHighlightsCacheEntry(highlights []models.VideoHighlight, gameDate, now time.Time) highlightsCacheEntry {
	if len(highlights) > 0 {
		return highlightsCacheEntry{
			Highlights:  highlights,
			CachedAt:    now,
			NextFetchAt: nextFoundHighlightsFetch(gameDate, now),
			StopAfter:   highlightStopAfterTime(gameDate),
		}
	}
	stopAfter := highlightStopAfterTime(gameDate)
	pending := gameDate.IsZero() || now.Before(stopAfter)
	next := now.Add(highlightPendingRetry)
	if !pending {
		next = now.Add(highlightFoundTTL)
	}
	return highlightsCacheEntry{
		Pending:     pending,
		CachedAt:    now,
		NextFetchAt: next,
		StopAfter:   stopAfter,
	}
}

func refreshedHighlightsCacheEntry(entry highlightsCacheEntry, gameDate, now time.Time) highlightsCacheEntry {
	entry.Pending = false
	entry.CachedAt = now
	entry.NextFetchAt = nextFoundHighlightsFetch(gameDate, now)
	if entry.StopAfter.IsZero() {
		entry.StopAfter = highlightStopAfterTime(gameDate)
	}
	return entry
}

func nextFoundHighlightsFetch(gameDate, now time.Time) time.Time {
	if gameDate.IsZero() || now.Before(gameDate.Add(highlightUpgradeWindow)) {
		return now.Add(highlightUpgradeRetry)
	}
	return now.Add(highlightFoundTTL)
}

func highlightStopAfterTime(gameDate time.Time) time.Time {
	if gameDate.IsZero() {
		return time.Time{}
	}
	return gameDate.Add(highlightStopAfter)
}

func (s *ESPNStore) fetchGameHighlights(cfg sportCfg, g models.Game) []models.VideoHighlight {
	if cfg.Sport == models.MLB {
		if highlights := s.fetchMLBHighlights(g); len(highlights) > 0 {
			return highlights
		}
	}
	if cfg.SummaryURL != "" {
		return s.fetchESPNHighlights(cfg, g.ID)
	}
	return nil
}

func (s *ESPNStore) fetchESPNHighlights(cfg sportCfg, eventID string) []models.VideoHighlight {
	var summary espnSummaryResp
	if err := s.fetchJSON(fmt.Sprintf(cfg.SummaryURL, eventID), &summary); err != nil {
		return nil
	}
	videos := preferredESPNVideos(summary.Videos)
	out := make([]models.VideoHighlight, 0, len(videos))
	for _, video := range videos {
		h := models.VideoHighlight{
			Title:       firstNonEmpty(video.Headline, video.Title),
			Description: strings.TrimSpace(video.Description),
			Thumbnail:   firstNonEmpty(video.Thumbnail, firstESPNImage(video.Images)),
			URL:         firstNonEmpty(video.Links.Web.Href, video.Links.Source.Href),
			Provider:    "ESPN",
		}
		if h.Title == "" {
			h.Title = "Game highlights"
		}
		if h.URL == "" {
			continue
		}
		out = append(out, h)
	}
	return dedupeHighlights(out)
}

func preferredESPNVideos(videos []espnVideo) []espnVideo {
	if len(videos) == 0 {
		return nil
	}
	for _, video := range videos {
		if isESPNGameHighlights(video) {
			return []espnVideo{video}
		}
	}
	for _, video := range videos {
		if isESPNRecapVideo(video) {
			return []espnVideo{video}
		}
	}
	return []espnVideo{videos[0]}
}

func isESPNGameHighlights(video espnVideo) bool {
	text := strings.ToLower(firstNonEmpty(video.Headline, video.Title, video.Description))
	return strings.Contains(text, "game highlights") ||
		strings.Contains(text, "match highlights") ||
		strings.Contains(text, "extended highlights")
}

func isESPNRecapVideo(video espnVideo) bool {
	text := strings.ToLower(firstNonEmpty(video.Headline, video.Title, video.Description))
	return strings.Contains(text, "recap") ||
		strings.Contains(text, "highlights")
}

func (s *ESPNStore) fetchMLBHighlights(g models.Game) []models.VideoHighlight {
	gamePk := s.findMLBGamePk(g)
	if gamePk == 0 {
		return nil
	}
	var content mlbContentResp
	if err := s.fetchJSON(fmt.Sprintf(mlbContentURL, gamePk), &content); err != nil {
		return nil
	}
	items := content.Highlights.Highlights.Items
	preferredItems := preferredMLBHighlightItems(items)
	out := make([]models.VideoHighlight, 0, len(preferredItems))
	for _, item := range preferredItems {
		h := models.VideoHighlight{
			Title:       firstNonEmpty(item.Title, item.Headline),
			Description: firstNonEmpty(item.Description, item.Blurb),
			Thumbnail:   firstMLBImage(item.Image.Cuts),
			URL:         bestMLBPlaybackURL(item.Playbacks),
			Provider:    "MLB",
			PublishedAt: item.Date.Time,
		}
		if h.Title == "" {
			h.Title = "Game highlights"
		}
		if h.URL == "" {
			continue
		}
		out = append(out, h)
	}
	return dedupeHighlights(out)
}

func preferredMLBHighlightItems(items []mlbContentItem) []mlbContentItem {
	if len(items) == 0 {
		return nil
	}
	if item, ok := longestMLBItem(items, func(item mlbContentItem) bool {
		return isMLBNonRecapHighlight(item) && !isTinyMLBClip(item.Duration)
	}); ok {
		return []mlbContentItem{item}
	}
	if item, ok := longestMLBItem(items, func(item mlbContentItem) bool {
		return isMLBCondensedGame(item) && !isTinyMLBClip(item.Duration)
	}); ok {
		return []mlbContentItem{item}
	}
	if item, ok := longestMLBItem(items, func(item mlbContentItem) bool {
		return isMLBGameRecap(item) && !isTinyMLBClip(item.Duration)
	}); ok {
		return []mlbContentItem{item}
	}
	return []mlbContentItem{longestMLBItemFallback(items)}
}

func isMLBCondensedGame(item mlbContentItem) bool {
	text := mlbHighlightSearchText(item)
	return strings.Contains(text, "condensed game")
}

func isMLBNonRecapHighlight(item mlbContentItem) bool {
	text := mlbHighlightSearchText(item)
	return !strings.Contains(text, "recap") &&
		(strings.Contains(text, "game highlights") ||
			strings.Contains(text, "highlights") ||
			strings.Contains(text, "highlight"))
}

func isMLBGameRecap(item mlbContentItem) bool {
	text := mlbHighlightSearchText(item)
	return strings.Contains(text, "recap") ||
		strings.Contains(text, "highlights") ||
		strings.Contains(text, "dominates in") ||
		strings.Contains(text, "win vs.")
}

func mlbHighlightSearchText(item mlbContentItem) string {
	return strings.ToLower(strings.Join([]string{
		item.Title,
		item.Headline,
		item.Description,
		item.Blurb,
	}, " "))
}

func isTinyMLBClip(duration string) bool {
	seconds, ok := parseProviderDuration(duration)
	return ok && seconds > 0 && seconds < 45
}

func longestMLBItem(items []mlbContentItem, include func(mlbContentItem) bool) (mlbContentItem, bool) {
	var best mlbContentItem
	bestSeconds := -1
	for _, item := range items {
		if !include(item) {
			continue
		}
		seconds, ok := parseProviderDuration(item.Duration)
		if !ok {
			seconds = 0
		}
		if seconds > bestSeconds {
			best = item
			bestSeconds = seconds
		}
	}
	return best, bestSeconds >= 0
}

func longestMLBItemFallback(items []mlbContentItem) mlbContentItem {
	if item, ok := longestMLBItem(items, func(mlbContentItem) bool { return true }); ok {
		return item
	}
	return mlbContentItem{}
}

func parseProviderDuration(duration string) (int, bool) {
	duration = strings.TrimSpace(duration)
	if duration == "" {
		return 0, false
	}
	parts := strings.Split(duration, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	total := 0
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0, false
		}
		total = total*60 + value
	}
	return total, true
}

func recentResultsTTL(results []models.RecentResult) time.Duration {
	for _, result := range results {
		if result.HighlightsPending {
			return 15 * time.Minute
		}
	}
	return 10 * time.Minute
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstESPNImage(images []struct {
	URL string `json:"url"`
}) string {
	for _, image := range images {
		if image.URL != "" {
			return strings.TrimSpace(image.URL)
		}
	}
	return ""
}

func firstMLBImage(cuts []struct {
	Src string `json:"src"`
}) string {
	for i := len(cuts) - 1; i >= 0; i-- {
		if cuts[i].Src != "" {
			return strings.TrimSpace(cuts[i].Src)
		}
	}
	return ""
}

func bestMLBPlaybackURL(playbacks []struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}) string {
	if len(playbacks) == 0 {
		return ""
	}
	for _, want := range []string{"mp4Avc", "highBit", "HTTP_CLOUD_WIRED_WEB", "HTTP_CLOUD_WIRED"} {
		for _, playback := range playbacks {
			if strings.EqualFold(strings.TrimSpace(playback.Name), want) && strings.TrimSpace(playback.URL) != "" {
				return strings.TrimSpace(playback.URL)
			}
		}
	}
	for _, playback := range playbacks {
		if strings.TrimSpace(playback.URL) != "" {
			return strings.TrimSpace(playback.URL)
		}
	}
	return ""
}

func dedupeHighlights(highlights []models.VideoHighlight) []models.VideoHighlight {
	seen := map[string]bool{}
	out := make([]models.VideoHighlight, 0, len(highlights))
	for _, h := range highlights {
		key := h.URL
		if key == "" {
			key = h.Title
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
	}
	return out
}

func (s *ESPNStore) InvalidateStandings() {
	s.mu.Lock()
	s.standingsCache = standingsCache{}
	s.leagueCache = leagueStandingsCache{}
	s.mu.Unlock()
}

func (s *ESPNStore) applyCachedAIRecaps(results []models.RecentResult) []models.RecentResult {
	if len(results) == 0 || os.Getenv("OPENAI_API_KEY") == "" {
		return results
	}
	updated := make([]models.RecentResult, len(results))
	copy(updated, results)
	for i := range updated {
		if updated[i].GameID == "" {
			continue
		}
		if cached, ok := s.cachedAIRecap(updated[i].GameID); ok {
			updated[i] = applyAIRecap(updated[i], cached)
		}
	}
	return updated
}

// ── Misc ──────────────────────────────────────────────────────────────────────

func recentResultSummary(ev espnEvent, phillyTeam, opponent models.Team, phillyScore, oppScore int) (string, bool) {
	if len(ev.Competitions) > 0 {
		for _, headline := range ev.Competitions[0].Headlines {
			if summary := cleanRecapText(headline.Description); summary != "" {
				return summary, true
			}
			if summary := cleanRecapText(headline.ShortLinkText); summary != "" {
				return summary, true
			}
		}
	}

	verb := "beat"
	if phillyScore < oppScore {
		verb = "fell to"
	} else if phillyScore == oppScore {
		verb = "tied"
	}

	return fmt.Sprintf("%s %s the %s %d-%d.", phillyTeam.Name, verb, opponent.Name, phillyScore, oppScore), false
}

func ensurePeriod(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?") {
		return s
	}
	return s + "."
}

func cleanRecapText(summary string) string {
	summary = strings.TrimSpace(summary)
	return strings.TrimLeft(summary, "\u2014\u2013- \t\r\n")
}

func recentResultBullets(summary string, phillyTeam, opponent models.Team) []string {
	summary = cleanRecapText(summary)
	if summary == "" {
		return nil
	}

	summary = strings.TrimSuffix(summary, ".")
	clauses := make([]string, 0, 3)
	for _, chunk := range strings.Split(summary, ",") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		if strings.HasPrefix(strings.ToLower(chunk), "ending ") {
			clauses = append(clauses, streakBullet(chunk, phillyTeam, opponent))
			continue
		}

		for _, part := range splitHighlightClause(chunk) {
			part = trimContext(part, phillyTeam)
			if part != "" {
				clauses = append(clauses, ensurePeriod(part))
			}
		}
	}

	bullets := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, clause := range clauses {
		key := strings.ToLower(clause)
		if seen[key] {
			continue
		}
		seen[key] = true
		bullets = append(bullets, clause)
		if len(bullets) == 3 {
			break
		}
	}
	if len(bullets) == 0 {
		return []string{ensurePeriod(summary)}
	}
	return bullets
}

func splitHighlightClause(clause string) []string {
	lower := strings.ToLower(clause)
	if idx := strings.Index(lower, " and "); idx > 0 {
		left := strings.TrimSpace(clause[:idx])
		right := strings.TrimSpace(clause[idx+5:])
		if startsWithCapitalizedWord(left) && startsWithCapitalizedWord(right) {
			return []string{left, right}
		}
	}
	return []string{clause}
}

func trimContext(clause string, phillyTeam models.Team) string {
	for _, marker := range []string{
		" as the " + phillyTeam.City + " " + phillyTeam.Name + " ",
		" as the " + phillyTeam.Name + " ",
	} {
		if idx := strings.Index(strings.ToLower(clause), strings.ToLower(marker)); idx > 0 {
			return strings.TrimSpace(clause[:idx])
		}
	}
	return strings.TrimSpace(clause)
}

func streakBullet(clause string, phillyTeam, opponent models.Team) string {
	text := strings.TrimSpace(strings.TrimPrefix(clause, "ending "))
	text = strings.TrimSpace(strings.TrimPrefix(text, "Ending "))
	possessive := opponent.Name + "'s"
	if strings.HasSuffix(opponent.Name, "s") {
		possessive = opponent.Name + "'"
	}
	text = strings.ReplaceAll(text, "the "+possessive, opponent.City+"'s")
	text = strings.ReplaceAll(text, possessive, opponent.City+"'s")
	return ensurePeriod("The " + phillyTeam.Name + " ended " + text)
}

func startsWithCapitalizedWord(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	first := []rune(strings.Fields(s)[0])[0]
	return first >= 'A' && first <= 'Z'
}

func (s *ESPNStore) applyCachedOrQueueAIRecap(gameID string, result models.RecentResult, facts gameRecapFacts) models.RecentResult {
	if gameID == "" || os.Getenv("OPENAI_API_KEY") == "" {
		return result
	}
	if cached, ok := s.cachedAIRecap(gameID); ok {
		log.Printf("AI recap cache hit for game %s", gameID)
		return applyAIRecap(result, cached)
	}
	if !facts.HasProviderSummary {
		return result
	}
	go s.generateAndCacheAIRecap(gameID, facts)
	return result
}

func (s *ESPNStore) generateAndCacheAIRecap(gameID string, facts gameRecapFacts) {
	if s.markAIRecapInFlight(gameID) {
		return
	}
	defer s.clearAIRecapInFlight(gameID)

	log.Printf(
		"AI recap request starting for game %s: %s %s vs %s %s",
		gameID,
		facts.PhillyTeam.City,
		facts.PhillyTeam.Name,
		facts.Opponent.City,
		facts.Opponent.Name,
	)
	recap, err := s.generateAIRecap(context.Background(), facts)
	if err != nil {
		log.Printf("AI recap skipped for game %s: %v", gameID, err)
		return
	}
	recap.CachedAt = time.Now().UTC().Format(time.RFC3339)
	log.Printf("AI recap generated for game %s", gameID)

	s.mu.Lock()
	s.aiRecapCache[gameID] = recap
	saveErr := s.saveAIRecapCacheLocked()
	s.mu.Unlock()
	if saveErr != nil {
		log.Printf("AI recap cache save failed for game %s: %v", gameID, saveErr)
	}
}

func (s *ESPNStore) enhanceRecentResult(gameID string, result models.RecentResult, facts gameRecapFacts) models.RecentResult {
	if gameID == "" || os.Getenv("OPENAI_API_KEY") == "" {
		return result
	}
	if !facts.HasProviderSummary {
		return result
	}

	if cached, ok := s.cachedAIRecap(gameID); ok {
		log.Printf("AI recap cache hit for game %s", gameID)
		return applyAIRecap(result, cached)
	}

	log.Printf(
		"AI recap request starting for game %s: %s %s vs %s %s",
		gameID,
		facts.PhillyTeam.City,
		facts.PhillyTeam.Name,
		facts.Opponent.City,
		facts.Opponent.Name,
	)
	recap, err := s.generateAIRecap(context.Background(), facts)
	if err != nil {
		log.Printf("AI recap skipped for game %s: %v", gameID, err)
		return result
	}
	recap.CachedAt = time.Now().UTC().Format(time.RFC3339)
	log.Printf("AI recap generated for game %s", gameID)

	s.mu.Lock()
	s.aiRecapCache[gameID] = recap
	saveErr := s.saveAIRecapCacheLocked()
	s.mu.Unlock()
	if saveErr != nil {
		log.Printf("AI recap cache save failed for game %s: %v", gameID, saveErr)
	}

	return applyAIRecap(result, recap)
}

func aiRecapCachePath() string {
	if path := strings.TrimSpace(os.Getenv("AI_RECAP_CACHE_PATH")); path != "" {
		return path
	}
	return filepath.Join(".", "ai-recap-cache.json")
}

func highlightCachePath() string {
	if path := strings.TrimSpace(os.Getenv("HIGHLIGHT_CACHE_PATH")); path != "" {
		return path
	}
	return filepath.Join(".", "highlight-cache.json")
}

func (s *ESPNStore) loadAIRecapCache() {
	if s.aiCachePath == "" {
		return
	}

	body, err := os.ReadFile(s.aiCachePath)
	if err != nil {
		return
	}

	var cache aiRecapCacheFile
	if err := json.Unmarshal(body, &cache); err != nil || cache.Recaps == nil {
		return
	}

	s.mu.Lock()
	for gameID, recap := range cache.Recaps {
		if gameID != "" {
			s.aiRecapCache[gameID] = recap
		}
	}
	s.mu.Unlock()
}

func (s *ESPNStore) loadHighlightCache() {
	if s.highlightPath == "" {
		return
	}

	body, err := os.ReadFile(s.highlightPath)
	if err != nil {
		return
	}

	var cache highlightCacheFile
	if err := json.Unmarshal(body, &cache); err != nil || cache.Games == nil {
		return
	}

	s.mu.Lock()
	for gameID, entry := range cache.Games {
		if gameID != "" {
			s.highlights[gameID] = entry
		}
	}
	s.mu.Unlock()
}

func (s *ESPNStore) saveAIRecapCacheLocked() error {
	if s.aiCachePath == "" {
		return nil
	}

	s.pruneAIRecapCacheLocked()

	if err := os.MkdirAll(filepath.Dir(s.aiCachePath), 0750); err != nil {
		return err
	}

	cache := aiRecapCacheFile{
		Version: 1,
		Recaps:  s.aiRecapCache,
	}
	body, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.aiCachePath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.aiCachePath)
}

func (s *ESPNStore) saveHighlightCacheLocked() error {
	if s.highlightPath == "" {
		return nil
	}

	s.pruneHighlightCacheLocked()

	if err := os.MkdirAll(filepath.Dir(s.highlightPath), 0750); err != nil {
		return err
	}

	cache := highlightCacheFile{
		Version: 1,
		Games:   s.highlights,
	}
	body, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.highlightPath + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.highlightPath)
}

func (s *ESPNStore) pruneAIRecapCacheLocked() {
	maxEntries := aiRecapCacheMaxEntries()
	if maxEntries <= 0 || len(s.aiRecapCache) <= maxEntries {
		return
	}

	type cacheEntry struct {
		gameID   string
		cachedAt time.Time
	}
	entries := make([]cacheEntry, 0, len(s.aiRecapCache))
	for gameID, recap := range s.aiRecapCache {
		t, err := time.Parse(time.RFC3339, recap.CachedAt)
		if err != nil {
			t = time.Time{}
		}
		entries = append(entries, cacheEntry{gameID: gameID, cachedAt: t})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].cachedAt.After(entries[j].cachedAt)
	})
	for _, entry := range entries[maxEntries:] {
		delete(s.aiRecapCache, entry.gameID)
	}
}

func (s *ESPNStore) pruneHighlightCacheLocked() {
	maxEntries := highlightCacheMaxEntries()
	if maxEntries <= 0 || len(s.highlights) <= maxEntries {
		return
	}

	type cacheEntry struct {
		gameID   string
		cachedAt time.Time
	}
	entries := make([]cacheEntry, 0, len(s.highlights))
	for gameID, entry := range s.highlights {
		entries = append(entries, cacheEntry{gameID: gameID, cachedAt: entry.CachedAt})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].cachedAt.After(entries[j].cachedAt)
	})
	for _, entry := range entries[maxEntries:] {
		delete(s.highlights, entry.gameID)
	}
}

func aiRecapCacheMaxEntries() int {
	raw := strings.TrimSpace(os.Getenv("AI_RECAP_CACHE_MAX_ENTRIES"))
	if raw == "" {
		return 100
	}
	maxEntries, err := strconv.Atoi(raw)
	if err != nil {
		return 100
	}
	return maxEntries
}

func highlightCacheMaxEntries() int {
	raw := strings.TrimSpace(os.Getenv("HIGHLIGHT_CACHE_MAX_ENTRIES"))
	if raw == "" {
		return 200
	}
	maxEntries, err := strconv.Atoi(raw)
	if err != nil {
		return 200
	}
	return maxEntries
}

func (s *ESPNStore) cachedAIRecap(gameID string) (aiGameRecap, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	recap, ok := s.aiRecapCache[gameID]
	return recap, ok
}

func (s *ESPNStore) markAIRecapInFlight(gameID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aiRecapCache[gameID].Bullets != nil {
		return true
	}
	if s.aiInFlight[gameID] {
		return true
	}
	s.aiInFlight[gameID] = true
	return false
}

func (s *ESPNStore) clearAIRecapInFlight(gameID string) {
	s.mu.Lock()
	delete(s.aiInFlight, gameID)
	s.mu.Unlock()
}

func applyAIRecap(result models.RecentResult, recap aiGameRecap) models.RecentResult {
	bullets := cleanAIBullets(recap.Bullets)
	if len(bullets) > 0 {
		result.Bullets = bullets
	}
	return result
}

func cleanAIBullets(bullets []string) []string {
	cleaned := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, bullet := range bullets {
		bullet = ensurePeriod(strings.TrimSpace(bullet))
		if bullet == "." {
			continue
		}
		key := strings.ToLower(bullet)
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, bullet)
		if len(cleaned) == 3 {
			break
		}
	}
	return cleaned
}

func (s *ESPNStore) generateAIRecap(ctx context.Context, facts gameRecapFacts) (aiGameRecap, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return aiGameRecap{}, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-5-nano"
	}

	payload := map[string]interface{}{
		"model": model,
		"input": gameRecapPrompt(facts),
		"text": map[string]interface{}{
			"format": map[string]interface{}{
				"type":   "json_schema",
				"name":   "game_recap",
				"strict": true,
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"bullets": map[string]interface{}{
							"type":     "array",
							"minItems": 1,
							"maxItems": 3,
							"items":    map[string]interface{}{"type": "string"},
						},
					},
					"required":             []string{"bullets"},
					"additionalProperties": false,
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return aiGameRecap{}, err
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return aiGameRecap{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := *s.client
	client.Timeout = openAITimeout()
	resp, err := client.Do(req)
	if err != nil {
		return aiGameRecap{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return aiGameRecap{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aiGameRecap{}, fmt.Errorf("openai status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return aiGameRecap{}, err
	}
	if openAIResp.Error != nil {
		return aiGameRecap{}, fmt.Errorf("openai: %s", openAIResp.Error.Message)
	}

	text := strings.TrimSpace(openAIOutputText(openAIResp))
	if text == "" {
		return aiGameRecap{}, fmt.Errorf("openai response did not include output text")
	}

	var recap aiGameRecap
	if err := json.Unmarshal([]byte(text), &recap); err != nil {
		return aiGameRecap{}, err
	}
	if len(cleanAIBullets(recap.Bullets)) == 0 {
		return aiGameRecap{}, fmt.Errorf("openai recap was empty")
	}
	return recap, nil
}

func openAITimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("OPENAI_TIMEOUT_SECONDS")); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return 30 * time.Second
}

func openAIOutputText(resp openAIResponse) string {
	for _, output := range resp.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				return content.Text
			}
		}
	}
	return ""
}

func gameRecapPrompt(facts gameRecapFacts) string {
	location := "home"
	if !facts.Home {
		location = "road"
	}
	teamLabel := "Philadelphia team"
	resultLabel := "Result for Philadelphia"
	if facts.NeutralMatch {
		teamLabel = "Featured team"
		resultLabel = "Result for featured team"
	}
	return fmt.Sprintf(`You are formatting a sports game headline for a website.

The website already displays:
- Date
- Final score
- Both teams

Turn the headline into 2-4 clean bullet points that summarize the key storylines.

Rules:
- Do not repeat the final score.
- Do not repeat the matchup unless needed for clarity.
- Do not mention the date.
- Focus on player highlights, milestones, and why the game mattered.
- Keep each bullet short and easy to read.
- Use plain language.
- Do not add facts that are not in the headline.
- Do not guess stats, innings, records, or player performance.
- Return only valid JSON.
- Use this exact format:

{
  "bullets": [
    "First bullet",
    "Second bullet"
  ]
}
Headline:

{{HEADLINE}}

Facts:
Sport: %s
%s: %s %s
Opponent: %s %s
%s: %s
Score: %s %d, %s %d
Game location: %s
Venue: %s
City: %s
Date: %s
Provider description: %s
`,
		facts.Sport,
		teamLabel,
		facts.PhillyTeam.City,
		facts.PhillyTeam.Name,
		facts.Opponent.City,
		facts.Opponent.Name,
		resultLabel,
		facts.Result,
		facts.PhillyTeam.Name,
		facts.PhillyScore,
		facts.Opponent.Name,
		facts.OppScore,
		location,
		facts.Venue,
		facts.City,
		PhillyTime(facts.GameDate).Format("January 2, 2006"),
		facts.RawSummary,
	)
}

func (s *ESPNStore) GetTeams() []models.Team {
	return []models.Team{Eagles, Flyers, Phillies, Sixers, Union}
}

func (s *ESPNStore) GetGameByID(id string) (*models.Game, bool) {
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

	homeScore, _ := strconv.Atoi(string(home.Score))
	awayScore, _ := strconv.Atoi(string(away.Score))

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
	baseball := espnBaseballState(sport, espnGameStatus(comp.Status), comp.Situation)

	return models.Game{
		ID:          ev.ID,
		HomeTeam:    espnToTeam(home.Team, sport),
		AwayTeam:    espnToTeam(away.Team, sport),
		HomeScore:   homeScore,
		AwayScore:   awayScore,
		Status:      espnGameStatus(comp.Status),
		Period:      period,
		TimeLeft:    timeLeft,
		Baseball:    baseball,
		StartTime:   ev.Date.Time,
		Venue:       comp.Venue.FullName,
		City:        city,
		Broadcast:   broadcasts,
		Sport:       sport,
		IsPreseason: espnIsPreseason(ev),
	}, true
}

func espnIsPreseason(ev espnEvent) bool {
	for _, value := range []string{ev.Season.Slug, ev.Season.Name, ev.Name, ev.ShortName} {
		if strings.Contains(strings.ToLower(value), "preseason") {
			return true
		}
	}
	return ev.Season.Type == 1
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
		name = t.Nickname
	}
	if name == "" {
		name = strings.TrimSpace(strings.TrimPrefix(t.DisplayName, t.Location))
	}
	if name == "" {
		name = t.ShortDisplayName
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
	n := strings.ToUpper(strings.TrimSpace(s.Type.Name))
	state := strings.ToLower(strings.TrimSpace(s.Type.State))
	detail := strings.ToLower(strings.Join([]string{s.Type.Description, s.Type.Detail, s.Type.ShortDetail}, " "))
	switch {
	case s.Type.Completed, state == "post", strings.HasPrefix(n, "STATUS_FINAL"), n == "STATUS_FULL_TIME", strings.Contains(detail, "full time"), strings.Contains(detail, "full-time"):
		return models.StatusFinal
	case state == "in", n == "STATUS_IN_PROGRESS", n == "STATUS_HALFTIME", n == "STATUS_END_PERIOD",
		strings.Contains(n, "IN_PROGRESS"), strings.Contains(n, "HALFTIME"), strings.Contains(n, "HALF_TIME"),
		strings.Contains(detail, "1st half"), strings.Contains(detail, "first half"), strings.Contains(detail, "2nd half"), strings.Contains(detail, "second half"), strings.Contains(detail, "halftime"), strings.Contains(detail, "stoppage"):
		return models.StatusLive
	case strings.Contains(n, "DELAY"):
		return models.StatusDelayed
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

func espnBaseballState(sport models.Sport, status models.GameStatus, situation espnSituation) *models.BaseballState {
	if sport != models.MLB || status != models.StatusLive {
		return nil
	}

	batter := espnPlayerName(situation.Batter)
	pitcher := espnPlayerName(situation.Pitcher)
	if !situation.OnFirst && !situation.OnSecond && !situation.OnThird &&
		situation.Outs == 0 && situation.Balls == 0 && situation.Strikes == 0 &&
		batter == "" && pitcher == "" {
		return nil
	}

	return &models.BaseballState{
		OnFirst:  situation.OnFirst,
		OnSecond: situation.OnSecond,
		OnThird:  situation.OnThird,
		Outs:     situation.Outs,
		Balls:    situation.Balls,
		Strikes:  situation.Strikes,
		Batter:   batter,
		Pitcher:  pitcher,
	}
}

func (s *ESPNStore) enrichMLBGame(g models.Game) models.Game {
	if g.Sport != models.MLB || !isPhillyGame(g) || g.Status == models.StatusScheduled {
		return g
	}
	gamePk := s.findMLBGamePk(g)
	if gamePk == 0 {
		return g
	}

	var feed mlbLiveFeedResp
	url := fmt.Sprintf(mlbLiveFeedURL, gamePk)
	if err := s.fetchJSON(url, &feed); err != nil {
		return g
	}

	if g.Baseball == nil {
		g.Baseball = &models.BaseballState{}
	}
	linescore := feed.LiveData.Linescore
	if linescore.Outs != 0 || linescore.Balls != 0 || linescore.Strikes != 0 || linescore.Offense.Batter.FullName != "" || linescore.Defense.Pitcher.FullName != "" {
		g.Baseball.Outs = linescore.Outs
		g.Baseball.Balls = linescore.Balls
		g.Baseball.Strikes = linescore.Strikes
		g.Baseball.OnFirst = linescore.Offense.First.ID != 0
		g.Baseball.OnSecond = linescore.Offense.Second.ID != 0
		g.Baseball.OnThird = linescore.Offense.Third.ID != 0
		g.Baseball.Batter = strings.TrimSpace(linescore.Offense.Batter.FullName)
		g.Baseball.Pitcher = strings.TrimSpace(linescore.Defense.Pitcher.FullName)
		g.Baseball.PitcherStrikeouts = mlbPitcherStrikeouts(feed, linescore.Defense.Pitcher)
	}

	current := feed.LiveData.Plays.CurrentPlay
	if desc := cleanMLBPlayDescription(current.Result.Description); desc != "" {
		g.Baseball.CurrentPlay = desc
	}
	g.Baseball.Plays = latestMLBPlays(feed.LiveData.Plays.AllPlays, 4)
	g.Lineup = mlbLineupFromFeed(feed, g)
	return g
}

func (s *ESPNStore) enrichSoccerGame(g models.Game, summaryURL string) models.Game {
	if !isSoccerSport(g.Sport) || summaryURL == "" {
		return g
	}
	g.Soccer = s.fetchSoccerState(g.ID, summaryURL, g.AwayTeam, g.HomeTeam)
	if g.Soccer != nil && g.Soccer.Lineup != nil {
		g.Lineup = g.Soccer.Lineup
	}
	return g
}

func (s *ESPNStore) fetchSoccerState(eventID, summaryURL string, awayTeam, homeTeam models.Team) *models.SoccerState {
	if eventID == "" || summaryURL == "" {
		return nil
	}
	var summary espnSummaryResp
	if err := s.fetchJSON(fmt.Sprintf(summaryURL, eventID), &summary); err != nil {
		return nil
	}
	state := soccerStateFromSummary(summary, awayTeam, homeTeam)
	if state == nil || (!hasSoccerStats(state) && !hasLineupEntries(state.Lineup)) {
		return nil
	}
	return state
}

func (s *ESPNStore) cachedSoccerState(eventID, summaryURL string, awayTeam, homeTeam models.Team, ttl time.Duration) *models.SoccerState {
	if eventID == "" || summaryURL == "" {
		return nil
	}
	now := time.Now()
	s.mu.RLock()
	if cached, ok := s.soccerStateCache[eventID]; ok && now.Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.state
	}
	s.mu.RUnlock()

	state := s.fetchSoccerState(eventID, summaryURL, awayTeam, homeTeam)
	if state == nil && ttl > 30*time.Second {
		ttl = 30 * time.Second
	}
	s.mu.Lock()
	s.soccerStateCache[eventID] = soccerStateCacheEntry{state: state, expiresAt: now.Add(ttl)}
	s.mu.Unlock()
	return state
}

func soccerStateFromSummary(summary espnSummaryResp, awayTeam, homeTeam models.Team) *models.SoccerState {
	state := &models.SoccerState{
		AwayStats: soccerTeamStats(summary.Boxscore.Teams, "away", awayTeam),
		HomeStats: soccerTeamStats(summary.Boxscore.Teams, "home", homeTeam),
		Lineup:    soccerLineupFromRosters(summary.Rosters, awayTeam, homeTeam),
		Goals:     soccerGoalsFromEvents(summary.KeyEvents),
	}
	if !hasSoccerStats(state) && !hasLineupEntries(state.Lineup) {
		return nil
	}
	return state
}

func soccerGoalsFromEvents(events []espnSoccerEvent) []models.SoccerGoal {
	goals := make([]models.SoccerGoal, 0, 4)
	for _, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type.Type))
		if !event.ScoringPlay || event.Shootout || !strings.Contains(eventType, "goal") {
			continue
		}
		scorer := ""
		assist := ""
		if len(event.Participants) > 0 {
			scorer = espnPlayerName(event.Participants[0].Athlete)
		}
		if len(event.Participants) > 1 {
			assist = espnPlayerName(event.Participants[1].Athlete)
		}
		if scorer == "" {
			scorer = strings.TrimSpace(event.ShortText)
		}
		if scorer == "" {
			continue
		}
		goals = append(goals, models.SoccerGoal{
			Team:    espnToTeam(event.Team, models.FIFA),
			Scorer:  scorer,
			Assist:  assist,
			Minute:  strings.TrimSpace(event.Clock.DisplayValue),
			OwnGoal: eventType == "own-goal",
		})
	}
	return goals
}

func soccerTeamStats(teams []espnBoxscoreTeamStats, homeAway string, fallback models.Team) models.SoccerTeamStats {
	for _, team := range teams {
		if strings.EqualFold(team.HomeAway, homeAway) || sameProviderTeam(team.Team, fallback) {
			return models.SoccerTeamStats{
				Shots:         espnStatDisplay(team.Statistics, "totalShots"),
				ShotsOnTarget: espnStatDisplay(team.Statistics, "shotsOnTarget"),
				Possession:    soccerPossessionDisplay(team.Statistics),
				YellowCards:   espnPositiveStatDisplay(team.Statistics, "yellowCards"),
				RedCards:      espnPositiveStatDisplay(team.Statistics, "redCards"),
			}
		}
	}
	return models.SoccerTeamStats{}
}

func soccerPossessionDisplay(stats []espnStat) string {
	value := strings.TrimSpace(espnStatDisplay(stats, "possessionPct", "possession", "possessionPercentage"))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "%") {
		return value
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	return fmt.Sprintf("%.0f%%", n)
}

func espnPositiveStatDisplay(stats []espnStat, names ...string) string {
	value := espnStatDisplay(stats, names...)
	if value == "" {
		return ""
	}
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n <= 0 {
		return ""
	}
	return value
}

func espnStatDisplay(stats []espnStat, names ...string) string {
	for _, name := range names {
		for _, stat := range stats {
			if strings.EqualFold(stat.Name, name) || strings.EqualFold(stat.Abbreviation, name) || strings.EqualFold(stat.DisplayName, name) || strings.EqualFold(stat.ShortName, name) {
				if strings.TrimSpace(stat.DisplayValue) != "" {
					return stat.DisplayValue
				}
				return strconv.Itoa(int(stat.Value))
			}
		}
	}
	return ""
}

func soccerLineupFromRosters(rosters []espnRoster, awayTeam, homeTeam models.Team) *models.BaseballLineup {
	lineup := &models.BaseballLineup{
		AwayTeam: awayTeam,
		HomeTeam: homeTeam,
	}
	for _, roster := range rosters {
		team := soccerRosterTeam(roster.Team)
		entries := soccerLineupEntries(roster.Roster)
		switch {
		case strings.EqualFold(roster.HomeAway, "away") || sameProviderTeam(roster.Team, awayTeam):
			if team.ID != "" {
				lineup.AwayTeam = team
			}
			lineup.Away = entries
		case strings.EqualFold(roster.HomeAway, "home") || sameProviderTeam(roster.Team, homeTeam):
			if team.ID != "" {
				lineup.HomeTeam = team
			}
			lineup.Home = entries
		}
	}
	if !hasLineupEntries(lineup) {
		return nil
	}
	return lineup
}

func soccerRosterTeam(team espnTeam) models.Team {
	t := espnToTeam(team, models.MLS)
	if t.LogoURL == "" {
		t.LogoURL = strings.TrimSpace(team.Logo)
	}
	return t
}

func soccerLineupEntries(roster []espnRosterAthlete) []models.BaseballLineupEntry {
	entries := make([]models.BaseballLineupEntry, 0, 11)
	for _, player := range roster {
		if !soccerPlayerOnField(player) {
			continue
		}
		name := espnPlayerName(player.Athlete)
		if name == "" {
			continue
		}
		entries = append(entries, models.BaseballLineupEntry{
			Order:          soccerJerseyNumber(player.Jersey),
			Name:           name,
			Position:       soccerPositionLabel(player.Position.Abbreviation, player.Position.DisplayName),
			BattingAverage: "",
		})
	}
	return entries
}

func soccerPlayerOnField(player espnRosterAthlete) bool {
	if player.SubbedOut {
		return false
	}
	return player.Starter || player.SubbedIn
}

func soccerJerseyNumber(jersey string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(jersey))
	return n
}

func soccerPositionLabel(abbr, name string) string {
	if strings.TrimSpace(abbr) != "" {
		return strings.TrimSpace(abbr)
	}
	return strings.TrimSpace(name)
}

func hasSoccerStats(state *models.SoccerState) bool {
	if state == nil {
		return false
	}
	if len(state.Goals) > 0 {
		return true
	}
	for _, stat := range []string{
		state.AwayStats.Shots, state.AwayStats.ShotsOnTarget, state.AwayStats.Possession, state.AwayStats.YellowCards, state.AwayStats.RedCards,
		state.HomeStats.Shots, state.HomeStats.ShotsOnTarget, state.HomeStats.Possession, state.HomeStats.YellowCards, state.HomeStats.RedCards,
	} {
		if strings.TrimSpace(stat) != "" {
			return true
		}
	}
	return false
}

func sameProviderTeam(team espnTeam, model models.Team) bool {
	if team.ID != "" && model.ID != "" && strings.EqualFold(team.ID, model.ID) {
		return true
	}
	name := firstNonEmpty(team.DisplayName, team.ShortDisplayName, team.Location, team.Name, team.Nickname)
	return name != "" && strings.EqualFold(name, strings.TrimSpace(model.City+" "+model.Name))
}

func (s *ESPNStore) GetGameLineup(id string) (*models.BaseballLineup, bool) {
	game, ok := s.GetGameByID(id)
	if !ok {
		return s.fetchSoccerLineupByID(id)
	}
	if isSoccerSport(game.Sport) {
		if game.Lineup != nil && hasCompleteLineupEntries(game.Lineup) {
			return game.Lineup, true
		}
		summaryURL := soccerSummaryURLForSport(game.Sport)
		return s.cachedSoccerLineup(game.ID, summaryURL, game.AwayTeam, game.HomeTeam)
	}
	if game.Sport != models.MLB {
		return nil, false
	}
	if game.Lineup != nil && hasCompleteLineupEntries(game.Lineup) {
		return game.Lineup, true
	}
	return s.cachedMLBLineup(*game)
}

func (s *ESPNStore) fetchSoccerLineupByID(id string) (*models.BaseballLineup, bool) {
	if id == "" {
		return nil, false
	}
	cup := s.GetWorldCup()
	for _, matches := range [][]models.WorldCupMatch{cup.Live, cup.Upcoming, cup.Recent} {
		for _, match := range matches {
			if match.ID != id {
				continue
			}
			if match.Soccer != nil && hasCompleteLineupEntries(match.Soccer.Lineup) {
				return match.Soccer.Lineup, true
			}
			return s.cachedSoccerLineup(id, worldCupSummaryURL, match.AwayTeam, match.HomeTeam)
		}
	}
	return s.cachedSoccerLineup(id, worldCupSummaryURL, models.Team{}, models.Team{})
}

func (s *ESPNStore) GetGameBoxScore(id string) (*models.BoxScore, bool) {
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
	if game.Sport == models.MLB {
		if boxScore := s.fetchMLBBoxScore(*game); boxScore != nil {
			return boxScore, true
		}
	}
	summaryURL := soccerSummaryURLForSport(game.Sport)
	if summaryURL == "" {
		return nil, false
	}
	var summary espnSummaryResp
	if err := s.fetchJSON(fmt.Sprintf(summaryURL, game.ID), &summary); err != nil {
		return nil, false
	}
	boxScore := espnSummaryBoxScore(*game, summary.Boxscore)
	return boxScore, boxScore != nil
}

func worldCupGameByID(cup models.WorldCup, id string) (*models.Game, bool) {
	convert := func(match models.WorldCupMatch) (*models.Game, bool) {
		if match.ID != id {
			return nil, false
		}
		return &models.Game{
			ID:        match.ID,
			HomeTeam:  match.HomeTeam,
			AwayTeam:  match.AwayTeam,
			HomeScore: match.HomeScore,
			AwayScore: match.AwayScore,
			Status:    match.Status,
			Period:    match.Period,
			TimeLeft:  match.TimeLeft,
			StartTime: match.StartTime,
			Venue:     match.Venue,
			City:      match.City,
			Broadcast: match.Broadcast,
			Sport:     models.FIFA,
		}, true
	}
	for _, matches := range [][]models.WorldCupMatch{cup.Live, cup.Recent, cup.Upcoming} {
		for _, match := range matches {
			if game, ok := convert(match); ok {
				return game, true
			}
		}
	}
	for _, rounds := range [][]models.WorldCupRound{cup.Bracket, cup.LeftBracket, cup.RightBracket} {
		for _, round := range rounds {
			for _, match := range round.Matches {
				if game, ok := convert(match); ok {
					return game, true
				}
			}
		}
	}
	return convert(cup.Final)
}

func recentResultGame(results []models.RecentResult, id string) (*models.Game, bool) {
	for _, result := range results {
		if result.GameID != id {
			continue
		}
		phillyScore, opponentScore := recentResultScores(result.Record)
		game := &models.Game{
			ID:        result.GameID,
			Status:    models.StatusFinal,
			StartTime: result.GameDate,
			Sport:     result.Team.Sport,
		}
		if result.Home {
			game.HomeTeam, game.AwayTeam = result.Team, result.Opponent
			game.HomeScore, game.AwayScore = phillyScore, opponentScore
		} else {
			game.HomeTeam, game.AwayTeam = result.Opponent, result.Team
			game.HomeScore, game.AwayScore = opponentScore, phillyScore
		}
		return game, true
	}
	return nil, false
}

func recentResultScores(record string) (int, int) {
	fields := strings.Fields(record)
	if len(fields) < 2 {
		return 0, 0
	}
	parts := strings.SplitN(fields[len(fields)-1], "-", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	phillyScore, _ := strconv.Atoi(parts[0])
	opponentScore, _ := strconv.Atoi(parts[1])
	return phillyScore, opponentScore
}

func (s *ESPNStore) fetchMLBBoxScore(game models.Game) *models.BoxScore {
	gamePk := s.findMLBGamePk(game)
	if gamePk == 0 {
		return nil
	}
	var feed mlbLiveFeedResp
	if err := s.fetchJSON(fmt.Sprintf(mlbLiveFeedURL, gamePk), &feed); err != nil {
		return nil
	}
	boxScore := &models.BoxScore{AwayTeam: game.AwayTeam, HomeTeam: game.HomeTeam}
	columns := make([]string, 0, len(feed.LiveData.Linescore.Innings)+4)
	awayValues := make([]string, 0, len(columns))
	homeValues := make([]string, 0, len(columns))
	for _, inning := range feed.LiveData.Linescore.Innings {
		columns = append(columns, strconv.Itoa(inning.Num))
		awayValues = append(awayValues, optionalInt(inning.Away.Runs))
		homeValues = append(homeValues, optionalInt(inning.Home.Runs))
	}
	columns = append(columns, "R", "H", "E", "LOB")
	awayTotals := feed.LiveData.Linescore.Teams.Away
	homeTotals := feed.LiveData.Linescore.Teams.Home
	awayValues = append(awayValues, strconv.Itoa(awayTotals.Runs), strconv.Itoa(awayTotals.Hits), strconv.Itoa(awayTotals.Errors), strconv.Itoa(awayTotals.LeftOnBase))
	homeValues = append(homeValues, strconv.Itoa(homeTotals.Runs), strconv.Itoa(homeTotals.Hits), strconv.Itoa(homeTotals.Errors), strconv.Itoa(homeTotals.LeftOnBase))
	boxScore.Sections = append(boxScore.Sections, models.BoxScoreSection{
		Title: "Linescore", Columns: columns,
		Rows: []models.BoxScoreRow{
			{Label: game.AwayTeam.Abbr, Values: awayValues},
			{Label: game.HomeTeam.Abbr, Values: homeValues},
		},
	})
	for _, team := range []struct {
		model models.Team
		data  mlbBoxscoreTeam
	}{
		{game.AwayTeam, feed.LiveData.Boxscore.Teams.Away},
		{game.HomeTeam, feed.LiveData.Boxscore.Teams.Home},
	} {
		if batting := mlbBattingSection(team.model, team.data); len(batting.Rows) > 0 {
			boxScore.Sections = append(boxScore.Sections, batting)
		}
		if pitching := mlbPitchingSection(team.model, team.data); len(pitching.Rows) > 0 {
			boxScore.Sections = append(boxScore.Sections, pitching)
		}
	}
	if len(boxScore.Sections) == 1 && len(columns) == 4 {
		return nil
	}
	return boxScore
}

func optionalInt(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

func mlbBattingSection(team models.Team, data mlbBoxscoreTeam) models.BoxScoreSection {
	section := models.BoxScoreSection{
		Title: "Batting", Team: team,
		Columns: []string{"AB", "R", "H", "RBI", "BB", "SO", "HR"},
	}
	seen := map[int]bool{}
	for _, id := range data.Batters {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		player, ok := data.Players["ID"+strconv.Itoa(id)]
		if !ok || strings.TrimSpace(player.Person.FullName) == "" {
			continue
		}
		stats := player.Stats.Batting
		section.Rows = append(section.Rows, models.BoxScoreRow{
			Label: player.Person.FullName,
			Values: []string{
				strconv.Itoa(stats.AtBats), strconv.Itoa(stats.Runs), strconv.Itoa(stats.Hits),
				strconv.Itoa(stats.RBI), strconv.Itoa(stats.BaseOnBalls), strconv.Itoa(stats.StrikeOuts),
				strconv.Itoa(stats.HomeRuns),
			},
		})
	}
	return section
}

func mlbPitchingSection(team models.Team, data mlbBoxscoreTeam) models.BoxScoreSection {
	section := models.BoxScoreSection{
		Title: "Pitching", Team: team,
		Columns: []string{"IP", "H", "R", "ER", "BB", "SO", "HR", "P"},
	}
	for _, id := range data.Pitchers {
		player, ok := data.Players["ID"+strconv.Itoa(id)]
		if !ok || strings.TrimSpace(player.Person.FullName) == "" {
			continue
		}
		stats := player.Stats.Pitching
		strikeouts := 0
		if stats.StrikeOuts != nil {
			strikeouts = *stats.StrikeOuts
		}
		section.Rows = append(section.Rows, models.BoxScoreRow{
			Label: player.Person.FullName,
			Values: []string{
				stats.InningsPitched, strconv.Itoa(stats.Hits), strconv.Itoa(stats.Runs),
				strconv.Itoa(stats.EarnedRuns), strconv.Itoa(stats.BaseOnBalls), strconv.Itoa(strikeouts),
				strconv.Itoa(stats.HomeRuns), strconv.Itoa(stats.NumberOfPitches),
			},
		})
	}
	return section
}

func espnSummaryBoxScore(game models.Game, source espnBoxscore) *models.BoxScore {
	boxScore := &models.BoxScore{AwayTeam: game.AwayTeam, HomeTeam: game.HomeTeam}
	for _, sourceTeam := range source.Players {
		team := game.AwayTeam
		if sameProviderTeam(sourceTeam.Team, game.HomeTeam) {
			team = game.HomeTeam
		}
		for _, group := range sourceTeam.Statistics {
			columns := group.Labels
			if len(columns) == 0 {
				columns = group.Names
			}
			section := models.BoxScoreSection{
				Title: firstNonEmpty(group.DisplayName, group.Name, "Player Stats"),
				Team:  team, Columns: columns,
			}
			for _, athlete := range group.Athletes {
				name := espnPlayerName(athlete.Athlete)
				if name == "" {
					continue
				}
				section.Rows = append(section.Rows, models.BoxScoreRow{Label: name, Values: athlete.Stats})
			}
			if len(section.Rows) > 0 {
				boxScore.Sections = append(boxScore.Sections, section)
			}
		}
	}
	if len(source.Teams) == 2 {
		awayStats, homeStats := source.Teams[0], source.Teams[1]
		if sameProviderTeam(awayStats.Team, game.HomeTeam) {
			awayStats, homeStats = homeStats, awayStats
		}
		rows := teamBoxScoreRows(game.Sport, awayStats.Statistics, homeStats.Statistics)
		title := "Team Stats"
		if isSoccerSport(game.Sport) {
			title = "Match Stats"
		}
		if len(rows) > 0 {
			boxScore.Sections = append([]models.BoxScoreSection{{
				Title:   title,
				Columns: []string{game.AwayTeam.Abbr, game.HomeTeam.Abbr},
				Rows:    rows,
			}}, boxScore.Sections...)
		}
	}
	if len(boxScore.Sections) == 0 {
		return nil
	}
	return boxScore
}

func teamBoxScoreRows(sport models.Sport, awayStats, homeStats []espnStat) []models.BoxScoreRow {
	if isSoccerSport(sport) {
		return soccerBoxScoreRows(awayStats, homeStats)
	}
	rows := make([]models.BoxScoreRow, 0, len(awayStats))
	for _, awayStat := range awayStats {
		rows = append(rows, models.BoxScoreRow{
			Label:  humanizeStatLabel(firstNonEmpty(awayStat.DisplayName, awayStat.Abbreviation, awayStat.Name)),
			Values: []string{espnStatValue(awayStat, false), espnStatDisplay(homeStats, awayStat.Name)},
		})
	}
	return rows
}

func soccerBoxScoreRows(awayStats, homeStats []espnStat) []models.BoxScoreRow {
	type statSpec struct {
		label   string
		percent bool
		names   []string
	}
	specs := []statSpec{
		{label: "Possession", percent: true, names: []string{"possessionPct", "possession"}},
		{label: "Shots", names: []string{"totalShots", "shots"}},
		{label: "Shots on target", names: []string{"shotsOnTarget"}},
		{label: "Corners", names: []string{"wonCorners", "corners"}},
		{label: "Pass accuracy", percent: true, names: []string{"passPct", "passAccuracy"}},
		{label: "Fouls", names: []string{"foulsCommitted", "fouls"}},
		{label: "Offsides", names: []string{"offsides"}},
		{label: "Saves", names: []string{"saves"}},
		{label: "Yellow cards", names: []string{"yellowCards"}},
		{label: "Red cards", names: []string{"redCards"}},
	}
	rows := make([]models.BoxScoreRow, 0, len(specs)+1)
	for _, spec := range specs {
		away, awayOK := findESPNStat(awayStats, spec.names...)
		home, homeOK := findESPNStat(homeStats, spec.names...)
		if !awayOK && !homeOK {
			continue
		}
		rows = append(rows, models.BoxScoreRow{
			Label: spec.label,
			Values: []string{
				espnStatValue(away, spec.percent),
				espnStatValue(home, spec.percent),
			},
		})
	}
	awayAccurate, awayAccurateOK := findESPNStat(awayStats, "accuratePasses")
	awayTotal, awayTotalOK := findESPNStat(awayStats, "totalPasses")
	homeAccurate, homeAccurateOK := findESPNStat(homeStats, "accuratePasses")
	homeTotal, homeTotalOK := findESPNStat(homeStats, "totalPasses")
	if (awayAccurateOK && awayTotalOK) || (homeAccurateOK && homeTotalOK) {
		insertAt := 5
		if len(rows) < insertAt {
			insertAt = len(rows)
		}
		rows = append(rows[:insertAt], append([]models.BoxScoreRow{{
			Label: "Completed passes",
			Values: []string{
				fmt.Sprintf("%s / %s", espnStatValue(awayAccurate, false), espnStatValue(awayTotal, false)),
				fmt.Sprintf("%s / %s", espnStatValue(homeAccurate, false), espnStatValue(homeTotal, false)),
			},
		}}, rows[5:]...)...)
	}
	return rows
}

func findESPNStat(stats []espnStat, names ...string) (espnStat, bool) {
	for _, name := range names {
		for _, stat := range stats {
			if strings.EqualFold(stat.Name, name) || strings.EqualFold(stat.Abbreviation, name) ||
				strings.EqualFold(stat.DisplayName, name) || strings.EqualFold(stat.ShortName, name) {
				return stat, true
			}
		}
	}
	return espnStat{}, false
}

func espnStatValue(stat espnStat, percent bool) string {
	display := strings.TrimSpace(stat.DisplayValue)
	value := stat.Value
	if percent {
		if display != "" {
			parsed, err := strconv.ParseFloat(strings.TrimSuffix(display, "%"), 64)
			if err == nil {
				value = parsed
			}
		}
		if value > 0 && value <= 1 {
			value *= 100
		}
		return strconv.FormatFloat(value, 'f', 1, 64) + "%"
	}
	if display != "" {
		return display
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func humanizeStatLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Stat"
	}
	var out []rune
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, ' ')
		}
		out = append(out, r)
	}
	label := strings.ToLower(string(out))
	return strings.ToUpper(label[:1]) + label[1:]
}

func soccerSummaryURLForSport(sport models.Sport) string {
	switch sport {
	case models.MLS:
		for _, cfg := range sportConfigs {
			if cfg.Sport == models.MLS {
				return cfg.SummaryURL
			}
		}
	case models.FIFA:
		return worldCupSummaryURL
	}
	return ""
}

func isSoccerSport(sport models.Sport) bool {
	return sport == models.MLS || sport == models.FIFA
}

func (s *ESPNStore) prefetchUpcomingLineups(games []models.Game, now time.Time) {
	for i := range games {
		if !shouldPrefetchLineup(games[i], now) {
			continue
		}
		if lineup, ok := s.cachedMLBLineup(games[i]); ok {
			games[i].Lineup = lineup
		}
	}
}

func (s *ESPNStore) attachUpcomingProbablePitchers(games []models.Game) {
	for i := range games {
		if games[i].Sport != models.MLB || games[i].Probable != nil {
			continue
		}
		games[i].Probable = s.fetchMLBProbablePitchers(games[i])
	}
}

func shouldPrefetchLineup(g models.Game, now time.Time) bool {
	if g.Sport != models.MLB || g.Lineup != nil {
		return false
	}
	if !isSamePhillyDay(g.StartTime, now) {
		return false
	}
	start := PhillyTime(g.StartTime)
	if start.Before(now) {
		return false
	}
	return !start.After(now.Add(lineupPrefetchWindow))
}

func shouldPrefetchWorldCupLineup(match models.WorldCupMatch, now time.Time) bool {
	if match.Status != models.StatusScheduled || (match.Soccer != nil && hasCompleteLineupEntries(match.Soccer.Lineup)) {
		return false
	}
	start := PhillyTime(match.StartTime)
	if start.Before(now) {
		return false
	}
	return !start.After(now.Add(lineupPrefetchWindow))
}

func isSamePhillyDay(t, ref time.Time) bool {
	tt := PhillyTime(t)
	rr := PhillyTime(ref)
	ty, tm, td := tt.Date()
	ry, rm, rd := rr.Date()
	return ty == ry && tm == rm && td == rd
}

func (s *ESPNStore) cachedMLBLineup(game models.Game) (*models.BaseballLineup, bool) {
	now := time.Now()
	s.mu.RLock()
	if cached, ok := s.lineupCache[game.ID]; ok && now.Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.lineup, hasLineupEntries(cached.lineup)
	}
	s.mu.RUnlock()

	lineup := s.fetchMLBLineup(game)
	ttl := lineupCacheTTL(lineup)

	s.mu.Lock()
	s.lineupCache[game.ID] = lineupCacheEntry{lineup: lineup, expiresAt: now.Add(ttl)}
	s.mu.Unlock()

	return lineup, hasLineupEntries(lineup)
}

func (s *ESPNStore) cachedSoccerLineup(eventID, summaryURL string, awayTeam, homeTeam models.Team) (*models.BaseballLineup, bool) {
	if eventID == "" || summaryURL == "" {
		return nil, false
	}
	now := time.Now()
	s.mu.RLock()
	if cached, ok := s.lineupCache[eventID]; ok && now.Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.lineup, hasCompleteLineupEntries(cached.lineup)
	}
	s.mu.RUnlock()

	var lineup *models.BaseballLineup
	if state := s.fetchSoccerState(eventID, summaryURL, awayTeam, homeTeam); state != nil {
		lineup = state.Lineup
	}

	s.mu.Lock()
	s.lineupCache[eventID] = lineupCacheEntry{lineup: lineup, expiresAt: now.Add(soccerLineupCacheTTL(lineup))}
	s.mu.Unlock()

	return lineup, hasCompleteLineupEntries(lineup)
}

func lineupCacheTTL(lineup *models.BaseballLineup) time.Duration {
	if hasCompleteLineupEntries(lineup) {
		return 12 * time.Hour
	}
	if hasLineupEntries(lineup) {
		return 2 * time.Minute
	}
	return 10 * time.Minute
}

func soccerLineupCacheTTL(lineup *models.BaseballLineup) time.Duration {
	if hasCompleteLineupEntries(lineup) {
		return 12 * time.Hour
	}
	return 30 * time.Second
}

func (s *ESPNStore) fetchMLBLineup(g models.Game) *models.BaseballLineup {
	gamePk := s.findMLBGamePk(g)
	if gamePk == 0 {
		return nil
	}
	var feed mlbLiveFeedResp
	if err := s.fetchJSON(fmt.Sprintf(mlbLiveFeedURL, gamePk), &feed); err != nil {
		return nil
	}
	return mlbLineupFromFeed(feed, g)
}

func (s *ESPNStore) fetchMLBProbablePitchers(g models.Game) *models.BaseballProbablePitchers {
	game := s.findMLBScheduleGame(g)
	if game.GamePk == 0 {
		return nil
	}
	probable := &models.BaseballProbablePitchers{
		Away: mlbProbablePitcher(game.Teams.Away.ProbablePitcher),
		Home: mlbProbablePitcher(game.Teams.Home.ProbablePitcher),
	}
	if probable.Away.Name == "" && probable.Home.Name == "" {
		return nil
	}
	return probable
}

func mlbProbablePitcher(person mlbPerson) models.BaseballLineupPitcher {
	name := strings.TrimSpace(person.FullName)
	if name == "" {
		return models.BaseballLineupPitcher{}
	}
	handedness := strings.TrimSpace(person.PitchHand.Code)
	if handedness == "" {
		handedness = strings.TrimSpace(person.PitchHand.Description)
	}
	return models.BaseballLineupPitcher{Name: name, Handedness: handedness}
}

func mlbLineupFromFeed(feed mlbLiveFeedResp, g models.Game) *models.BaseballLineup {
	lineup := &models.BaseballLineup{
		AwayTeam:    g.AwayTeam,
		HomeTeam:    g.HomeTeam,
		AwayPitcher: mlbStartingPitcher(feed.LiveData.Boxscore.Teams.Away),
		HomePitcher: mlbStartingPitcher(feed.LiveData.Boxscore.Teams.Home),
		Away:        mlbLineupEntries(feed.LiveData.Boxscore.Teams.Away),
		Home:        mlbLineupEntries(feed.LiveData.Boxscore.Teams.Home),
	}
	if !hasLineupEntries(lineup) {
		return nil
	}
	return lineup
}

func mlbStartingPitcher(team mlbBoxscoreTeam) models.BaseballLineupPitcher {
	for _, id := range team.Pitchers {
		if id == 0 {
			continue
		}
		player, ok := team.Players["ID"+strconv.Itoa(id)]
		if !ok {
			continue
		}
		if pitcher := mlbLineupPitcher(player); pitcher.Name != "" {
			return pitcher
		}
	}
	for _, player := range team.Players {
		position := strings.TrimSpace(player.Position.Abbreviation)
		if position != "P" {
			continue
		}
		if pitcher := mlbLineupPitcher(player); pitcher.Name != "" {
			return pitcher
		}
	}
	return models.BaseballLineupPitcher{}
}

func mlbLineupPitcher(player mlbBoxscorePlayer) models.BaseballLineupPitcher {
	name := strings.TrimSpace(player.Person.FullName)
	if name == "" {
		return models.BaseballLineupPitcher{}
	}
	handedness := strings.TrimSpace(player.PitchHand.Code)
	if handedness == "" {
		handedness = strings.TrimSpace(player.PitchHand.Description)
	}
	return models.BaseballLineupPitcher{
		Name:       name,
		Handedness: handedness,
		ERA:        mlbPitcherERA(player),
	}
}

func mlbPitcherERA(player mlbBoxscorePlayer) string {
	for _, value := range []string{
		player.SeasonStats.Pitching.ERA,
		player.CareerStats.Pitching.ERA,
		player.Stats.Pitching.ERA,
	} {
		if stat := strings.TrimSpace(value); stat != "" && stat != ".---" && stat != "-.--" {
			return stat
		}
	}
	return ""
}

func mlbBattingAverage(player mlbBoxscorePlayer) string {
	for _, value := range []string{
		player.SeasonStats.Batting.Avg,
		player.CareerStats.Batting.Avg,
		player.Stats.Batting.Avg,
	} {
		if stat := strings.TrimSpace(value); stat != "" && stat != ".---" && stat != "---" {
			return stat
		}
	}
	return ""
}

func mlbLineupEntries(team mlbBoxscoreTeam) []models.BaseballLineupEntry {
	playerIDs := team.BattingOrder
	if len(playerIDs) == 0 {
		playerIDs = team.Batters
	}
	entries := make([]models.BaseballLineupEntry, 0, len(playerIDs))
	seen := map[int]bool{}
	for _, id := range playerIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		player, ok := team.Players["ID"+strconv.Itoa(id)]
		if !ok {
			continue
		}
		name := strings.TrimSpace(player.Person.FullName)
		if name == "" {
			continue
		}
		order := len(entries) + 1
		if parsed, err := strconv.Atoi(strings.TrimSpace(player.BattingOrder)); err == nil && parsed > 0 {
			order = parsed / 100
			if order == 0 {
				order = parsed
			}
		}
		position := strings.TrimSpace(player.Position.Abbreviation)
		if position == "" {
			position = strings.TrimSpace(player.Position.Name)
		}
		entries = append(entries, models.BaseballLineupEntry{
			Order:          order,
			Name:           name,
			Position:       position,
			BattingAverage: mlbBattingAverage(player),
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Order == entries[j].Order {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Order < entries[j].Order
	})
	if len(entries) > 9 {
		entries = entries[:9]
	}
	return entries
}

func hasLineupEntries(lineup *models.BaseballLineup) bool {
	return lineup != nil && (len(lineup.Away) > 0 || len(lineup.Home) > 0)
}

func hasCompleteLineupEntries(lineup *models.BaseballLineup) bool {
	return lineup != nil && len(lineup.Away) > 0 && len(lineup.Home) > 0
}

func mlbPitcherStrikeouts(feed mlbLiveFeedResp, pitcher mlbPerson) string {
	if pitcher.ID == 0 && strings.TrimSpace(pitcher.FullName) == "" {
		return ""
	}
	for _, team := range []mlbBoxscoreTeam{feed.LiveData.Boxscore.Teams.Away, feed.LiveData.Boxscore.Teams.Home} {
		for _, player := range team.Players {
			if !mlbPersonMatches(player.Person, pitcher) {
				continue
			}
			if player.Stats.Pitching.StrikeOuts == nil {
				return ""
			}
			return strconv.Itoa(*player.Stats.Pitching.StrikeOuts)
		}
	}
	return ""
}

func mlbPersonMatches(a, b mlbPerson) bool {
	if a.ID != 0 && b.ID != 0 {
		return a.ID == b.ID
	}
	return normalizePlayerName(a.FullName) != "" && normalizePlayerName(a.FullName) == normalizePlayerName(b.FullName)
}

func (s *ESPNStore) findMLBGamePk(g models.Game) int {
	return s.findMLBScheduleGame(g).GamePk
}

func (s *ESPNStore) findMLBScheduleGame(g models.Game) mlbScheduleGame {
	date := PhillyTime(g.StartTime).Format("2006-01-02")
	url := fmt.Sprintf(mlbScheduleURL, date)
	var schedule mlbScheduleResp
	if err := s.fetchJSON(url, &schedule); err != nil {
		return mlbScheduleGame{}
	}
	for _, d := range schedule.Dates {
		for _, game := range d.Games {
			if mlbScheduleMatchesGame(game, g) {
				return game
			}
		}
	}
	return mlbScheduleGame{}
}

func mlbScheduleMatchesGame(mlbGame mlbScheduleGame, g models.Game) bool {
	homeName := strings.ToLower(mlbGame.Teams.Home.Team.Name)
	awayName := strings.ToLower(mlbGame.Teams.Away.Team.Name)
	return teamMatchesMLBName(g.HomeTeam, homeName) && teamMatchesMLBName(g.AwayTeam, awayName)
}

func teamMatchesMLBName(team models.Team, mlbName string) bool {
	if mlbName == "" {
		return false
	}
	for _, value := range []string{team.City, team.Name, team.Abbr, team.City + " " + team.Name} {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && strings.Contains(mlbName, value) {
			return true
		}
	}
	return false
}

func latestMLBPlays(plays []mlbPlay, max int) []models.BaseballPlay {
	if max <= 0 {
		return nil
	}
	out := make([]models.BaseballPlay, 0, max)
	for i := len(plays) - 1; i >= 0 && len(out) < max; i-- {
		desc := cleanMLBPlayDescription(plays[i].Result.Description)
		if desc == "" {
			desc = cleanMLBPlayDescription(plays[i].Result.Event)
		}
		if desc == "" {
			continue
		}
		out = append(out, models.BaseballPlay{
			Inning:      plays[i].About.Inning,
			HalfInning:  formatHalfInning(plays[i].About.HalfInning),
			Description: desc,
		})
	}
	return out
}

func formatHalfInning(half string) string {
	half = strings.ToLower(strings.TrimSpace(half))
	if half == "" {
		return ""
	}
	return strings.ToUpper(half[:1]) + half[1:]
}

func cleanMLBPlayDescription(desc string) string {
	return strings.TrimSpace(desc)
}

func (s *ESPNStore) fetchPitcherStrikeouts(eventID, pitcherName string) string {
	var summary espnSummaryResp
	url := fmt.Sprintf("https://site.web.api.espn.com/apis/site/v2/sports/baseball/mlb/summary?event=%s", eventID)
	if err := s.fetchJSON(url, &summary); err != nil {
		return ""
	}
	return pitcherStrikeouts(summary.Boxscore, pitcherName)
}

func pitcherStrikeouts(boxscore espnBoxscore, pitcherName string) string {
	pitcherName = normalizePlayerName(pitcherName)
	if pitcherName == "" {
		return ""
	}

	for _, team := range boxscore.Players {
		for _, group := range team.Statistics {
			kIndex := statIndex(group.Names, "K")
			if kIndex < 0 {
				continue
			}
			for _, row := range group.Athletes {
				if normalizePlayerName(espnPlayerName(row.Athlete)) != pitcherName {
					continue
				}
				if kIndex < len(row.Stats) {
					return strings.TrimSpace(row.Stats[kIndex])
				}
				return ""
			}
		}
	}
	return ""
}

func statIndex(names []string, want string) int {
	for i, name := range names {
		if strings.EqualFold(strings.TrimSpace(name), want) {
			return i
		}
	}
	return -1
}

func normalizePlayerName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

func espnPlayerName(player espnPlayer) string {
	for _, name := range []string{
		player.DisplayName,
		player.ShortName,
		player.FullName,
		player.Name,
		player.Athlete.DisplayName,
		player.Athlete.ShortName,
		player.Athlete.FullName,
		player.Athlete.Name,
	} {
		name = strings.TrimSpace(name)
		if name != "" {
			return name
		}
	}
	return ""
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
	for _, result := range s.GetRecentResults() {
		if !isInSeason(result.Team.Sport) {
			continue
		}
		keys[string(result.Team.Sport)+":"+result.Team.ID] = true
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
		models.NHL: {"15": Flyers},
		models.MLS: {"10739": Union},
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
		abbr = strings.ToLower(team.Name)
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
	case models.FIFA:
		return worldCupFlagLogoURL(team)
	}
	return ""
}

func worldCupFlagLogoURL(team models.Team) string {
	code := worldCupFlagCode(team.Abbr)
	if code == "" {
		code = worldCupFlagCode(team.Name)
	}
	if code == "" {
		return ""
	}
	return "https://flagcdn.com/w80/" + code + ".png"
}

func worldCupFlagCode(value string) string {
	key := strings.ToUpper(strings.TrimSpace(value))
	key = strings.NewReplacer(
		"Á", "A",
		"É", "E",
		"Í", "I",
		"Ó", "O",
		"Ú", "U",
		"Ç", "C",
		"'", "",
		".", "",
		"-", " ",
	).Replace(key)
	key = strings.Join(strings.Fields(key), " ")
	switch key {
	case "ALG":
		return "dz"
	case "ARG":
		return "ar"
	case "AUS":
		return "au"
	case "AUT":
		return "at"
	case "BEL":
		return "be"
	case "BIH":
		return "ba"
	case "BRA":
		return "br"
	case "CAN":
		return "ca"
	case "CIV":
		return "ci"
	case "COL":
		return "co"
	case "COD":
		return "cd"
	case "CGO":
		return "cg"
	case "CPV":
		return "cv"
	case "CRC":
		return "cr"
	case "CRO":
		return "hr"
	case "CUW":
		return "cw"
	case "CZE":
		return "cz"
	case "DEN":
		return "dk"
	case "ECU":
		return "ec"
	case "EGY":
		return "eg"
	case "ENG":
		return "gb-eng"
	case "ESP":
		return "es"
	case "FRA":
		return "fr"
	case "GER":
		return "de"
	case "GHA":
		return "gh"
	case "HAI":
		return "ht"
	case "HON":
		return "hn"
	case "IRN":
		return "ir"
	case "IRQ":
		return "iq"
	case "ITA":
		return "it"
	case "JAM":
		return "jm"
	case "JOR":
		return "jo"
	case "JPN":
		return "jp"
	case "KOR":
		return "kr"
	case "MAR":
		return "ma"
	case "MEX":
		return "mx"
	case "NED":
		return "nl"
	case "NZL":
		return "nz"
	case "NOR":
		return "no"
	case "PAN":
		return "pa"
	case "PAR":
		return "py"
	case "PER":
		return "pe"
	case "POR":
		return "pt"
	case "QAT":
		return "qa"
	case "RSA":
		return "za"
	case "KSA":
		return "sa"
	case "SCO":
		return "gb-sct"
	case "SEN":
		return "sn"
	case "SRB":
		return "rs"
	case "SWE":
		return "se"
	case "SUI":
		return "ch"
	case "TUN":
		return "tn"
	case "TUR":
		return "tr"
	case "UKR":
		return "ua"
	case "UAE":
		return "ae"
	case "UZB":
		return "uz"
	case "URU":
		return "uy"
	case "USA":
		return "us"
	case "WAL":
		return "gb-wls"
	}
	if code, ok := map[string]string{
		"ALGERIA":                      "dz",
		"ARGENTINA":                    "ar",
		"AUSTRALIA":                    "au",
		"AUSTRIA":                      "at",
		"BELGIUM":                      "be",
		"BOSNIA AND HERZEGOVINA":       "ba",
		"BOSNIA HERZEGOVINA":           "ba",
		"BOSNIA-HERZEGOVINA":           "ba",
		"BRAZIL":                       "br",
		"CANADA":                       "ca",
		"CAPE VERDE":                   "cv",
		"COLOMBIA":                     "co",
		"CONGO":                        "cg",
		"CONGO DR":                     "cd",
		"COSTA RICA":                   "cr",
		"CROATIA":                      "hr",
		"CURACAO":                      "cw",
		"CZECHIA":                      "cz",
		"DENMARK":                      "dk",
		"ECUADOR":                      "ec",
		"EGYPT":                        "eg",
		"ENGLAND":                      "gb-eng",
		"FRANCE":                       "fr",
		"GERMANY":                      "de",
		"GHANA":                        "gh",
		"HAITI":                        "ht",
		"HONDURAS":                     "hn",
		"IRAN":                         "ir",
		"IRAQ":                         "iq",
		"ITALY":                        "it",
		"IVORY COAST":                  "ci",
		"JAMAICA":                      "jm",
		"JAPAN":                        "jp",
		"JORDAN":                       "jo",
		"MEXICO":                       "mx",
		"MOROCCO":                      "ma",
		"NETHERLANDS":                  "nl",
		"NEW ZEALAND":                  "nz",
		"NORWAY":                       "no",
		"PANAMA":                       "pa",
		"PARAGUAY":                     "py",
		"PORTUGAL":                     "pt",
		"QATAR":                        "qa",
		"SAUDI ARABIA":                 "sa",
		"SCOTLAND":                     "gb-sct",
		"SENEGAL":                      "sn",
		"SERBIA":                       "rs",
		"SOUTH AFRICA":                 "za",
		"SOUTH KOREA":                  "kr",
		"DR CONGO":                     "cd",
		"DEMOCRATIC REPUBLIC OF CONGO": "cd",
		"SPAIN":                        "es",
		"SWEDEN":                       "se",
		"SWITZERLAND":                  "ch",
		"TUNISIA":                      "tn",
		"TURKEY":                       "tr",
		"UKRAINE":                      "ua",
		"UNITED STATES":                "us",
		"URUGUAY":                      "uy",
		"UZBEKISTAN":                   "uz",
		"WALES":                        "gb-wls",
	}[key]; ok {
		return code
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
		// October - May
		return m >= time.October || m <= time.May
	case models.NHL:
		// October - May
		return m >= time.October || m <= time.May
	case models.MLS:
		// March – November
		return m >= time.March && m <= time.November
	}
	return false
}
