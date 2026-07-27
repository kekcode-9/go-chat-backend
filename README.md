# Go Chat Backend

A scalable real-time chat backend built with Go.

## Features

- JWT authentication
- Refresh token sessions
- WebSocket messaging
- Redis Pub/Sub for horizontal scaling
- PostgreSQL persistence
- Multi-device support
- Conversation management
- User search
- Docker support

## Tech Stack

- Go
- PostgreSQL
- Redis
- WebSockets
- JWT
- Docker
- Goose migrations

## Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Goose

## Getting Started

Start PostgreSQL and Redis:

```bash
docker compose up -d
```

Run database migrations:

```bash
make up
```

Start the server:

```bash
go run ./cmd/server
```

## Project Structure

```
cmd/
internal/
migrations/
```

## License

MIT