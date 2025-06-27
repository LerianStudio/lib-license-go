package pkg

import (
	"slices"
	"strings"
)

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
