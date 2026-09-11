package solo

import (
	"fmt"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func confirmSoloWrite(cmd *cobra.Command, action string) error {
	yesFlag := cmd.Flags().Lookup("yes")
	if yesFlag == nil {
		yesFlag = cmd.InheritedFlags().Lookup("yes")
	}
	if yesFlag == nil {
		yesFlag = cmd.Root().PersistentFlags().Lookup("yes")
	}
	if yesFlag == nil {
		return nil
	}
	yes := yesFlag.Value.String() == "true"
	if yes {
		return nil
	}
	confirmed, err := utils.NewConsole().PromptYesNo(cmd.Context(), fmt.Sprintf("Confirm %s?", action), false)
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("operation cancelled")
	}
	return nil
}
