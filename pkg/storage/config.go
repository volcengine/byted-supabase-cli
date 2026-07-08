package storage

import (
	"context"
	"net/http"

	"github.com/volcengine/byted-supabase-cli/pkg/fetcher"
)

// StorageConfigResponse is the Storage service runtime configuration returned by
// GET /storage/v1/config.
type StorageConfigResponse struct {
	FileSizeLimit      *int64 `json:"fileSizeLimit,omitempty"`
	TotalFileSizeLimit *int64 `json:"totalFileSizeLimit,omitempty"`
	Features           any    `json:"features,omitempty"`
}

// GetStorageConfig fetches the Storage service config, mirroring the typed
// accessors in buckets.go.
func (s *StorageAPI) GetStorageConfig(ctx context.Context) (StorageConfigResponse, error) {
	resp, err := s.Send(ctx, http.MethodGet, "/storage/v1/config", nil)
	if err != nil {
		return StorageConfigResponse{}, err
	}
	return fetcher.ParseJSON[StorageConfigResponse](resp.Body)
}
