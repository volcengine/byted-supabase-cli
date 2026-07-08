// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

var (
	configureProfile         string
	configureAccessKey       string
	configureSecretKey       string
	configureRegion          string
	configureEndpoint        string
	configureIsAgentPlan     bool
	configureAgentPlanSeatID string

	configureCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "configure",
		Short:   "Manage Volcengine CLI configuration",
	}

	configureSetCmd = &cobra.Command{
		Use:   "set",
		Short: "Set Volcengine AK/SK profile configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := configureProfileForSet()
			if strings.TrimSpace(configureAccessKey) == "" {
				return errors.New("missing required flag: --access-key")
			}
			if strings.TrimSpace(configureSecretKey) == "" {
				return errors.New("missing required flag: --secret-key")
			}
			region := configureRegionOrDefault()
			if err := volcengine.ValidateRegion(region); err != nil {
				return err
			}
			agentPlanProfile, agentPlanChanged, err := resolveAgentPlanProfileUpdate(
				cmd.Flags().Changed("is-agent-plan"),
				configureIsAgentPlan,
				cmd.Flags().Changed("agent-plan-seat-id"),
				configureAgentPlanSeatID,
			)
			if err != nil {
				return err
			}
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			if cfg.Profiles == nil {
				cfg.Profiles = map[string]*volcengine.Profile{}
			}
			cfg.Profiles[profileName] = &volcengine.Profile{
				Name:      profileName,
				Mode:      "ak",
				AccessKey: strings.TrimSpace(configureAccessKey),
				SecretKey: strings.TrimSpace(configureSecretKey),
				Region:    region,
				Endpoint:  strings.TrimSpace(configureEndpoint),
			}
			cfg.Current = profileName
			if err := volcengine.SaveFileConfig(cfg); err != nil {
				return err
			}
			if agentPlanChanged {
				if err := volcengine.SetSupabaseProfileConfig(profileName, agentPlanProfile); err != nil {
					return err
				}
			}
			path, err := volcengine.VolcengineConfigFilePath()
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Saved Volcengine profile %q to %s.\n", profileName, path)
			return nil
		},
	}

	configureGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Show Volcengine profile configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			profileName := selectedConfigureProfile()
			profile, ok := cfg.Profiles[profileName]
			if !ok {
				return errors.Errorf("Volcengine profile %q not found", profileName)
			}
			supabaseCfg, err := volcengine.LoadSupabaseFileConfig()
			if err != nil {
				return err
			}
			return renderConfigureProfiles(cfg.Current, []*volcengine.Profile{profile}, supabaseCfg)
		},
	}

	configureListCmd = &cobra.Command{
		Use:   "list",
		Short: "List Volcengine profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			names := sortedConfigureProfileNames(cfg)
			profiles := make([]*volcengine.Profile, 0, len(names))
			for _, name := range names {
				profiles = append(profiles, cfg.Profiles[name])
			}
			supabaseCfg, err := volcengine.LoadSupabaseFileConfig()
			if err != nil {
				return err
			}
			return renderConfigureProfiles(cfg.Current, profiles, supabaseCfg)
		},
	}

	configureProfileCmd = &cobra.Command{
		Use:   "profile [name]",
		Short: "Switch the current Volcengine profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := selectedConfigureProfile(args...)
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[profileName]; !ok {
				return errors.Errorf("Volcengine profile %q not found", profileName)
			}
			cfg.Current = profileName
			if err := volcengine.SaveFileConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Switched to Volcengine profile %q.\n", profileName)
			return nil
		},
	}

	configureRegionCmd = &cobra.Command{
		Use:   "region [region]",
		Short: "Update the default region of a Volcengine profile",
		Long: `Update only the default region of a Volcengine profile.

This command supports both AK/SK and console-login profiles. It does not modify
access keys, endpoint, login session, token cache, or the current profile.`,
		Example: `byted-supabase-cli configure region cn-beijing
byted-supabase-cli configure region cn-shanghai --profile dev`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			region := strings.TrimSpace(args[0])
			if region == "" {
				return errors.New("missing region")
			}
			if err := volcengine.ValidateRegion(region); err != nil {
				return err
			}
			profileName := selectedConfigureProfile()
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			profile, ok := cfg.Profiles[profileName]
			if !ok || profile == nil {
				return errors.Errorf("Volcengine profile %q not found", profileName)
			}
			profile.Region = region
			if err := volcengine.SaveFileConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Updated Volcengine profile %q region to %q.\n", profileName, region)
			return nil
		},
	}

	configureAgentPlanCmd = &cobra.Command{
		Use:   "agent-plan",
		Short: "Update Agent Plan defaults for a Volcengine profile",
		Long: `Update only the Agent Plan defaults associated with a Volcengine profile.

This command does not modify access keys, region, endpoint, login session, token
cache, or the current profile. Agent Plan defaults are stored separately from
the Volcengine CLI configuration.`,
		Example: `byted-supabase-cli configure agent-plan --is-agent-plan
byted-supabase-cli configure agent-plan --agent-plan-seat-id seat-xxx
byted-supabase-cli configure agent-plan --is-agent-plan=false --profile dev`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := selectedConfigureProfile()
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			if profile, ok := cfg.Profiles[profileName]; !ok || profile == nil {
				return errors.Errorf("Volcengine profile %q not found", profileName)
			}
			agentPlanProfile, changed, err := resolveAgentPlanProfileUpdate(
				cmd.Flags().Changed("is-agent-plan"),
				configureIsAgentPlan,
				cmd.Flags().Changed("agent-plan-seat-id"),
				configureAgentPlanSeatID,
			)
			if err != nil {
				return err
			}
			if !changed {
				return errors.New("supply --is-agent-plan or --agent-plan-seat-id")
			}
			if err := volcengine.SetSupabaseProfileConfig(profileName, agentPlanProfile); err != nil {
				return err
			}
			if agentPlanProfile.AgentPlanSeatID != "" {
				fmt.Fprintf(os.Stdout, "Updated Volcengine profile %q Agent Plan defaults to enterprise edition.\n", profileName)
			} else if agentPlanProfile.IsAgentPlan {
				fmt.Fprintf(os.Stdout, "Updated Volcengine profile %q Agent Plan defaults to personal edition.\n", profileName)
			} else {
				fmt.Fprintf(os.Stdout, "Disabled Agent Plan defaults for Volcengine profile %q.\n", profileName)
			}
			return nil
		},
	}

	configureDeleteCmd = &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a Volcengine profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			profileName, err := configureProfileForDelete(cmd.Context(), cfg, args...)
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[profileName]; !ok {
				return errors.Errorf("Volcengine profile %q not found", profileName)
			}
			shouldDelete, err := utils.NewConsole().PromptYesNo(cmd.Context(), fmt.Sprintf("Do you want to delete Volcengine profile %q?", profileName), false)
			if err != nil {
				return err
			}
			if !shouldDelete {
				return errors.New("profile deletion canceled")
			}
			delete(cfg.Profiles, profileName)
			if cfg.Current == profileName {
				names := sortedConfigureProfileNames(cfg)
				cfg.Current = "default"
				if len(names) > 0 {
					cfg.Current = names[0]
				}
			}
			if err := volcengine.SaveFileConfig(cfg); err != nil {
				return err
			}
			if err := volcengine.DeleteSupabaseProfileConfig(profileName); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Deleted Volcengine profile %q.\n", profileName)
			return nil
		},
	}
)

