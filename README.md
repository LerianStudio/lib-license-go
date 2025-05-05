# Plugin License SDK

A lightweight Go SDK + Fiber middleware to validate plugin licenses against the Lerian backend.

## Features

* Ristretto in-memory cache for fast look-ups
* Periodic background refresh (weekly)
* Fiber middleware → drop-in guard for any route
* Fetches license validity & enabled plugins from Gateway (AWS API Gateway)

## 🚀 How to Use

### 1. Set the needed environment variables:

In your environment configuration or `.env` file, set the following environment variables:

```dotenv
APPLICATION_NAME=your-application-name
LICENSE_KEY=your-plugin-license-key
MIDAZ_ORGANIZATION_ID=your-organization-id
LERIAN_API_GATEWAY_URL=https://your-api-gateway-url
```

### 2. Create a new instance of the middleware:

In your `config.go` file, configure the environment variables for the Auth Service:

```go
type Config struct {
    ApplicationName        string   `env:"APPLICATION_NAME"`
    LicenseKey             string   `env:"LICENSE_KEY"`
    MidazOrganizationID    string   `env:"MIDAZ_ORGANIZATION_ID"`
    LerianAPIGatewayURL    string   `env:"LERIAN_API_GATEWAY_URL"`
}

cfg := &Config{}

logger := zap.InitializeLogger()
```

```go
import libLicense "github.com/LerianStudio/lib-license"

licenseClient := libLicense.NewLicenseClient(cfg.LicenseKey, cfg.MidazOrganizationID, cfg.LerianAPIGatewayURL, &logger)
```

### 3. Use the middleware in your Fiber application:

```go
func NewRoutes(license *libLicense.Validator, [...]) *fiber.App {
    f := fiber.New(fiber.Config{
        DisableStartupMessage: true,
    })
    
    f.Use(license.Middleware())
    
    // Applications routes
    f.Get("/v1/applications", applicationHandler.GetApplications)

    f.Shutdown(func() {
        license.ShutdownBackgroundRefresh()
    })
}
```

## 📧 Contact

For questions or support, contact us at: [contato@lerian.studio](mailto:contato@lerian.studio).
