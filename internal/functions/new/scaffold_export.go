// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package new

// PythonRunSh and PythonRequirements expose the embedded Python scaffolding
// templates (run.sh launcher + requirements.txt) so they have a single source of
// truth. The CLI `functions new` writes them to disk; the MCP deploy tool reuses
// them to inject the same scaffolding into a single-file deploy (the branch
// gateway rejects a Python function that is missing run.sh).
func PythonRunSh() string { return pythonRunEmbed }

// PythonRequirements returns the default Python requirements.txt content.
func PythonRequirements() string { return pythonRequirementsEmbed }