func selectedConfigureProfile(args ...string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0])
	}
	if strings.TrimSpace(configureProfile) != "" {
		return strings.TrimSpace(configureProfile)
	}
	cfg, err := volcengine.LoadFileConfig()
	if err == nil && strings.TrimSpace(cfg.Current) != "" {
		return strings.TrimSpace(cfg.Current)
	}
	return "default"
}

func configureProfileForSet() string {
	if strings.TrimSpace(configureProfile) != "" {
		return strings.TrimSpace(configureProfile)
	}
	return "default"
}

func configureRegionOrDefault() string {
	if region := strings.TrimSpace(configureRegion); region != "" {
		return region
	}
	return volcengine.DefaultConsoleLoginRegion
}

func sortedConfigureProfileNames(cfg volcengine.FileConfig) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func configureProfileForDelete(ctx context.Context, cfg volcengine.FileConfig, args ...string) (string, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0]), nil
	}
	if strings.TrimSpace(configureProfile) != "" {
		return strings.TrimSpace(configureProfile), nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("missing profile name. Supply [name] or --profile.")
	}
	names := sortedConfigureProfileNames(cfg)
	if len(names) == 0 {
		return "", errors.New("no Volcengine profiles found")
	}
	items := make([]utils.PromptItem, 0, len(names))
	for _, name := range names {
		details := ""
		if profile := cfg.Profiles[name]; profile != nil {
			details = "mode: " + profile.Mode + ", region: " + profile.Region
		}
		items = append(items, utils.PromptItem{
			Summary: name,
			Details: details,
		})
	}
	choice, err := utils.PromptChoice(ctx, "Which Volcengine profile do you want to delete?", items)
	if err != nil {
		return "", err
	}
	return choice.Summary, nil
}

