# Philly Gametime

Go web app for quick-glance Philadelphia sports scores, schedules, standings, recent results, and TV/stream info.

The app covers the Eagles, Phillies, 76ers, Flyers, and Union. By default it uses ESPN-backed data and ranks local Philly broadcasts ahead of national feeds when possible.

## Features

- Home dashboard with today's Philly action, upcoming games, recent results, and standings
- Live scores page
- Upcoming schedule page
- Team directory
- Stats/standings page
- TV/stream guide with Philly-first broadcast sorting
- Server-sent events endpoint for score and sport-specific event hooks
- Mock data mode for design/development previews

## Requirements

- Go 1.22 or newer
- Network access for live ESPN-backed data

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

To return to live data in the same shell, clear the variable:

```powershell
Remove-Item Env:\PHILLY_DATA
go run .
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

## Build And Test

```powershell
go test ./...
go build ./...
```

## Docker

Build the image:

```powershell
docker build -t philly-gametime .
```

Run on port `8080`:

```powershell
docker run --rm -p 8080:8080 philly-gametime
```

Run with mock data:

```powershell
docker run --rm -p 8080:8080 -e PHILLY_DATA=mock philly-gametime
```

## Deploy

This repo uses the same direct Lightsail instance workflow as the HoustonTrio site: build a Linux binary locally, upload it over SSH/SCP, upload templates/static assets, then restart a systemd service.

Example deploy:

```powershell
.\deploy-lightsail.ps1 -StaticIp "YOUR_LIGHTSAIL_STATIC_IP" -PemPath ".\LightsailDefaultKey-us-east-1.pem"
```

Deploy and restart the service in one command:

```powershell
.\deploy-lightsail.ps1 -StaticIp "YOUR_LIGHTSAIL_STATIC_IP" -PemPath ".\LightsailDefaultKey-us-east-1.pem" -Restart
```

Current production deploy command:

```powershell
.\deploy-lightsail.ps1 -StaticIp "52.1.97.78" -PemPath "C:\Development\Repos\HoustonTrio\LightsailDefaultKey-us-east-1.pem" -Restart
```

The script uploads:

- `philly-gametime` Linux binary
- `templates/`
- `static/`

### First-Time Lightsail Setup

Copy the included systemd unit to the server once:

```powershell
scp -O -i .\LightsailDefaultKey-us-east-1.pem .\deploy\philly-gametime.service ubuntu@YOUR_LIGHTSAIL_STATIC_IP:/tmp/philly-gametime.service
ssh -i .\LightsailDefaultKey-us-east-1.pem ubuntu@YOUR_LIGHTSAIL_STATIC_IP
```

Then on the server:

```bash
sudo mv /tmp/philly-gametime.service /etc/systemd/system/philly-gametime.service
sudo systemctl daemon-reload
sudo systemctl enable philly-gametime
sudo systemctl start philly-gametime
sudo systemctl status philly-gametime
```

The unit expects the app at:

```text
/home/ubuntu/philly-gametime
```

It runs the app on `PORT=8080`. Use your existing Nginx/Caddy/Apache reverse proxy, or open port `8080` in Lightsail if you want to access it directly.

Production currently runs behind Caddy on the shared Lightsail instance. If another app already uses `8080`, configure this service to use a different port such as `8081`, then route the domain in `/etc/caddy/Caddyfile`:

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

### SSH Into Lightsail

```powershell
ssh -i .\LightsailDefaultKey-us-east-1.pem ubuntu@YOUR_LIGHTSAIL_STATIC_IP
```

The service uses live ESPN-backed data by default. Do not set `PHILLY_DATA=mock` in production unless you want seeded preview data.

## Notes

- The default store calls ESPN scoreboard/schedule endpoints.
- Local Philly broadcast names are prioritized, including `NBC Sports Philadelphia`, `NBCSP`, `NBC10`, `PHL17`, `6abc`, and `FOX 29`.
- Game dates and times are displayed in Philadelphia time (`America/New_York`) regardless of the server timezone.
- The committed UI currently uses the PG-style header mark with Flyers orange, Eagles teal, and Phillies red score bars.
