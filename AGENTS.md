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

- **Single binary distribution**: Frontend is statically built and embedded via `go:embed` (`web/handler.go`)
- **OpenResponses spec**: Use `previous_response_id` for conversation chaining, items as atomic units
- **Idiomatic Go structure**: Packages named by responsibility, not architectural layers
- **SQLite**: Embedded database fits single-binary goal, no external dependencies

## Project Structure

```
internal/
├── agent/          # Agent CRUD service
├── agentloop/      # Shared LLM agentic loop (streaming, tool execution)
├── conversation/   # Conversation service
├── notification/   # Notification channels service
├── openrouter/     # OpenResponses client
├── pubsub/         # In-memory pub/sub broker
├── runner/         # Agent runner and LLM adapter
├── scheduler/      # Trigger scheduling
├── server/         # HTTP server, ConnectRPC handlers
├── store/          # SQLite setup and migrations
├── tool/           # Tool definitions and execution
├── trigger/        # Trigger service
└── webhook/        # Webhook handler
web/                # Frontend (React + TanStack Router + Tailwind)
├── handler.go      # Embeds dist/ and serves SPA
└── dist/           # Production build output (embedded in binary)
```

## How It Works

A chat request flows: **ConnectRPC → conversation.Service → agentloop.Loop → OpenRouter**
- `agentloop.Loop` streams LLM responses, executes tools concurrently, and publishes events to `pubsub.Broker`
- `conversation.WatchEvents` subscribes to the broker and forwards events to the frontend via server-streaming RPC
- `runner.Runner` uses the same `agentloop.Loop` for autonomous/scheduled runs (webhooks, triggers)
- `tool.Executor` holds a `tool.Registry` of available tools; each agent has an `enabled_tools` allowlist
- `tool.Executor.ProcessOutput` executes tool calls concurrently with an `onResult` callback for streaming

## Key Relationships

- `conversation.Service` and `runner.Runner` both use `agentloop.Loop` — the shared LLM loop
- `pubsub.Broker` is generic infrastructure; event types live in `agentloop`
- Proto definitions live in `proto/`; `mise run gen` outputs to `internal/conversation/` and `web/src/lib/rpc/`
- `store/` uses sqlc — queries in `store/queries.sql`, schema in `store/migrations/`

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