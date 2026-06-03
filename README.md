# Philly Gametime

Go web app for quick-glance Philadelphia sports scores, schedules, standings, recent results, and TV/stream info.

The app covers the Eagles, Phillies, 76ers, Flyers, and Union. It uses ESPN-backed data by default, prioritizes local Philly broadcasts when available, and displays all game times in Philadelphia time (`America/New_York`).

## Features

- Home dashboard with today's Philly action, upcoming games, recent results, and standings
- Live scores page
- Live MLB game state with base runners, balls, strikes, outs, current batter, current pitcher, and the pitcher's current-game strikeouts while games are live
- Final MLB games collapse back to score-only cards instead of showing stale count/base/play state
- Upcoming schedule page
- Full schedule page with team filtering, month controls, mobile agenda layout, and next-game jump behavior
- Team directory plus team detail pages with live/next game, upcoming games, standings, and recent result
- Post-game recap highlight links for recent results, preferring one short recap/game-highlights video when available
- Stats/standings page with league standings by sport plus division/conference/full-league scopes when available
- TV/stream guide with Philly-first broadcast sorting
- Theme picker with Basic, Light, Dark, Midnight, and multiple Neon modes
- Server-sent events endpoint for score and sport-specific event hooks
- Mock data mode for local design/development previews

## Requirements

- Go 1.22 or newer
- Network access for live ESPN-backed data and MLB highlight metadata
- OpenSSH client for Lightsail deploys (`ssh` and `scp`)

## Run Locally

Live ESPN-backed provider:

```powershell
go run .
```

Open:

```text
http://localhost:8080
```

Use a custom port:

```powershell
$env:PORT="8090"
go run .
```

## Mock Data Mode

Seeded preview data is available only when explicitly requested:

```powershell
$env:PHILLY_DATA="mock"
$env:PORT="8090"
go run .
```

Open:

```text
http://localhost:8090
```

Mock mode includes a live Phillies game with baseball count state and pitcher strikeouts so the live-game UI can be verified locally. It also includes team schedules, standings, recent results, and team detail data for local page previews.

To return to live data in the same shell:

```powershell
Remove-Item Env:\PHILLY_DATA
go run .
```

## Optional AI Recap Cleanup

Post-game recent-result bullets can be cleaned up with the OpenAI API. This is optional; without an API key, the app uses the ESPN description and local fallback bullets.

Recommended model: `gpt-4.1-mini` for lower latency. `gpt-5-nano` also works if available, but may be slower for this small cleanup task.

```powershell
$env:OPENAI_API_KEY="your_api_key"
$env:OPENAI_MODEL="gpt-4.1-mini"
$env:OPENAI_TIMEOUT_SECONDS="30"
$env:AI_RECAP_CACHE_PATH="ai-recap-cache.json"
$env:AI_RECAP_CACHE_MAX_ENTRIES="100"
go run .
```

Environment variables:

- `OPENAI_API_KEY`: enables OpenAI recap cleanup.
- `OPENAI_MODEL`: model used for cleanup. Defaults to `gpt-5-nano` if unset.
- `OPENAI_TIMEOUT_SECONDS`: timeout for OpenAI calls. Defaults to `30`.
- `OPENAI_BASE_URL`: optional override for compatible APIs. Defaults to `https://api.openai.com/v1`.
- `AI_RECAP_CACHE_PATH`: file path for persisted recap bullets. Defaults to `./ai-recap-cache.json`.
- `AI_RECAP_CACHE_MAX_ENTRIES`: max successful game recaps to keep in the cache file. Defaults to `100`.
- `HIGHLIGHT_CACHE_PATH`: file path for persisted post-game highlight lookup state. Defaults to `./highlight-cache.json`.
- `HIGHLIGHT_CACHE_MAX_ENTRIES`: max game highlight entries to keep in the cache file. Defaults to `200`.

Order of operations:

1. Get completed game data and provider description from ESPN.
2. Render the page immediately with ESPN/local fallback bullets.
3. Start OpenAI cleanup in the background for only the recent-result cards that are actually displayed.
4. Save successful cleaned bullets by ESPN game ID.
5. Use cached cleaned bullets on the next refresh or app restart.

The cache file does not grow forever by default. It keeps the newest `AI_RECAP_CACHE_MAX_ENTRIES` successful recaps and prunes older entries whenever it saves. Failed calls are not cached. To clear the cache while tuning prompts/models:

```powershell
Remove-Item .\ai-recap-cache.json -ErrorAction SilentlyContinue
```

Do not expose `OPENAI_API_KEY` in browser code or commit it to the repo.

## Post-Game Highlights

Recent-result cards can show one official provider highlight link after a game ends.

Provider behavior:

