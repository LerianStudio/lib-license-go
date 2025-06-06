// Package interfaces contains interfaces extracted from the codebase for mocking
package interfaces

import (
	"context"

	"github.com/LerianStudio/lib-license-go/model"
)

// LicenseValidator defines the interface for license validation
type LicenseValidator interface {
	Validate(ctx context.Context) (model.ValidationResult, error)
	ShutdownBackgroundRefresh()
	StartBackgroundRefresh(ctx context.Context)
}
