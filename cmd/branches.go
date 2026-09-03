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
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

func branchesCommandGroup() CommandGroup {
	return CommandGroup{
		Group: nil,
		Factories: []CommandFactory{
			newBranchesCommandFactory,
		},
	}
}

func promptVolcengineBranchName(ctx context.Context) (string, error) {
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing branch name. Supply branches create [name].")
	}
	name, err := console.PromptText(ctx, "Enter branch name: ")
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("branch name cannot be empty")
	}
	return name, nil
}

func promptVolcengineBranchID(ctx context.Context, workspaceID string, branchID *string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("missing branch id. Supply the branch ID argument.")
	}
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	var branches []volcengine.Branch
	for _, region := range cfgSet.Regions {
		result, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).DescribeAllBranches(ctx, volcengine.DescribeBranchesParams{
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return errors.Errorf("failed to list volcengine branches in region %s: %w", region, err)
		}
		branches = append(branches, result.Branches...)
	}
	if len(branches) == 0 {
		return errors.New("no volcengine branches found")
	}
	items := make([]utils.PromptItem, len(branches))
	for i, branch := range branches {
		items[i] = utils.PromptItem{
			Summary: branch.BranchID,
			Details: "name: " + branch.BranchName + ", status: " + branch.BranchStatus,
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a branch:", items)
	if err != nil {
		return err
	}
	*branchID = choice.Summary
	fmt.Fprintln(os.Stderr, "Selected branch ID:", *branchID)
	return nil
}

func promptVolcengineBranchUpdateName(ctx context.Context, name **string) error {
	console := utils.NewConsole()
	if !console.IsTTY {
		return errors.New("nothing to update. Supply --name.")
	}
	input, err := console.PromptText(ctx, "Enter new branch name: ")
	if err != nil {
		return err
	}
	if input = strings.TrimSpace(input); input != "" {
		*name = &input
		return nil
	}
	return errors.New("branch name cannot be empty")
}

func ensureVolcengineRestoreTime(ctx context.Context, value *string) error {
	*value = strings.TrimSpace(*value)
	if *value == "" {
		console := utils.NewConsole()
		if !console.IsTTY {
			return errors.New("missing restore time. Supply --restore-time in RFC3339 format.")
		}
		input, err := console.PromptText(ctx, "Enter restore time in RFC3339 format: ")
		if err != nil {
			return err
		}
		*value = strings.TrimSpace(input)
	}
	if *value == "" {
		return errors.New("restore time cannot be empty")
	}
	if _, err := time.Parse(time.RFC3339, *value); err != nil {
		return errors.Errorf("invalid --restore-time %q, expected RFC3339 format like 2026-05-20T10:00:00Z", *value)
	}
	return nil
}

func ensureStudioUsername(ctx context.Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" {
		return value, nil
	}
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing required flag: --username")
	}
	value, err := console.PromptText(ctx, "Enter Studio username: ")
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("Studio username cannot be empty")
	}
	return value, nil
}

func ensureStudioPassword(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("missing required flag: --password")
	}
	fmt.Fprint(os.Stderr, "Enter new Studio password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", errors.Errorf("failed to read Studio password: %w", err)
	}
	fmt.Fprint(os.Stderr, "Confirm new Studio password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", errors.Errorf("failed to read Studio password confirmation: %w", err)
	}
	if len(password) == 0 {
		return "", errors.New("Studio password cannot be empty")
	}
	if string(password) != string(confirmation) {
		return "", errors.New("Studio passwords do not match")
	}
	return string(password), nil
}
