package webhook

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dstotijn/blippy/internal/runner"
	"github.com/dstotijn/blippy/internal/store"
)

// Handler handles incoming webhook requests that trigger agent runs.
type Handler struct {
	queries *store.Queries
	runner  *runner.Runner
	logger  *slog.Logger
}

// New creates a new webhook Handler.
func New(queries *store.Queries, runner *runner.Runner, logger *slog.Logger) *Handler {
	return &Handler{
		queries: queries,
		runner:  runner,
		logger:  logger,
	}
}

// TriggerRequest is the expected payload for webhook trigger requests.
type TriggerRequest struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

// TriggerResponse is returned after triggering an agent.
type TriggerResponse struct {
	ConversationID string `json:"conversation_id"`
	Response       string `json:"response"`
}

// ServeHTTP handles POST /webhooks/trigger requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.AgentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	// Verify agent exists
	_, err := h.queries.GetAgent(r.Context(), req.AgentID)
	if err != nil {
		h.logger.Warn("webhook trigger for unknown agent", "agent_id", req.AgentID, "error", err)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Run the agent
	result, err := h.runner.Run(r.Context(), runner.RunOpts{
		AgentID: req.AgentID,
		Prompt:  req.Prompt,
		Depth:   0,
	})
	if err != nil {
		h.logger.Error("webhook trigger failed", "agent_id", req.AgentID, "error", err)
		http.Error(w, "Agent run failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.logger.Info("webhook trigger completed", "agent_id", req.AgentID, "conversation_id", result.ConversationID)

	resp := TriggerResponse{
		ConversationID: result.ConversationID,
		Response:       result.Response,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
