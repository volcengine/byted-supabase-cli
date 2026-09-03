// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

type configureParentOptions struct {
	Profile string
}

type configureSetOptions struct {
	AccessKey       string
	SecretKey       string
	Region          string
	Endpoint        string
	IsAgentPlan     bool
	AgentPlanSeatID string
}

type configureAgentPlanOptions struct {
	IsAgentPlan     bool
	AgentPlanSeatID string
}

func newConfigureCommand(ctx RuntimeContext) *cobra.Command {
	var options configureParentOptions
	cmd := &cobra.Command{GroupID: groupLocalDev, Use: "configure", Short: "Manage Volcengine CLI Configuration"}
	cmd.AddCommand(
		newConfigureSetCommand(ctx, &options),
		newConfigureGetCommand(ctx, &options),
		newConfigureProfileCommand(ctx, &options),
		newConfigureRegionCommand(ctx, &options),
		newConfigureAgentPlanCommand(ctx, &options),
		newConfigureDeleteCommand(ctx, &options),
		newConfigureListCommand(ctx),
	)
	cmd.PersistentFlags().StringVar(&options.Profile, "profile", "", "Volcengine profile name.")
	return cmd
}

func newConfigureSetCommand(ctx RuntimeContext, parent *configureParentOptions) *cobra.Command {
	var options configureSetOptions
	cmd := &cobra.Command{
		Use: "set", Short: "Set Volcengine AK/SK profile configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := configureProfileForSetFactory(parent.Profile)
			if strings.TrimSpace(options.AccessKey) == "" {
				return errors.New("missing required flag: --access-key")
			}
			if strings.TrimSpace(options.SecretKey) == "" {
				return errors.New("missing required flag: --secret-key")
			}
			region := configureRegionOrDefaultFactory(options.Region)
			if err := volcengine.ValidateRegion(region); err != nil {
				return err
			}
			agentPlanProfile, changed, err := resolveAgentPlanProfileUpdate(
				cmd.Flags().Changed("is-agent-plan"), options.IsAgentPlan,
				cmd.Flags().Changed("agent-plan-seat-id"), options.AgentPlanSeatID,
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
				Name: profileName, Mode: "ak",
				AccessKey: strings.TrimSpace(options.AccessKey),
				SecretKey: strings.TrimSpace(options.SecretKey),
				Region:    region, Endpoint: strings.TrimSpace(options.Endpoint),
			}
			cfg.Current = profileName
			if err := volcengine.SaveFileConfig(cfg); err != nil {
				return err
			}
			if changed {
				if err := volcengine.SetSupabaseProfileConfig(profileName, agentPlanProfile); err != nil {
					return err
				}
			}
			path, err := volcengine.VolcengineConfigFilePath()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(ctx.Output, "Saved Volcengine profile %q to %s.\n", profileName, path)
			return err
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.AccessKey, "access-key", "", "Volcengine access key.")
	flags.StringVar(&options.AccessKey, "ak", "", "Alias of --access-key.")
	flags.StringVar(&options.SecretKey, "secret-key", "", "Volcengine secret key.")
	flags.StringVar(&options.SecretKey, "sk", "", "Alias of --secret-key.")
	flags.StringVar(&options.Region, "region", "", "Default Volcengine region. Defaults to "+volcengine.DefaultConsoleLoginRegion+" when omitted.")
	flags.StringVar(&options.Endpoint, "endpoint", "", "Default Volcengine endpoint.")
	flags.BoolVar(&options.IsAgentPlan, "is-agent-plan", false, "Use Agent Plan by default when creating workspaces with this profile.")
	flags.StringVar(&options.AgentPlanSeatID, "agent-plan-seat-id", "", "Default Agent Plan seat ID for enterprise edition.")
	return cmd
}

func newConfigureGetCommand(ctx RuntimeContext, parent *configureParentOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "get", Short: "Show Volcengine profile configuration", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := volcengine.LoadFileConfig()
		if err != nil {
			return err
		}
		name := selectedConfigureProfileFactory(parent.Profile)
		item, ok := cfg.Profiles[name]
		if !ok {
			return errors.Errorf("Volcengine profile %q not found", name)
		}
		supabaseCfg, err := volcengine.LoadSupabaseFileConfig()
		if err != nil {
			return err
		}
		return renderConfigureProfilesFactory(cfg.Current, []*volcengine.Profile{item}, supabaseCfg, ctx.Output)
	}}
	return cmd
}

