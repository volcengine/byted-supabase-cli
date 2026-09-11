// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/link"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func TestSoloCredentialRefUsesCanonicalSupabaseURL(t *testing.T) {
	require.Equal(t,
		soloCredentialRef("https://example.supabase.co/"),
		soloCredentialRef(" https://example.supabase.co "),
	)
	require.Equal(t,
		soloCredentialRef("https://example.supabase.co"),
		soloCredentialRef("https://example.supabase.co"),
	)
}

func TestLinkStatusUsesEnvironmentProfile(t *testing.T) {
	t.Setenv("SUPABASE_URL", "https://example.supabase.co")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "service-role-key")
	t.Setenv("SUPABASE_ANON_KEY", "")
	t.Setenv("SUPABASE_DB_URL", "")
	t.Setenv("SUPABASE_DATABASE_URL", "")
	var output bytes.Buffer
	oldOutput := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputJson
	t.Cleanup(func() { utils.OutputFormat = oldOutput })

	require.NoError(t, runLinkStatus(RuntimeContext{FS: afero.NewMemMapFs(), Output: &output}))
	require.Contains(t, output.String(), `"config_source": "environment"`)
	require.Contains(t, output.String(), `"solo_config_valid": true`)
}

func TestRenderLinkStatusShowsInvalidEnvironment(t *testing.T) {
	var output bytes.Buffer
	oldOutput := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputPretty
	t.Cleanup(func() { utils.OutputFormat = oldOutput })

	err := renderLinkStatus(RuntimeContext{Output: &output}, linkStatus{
		Type:            string(link.LinkTypeSolo),
		SoloConfigError: "missing required environment variable(s): SUPABASE_URL",
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "The branch link configuration is invalid.")
	require.Contains(t, output.String(), "SUPABASE_URL")
	requireNoInternalVocabulary(t, output.String())
}

func TestLinkFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newLinkCommand(ctx)
	second := newLinkCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "link", first.Name())
	require.NoError(t, first.Flags().Set("workspace-id", "workspace-first"))
	require.NoError(t, first.Flags().Set("volc-project-name", "project-first"))
	require.Equal(t, "workspace-first", first.Flag("workspace-id").Value.String())
	require.Equal(t, "project-first", first.Flag("volc-project-name").Value.String())
	require.Equal(t, "", second.Flag("workspace-id").Value.String())
	require.Equal(t, "", second.Flag("volc-project-name").Value.String())
}

func TestRunSoloLinkFactoryPersistsProjectLinkWithoutKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fsys := afero.NewMemMapFs()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/v1/" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	err := runSoloLinkFactory(linkOptions{
		SupabaseURL:    server.URL,
		AnonKey:        "anon-key",
		ServiceRoleKey: "service-role-key",
	}, fsys)
	require.NoError(t, err)

	state, err := link.LoadState(fsys)
	require.NoError(t, err)
	require.Equal(t, link.LinkTypeSolo, state.Type)
	require.Equal(t, server.URL, state.SupabaseURL)
	require.NotEmpty(t, state.CredentialRef)

	data, err := afero.ReadFile(fsys, link.StatePath)
	require.NoError(t, err)
	require.NotContains(t, string(data), "anon-key")
	require.NotContains(t, string(data), "service-role-key")
}

func TestRunSoloLinkFactoryAcceptsLinkWithoutDatabaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	err := runSoloLinkFactory(linkOptions{
		SupabaseURL:    server.URL,
		AnonKey:        "anon-key",
		ServiceRoleKey: "service-role-key",
	}, afero.NewMemMapFs())

	require.NoError(t, err)
}

func TestProbeSoloKey(t *testing.T) {
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/rest/v1/", request.URL.Path)
		keys = append(keys, request.Header.Get("apikey"))
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	require.NoError(t, probeSoloKey(server.URL, "anon", "anon-key"))
	require.NoError(t, probeSoloKey(server.URL, "service_role", "service-role-key"))
	require.Equal(t, []string{"anon-key", "service-role-key"}, keys)
}

func TestProbeSoloKeyRejectsNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"invalid token"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	require.ErrorContains(t, probeSoloKey(server.URL, "service_role", "service-role-key"), "expected HTTP 200")
}

func TestRunSoloLinkFactoryProbesOnlyServiceRoleKeyWhenAnonKeyOmitted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	require.NoError(t, runSoloLinkFactory(linkOptions{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	}, afero.NewMemMapFs()))
	require.Equal(t, 1, requests)
}

func TestRunSoloLinkFactoryAcceptsServiceRoleKeyWithoutAnonKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		require.Equal(t, "service-role-key", request.Header.Get("apikey"))
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	require.NoError(t, runSoloLinkFactory(linkOptions{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	}, afero.NewMemMapFs()))
	require.Equal(t, 1, requests)
}

func TestRunSoloLinkFactoryRequiresServiceRoleKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := runSoloLinkFactory(linkOptions{
		SupabaseURL: "https://example.com",
		AnonKey:     "anon-key",
	}, afero.NewMemMapFs())

	require.ErrorContains(t, err, "missing --service-role-key")
}

func TestPrintLinkSuccessExplainsDirectLink(t *testing.T) {
	var output bytes.Buffer
	printLinkSuccess(&output, linkOptions{SupabaseURL: "https://example.com/"})

	require.Contains(t, output.String(), "Supabase branch linked successfully:\n  https://example.com")
	require.Contains(t, output.String(), "Command list has changed.")
	require.Contains(t, output.String(), "View status:\n  byted-supabase-cli link status")
	require.Contains(t, output.String(), "To go back to managing cloud resources with your Volcengine account:\n  byted-supabase-cli unlink")
	require.NotContains(t, output.String(), "Mode")
}

func TestSoloLinkCommandDoesNotExposeManagementFlags(t *testing.T) {
	command := newSoloLinkCommand(RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}})

	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.Flags().Lookup("project-ref"))
	require.Nil(t, command.Flags().Lookup("volc-project-name"))
	require.NotNil(t, command.Flags().Lookup("supabase-url"))
	require.NotNil(t, command.Flags().Lookup("supabase-url").Annotations)
	require.NotNil(t, command.Flags().Lookup("service-role-key").Annotations)
	require.Contains(t, command.Flags().Lookup("supabase-url").Annotations[cobra.BashCompOneRequiredFlag], "true")
	require.Contains(t, command.Flags().Lookup("service-role-key").Annotations[cobra.BashCompOneRequiredFlag], "true")
	require.Nil(t, command.Flags().Lookup("database-url"))

	status, _, err := command.Find([]string{"status"})
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestManagementLinkCommandDoesNotExposeDirectFlags(t *testing.T) {
	command := newManagementLinkCommand(RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}})

	require.NotNil(t, command.Flags().Lookup("workspace-id"))
	require.NotNil(t, command.Flags().Lookup("project-ref"))
	require.NotNil(t, command.Flags().Lookup("volc-project-name"))
	require.Nil(t, command.Flags().Lookup("supabase-url"))
	require.Nil(t, command.Flags().Lookup("anon-key"))
	require.Nil(t, command.Flags().Lookup("service-role-key"))
	require.Nil(t, command.Flags().Lookup("database-url"))
	require.Contains(t, command.Long, "Option 1: Link using a Volcengine account")
	require.Contains(t, command.Long, "Option 2: Link using a branch URL and API keys")
	require.Contains(t, command.Long, "--supabase-url <url>")
	require.Contains(t, command.Long, "The service role key is required")
	require.Contains(t, command.Long, "The anon key is optional")
	require.NotContains(t, command.Long, "Mode")
	require.NotContains(t, command.Long, "mode")
	requireNoInternalVocabulary(t, command.Long)
}

func requireNoInternalVocabulary(t *testing.T, text string) {
	t.Helper()
	for _, word := range []string{"Solo", "solo", "data plane", "data-plane", "instance"} {
		require.NotContains(t, text, word)
	}
}

