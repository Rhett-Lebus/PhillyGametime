# Philly Gametime

Go web app for quick-glance Philadelphia sports scores, schedules, standings, recent results, and TV/stream info.

The app covers the Eagles, Phillies, 76ers, Flyers, and Union. It uses ESPN-backed data by default, prioritizes local Philly broadcasts when available, and displays all game times in Philadelphia time (`America/New_York`).

## Features

- Home dashboard with today's Philly action, upcoming games, recent results, and standings
- Live scores page
- Upcoming schedule page
- Team directory
- Stats/standings page
- TV/stream guide with Philly-first broadcast sorting
- Theme picker with Basic, Light, Dark, and Neon modes
- Server-sent events endpoint for score and sport-specific event hooks
- Mock data mode for local design/development previews

## Requirements

- Go 1.22 or newer
- Network access for live ESPN-backed data
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

To return to live data in the same shell:

```powershell
Remove-Item Env:\PHILLY_DATA
go run .
```

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
- `GET /teams`
- `GET /stats`
- `GET /tv`

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
.\deploy-lightsail.ps1 -StaticIp "52.1.97.78" -PemPath "C:\Development\Repos\HoustonTrio\LightsailDefaultKey-us-east-1.pem" -Restart
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
scp -O -i "C:\Development\Repos\HoustonTrio\LightsailDefaultKey-us-east-1.pem" .\deploy\philly-gametime.service ubuntu@52.1.97.78:/tmp/philly-gametime.service
ssh -i "C:\Development\Repos\HoustonTrio\LightsailDefaultKey-us-east-1.pem" ubuntu@52.1.97.78
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
- The committed UI uses the PG-style header mark with Flyers orange, Eagles teal, and Phillies red score bars.
