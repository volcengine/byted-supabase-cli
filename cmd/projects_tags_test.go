// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVolcengineTags(t *testing.T) {
	tags, err := parseVolcengineTags([]string{
		"env=dev",
		`"=="=""`,
		`"==="=""`,
		`a\=b=c\,d`,
		`"a b"=space-key`,
	})
	require.NoError(t, err)
	require.Len(t, tags, 5)
	assert.Equal(t, "env", tags[0].Key)
	assert.Equal(t, "dev", tags[0].Value)
	assert.Equal(t, "==", tags[1].Key)
	assert.Equal(t, "", tags[1].Value)
	assert.Equal(t, "===", tags[2].Key)
	assert.Equal(t, "", tags[2].Value)
	assert.Equal(t, "a=b", tags[3].Key)
	assert.Equal(t, "c,d", tags[3].Value)
	assert.Equal(t, "a b", tags[4].Key)
	assert.Equal(t, "space-key", tags[4].Value)
}

func TestParseVolcengineTagsSplitsMultiplePairsInOneFlag(t *testing.T) {
	tags, err := parseVolcengineTags([]string{`"=="="" "==="=""`})
	require.NoError(t, err)
	require.Len(t, tags, 2)
	assert.Equal(t, "==", tags[0].Key)
	assert.Equal(t, "", tags[0].Value)
	assert.Equal(t, "===", tags[1].Key)
	assert.Equal(t, "", tags[1].Value)
}

func TestParseVolcengineTagsRejectsInvalidPairs(t *testing.T) {
	_, err := parseVolcengineTags([]string{"===''"})
	assert.Error(t, err)

	_, err = parseVolcengineTags([]string{"'unterminated=value"})
	assert.Error(t, err)

	_, err = parseVolcengineTags([]string{"'=='=''"})
	assert.Error(t, err)
}
