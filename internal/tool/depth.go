package tool

import "context"

type depthKey struct{}

const DefaultMaxDepth = 5

// WithDepth returns a new context with the specified depth value.
func WithDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, depthKey{}, depth)
}

// GetDepth returns the current depth from context, defaulting to 0.
func GetDepth(ctx context.Context) int {
	depth, ok := ctx.Value(depthKey{}).(int)
	if !ok {
		return 0
	}
	return depth
}
