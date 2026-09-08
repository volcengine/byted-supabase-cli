package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func volcengineClientFromAuth(cmd *cobra.Command) (*volcengine.Client, error) {
	authContext, err := AuthContextFrom(cmd.Context())
	if err != nil {
		return nil, err
	}
	if authContext.VolcengineClient == nil {
		return nil, errors.New("Volcengine client is not available for this command")
	}
	return authContext.VolcengineClient, nil
}

func volcengineConfigFromAuth(cmd *cobra.Command) (volcengine.Config, error) {
	authContext, err := AuthContextFrom(cmd.Context())
	if err != nil {
		return volcengine.Config{}, err
	}
	if authContext.VolcengineConfig == nil {
		return volcengine.Config{}, errors.New("Volcengine config is not available for this command")
	}
	return *authContext.VolcengineConfig, nil
}