func renderConfigureProfiles(current string, profiles []*volcengine.Profile, supabaseCfg volcengine.SupabaseFileConfig) error {
	if utils.OutputFormat.Value != utils.OutputPretty {
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, maskConfigureProfiles(profiles, supabaseCfg))
	}
	rows := []string{"| CURRENT | PROFILE | MODE | ACCESS KEY | SECRET KEY | REGION | ENDPOINT | IS AGENT PLAN | AGENT PLAN SEAT ID |", "|---|---|---|---|---|---|---|---|---|"}
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		agentPlan := supabaseCfg[profile.Name]
		currentMark := ""
		if profile.Name == current {
			currentMark = "*"
		}
		rows = append(rows, fmt.Sprintf(
			"| %s | %s | %s | %s | %s | %s | %s | %t | %s |",
			currentMark,
			profile.Name,
			profile.Mode,
			maskValue(profile.AccessKey),
			maskValue(profile.SecretKey),
			profile.Region,
			profile.Endpoint,
			agentPlan.IsAgentPlan,
			maskValue(agentPlan.AgentPlanSeatID),
		))
	}
	return utils.RenderTable(strings.Join(rows, "\n"))
}

type maskedConfigureProfile struct {
	Name            string `json:"name"`
	Mode            string `json:"mode"`
	AccessKey       string `json:"access_key"`
	SecretKey       string `json:"secret_key"`
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint"`
	IsAgentPlan     bool   `json:"is_agent_plan"`
	AgentPlanSeatID string `json:"agent_plan_seat_id"`
}

func maskConfigureProfiles(profiles []*volcengine.Profile, supabaseCfg volcengine.SupabaseFileConfig) []maskedConfigureProfile {
	result := make([]maskedConfigureProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		agentPlan := supabaseCfg[profile.Name]
		result = append(result, maskedConfigureProfile{
			Name:            profile.Name,
			Mode:            profile.Mode,
			AccessKey:       maskValue(profile.AccessKey),
			SecretKey:       maskValue(profile.SecretKey),
			Region:          profile.Region,
			Endpoint:        profile.Endpoint,
			IsAgentPlan:     agentPlan.IsAgentPlan,
			AgentPlanSeatID: maskValue(agentPlan.AgentPlanSeatID),
		})
	}
	return result
}

func resolveAgentPlanProfileUpdate(isAgentPlanChanged, isAgentPlan, seatIDChanged bool, seatID string) (volcengine.SupabaseProfileConfig, bool, error) {
	if !isAgentPlanChanged && !seatIDChanged {
		return volcengine.SupabaseProfileConfig{}, false, nil
	}
	seatID = strings.TrimSpace(seatID)
	if seatID != "" {
		if isAgentPlanChanged && !isAgentPlan {
			return volcengine.SupabaseProfileConfig{}, false, errors.New("--is-agent-plan=false cannot be combined with a non-empty --agent-plan-seat-id")
		}
		return volcengine.SupabaseProfileConfig{IsAgentPlan: true, AgentPlanSeatID: seatID}, true, nil
	}
	if isAgentPlanChanged {
		return volcengine.SupabaseProfileConfig{IsAgentPlan: isAgentPlan}, true, nil
	}
	return volcengine.SupabaseProfileConfig{IsAgentPlan: true}, true, nil
}

func maskValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}

func init() {
	configureCmd.PersistentFlags().StringVar(&configureProfile, "profile", "", "Volcengine profile name.")

	configureSetFlags := configureSetCmd.Flags()
	configureSetFlags.StringVar(&configureAccessKey, "access-key", "", "Volcengine access key.")
	configureSetFlags.StringVar(&configureAccessKey, "ak", "", "Alias of --access-key.")
	configureSetFlags.StringVar(&configureSecretKey, "secret-key", "", "Volcengine secret key.")
	configureSetFlags.StringVar(&configureSecretKey, "sk", "", "Alias of --secret-key.")
	configureSetFlags.StringVar(&configureRegion, "region", "", "Default Volcengine region. Defaults to "+volcengine.DefaultConsoleLoginRegion+" when omitted.")
	configureSetFlags.StringVar(&configureEndpoint, "endpoint", "", "Default Volcengine endpoint.")
	configureSetFlags.BoolVar(&configureIsAgentPlan, "is-agent-plan", false, "Use Agent Plan by default when creating workspaces with this profile.")
	configureSetFlags.StringVar(&configureAgentPlanSeatID, "agent-plan-seat-id", "", "Default Agent Plan seat ID for enterprise edition.")

	configureAgentPlanFlags := configureAgentPlanCmd.Flags()
	configureAgentPlanFlags.BoolVar(&configureIsAgentPlan, "is-agent-plan", false, "Use Agent Plan by default when creating workspaces with this profile.")
	configureAgentPlanFlags.StringVar(&configureAgentPlanSeatID, "agent-plan-seat-id", "", "Default Agent Plan seat ID for enterprise edition.")

	configureCmd.AddCommand(configureSetCmd)
	configureCmd.AddCommand(configureGetCmd)
	configureCmd.AddCommand(configureListCmd)
	configureCmd.AddCommand(configureProfileCmd)
	configureCmd.AddCommand(configureRegionCmd)
	configureCmd.AddCommand(configureAgentPlanCmd)
	configureCmd.AddCommand(configureDeleteCmd)
	rootCmd.AddCommand(configureCmd)
}
