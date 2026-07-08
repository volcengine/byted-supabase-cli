// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRegion(t *testing.T) {
	for _, region := range []string{
		"cn-beijing",
		"cn-shanghai",
		"cn-chongqing-sdv",
		"cn-nanjing-bbit",
		"cn-guilin-boe",
	} {
		t.Run(region, func(t *testing.T) {
			require.NoError(t, ValidateRegion(region))
		})
	}

	for _, region := range []string{"", "cn_shanghai", "shanghai"} {
		t.Run("invalid_"+region, func(t *testing.T) {
			err := ValidateRegion(region)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "region")
		})
	}
}
