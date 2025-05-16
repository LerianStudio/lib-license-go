package model

type Config struct {
	ApplicationName   string `json:"applicationName"`
	LicenseKey        string `json:"licenseKey"`
	OrganizationID    string `json:"organizationId"`
	PluginEnvironment string `json:"pluginEnvironment"`
	Fingerprint       string `json:"fingerprint"`
}
