package solo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

func TestNewStorageAPIUsesSupabaseDataPlane(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		switch request.URL.Path {
		case "/storage/v1/bucket":
			_, _ = writer.Write([]byte(`[{"id":"public","name":"public","public":true}]`))
		case "/storage/v1/bucket/private":
			_, _ = writer.Write([]byte(`{"id":"private","name":"private","public":false}`))
		case "/storage/v1/object/list/private":
			_, _ = writer.Write([]byte(`[{"name":"readme.md","id":"object-1"}]`))
		case "/storage/v1/object/private/readme.md":
			_, _ = writer.Write([]byte("content"))
		case "/storage/v1/object/move":
			_, _ = writer.Write([]byte(`{"message":"moved"}`))
		case "/storage/v1/object/private":
			_, _ = writer.Write([]byte(`[]`))
		default:
			_, _ = writer.Write([]byte(`{"name":"private","message":"ok"}`))
		}
	}))
	defer server.Close()

	api, err := NewStorageAPI(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	buckets, err := api.ListBuckets(context.Background())
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	_, err = api.GetBucket(context.Background(), "private")
	require.NoError(t, err)
	objects, err := api.ListObjects(context.Background(), "private", "", 0)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	_, err = api.MoveObject(context.Background(), "private", "readme.md", "docs/readme.md")
	require.NoError(t, err)
	_, err = api.DeleteObjects(context.Background(), "private", []string{"readme.md"})
	require.NoError(t, err)

	var downloaded strings.Builder
	err = api.DownloadObjectStream(context.Background(), "private/readme.md", &downloaded)
	require.NoError(t, err)
	require.Equal(t, "content", downloaded.String())

	require.Equal(t, []string{
		"GET /storage/v1/bucket",
		"GET /storage/v1/bucket/private",
		"POST /storage/v1/object/list/private",
		"POST /storage/v1/object/move",
		"DELETE /storage/v1/object/private",
		"GET /storage/v1/object/private/readme.md",
	}, requests)
}

func TestStorageUploadUsesObjectPathAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/storage/v1/object/private/docs/readme.md", request.URL.Path)
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		require.Equal(t, "text/plain", request.Header.Get("Content-Type"))
		require.Equal(t, "max-age=60", request.Header.Get("Cache-Control"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.Equal(t, "hello", string(body))
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	api, err := NewStorageAPI(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)
	err = api.UploadObjectStream(context.Background(), "private/docs/readme.md", strings.NewReader("hello"), storage.FileOptions{
		ContentType:  "text/plain",
		CacheControl: "max-age=60",
	})
	require.NoError(t, err)
}

func TestNewStorageAPIRequiresServiceRoleKey(t *testing.T) {
	_, err := NewStorageAPI(Profile{SupabaseURL: "https://example.supabase.co"})
	require.ErrorContains(t, err, "Service Role Key")
}
