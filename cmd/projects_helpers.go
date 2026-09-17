// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

const (
	volcengineDefaultListLimit     = 10
	volcenginePromptWorkspaceLimit = 100
)

var volcRegion string

func validateVolcenginePageFlags(limit, offset int, allowZeroLimit bool) error {
	if limit < 0 || (!allowZeroLimit && limit == 0) || limit > 100 {
		if allowZeroLimit {
			return errors.New("invalid --limit. Expected 0 to list all items, or a value from 1 to 100.")
		}
		return errors.New("invalid --limit. Expected a value from 1 to 100.")
	}
	if offset < 0 {
		return errors.New("invalid --offset. Expected a value greater than or equal to 0.")
	}
	return nil
}

func normalizeOperationStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "start":
		return "Start", nil
	case "running":
		return "Running", nil
	case "success":
		return "Success", nil
	case "failed":
		return "Failed", nil
	default:
		return "", errors.New("invalid --status. Expected Start, Running, Success, or Failed.")
	}
}

func validateOperationTimeRange(start, end string) error {
	var startTime, endTime time.Time
	var err error
	if strings.TrimSpace(start) != "" {
		startTime, err = time.Parse(time.RFC3339, strings.TrimSpace(start))
		if err != nil {
			return errors.New("invalid --start-time. Expected RFC3339 format like 2026-05-25T10:00:00Z.")
		}
	}
	if strings.TrimSpace(end) != "" {
		endTime, err = time.Parse(time.RFC3339, strings.TrimSpace(end))
		if err != nil {
			return errors.New("invalid --end-time. Expected RFC3339 format like 2026-05-25T10:00:00Z.")
		}
	}
	if !startTime.IsZero() && !endTime.IsZero() && startTime.After(endTime) {
		return errors.New("--start-time must be earlier than or equal to --end-time.")
	}
	return nil
}

func promptVolcengineDeletionProtection(ctx context.Context) (bool, error) {
	items := []utils.PromptItem{
		{
			Summary: "enable",
			Details: "Enable deletion protection.",
		},
		{
			Summary: "disable",
			Details: "Disable deletion protection.",
		},
	}
	choice, err := utils.PromptChoice(ctx, "Select deletion protection policy:", items)
	if err != nil {
		return false, err
	}
	return choice.Summary == "enable", nil
}

func promptVolcengineTags(ctx context.Context) ([]string, error) {
	input, err := utils.NewConsole().PromptText(ctx, "Enter tags in key=value format, separated by spaces: ")
	if err != nil {
		return nil, err
	}
	values, err := splitVolcengineTagFields(input)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errors.New("missing required flag: --tag")
	}
	return values, nil
}