- MLB/Phillies games use MLB StatsAPI content when available.
- MLB videos prefer a short game recap/highlights video, then `Condensed Game`, then the first available MLB video.
- MLB duration metadata is used to avoid very short single-play clips and long condensed games when a better recap is available.
- ESPN-backed videos for other sports prefer `Game Highlights`, `Match Highlights`, or `Extended Highlights`, then recap/highlights videos, then the first available video.
- The app links to provider-hosted video URLs and does not download or rehost video.

Retry behavior:

- If a completed game has no video yet, the card can show `Highlights pending. Checking again soon.`
- Pending highlights retry after about 15 minutes.
- Found highlights refresh about every 45 minutes for the first 12 hours after game time so a better recap can replace an early clip.
- After that upgrade window, found highlights are cached for 24 hours and persisted to `HIGHLIGHT_CACHE_PATH`.
- Games older than 48 hours stop retrying if no highlight was found.

## Build And Test

```powershell
go test ./...
go build ./...
```

## Routes

Pages:

- `GET /`
- `GET /scores`
- `GET /upcoming`
- `GET /schedule`
- `GET /teams`
- `GET /teams/{id}` where `{id}` is `eagles`, `phillies`, `sixers`, `flyers`, or `union`
- `GET /stats`
- `GET /tv`

Navigation behavior:

- Footer team links open the corresponding team detail page.
- Team detail pages link to `/schedule#team-id` for the full filtered schedule.
- The schedule page reads the hash (`/schedule#phillies`, `/schedule#eagles`, etc.) and selects that team.
- On mobile, the schedule agenda scrolls to today's game day when present; otherwise it scrolls to the next available game day.

API and events:

- `GET /api/scores`
- `GET /api/upcoming`
- `GET /api/standings`
- `GET /events`

## Event Types

The `events.Bus` publishes:

- `score_update`
- `game_start`
- `game_end`
- `goal_scored`
- `touchdown`
- `home_run`
- `basket`

This is intended as the integration point for future lighting/DMX, notification, or automation hooks.

## Deployment

Production runs on the shared AWS Lightsail instance at `52.1.97.78`, behind Caddy.

Normal production deploy:

```powershell
.\deploy-lightsail.ps1 -StaticIp "52.1.97.78" -PemPath "C:\Development\Repos\PhillyGametime\LightsailDefaultKey-us-east-1.pem" -Restart
```

The script:

- builds a Linux `amd64` binary named `philly-gametime`
- uploads the binary to `/home/ubuntu/philly-gametime`
- uploads `templates/`
- uploads `static/`
- restarts the `philly-gametime` systemd service when `-Restart` is used

### First-Time Server Setup

The systemd unit lives at:

```text
deploy/philly-gametime.service
```

Upload it once:

```powershell
scp -O -i "C:\Development\Repos\PhillyGametime\LightsailDefaultKey-us-east-1.pem" .\deploy\philly-gametime.service ubuntu@52.1.97.78:/tmp/philly-gametime.service
ssh -i "C:\Development\Repos\PhillyGametime\LightsailDefaultKey-us-east-1.pem" ubuntu@52.1.97.78
```

Then on the server:

```bash
sudo mv /tmp/philly-gametime.service /etc/systemd/system/philly-gametime.service
sudo systemctl daemon-reload
sudo systemctl enable philly-gametime
sudo systemctl start philly-gametime
sudo systemctl status philly-gametime
```

If another app already uses port `8080`, set Philly Gametime to a different internal port such as `8081` in `/etc/systemd/system/philly-gametime.service`:

```ini
Environment=PORT=8081
```

Then reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart philly-gametime
```

### Caddy

The DNS records for `phillygametime.com` and `www.phillygametime.com` should point to the Lightsail static IP.

Current production DNS:

```text
phillygametime.com      A  52.1.97.78
www.phillygametime.com  A  52.1.97.78
```

Caddy route:

```caddy
phillygametime.com, www.phillygametime.com {
    reverse_proxy 127.0.0.1:8081
}
```

After Caddy-only changes, reload Caddy instead of redeploying the app:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## Notes

- The default store calls ESPN scoreboard/schedule endpoints.
- Local Philly broadcast names are prioritized, including `NBC Sports Philadelphia`, `NBCSP`, `NBC10`, `PHL17`, `6abc`, and `FOX 29`.
- Game dates and times are displayed in Philadelphia time (`America/New_York`) regardless of the server timezone.
- Team detail pages are assembled from the existing store data (`GetTodaysGames`, `GetFullSchedules`, `GetRecentResults`, and `GetStandings`) and do not add separate provider calls.
- The committed UI uses the PG-style header mark with Flyers orange, Eagles teal, and Phillies red score bars.
