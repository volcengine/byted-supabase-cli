// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Package backend is the control-plane seam for the CLI.
//
// By default no backend is registered and the CLI talks to Volcengine open APIs
// directly (HMAC-signed requests to {service}.{region}.volcengineapi.com). A
// downstream distribution can install a Backend via Set to reroute control-plane
// calls and supply alternative credentials — for example routing through a
// multi-cloud proxy gateway.
//
// The dependency direction is strictly one-way: this open-source module never
// imports any distribution-specific package. Distributions depend on this package,
// not the other way around.
package backend

import "net/http"

// Credentials is the neutral credential/region set exchanged with a Backend. It
// deliberately avoids exposing the internal volcengine.Config type so the seam
// stays importable from other modules.
type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Endpoint        string
}

// Backend reroutes the CLI's control-plane transport and credentials. A nil
// backend (the open-source default) means direct Volcengine calls.
type Backend interface {
	// WrapTransport decorates a service's HTTP transport. It must return base
	// unchanged when the backend is inactive. service is the Volcengine service
	// name (e.g. "aidap", "vpc") and region the target region.
	WrapTransport(service, region string, base http.RoundTripper) http.RoundTripper

	// PrepareCredentials may supply or override credentials before the client is
	// built. When handled is true the CLI skips its normal AK/SK + profile
	// resolution and uses the returned Credentials as-is.
	PrepareCredentials(in Credentials) (out Credentials, handled bool, err error)

	// UsesPersistentProfileCredentials reports whether credentials originate from a
	// persisted local profile. Backends that authenticate ephemerally (e.g. via a
	// proxy token) return false.
	UsesPersistentProfileCredentials() bool
}

var registered Backend

// Set installs the control-plane backend. Call it once during process startup,
// before any client is constructed. Passing nil restores direct mode.
func Set(b Backend) { registered = b }

// Get returns the registered backend, or nil when none is installed.
func Get() Backend { return registered }