func TestDirectLinkCommandLongDescribesCredentialsOnlyPath(t *testing.T) {
	command := newSoloLinkCommand(RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}})

	require.Contains(t, command.Long, "No Volcengine account is required.")
	require.Contains(t, command.Long, "--supabase-url <url>")
	require.Contains(t, command.Long, "The service role key is required")
	require.Contains(t, command.Long, "The anon key is optional")
	require.NotContains(t, command.Long, "Mode")
	require.NotContains(t, command.Long, "mode")
	requireNoInternalVocabulary(t, command.Long)
	require.NotContains(t, command.Flags().Lookup("supabase-url").Usage, "Solo")
	require.NotContains(t, command.Flags().Lookup("anon-key").Usage, "Solo")
	require.NotContains(t, command.Flags().Lookup("service-role-key").Usage, "Solo")
	require.Nil(t, command.Flags().Lookup("database-url"))
}

func TestRenderLinkStatus(t *testing.T) {
	var output bytes.Buffer
	oldOutput := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputPretty
	t.Cleanup(func() { utils.OutputFormat = oldOutput })

	err := renderLinkStatus(RuntimeContext{Output: &output}, linkStatus{
		Mode:                 "solo",
		Linked:               true,
		Type:                 string(link.LinkTypeSolo),
		SupabaseURL:          "https://example.com",
		AnonKeyConfigured:    true,
		ServiceKeyConfigured: true,
		DatabaseConfigured:   false,
		SoloConfigValid:      true,
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "Linked via: branch URL and keys")
	require.Contains(t, output.String(), "Supabase URL: https://example.com")
	require.Contains(t, output.String(), "Database URL: not configured")
	require.Contains(t, output.String(), "Link config: valid")
	require.NotContains(t, output.String(), "Mode")
}

func TestRenderLinkStatusShowsInvalidSoloConfig(t *testing.T) {
	var output bytes.Buffer
	oldOutput := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputPretty
	t.Cleanup(func() { utils.OutputFormat = oldOutput })

	err := renderLinkStatus(RuntimeContext{Output: &output}, linkStatus{
		Type:            string(link.LinkTypeSolo),
		Linked:          true,
		SupabaseURL:     "https://example.com",
		SoloConfigError: `linked-branch profile "link-missing" was not found`,
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "The branch link configuration is invalid.")
	require.Contains(t, output.String(), `linked-branch profile "link-missing" was not found`)
	requireNoInternalVocabulary(t, output.String())
}

func TestRenderLinkStatusForVolcengineAccount(t *testing.T) {
	var output bytes.Buffer
	oldOutput := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputPretty
	t.Cleanup(func() { utils.OutputFormat = oldOutput })

	err := renderLinkStatus(RuntimeContext{Output: &output}, linkStatus{
		Mode:        "management",
		Linked:      true,
		Type:        string(link.LinkTypeManagement),
		WorkspaceID: "exact-plum-af53ccde",
		BranchID:    "br-right-pika-6f3ab280",
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), "Linked via: Volcengine account")
	require.Contains(t, output.String(), "Workspace ID: exact-plum-af53ccde")
	require.Contains(t, output.String(), "Branch ID:    br-right-pika-6f3ab280")
	require.NotContains(t, output.String(), "Mode")
}

func TestRenderLinkStatusKeepsJSONFieldsStable(t *testing.T) {
	var output bytes.Buffer
	oldOutput := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputJson
	t.Cleanup(func() { utils.OutputFormat = oldOutput })

	err := renderLinkStatus(RuntimeContext{Output: &output}, linkStatus{
		Mode:        "management",
		Linked:      true,
		Type:        string(link.LinkTypeManagement),
		WorkspaceID: "exact-plum-af53ccde",
	})
	require.NoError(t, err)
	require.Contains(t, output.String(), `"mode": "management"`)
	require.Contains(t, output.String(), `"type": "management"`)
}
