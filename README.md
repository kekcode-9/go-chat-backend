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

## Database Schema

`migrations/` defines database changes applied through Goose. `schema.sql` is the complete application schema snapshot after all current migrations are applied, excluding Goose's own migration-tracking table and rollback statements.

Every schema change must update both the relevant migration and `schema.sql`, along with the design documents and DBML when affected. Snapshot synchronization is maintained explicitly; it is not automatic. Running the application does not execute `schema.sql`.

## Project Structure

```
cmd/
internal/
migrations/
```

## License

MIT
