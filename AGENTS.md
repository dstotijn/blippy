# Blippy

A self-hosted AI agent platform distributed as a single static Go binary with an embedded web UI.

> **Claude:** When making changes that affect this file's content (new packages, env vars, commands), update it.

## Tech Stack

- **Backend**: Go
- **Frontend**: TanStack Router (React + TypeScript + Tailwind CSS)
- **RPC**: ConnectRPC
- **LLM**: OpenResponses spec via OpenRouter
- **Persistence**: SQLite (embedded)
- **Code Execution**: Sprites (Fly.io)
- **Task Runner**: mise

## Architecture Decisions

- **Single binary distribution**: Frontend will be statically built and embedded into the Go binary (Phase 4)
- **OpenResponses spec**: Use `previous_response_id` for conversation chaining, items as atomic units
- **Idiomatic Go structure**: Packages named by responsibility, not architectural layers
- **SQLite**: Embedded database fits single-binary goal, no external dependencies

## Project Structure

```
internal/
├── agent/          # Agent CRUD service
├── conversation/   # Conversation service
├── notification/   # Notification channels service
├── openrouter/     # OpenResponses client
├── runner/         # Agent runner and LLM adapter
├── scheduler/      # Trigger scheduling
├── server/         # HTTP server, ConnectRPC handlers
├── store/          # SQLite setup and migrations
├── tool/           # Tool definitions and execution
├── trigger/        # Trigger service
└── webhook/        # Webhook handler
```

## Development Commands

```bash
mise run dev          # Run backend and frontend dev servers
mise run build        # Build production binary
mise run test         # Run Go tests
mise run gen          # Generate protobuf code
mise run web:dev      # Run frontend dev server only
mise run web:build    # Build frontend for production
mise run web:install  # Install frontend dependencies
mise run web:check    # Lint and format frontend code
```

## Configuration

Environment variables:

- `OPENROUTER_API_KEY` - Required
- `MODEL` - LLM model (default: `google/gemini-3-flash-preview`)
- `SPRITES_API_KEY` - Required for code execution
- `DATABASE_PATH` - SQLite location (default: `./blippy.db`)
- `PORT` - HTTP port (default: `8080`)

## External Documentation

- **Sprites SDK**: https://docs.sprites.dev/llms-full.txt (Go SDK: `github.com/superfly/sprites-go`)

## Patterns

<!-- Add patterns as we establish them -->

## Things to Avoid

<!-- Add anti-patterns as we discover them -->