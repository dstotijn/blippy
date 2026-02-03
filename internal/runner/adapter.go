package runner

import "context"

// Adapter wraps Runner to implement tool.AgentCaller.
type Adapter struct {
	runner *Runner
}

// NewAdapter creates a new Adapter.
func NewAdapter(r *Runner) *Adapter {
	return &Adapter{runner: r}
}

// RunAgent implements tool.AgentCaller.
func (a *Adapter) RunAgent(ctx context.Context, agentID, prompt string, depth int) (string, error) {
	result, err := a.runner.Run(ctx, RunOpts{
		AgentID: agentID,
		Prompt:  prompt,
		Depth:   depth,
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}
