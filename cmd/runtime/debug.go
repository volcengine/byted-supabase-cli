package runtime

import (
	"fmt"
	"net/http"
	"os"

	"github.com/volcengine/byted-supabase-cli/internal/debug"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func ConfigureDebug(enabled, volcengineCommand bool, rootShort string, printVolcengineProfile func()) {
	if !enabled {
		return
	}
	http.DefaultTransport = debug.NewTransport()
	fmt.Fprintln(os.Stderr, rootShort)
	if volcengineCommand {
		printVolcengineProfile()
		return
	}
	fmt.Fprintf(os.Stderr, "Using profile: %s (%s)\n", utils.CurrentProfile.Name, utils.CurrentProfile.ProjectHost)
}
