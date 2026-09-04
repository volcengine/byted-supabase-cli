package bootstrap

import "github.com/getsentry/sentry-go"

type SentryConfigurator func(*sentry.Scope)

func ConfigureSentryScope(configurator SentryConfigurator) {
	sentry.ConfigureScope(configurator)
}
