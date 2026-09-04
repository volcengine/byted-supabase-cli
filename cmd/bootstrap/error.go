package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-errors/errors"
)

type ErrorHooks struct {
	Suggestion    func() string
	SetSuggestion func(string)
	Debug         func() bool
	Format        func(error) string
	Scope         func(*sentry.Scope)
	CreateTicket  func() bool
	SentryDSN     func() string
}

func RecoverAndExit(hooks ErrorHooks) {
	recovered := recover()
	if recovered == nil {
		return
	}
	var message string
	switch value := recovered.(type) {
	case string:
		message = value
	case error:
		if !errors.Is(value, context.Canceled) && hooks.Suggestion != nil && hooks.SetSuggestion != nil && hooks.Suggestion() == "" && (hooks.Debug == nil || !hooks.Debug()) {
			hooks.SetSuggestion("Run with --debug for more information.")
		}
		message = value.Error()
		if hooks.Format != nil {
			message = hooks.Format(value)
		}
	default:
		message = fmt.Sprintf("%#v", recovered)
	}
	fmt.Fprintln(os.Stderr, message)
	if hooks.Suggestion != nil {
		if suggestion := hooks.Suggestion(); suggestion != "" {
			fmt.Fprintln(os.Stderr, suggestion)
		}
	}
	if hooks.CreateTicket != nil && hooks.CreateTicket() && hooks.SentryDSN != nil && hooks.SentryDSN() != "" {
		if hooks.Scope != nil {
			sentry.ConfigureScope(hooks.Scope)
		}
		if eventID := sentry.CurrentHub().Recover(recovered); eventID != nil && sentry.Flush(2*time.Second) {
			fmt.Fprintf(os.Stderr, "Sent crash report: %s\n", *eventID)
		}
	}
	os.Exit(1)
}

func FormatErrorDetails(message string, requestID, statusCode, errorCode, detail string) string {
	var builder strings.Builder
	builder.WriteString(message)
	if requestID != "" {
		fmt.Fprintf(&builder, "\nRequest ID: %s", requestID)
	}
	if statusCode != "" {
		fmt.Fprintf(&builder, "\nStatus Code: %s", statusCode)
	}
	if errorCode != "" {
		fmt.Fprintf(&builder, "\nError Code: %s", errorCode)
	}
	if detail != "" {
		fmt.Fprintf(&builder, "\nMessage: %s", detail)
	}
	return builder.String()
}
