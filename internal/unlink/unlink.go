package unlink

import (
	"context"
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	linkstate "github.com/volcengine/byted-supabase-cli/internal/link"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/credentials"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

func Run(ctx context.Context, fsys afero.Fs, yes bool) error {
	if state, err := linkstate.LoadState(fsys); err == nil {
		if state.Type == linkstate.LinkTypeSolo {
			if err := confirmSoloUnlink(ctx, state.SupabaseURL, yes); err != nil {
				return err
			}
			if err := linkstate.ClearState(fsys); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Successfully unlinked target %q.\n", state.SupabaseURL)
			fmt.Fprintln(os.Stdout, "The next command will use the management command tree.")
			return nil
		}
	}
	if projectRef, err := afero.ReadFile(fsys, utils.ProjectRefPath); errors.Is(err, os.ErrNotExist) {
		return errors.New(utils.ErrNotLinked)
	} else if err != nil {
		return errors.Errorf("failed to load project ref: %w", err)
	} else if err := Unlink(string(projectRef), fsys); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Finished "+utils.Aqua("supabase unlink")+".")
	return nil
}

func Unlink(projectRef string, fsys afero.Fs) error {
	fmt.Fprintln(os.Stderr, "Unlinking project:", projectRef)
	var allErrors []error
	// Remove temp directory
	if err := fsys.RemoveAll(utils.TempDir); err != nil {
		wrapped := errors.Errorf("failed to remove temp directory: %w", err)
		allErrors = append(allErrors, wrapped)
	}
	if err := linkstate.ClearState(fsys); err != nil {
		allErrors = append(allErrors, err)
	}
	// Remove linked credentials
	if err := credentials.StoreProvider.Delete(projectRef); err != nil &&
		!errors.Is(err, credentials.ErrNotSupported) &&
		!errors.Is(err, keyring.ErrNotFound) {
		allErrors = append(allErrors, err)
	}
	return errors.Join(allErrors...)
}

func confirmSoloUnlink(ctx context.Context, supabaseURL string, yes bool) error {
	if yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("confirmation required in non-interactive mode; re-run with --yes to confirm")
	}
	confirmed, err := utils.NewConsole().PromptYesNo(ctx,
		fmt.Sprintf("Remove the link to Supabase branch %q?", supabaseURL), false)
	if err != nil {
		return err
	}
	if !confirmed {
		return errors.New("unlink cancelled")
	}
	return nil
}
