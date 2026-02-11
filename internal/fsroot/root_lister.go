package fsroot

import (
	"context"
	"fmt"

	"github.com/dstotijn/blippy/internal/store"
	"github.com/dstotijn/blippy/internal/tool"
)

// RootLister provides filesystem root lookups for tools.
// Implements tool.FilesystemRootLister.
type RootLister struct {
	queries *store.Queries
}

// NewRootLister creates a new RootLister.
func NewRootLister(queries *store.Queries) *RootLister {
	return &RootLister{queries: queries}
}

// ListFilesystemRootsByIDs returns roots matching the given IDs.
func (l *RootLister) ListFilesystemRootsByIDs(ctx context.Context, ids []string) ([]tool.FilesystemRoot, error) {
	allRoots, err := l.queries.ListFilesystemRoots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list filesystem roots: %w", err)
	}

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	var result []tool.FilesystemRoot
	for _, r := range allRoots {
		if idSet[r.ID] {
			result = append(result, tool.FilesystemRoot{
				ID:          r.ID,
				Name:        r.Name,
				Path:        r.Path,
				Description: r.Description,
			})
		}
	}
	return result, nil
}
