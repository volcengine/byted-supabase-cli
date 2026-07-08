package utils

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/byted-supabase-cli/internal/testing/fstest"
)

func TestPromptYesNo(t *testing.T) {
	t.Cleanup(func() {
		AgentMode.Value = "auto"
		viper.Set("YES", false)
	})
	AgentMode.Value = "no"
	viper.Set("YES", false)

	t.Run("rejects non-interactive without yes", func(t *testing.T) {
		c := NewConsole()
		c.IsTTY = false
		// Run test
		val, err := c.PromptYesNo(context.Background(), "test", false)
		// Check error
		assert.ErrorContains(t, err, "confirmation required")
		assert.False(t, val)
	})

	t.Run("allows yes flag outside tty", func(t *testing.T) {
		viper.Set("YES", true)
		t.Cleanup(func() {
			viper.Set("YES", false)
		})
		c := NewConsole()
		c.IsTTY = false
		// Run test
		val, err := c.PromptYesNo(context.Background(), "test", false)
		// Check error
		assert.NoError(t, err)
		assert.True(t, val)
	})

	t.Run("rejects agent mode", func(t *testing.T) {
		AgentMode.Value = "yes"
		t.Cleanup(func() {
			AgentMode.Value = "no"
		})
		c := NewConsole()
		c.IsTTY = true
		// Run test
		val, err := c.PromptYesNo(context.Background(), "test", false)
		// Check error
		assert.ErrorContains(t, err, "confirmation required")
		assert.False(t, val)
	})

	t.Run("parses tty stdin", func(t *testing.T) {
		t.Cleanup(fstest.MockStdin(t, "y"))
		c := NewConsole()
		c.IsTTY = true
		// Run test
		val, err := c.PromptYesNo(context.Background(), "test", false)
		// Check error
		assert.NoError(t, err)
		assert.True(t, val)
	})
}

func TestPromptText(t *testing.T) {
	t.Run("defaults on timeout", func(t *testing.T) {
		t.Cleanup(fstest.MockStdin(t, ""))
		c := NewConsole()
		// Run test
		val, err := c.PromptText(context.Background(), "test")
		// Check error
		assert.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("throws error on cancel", func(t *testing.T) {
		c := NewConsole()
		// Setup cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Run test
		val, err := c.PromptText(ctx, "test")
		// Check error
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, val)
	})
}