func newConfigureListCommand(ctx RuntimeContext) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List Volcengine profiles", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := volcengine.LoadFileConfig()
		if err != nil {
			return err
		}
		names := sortedConfigureProfileNamesFactory(cfg)
		profiles := make([]*volcengine.Profile, 0, len(names))
		for _, name := range names {
			profiles = append(profiles, cfg.Profiles[name])
		}
		supabaseCfg, err := volcengine.LoadSupabaseFileConfig()
		if err != nil {
			return err
		}
		return renderConfigureProfilesFactory(cfg.Current, profiles, supabaseCfg, ctx.Output)
	}}
}

func newConfigureProfileCommand(ctx RuntimeContext, parent *configureParentOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "profile [name]", Short: "Switch the current Volcengine profile", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := selectedConfigureProfileFactory(parent.Profile, args...)
		cfg, err := volcengine.LoadFileConfig()
		if err != nil {
			return err
		}
		if _, ok := cfg.Profiles[name]; !ok {
			return errors.Errorf("Volcengine profile %q not found", name)
		}
		cfg.Current = name
		if err := volcengine.SaveFileConfig(cfg); err != nil {
			return err
		}
		_, err = fmt.Fprintf(ctx.Output, "Switched to Volcengine profile %q.\n", name)
		return err
	}}
	return cmd
}

func newConfigureRegionCommand(ctx RuntimeContext, parent *configureParentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use: "region [region]", Short: "Update the default region of a Volcengine profile",
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
			name := selectedConfigureProfileFactory(parent.Profile)
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			item, ok := cfg.Profiles[name]
			if !ok || item == nil {
				return errors.Errorf("Volcengine profile %q not found", name)
			}
			item.Region = region
			if err := volcengine.SaveFileConfig(cfg); err != nil {
				return err
			}
			_, err = fmt.Fprintf(ctx.Output, "Updated Volcengine profile %q region to %q.\n", name, region)
			return err
		},
	}
	return cmd
}

func newConfigureAgentPlanCommand(ctx RuntimeContext, parent *configureParentOptions) *cobra.Command {
	var options configureAgentPlanOptions
	cmd := &cobra.Command{
		Use: "agent-plan", Short: "Update Agent Plan defaults for a Volcengine profile",
		Long: `Update only the Agent Plan defaults associated with a Volcengine profile.

This command does not modify access keys, region, endpoint, login session, token
cache, or the current profile. Agent Plan defaults are stored separately from
the Volcengine CLI configuration.`,
		Example: `byted-supabase-cli configure agent-plan --is-agent-plan
byted-supabase-cli configure agent-plan --agent-plan-seat-id seat-xxx
byted-supabase-cli configure agent-plan --is-agent-plan=false --profile dev`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("is-agent-plan") && !cmd.Flags().Changed("agent-plan-seat-id") {
				return cmd.Help()
			}
			name := selectedConfigureProfileFactory(parent.Profile)
			cfg, err := volcengine.LoadFileConfig()
			if err != nil {
				return err
			}
			if item, ok := cfg.Profiles[name]; !ok || item == nil {
				return errors.Errorf("Volcengine profile %q not found", name)
			}
			agentPlan, changed, err := resolveAgentPlanProfileUpdate(
				cmd.Flags().Changed("is-agent-plan"), options.IsAgentPlan,
				cmd.Flags().Changed("agent-plan-seat-id"), options.AgentPlanSeatID,
			)
			if err != nil {
				return err
			}
			if !changed {
				return errors.New("supply --is-agent-plan or --agent-plan-seat-id")
			}
			if err := volcengine.SetSupabaseProfileConfig(name, agentPlan); err != nil {
				return err
			}
			message := fmt.Sprintf("Disabled Agent Plan defaults for Volcengine profile %q.\n", name)
			if agentPlan.AgentPlanSeatID != "" {
				message = fmt.Sprintf("Updated Volcengine profile %q Agent Plan defaults to enterprise edition.\n", name)
			} else if agentPlan.IsAgentPlan {
				message = fmt.Sprintf("Updated Volcengine profile %q Agent Plan defaults to personal edition.\n", name)
			}
			_, err = fmt.Fprint(ctx.Output, message)
			return err
		},
	}
	cmd.Flags().BoolVar(&options.IsAgentPlan, "is-agent-plan", false, "Use Agent Plan by default when creating workspaces with this profile.")
	cmd.Flags().StringVar(&options.AgentPlanSeatID, "agent-plan-seat-id", "", "Default Agent Plan seat ID for enterprise edition.")
	return cmd
}

