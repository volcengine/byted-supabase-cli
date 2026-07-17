// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package agent

import "testing"

func TestSetGetRoundTrip(t *testing.T) {
	if Get() != nil {
		t.Fatal("no agent should be registered by default")
	}
	a := Base{}
	Set(a)
	t.Cleanup(func() { Set(nil) })
	if Get() != a {
		t.Fatal("Get must return the registered agent")
	}
	Set(nil)
	if Get() != nil {
		t.Fatal("Set(nil) must clear the registration")
	}
}

func TestBaseIsNoOp(t *testing.T) {
	var a Agent = Base{}
	if got := a.Skill(); got != (Skill{}) {
		t.Fatalf("Base.Skill() = %+v, want zero value", got)
	}
	if got := a.MCPServer(); got != (MCPServer{}) {
		t.Fatalf("Base.MCPServer() = %+v, want zero value", got)
	}
	if !a.IncludeMCPFeature("pages") {
		t.Fatal("Base must include every feature")
	}
}
