// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package connection

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPostgresURL(t *testing.T) {
	t.Run("returns raw url", func(t *testing.T) {
		assert.Equal(t,
			"postgresql://postgres:secret@db.example.com/postgres?sslmode=require",
			extractPostgresURL("postgresql://postgres:secret@db.example.com/postgres?sslmode=require"),
		)
	})

	t.Run("extracts psql command url", func(t *testing.T) {
		assert.Equal(t,
			"postgresql://postgres:secret@db.example.com/postgres?sslmode=require",
			extractPostgresURL("psql 'postgresql://postgres:secret@db.example.com/postgres?sslmode=require'"),
		)
	})
}