func newConfigureDeleteCommand(ctx RuntimeContext, parent *configureParentOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "delete [name]", Short: "Delete a Volcengine profile", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := volcengine.LoadFileConfig()
		if err != nil {
			return err
		}
		name, err := configureProfileForDeleteFactory(cmd.Context(), cfg, parent.Profile, args...)
		if err != nil {
			return err
		}
		if _, ok := cfg.Profiles[name]; !ok {
			return errors.Errorf("Volcengine profile %q not found", name)
		}
		shouldDelete, err := utils.NewConsole().PromptYesNo(cmd.Context(), fmt.Sprintf("Do you want to delete Volcengine profile %q?", name), false)
		if err != nil {
			return err
		}
		if !shouldDelete {
			return errors.New("profile deletion canceled")
		}
		delete(cfg.Profiles, name)
		if cfg.Current == name {
			names := sortedConfigureProfileNamesFactory(cfg)
			cfg.Current = "default"
			if len(names) > 0 {
				cfg.Current = names[0]
			}
		}
		if err := volcengine.SaveFileConfig(cfg); err != nil {
			return err
		}
		if err := volcengine.DeleteSupabaseProfileConfig(name); err != nil {
			return err
		}
		_, err = fmt.Fprintf(ctx.Output, "Deleted Volcengine profile %q.\n", name)
		return err
	}}
	return cmd
}

func selectedConfigureProfileFactory(profile string, args ...string) string {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0])
	}
	if strings.TrimSpace(profile) != "" {
		return strings.TrimSpace(profile)
	}
	cfg, err := volcengine.LoadFileConfig()
	if err == nil && strings.TrimSpace(cfg.Current) != "" {
		return strings.TrimSpace(cfg.Current)
	}
	return "default"
}

func configureProfileForSetFactory(profile string) string {
	if strings.TrimSpace(profile) != "" {
		return strings.TrimSpace(profile)
	}
	return "default"
}

func configureRegionOrDefaultFactory(region string) string {
	if region := strings.TrimSpace(region); region != "" {
		return region
	}
	return volcengine.DefaultConsoleLoginRegion
}

func sortedConfigureProfileNamesFactory(cfg volcengine.FileConfig) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func configureProfileForDeleteFactory(ctx context.Context, cfg volcengine.FileConfig, profile string, args ...string) (string, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.TrimSpace(args[0]), nil
	}
	if strings.TrimSpace(profile) != "" {
		return strings.TrimSpace(profile), nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("missing profile name. Supply [name] or --profile.")
	}
	names := sortedConfigureProfileNamesFactory(cfg)
	if len(names) == 0 {
		return "", errors.New("no Volcengine profiles found")
	}
	items := make([]utils.PromptItem, 0, len(names))
	for _, name := range names {
		details := ""
		if item := cfg.Profiles[name]; item != nil {
			details = "mode: " + item.Mode + ", region: " + item.Region
		}
		items = append(items, utils.PromptItem{Summary: name, Details: details})
	}
	choice, err := utils.PromptChoice(ctx, "Which Volcengine profile do you want to delete?", items)
	if err != nil {
		return "", err
	}
	return choice.Summary, nil
}

func renderConfigureProfilesFactory(current string, profiles []*volcengine.Profile, supabaseCfg volcengine.SupabaseFileConfig, output io.Writer) error {
	if utils.OutputFormat.Value != utils.OutputPretty {
		return utils.EncodeOutput(utils.OutputFormat.Value, output, maskConfigureProfiles(profiles, supabaseCfg))
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
		rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %t | %s |", currentMark, profile.Name, profile.Mode, maskValue(profile.AccessKey), maskValue(profile.SecretKey), profile.Region, profile.Endpoint, agentPlan.IsAgentPlan, maskValue(agentPlan.AgentPlanSeatID)))
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
		result = append(result, maskedConfigureProfile{Name: profile.Name, Mode: profile.Mode, AccessKey: maskValue(profile.AccessKey), SecretKey: maskValue(profile.SecretKey), Region: profile.Region, Endpoint: profile.Endpoint, IsAgentPlan: agentPlan.IsAgentPlan, AgentPlanSeatID: maskValue(agentPlan.AgentPlanSeatID)})
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
