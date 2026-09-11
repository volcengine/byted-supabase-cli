package solo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/volcengine/byted-supabase-cli/pkg/fetcher"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

const storageRequestTimeout = 30 * time.Second

func NewStorageAPI(profile Profile) (storage.StorageAPI, error) {
	baseURL, err := normalizeURL(profile.SupabaseURL)
	if err != nil {
		return storage.StorageAPI{}, err
	}
	if strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return storage.StorageAPI{}, &MissingCredentialError{Name: "Service Role Key"}
	}
	return storage.StorageAPI{Fetcher: fetcher.NewServiceGateway(
		baseURL,
		profile.ServiceRoleKey,
		fetcher.WithHTTPClient(&http.Client{Timeout: storageRequestTimeout}),
		fetcher.WithRequestEditor(func(request *http.Request) {
			request.Header.Set("X-Request-From", "byted-supabase-cli")
		}),
	)}, nil
}

func LoadStorageAPI() (storage.StorageAPI, error) {
	profile, err := LoadCurrentProfile()
	if err != nil {
		return storage.StorageAPI{}, err
	}
	return NewStorageAPI(profile)
}

type MissingCredentialError struct {
	Name string
}

func (e *MissingCredentialError) Error() string {
	return "missing " + e.Name + " for operations on the linked branch"
}

func serviceURL(raw string) (string, error) {
	return normalizeURL(raw)
}

func normalizeURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("invalid Supabase URL %q", raw)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
