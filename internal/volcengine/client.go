// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/backend"
	"github.com/volcengine/volcengine-go-sdk/service/aidap"
	"github.com/volcengine/volcengine-go-sdk/service/vpc"
	sdk "github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/endpoints"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
	"github.com/volcengine/volcengine-go-sdk/volcengine/universal"
)

type Client struct {
	cfg      Config
	aidap    *aidap.AIDAP
	vpc      *vpc.VPC
	uni      *universal.Universal
	pagesUni *universal.Universal
	arkUni   *universal.Universal
	iam      *universal.Universal
}

const managementAPIRequestTimeout = 60 * time.Second

func NewClient(cfg Config) *Client {
	return newClient(cfg, nil)
}

func NewWriteClient(cfg Config) *Client {
	maxRetries := 0
	return newClient(cfg, &maxRetries)
}

func newClient(cfg Config, maxRetries *int) *Client {
	credentials := credentials.NewStaticCredentials(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)
	extendRequest := func(ctx context.Context, req *http.Request) {
		req.Header.Set(HeaderFrom, requestSource)
	}
	// An installed control-plane backend may wrap each service's transport so the
	// signed open-API request is rerouted (e.g. to a multi-cloud proxy) before it is
	// sent. With no backend registered this is a no-op (direct Volcengine calls).
	aidapTransport := newTransport()
	vpcTransport := newTransport()
	pagesTransport := newTransport()
	arkTransport := newTransport()
	iamTransport := newTransport()
	if b := backend.Get(); b != nil {
		aidapTransport = b.WrapTransport(aidapServiceName, cfg.Region, aidapTransport)
		vpcTransport = b.WrapTransport(vpc.ServiceName, cfg.Region, vpcTransport)
		pagesTransport = b.WrapTransport(pagesServiceName, pagesRegion, pagesTransport)
		arkTransport = b.WrapTransport(arkServiceName, arkRegion, arkTransport)
		iamTransport = b.WrapTransport(iamServiceName, cfg.Region, iamTransport)
	}
	aidapConfig := sdk.NewConfig().
		WithCredentials(credentials).
		WithRegion(cfg.Region).
		WithHTTPClient(&http.Client{Timeout: managementAPIRequestTimeout, Transport: aidapTransport}).
		WithExtendHttpRequest(extendRequest).
		WithSimpleError(true)
	vpcConfig := sdk.NewConfig().
		WithCredentials(credentials).
		WithRegion(cfg.Region).
		WithHTTPClient(&http.Client{Timeout: managementAPIRequestTimeout, Transport: vpcTransport}).
		WithExtendHttpRequest(extendRequest).
		WithSimpleError(true)
	pagesConfig := sdk.NewConfig().
		WithCredentials(credentials).
		WithRegion(pagesRegion).
		WithHTTPClient(&http.Client{Timeout: managementAPIRequestTimeout, Transport: pagesTransport}).
		WithExtendHttpRequest(extendRequest).
		WithEndpointResolver(endpoints.NewStandardEndpointResolver()).
		WithIPVersion(endpoints.IPVersionDualStack).
		WithSimpleError(true)
	// Ark hosts the personal/enterprise Agent Plan quota query APIs (GetPersonalPlan,
	// GetSeatAFPUsage). These live on the "ark" service (version 2024-01-01), not aidap,
	// so they need a dedicated universal client with ark signing/endpoint resolution.
	arkConfig := sdk.NewConfig().
		WithCredentials(credentials).
		WithRegion(arkRegion).
		WithHTTPClient(&http.Client{Timeout: managementAPIRequestTimeout, Transport: arkTransport}).
		WithExtendHttpRequest(extendRequest).
		WithEndpointResolver(endpoints.NewStandardEndpointResolver()).
		WithSimpleError(true)
	// IAM is a GLOBAL service: the standard resolver would regionalize the host
	// (iam.<region>.volcengineapi.com, which does not resolve), so the global endpoint
	// is pinned explicitly. Used only for the pre-create service-linked-role check.
	iamConfig := sdk.NewConfig().
		WithCredentials(credentials).
		WithRegion(cfg.Region).
		WithHTTPClient(&http.Client{Timeout: managementAPIRequestTimeout, Transport: iamTransport}).
		WithExtendHttpRequest(extendRequest).
		WithEndpoint(iamGlobalEndpoint).
		WithSimpleError(true)
	if maxRetries != nil {
		aidapConfig.WithMaxRetries(*maxRetries)
		vpcConfig.WithMaxRetries(*maxRetries)
		pagesConfig.WithMaxRetries(*maxRetries)
		arkConfig.WithMaxRetries(*maxRetries)
		iamConfig.WithMaxRetries(*maxRetries)
	}
	if cfg.Endpoint != "" {
		aidapConfig.WithEndpoint(cfg.Endpoint)
		vpcConfig.WithEndpoint(cfg.Endpoint)
	} else {
		aidapConfig.WithEndpointResolver(endpoints.NewStandardEndpointResolver())
		vpcConfig.WithEndpointResolver(endpoints.NewStandardEndpointResolver())
	}
	aidapSession := session.Must(session.NewSession(aidapConfig))
	vpcSession := session.Must(session.NewSession(vpcConfig))
	pagesSession := session.Must(session.NewSession(pagesConfig))
	arkSession := session.Must(session.NewSession(arkConfig))
	iamSession := session.Must(session.NewSession(iamConfig))
	return &Client{
		cfg:      cfg,
		aidap:    aidap.New(aidapSession),
		vpc:      vpc.New(vpcSession),
		uni:      universal.New(aidapSession),
		pagesUni: universal.New(pagesSession),
		arkUni:   universal.New(arkSession),
		iam:      universal.New(iamSession),
	}
}

func newTransport() http.RoundTripper {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialIPv4First,
		ForceAttemptHTTP2:     false,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: managementAPIRequestTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func dialIPv4First(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.Errorf("failed to split address %q: %w", address, err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.Errorf("invalid port %q: %w", port, err)
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	addrs = sortIPv4First(addrs)
	dialer := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, addr := range addrs {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), strconv.Itoa(portNumber)))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return dialer.DialContext(ctx, network, address)
}

func sortIPv4First(addrs []net.IPAddr) []net.IPAddr {
	sorted := append([]net.IPAddr(nil), addrs...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].IP.To4() == nil && sorted[j].IP.To4() != nil {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intValue(value *int32) int {
	if value == nil {
		return 0
	}
	return int(*value)
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func floatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func stringSlicePointers(values []string) []*string {
	result := make([]*string, len(values))
	for i := range values {
		result[i] = &values[i]
	}
	return result
}

func stringPointersValue(values []*string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, *value)
		}
	}
	return result
}
