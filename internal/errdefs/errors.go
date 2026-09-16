package errdefs

import "errors"

var ErrMissingWorkspaceID = errors.New("No workspace selected. Use --workspace-id or --project-ref, or run `byted-supabase-cli link` to link one.")
