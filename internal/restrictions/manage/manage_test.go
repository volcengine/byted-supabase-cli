package manage

import (
	"testing"

	"github.com/go-errors/errors"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/volcengine-go-sdk/volcengine/volcengineerr"
)

func TestIsMissingDefaultACL(t *testing.T) {
	err := volcengineerr.New("AccessControlListNameNotFound", "名单[acl-default]不存在。", nil)
	require.True(t, isMissingDefaultACL(errors.Errorf("describe ACL failed: %w", err)))
}

func TestIsMissingDefaultACLDoesNotSwallowOtherErrors(t *testing.T) {
	err := volcengineerr.New("AccessControlListNotFound", "名单不存在。", nil)
	require.False(t, isMissingDefaultACL(err))
}
