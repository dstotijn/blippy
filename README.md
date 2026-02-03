# Blippy

A self-hosted AI agent platform with an embedded web UI.

## Features

- **Multi-agent support** - Create and manage multiple AI agents with custom system prompts
- **Tool execution** - Agents can fetch web content and execute bash commands via [Sprites](https://sprites.dev) sandboxes
- **Scheduling** - Trigger agent runs on schedules or via webhooks
- **Notifications** - Configure notification channels for agent outputs
- **Agent orchestration** - Agents can call other agents for complex workflows
- **Conversation history** - Full conversation persistence with tool execution logs
- **Modern web UI** - React-based interface for managing agents and conversations

## Architecture

- **Backend**: Go with ConnectRPC
- **Frontend**: React + TypeScript + Tailwind CSS (TanStack Router)
- **Database**: SQLite (embedded)
- **LLM**: OpenRouter API (OpenResponses spec)
- **Code execution**: [Sprites](https://sprites.dev) (Fly.io sandboxed environments)

## Prerequisites

- An [OpenRouter](https://openrouter.ai/) API key
- (Optional) A [Sprites](https://sprites.dev/) API key for bash tool execution

## Installation

TODO

## Configuration

Set the following environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENROUTER_API_KEY` | Yes | - | OpenRouter API key |
| `MODEL` | No | `google/gemini-3-flash-preview` | LLM model to use |
| `SPRITES_API_KEY` | No | - | Sprites API key (enables bash tool) |
| `DATABASE_PATH` | No | `./blippy.db` | SQLite database location |
| `PORT` | No | `8080` | HTTP server port |

## Usage

```
$ blippy
```

Then open http://localhost:8080 in your browser.

## Development

```bash
mise run dev          # Run backend and frontend dev servers
mise run build        # Build production binary
mise run test         # Run Go tests
mise run gen          # Generate protobuf and sqlc code
mise run web:check    # Lint and format frontend code
```

## License

[Apache 2.0](LICENSE)

---

© 2026 [David Stotijn](https://github.com/dstotijn)
