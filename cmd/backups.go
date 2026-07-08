package cmd

import (
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/backups/list"
	"github.com/volcengine/byted-supabase-cli/internal/backups/restore"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	backupsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "backups",
		Short:   "Manage Supabase physical backups",
	}

	backupListCmd = &cobra.Command{
		Use:   "list",
		Short: "Lists available physical backups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return list.Run(cmd.Context())
		},
	}

	timestamp int64

	backupRestoreCmd = &cobra.Command{
		Use:   "restore",
		Short: "Restore to a specific timestamp using PITR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return restore.Run(cmd.Context(), timestamp)
		},
	}
)

func init() {
	backupFlags := backupsCmd.PersistentFlags()
	backupFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project.")
	backupsCmd.AddCommand(backupListCmd)
	restoreFlags := backupRestoreCmd.Flags()
	restoreFlags.Int64VarP(&timestamp, "timestamp", "t", 0, "The recovery time target in seconds since epoch.")
	backupsCmd.AddCommand(backupRestoreCmd)
	// Volcengine restore is branch-based instead of project backup-based.
	// Keep the original command registered code here; expose restore capabilities under branches later.
	// rootCmd.AddCommand(backupsCmd)
}