func promptVolcengineTagKeys(ctx context.Context, workspaceID string) ([]string, error) {
	workspace, err := describeVolcengineWorkspaceForPrompt(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(workspace.WorkspaceTags) == 0 {
		return nil, errors.Errorf("workspace %s has no tags to delete", workspaceID)
	}
	items := make([]utils.PromptItem, 0, len(workspace.WorkspaceTags))
	for _, tag := range workspace.WorkspaceTags {
		if tag.Key == "" {
			continue
		}
		details := "value: " + tag.Value
		if tag.System {
			details += ", system: true"
		}
		items = append(items, utils.PromptItem{
			Summary: tag.Key,
			Details: details,
		})
	}
	if len(items) == 0 {
		return nil, errors.Errorf("workspace %s has no tags to delete", workspaceID)
	}
	choice, err := utils.PromptChoice(ctx, "Which tag key do you want to delete?", items)
	if err != nil {
		return nil, err
	}
	return []string{choice.Summary}, nil
}

func promptVolcengineFloat64(ctx context.Context, label string, current float64) (float64, error) {
	input, err := utils.NewConsole().PromptText(ctx, fmt.Sprintf("%s (current: %v): ", label, current))
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(input) == "" {
		return current, nil
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
	if err != nil {
		return 0, errors.Errorf("invalid %s: %w", label, err)
	}
	return value, nil
}

func promptVolcengineInt(ctx context.Context, label string, current int) (int, error) {
	input, err := utils.NewConsole().PromptText(ctx, fmt.Sprintf("%s (current: %d): ", label, current))
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(input) == "" {
		return current, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, errors.Errorf("invalid %s: %w", label, err)
	}
	return value, nil
}

func describeVolcengineComputeSettingsForPrompt(ctx context.Context, workspaceID, serviceType string) (volcengine.Compute, error) {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return volcengine.Compute{}, err
	}
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		client := volcengine.NewClient(cfg)
		detail, err := client.DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := detail.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Compute{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		defaultBranch, err := client.ResolveDefaultBranch(ctx, workspace.WorkspaceID)
		if err != nil {
			return volcengine.Compute{}, err
		}
		branchID := defaultBranch.BranchID
		if branchID == "" {
			return volcengine.Compute{}, errors.Errorf("failed to resolve default branch for workspace %s", workspace.WorkspaceID)
		}
		computes, err := client.DescribeComputes(ctx, workspace.WorkspaceID, branchID, serviceType)
		if err != nil {
			return volcengine.Compute{}, err
		}
		for _, compute := range computes.Computes {
			if strings.EqualFold(compute.ServiceType, serviceType) {
				return compute, nil
			}
		}
		return volcengine.Compute{}, errors.Errorf("no %s compute found for workspace %s branch %s", serviceType, workspace.WorkspaceID, branchID)
	}
	if lastErr != nil {
		return volcengine.Compute{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Compute{}, errors.Errorf("failed to describe volcengine project %s", workspaceID)
}

func validateVolcengineComputeSettingsValues(serviceType string, minCU, maxCU float64, suspendTimeout *int) error {
	if !isValidVolcengineComputeCU(minCU) {
		return errors.New("invalid --min-cu. Expected 0.25, 0.5, or an integer between 1 and 32.")
	}
	if !isValidVolcengineComputeCU(maxCU) {
		return errors.New("invalid --max-cu. Expected 0.25, 0.5, or an integer between 1 and 32.")
	}
	if serviceType == volcengine.ServiceTypeSupabase && minCU < 0.5 {
		return errors.New("invalid --min-cu for Supabase service type. Expected 0.5 or greater.")
	}
	if minCU < 0.25 {
		return errors.New("invalid --min-cu. Expected 0.25 or greater.")
	}
	if maxCU > 32 {
		return errors.New("invalid --max-cu. Expected 32 or less.")
	}
	if maxCU < minCU {
		return errors.New("invalid --max-cu. Expected --max-cu to be greater than or equal to --min-cu.")
	}
	if maxCU > minCU*8 {
		return errors.New("invalid compute CU range. Expected --max-cu to be at most 8 times --min-cu.")
	}
	if suspendTimeout != nil {
		if *suspendTimeout < -1 || (*suspendTimeout > 0 && *suspendTimeout < 300) || *suspendTimeout > 604800 {
			return errors.New("invalid --suspend-timeout-seconds. Expected -1, 0, or a value between 300 and 604800.")
		}
	}
	return nil
}

func isValidVolcengineComputeCU(value float64) bool {
	if value == 0.25 || value == 0.5 {
		return true
	}
	if value < 1 || value > 32 {
		return false
	}
	return value == float64(int(value))
}

func describeVolcengineWorkspaceForPrompt(ctx context.Context, workspaceID string) (volcengine.Workspace, error) {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return volcengine.Workspace{}, err
	}
	var lastErr error
	for _, region := range cfgSet.Regions {
		result, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := result.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Workspace{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		return workspace, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, errors.Errorf("volcengine project %s not found", workspaceID)
}

func parseVolcengineTags(values []string) ([]volcengine.WorkspaceTag, error) {
	pairs, err := expandVolcengineTagValues(values)
	if err != nil {
		return nil, err
	}
	tags := make([]volcengine.WorkspaceTag, 0, len(pairs))
	for _, value := range pairs {
		key, tagValue, ok := parseVolcengineTagPair(value)
		if !ok || key == "" {
			return nil, errors.Errorf("invalid tag %q. Use key=value format.", value)
		}
		tags = append(tags, volcengine.WorkspaceTag{
			Key:   key,
			Value: tagValue,
		})
	}
	return tags, nil
}

func expandVolcengineTagValues(values []string) ([]string, error) {
	var pairs []string
	for _, value := range values {
		fields, err := splitVolcengineTagFields(value)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, fields...)
	}
	return pairs, nil
}

func splitVolcengineTagFields(input string) ([]string, error) {
	var fields []string
	var builder strings.Builder
	var quote rune
	escaped := false
	for _, r := range input {
		if escaped {
			builder.WriteRune('\\')
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			builder.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		switch {
		case r == '"':
			quote = r
			builder.WriteRune(r)
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if field := strings.TrimSpace(builder.String()); field != "" {
				fields = append(fields, field)
				builder.Reset()
			}
		default:
			builder.WriteRune(r)
		}
	}
	if escaped {
		builder.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote in tag value")
	}
	if field := strings.TrimSpace(builder.String()); field != "" {
		fields = append(fields, field)
	}
	return fields, nil
}

func parseVolcengineTagPair(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, '\'') {
		return "", "", false
	}
	separator := findVolcengineTagSeparator(value)
	if separator < 0 {
		return "", "", false
	}
	key, ok := unquoteVolcengineTagPart(value[:separator])
	if !ok {
		return "", "", false
	}
	tagValue, ok := unquoteVolcengineTagPart(value[separator+1:])
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(tagValue), true
}

func findVolcengineTagSeparator(value string) int {
	var quote rune
	escaped := false
	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' {
			quote = r
			continue
		}
		if r == '=' {
			return i
		}
	}
	return -1
}

func unquoteVolcengineTagPart(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if quote == '"' && value[len(value)-1] == quote {
			return unescapeVolcengineTagPart(value[1 : len(value)-1]), true
		}
		if quote == '"' || value[len(value)-1] == '"' {
			return "", false
		}
	}
	return unescapeVolcengineTagPart(value), true
}

