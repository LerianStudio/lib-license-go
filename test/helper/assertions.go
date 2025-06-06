package helper

import (
	"testing"

	"github.com/LerianStudio/lib-license-go/model"
	"github.com/stretchr/testify/assert"
)

// AssertValidationResult is a helper function to validate ValidationResult
func AssertValidationResult(t *testing.T, result *model.ValidationResult, expectedValid bool, expectedDays int) {
	t.Helper()
	assert.Equal(t, expectedValid, result.Valid, "validation result validity mismatch")

	if expectedValid {
		assert.Equal(t, expectedDays, result.ExpiryDaysLeft, "expiry days left mismatch")
	}
}
