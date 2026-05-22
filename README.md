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

This repo includes Lightsail container deployment templates in `deploy/`.

Recommended AWS Lightsail path:

1. Install and configure the AWS CLI.
2. Install Docker locally.
3. Create a Lightsail container service:

```powershell
aws lightsail create-container-service `
  --service-name philly-gametime `
  --power nano `
  --scale 1
```

4. Build the image:

```powershell
docker build -t philly-gametime:latest .
```

5. Push the image to the Lightsail container service:

```powershell
aws lightsail push-container-image `
  --service-name philly-gametime `
  --label app `
  --image philly-gametime:latest
```

6. Deploy the latest pushed image:

```powershell
aws lightsail create-container-service-deployment `
  --service-name philly-gametime `
  --containers file://deploy/lightsail-containers.json `
  --public-endpoint file://deploy/lightsail-public-endpoint.json
```

7. Get the public URL:

```powershell
aws lightsail get-container-services --service-name philly-gametime
```

The deployment template uses image `:philly-gametime.app.latest`, which tells Lightsail to run the newest pushed image for service `philly-gametime` with label `app`.

The service uses live ESPN-backed data by default. Do not set `PHILLY_DATA=mock` in production unless you want seeded preview data.

## Notes

- The default store calls ESPN scoreboard/schedule endpoints.
- Local Philly broadcast names are prioritized, including `NBC Sports Philadelphia`, `NBCSP`, `NBC10`, `PHL17`, `6abc`, and `FOX 29`.
- The committed UI currently uses the PG-style header mark with Flyers orange, Eagles teal, and Phillies red score bars.
