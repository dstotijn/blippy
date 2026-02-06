package server

import (
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/dstotijn/blippy/internal/agent"
	"github.com/dstotijn/blippy/internal/conversation"
	"github.com/dstotijn/blippy/internal/notification"
	"github.com/dstotijn/blippy/internal/trigger"
	"github.com/dstotijn/blippy/internal/webhook"
	"github.com/dstotijn/blippy/web"
)

type Server struct {
	mux *http.ServeMux
}

func New(
	agentService *agent.Service,
	conversationService *conversation.Service,
	triggerService *trigger.Service,
	notificationService *notification.Service,
	webhookHandler *webhook.Handler,
) (*Server, error) {
	mux := http.NewServeMux()

	opts := []connect.HandlerOption{connect.WithCompressMinBytes(1024)}

	apiMux := http.NewServeMux()

	agentPath, agentHandler := agent.NewAgentServiceHandler(agentService, opts...)
	apiMux.Handle(agentPath, agentHandler)

	convPath, convHandler := conversation.NewConversationServiceHandler(conversationService, opts...)
	apiMux.Handle(convPath, convHandler)

	triggerPath, triggerHandler := trigger.NewTriggerServiceHandler(triggerService, opts...)
	apiMux.Handle(triggerPath, triggerHandler)

	notificationPath, notificationHandler := notification.NewNotificationChannelServiceHandler(notificationService, opts...)
	apiMux.Handle(notificationPath, notificationHandler)

	mux.Handle("/api/", http.StripPrefix("/api", apiMux))

	// Webhook trigger endpoint
	mux.Handle("/webhooks/trigger", webhookHandler)

	// Web UI (catch-all for SPA)
	webHandler, err := web.AppHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", webHandler)

	return &Server{mux: mux}, nil
}

func (s *Server) Handler() http.Handler {
	return h2c.NewHandler(corsMiddleware(s.mux), &http2.Server{})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
