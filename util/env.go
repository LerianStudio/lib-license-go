package util

import (
	"errors"
	"slices"
	"strings"

	"github.com/LerianStudio/lib-commons/commons"
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/model"
)

// ValidateEnvVariables validates the required environment variables
func ValidateEnvVariables(cfg *model.Config, l log.Logger) error {
	if cfg == nil {
		return errors.New("license client config is nil")
	}

	if commons.IsNilOrEmpty(&cfg.ApplicationName) {
		err := "missing application name environment variable"
		l.Error(err)

		return errors.New(err)
	}

	if commons.IsNilOrEmpty(&cfg.LicenseKey) {
		err := "missing license key environment variable"
		l.Error(err)

		return errors.New(err)
	}

	if commons.IsNilOrEmpty(&cfg.OrganizationIDs) {
		err := "missing organization IDs environment variable"
		l.Error(err)

		return errors.New(err)
	}

	return nil
}

// ParseOrganizationIDs splits the comma-separated organization IDs string into a slice
func ParseOrganizationIDs(orgIDsStr string) []string {
	if orgIDsStr == "" {
		return []string{}
	}

	// Split by comma and trim spaces
	orgIDs := strings.Split(orgIDsStr, ",")
	for i, id := range orgIDs {
		orgIDs[i] = strings.TrimSpace(id)
	}

	return orgIDs
}

// ContainsOrganizationID checks if the given organization ID is in the list of valid IDs
func ContainsOrganizationID(orgIDs []string, orgID string) bool {
	return slices.Contains(orgIDs, orgID)
}
