package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func configureRootCommand(root *cobra.Command, profile RuntimeProfile) {
	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("SUPABASE")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()
	})

	flags := root.PersistentFlags()
	flags.Bool("yes", false, "answer yes to all prompts")
	cobra.CheckErr(viper.BindPFlag("YES", flags.Lookup("yes")))
	flags.Bool("debug", false, "output debug logs to stderr")
	flags.String("workdir", "", "path to a Supabase project directory")
	flags.Bool("experimental", false, "enable experimental features")
	flags.String("network-id", "", "use the specified docker network instead of a generated one")
	if profile.Mode == RuntimeModeSolo {
		flags.StringVar(&rootProfile, "profile", "default", "use a specific linked-branch profile")
		flags.VarP(&utils.OutputFormat, "output", "o", "output format of status variables")
		flags.Var(&utils.DNSResolver, "dns-resolver", "lookup domain names using the specified resolver")
		flags.VarP(&utils.AgentMode, "agent", "", "Override agent detection: yes, no, or auto (default auto)")
		cobra.CheckErr(viper.BindPFlags(flags))
		cobra.CheckErr(flags.MarkHidden("experimental"))
		cobra.CheckErr(flags.MarkHidden("network-id"))
		cobra.CheckErr(flags.MarkHidden("dns-resolver"))
		root.AddGroup(&cobra.Group{ID: groupLocalDev, Title: "Configuration:"})
		root.AddGroup(&cobra.Group{ID: "tools", Title: "Tools:"})
		root.AddGroup(&cobra.Group{ID: "data-plane", Title: "Supabase Services:"})
		root.SetVersionTemplate("{{.Version}}\n")
		return
	}
	flags.StringVar(&rootProfile, "profile", "supabase", "use a specific profile; for Volcengine commands, overrides the current configure profile")
	flags.Lookup("profile").DefValue = "current"
	flags.StringVar(&volcRegion, "region", "", "Volcengine region for management API requests.")
	flags.VarP(&utils.OutputFormat, "output", "o", "output format of status variables")
	flags.Var(&utils.DNSResolver, "dns-resolver", "lookup domain names using the specified resolver")
	flags.VarP(&utils.AgentMode, "agent", "", "Override agent detection: yes, no, or auto (default auto)")
	cobra.CheckErr(viper.BindPFlags(flags))
	cobra.CheckErr(flags.MarkHidden("experimental"))
	cobra.CheckErr(flags.MarkHidden("network-id"))
	cobra.CheckErr(flags.MarkHidden("dns-resolver"))

	root.SetVersionTemplate("{{.Version}}\n")
	root.AddGroup(&cobra.Group{ID: groupLocalDev, Title: "Local Development:"})
	root.AddGroup(&cobra.Group{ID: groupManagementAPI, Title: "Management APIs:"})
}
