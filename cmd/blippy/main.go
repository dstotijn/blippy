package main

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/dstotijn/blippy/internal/agent"
	"github.com/dstotijn/blippy/internal/agentloop"
	"github.com/dstotijn/blippy/internal/conversation"
	"github.com/dstotijn/blippy/internal/fsroot"
	"github.com/dstotijn/blippy/internal/notification"
	"github.com/dstotijn/blippy/internal/openrouter"
	"github.com/dstotijn/blippy/internal/pubsub"
	"github.com/dstotijn/blippy/internal/runner"
	"github.com/dstotijn/blippy/internal/scheduler"
	"github.com/dstotijn/blippy/internal/server"
	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
	"github.com/dstotijn/blippy/internal/trigger"
	"github.com/dstotijn/blippy/internal/webhook"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	dbPath := cmp.Or(os.Getenv("DATABASE_PATH"), "./blippy.db")
	port := cmp.Or(os.Getenv("PORT"), "8080")
	openRouterAPIKey := os.Getenv("OPENROUTER_API_KEY")
	model := cmp.Or(os.Getenv("MODEL"), "google/gemini-3-flash-preview")
	spritesAPIKey := os.Getenv("SPRITES_API_KEY")

	if openRouterAPIKey == "" {
		return fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	queries := store.New(db)
	orClient := openrouter.NewClient(openRouterAPIKey)

	// Create adapter services for tools
	triggerCreator := trigger.NewCreator(queries)
	channelLister := notification.NewChannelLister(queries)
	rootLister := fsroot.NewRootLister(queries)

	// Set up tool registry
	toolRegistry := tool.NewRegistry()
	toolRegistry.Register(tool.NewFetchTool())
	if spritesAPIKey != "" {
		toolRegistry.Register(tool.NewBashTool(spritesAPIKey))
		log.Println("Bash tool enabled (SPRITES_API_KEY set)")
	}
	toolExecutor := tool.NewExecutor(toolRegistry, channelLister, rootLister)

	// Create broker for pub/sub events
	broker := pubsub.New()

	// Create shared agentic loop
	loop := &agentloop.Loop{
		Queries:      queries,
		ORClient:     orClient,
		ToolExecutor: toolExecutor,
		Broker:       broker,
		DefaultModel: model,
	}

	// Create runner for autonomous execution
	agentRunner := runner.New(queries, broker, loop)
	runnerAdapter := runner.NewAdapter(agentRunner)

	// Register autonomous tools
	toolRegistry.Register(tool.NewCallAgentTool(runnerAdapter))
	toolRegistry.Register(tool.NewScheduleAgentRunTool(triggerCreator))

	// Register memory tools
	toolRegistry.Register(tool.NewMemoryViewTool(queries))
	toolRegistry.Register(tool.NewMemoryCreateTool(queries))
	toolRegistry.Register(tool.NewMemoryEditTool(queries))
	toolRegistry.Register(tool.NewMemoryDeleteTool(queries))

	// Create and start scheduler
	logger := slog.Default()
	sched := scheduler.New(db, queries, agentRunner, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)
	defer sched.Stop()

	agentService := agent.NewService(db, orClient)
	conversationService := conversation.NewService(db, broker, loop)
	triggerRPCService := trigger.NewService(db)
	notificationRPCService := notification.NewService(db)
	fsrootRPCService := fsroot.NewService(db)
	webhookHandler := webhook.New(queries, agentRunner, logger)
	srv, err := server.New(agentService, conversationService, triggerRPCService, notificationRPCService, fsrootRPCService, webhookHandler)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	log.Printf("🤖 Blippy listening on :%s", port)
	return http.ListenAndServe(":"+port, srv.Handler())
}
