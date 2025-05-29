package util

import (
	"errors"

	"github.com/LerianStudio/lib-commons/commons"
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/model"
)

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

	if commons.IsNilOrEmpty(&cfg.OrganizationID) {
		err := "missing organization ID environment variable"

		l.Error(err)

		return errors.New(err)
	}

	return nil
}
