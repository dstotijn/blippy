package fsroot

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dstotijn/blippy/internal/store"
)

type Service struct {
	queries *store.Queries
}

func NewService(db *sql.DB) *Service {
	return &Service{
		queries: store.New(db),
	}
}

func (s *Service) CreateFilesystemRoot(ctx context.Context, req *connect.Request[CreateFilesystemRootRequest]) (*connect.Response[FilesystemRoot], error) {
	now := time.Now().UTC()

	root, err := s.queries.CreateFilesystemRoot(ctx, store.CreateFilesystemRootParams{
		ID:          uuid.NewString(),
		Name:        req.Msg.Name,
		Path:        req.Msg.Path,
		Description: req.Msg.Description,
		CreatedAt:   now.Format(time.RFC3339),
		UpdatedAt:   now.Format(time.RFC3339),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoFilesystemRoot(root)), nil
}

func (s *Service) GetFilesystemRoot(ctx context.Context, req *connect.Request[GetFilesystemRootRequest]) (*connect.Response[FilesystemRoot], error) {
	root, err := s.queries.GetFilesystemRoot(ctx, req.Msg.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("filesystem root not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoFilesystemRoot(root)), nil
}

func (s *Service) ListFilesystemRoots(ctx context.Context, req *connect.Request[ListFilesystemRootsRequest]) (*connect.Response[ListFilesystemRootsResponse], error) {
	roots, err := s.queries.ListFilesystemRoots(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoRoots := make([]*FilesystemRoot, len(roots))
	for i, r := range roots {
		protoRoots[i] = toProtoFilesystemRoot(r)
	}

	return connect.NewResponse(&ListFilesystemRootsResponse{Roots: protoRoots}), nil
}

func (s *Service) UpdateFilesystemRoot(ctx context.Context, req *connect.Request[UpdateFilesystemRootRequest]) (*connect.Response[FilesystemRoot], error) {
	now := time.Now().UTC()

	root, err := s.queries.UpdateFilesystemRoot(ctx, store.UpdateFilesystemRootParams{
		ID:          req.Msg.Id,
		Name:        req.Msg.Name,
		Path:        req.Msg.Path,
		Description: req.Msg.Description,
		UpdatedAt:   now.Format(time.RFC3339),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("filesystem root not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(toProtoFilesystemRoot(root)), nil
}

func (s *Service) DeleteFilesystemRoot(ctx context.Context, req *connect.Request[DeleteFilesystemRootRequest]) (*connect.Response[Empty], error) {
	if err := s.queries.DeleteFilesystemRoot(ctx, req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&Empty{}), nil
}

func toProtoFilesystemRoot(r store.FilesystemRoot) *FilesystemRoot {
	createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)

	return &FilesystemRoot{
		Id:          r.ID,
		Name:        r.Name,
		Path:        r.Path,
		Description: r.Description,
		CreatedAt:   timestamppb.New(createdAt),
		UpdatedAt:   timestamppb.New(updatedAt),
	}
}
