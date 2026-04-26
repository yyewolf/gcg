# GCG

GCG is a simple real-time browser strategy game inspired by Galcon.

The backend is written in Go and keeps the full game state in memory. The frontend is a PixiJS app built with Vite and served by the Go server as embedded static assets.

## Requirements

- Go 1.26+
- Node.js 22+
- npm
- Docker (optional)

## Run Locally

Build the frontend first:

```bash
cd web
npm ci
npm run build
```

Then start the server from the project root:

```bash
go run .
```

The game will be available at `http://localhost:8080`.

## Frontend Dev Server

If you want to work on the frontend separately:

```bash
cd web
npm ci
npm run dev
```

The Vite dev server runs on `http://localhost:5173` and proxies WebSocket traffic to the Go server on port `8080`.

## Docker

Build the image:

```bash
docker build -t gcg .
```

Run it:

```bash
docker run --rm -p 8080:8080 gcg
```

## Project Layout

- `main.go`: application entrypoint
- `internal/game`: in-memory game engine
- `internal/server`: HTTP and WebSocket server
- `web`: PixiJS frontend
- `docs`: extra design notes and algorithm explanations