func unescapeVolcengineTagPart(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	escaped := false
	for _, r := range value {
		if escaped {
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		builder.WriteRune(r)
	}
	if escaped {
		builder.WriteRune('\\')
	}
	return builder.String()
}

func parseVolcengineTagKeys(values []string) ([]string, error) {
	keys := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			return nil, errors.New("tag key cannot be empty")
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func ensureVolcengineRegion(ctx context.Context, fsys afero.Fs, allowLinked bool) error {
	if volcengine.HasRegionOverride() {
		return nil
	}
	if allowLinked {
		linkedRegion, err := volcengine.LoadLinkedRegion(fsys)
		if err != nil {
			return err
		}
		if linkedRegion != "" {
			volcengine.SetRegionOverride(linkedRegion)
			return nil
		}
	}
	if volcengine.RegionSetting() != "" {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		volcengine.SetRegionOverride(volcengine.DefaultRegion)
		fmt.Fprintln(os.Stderr, "No region configured; using default region:", volcengine.DefaultRegion)
		return nil
	}
	region, err := promptVolcengineRegion(ctx)
	if err != nil {
		return err
	}
	volcengine.SetRegionOverride(region)
	fmt.Fprintln(os.Stderr, "Selected region:", region)
	return nil
}

func promptVolcengineRegion(ctx context.Context) (string, error) {
	regions := volcengine.ConfiguredRegions()
	if len(regions) == 0 {
		return volcengine.DefaultRegion, nil
	}
	items := make([]utils.PromptItem, len(regions))
	for i, region := range regions {
		items[i] = utils.PromptItem{
			Summary: region,
			Details: region,
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a Volcengine region:", items)
	if err != nil {
		return "", err
	}
	return choice.Summary, nil
}
