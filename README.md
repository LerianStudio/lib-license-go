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
MIDAZ_LICENSE_KEY=your-plugin-license-key
MIDAZ_ORGANIZATION_IDS=your-organization-id1,your-organization-id2
```

### 2. Create a new instance of the middleware:

In your `config.go` file, configure the environment variables for the Auth Service:

```go
import libLicense "github.com/LerianStudio/lib-license-go/middleware"

type Config struct {
    ApplicationName        string   `env:"APPLICATION_NAME"`
    LicenseKey             string   `env:"MIDAZ_LICENSE_KEY"`
    MidazOrganizationIDs   string   `env:"MIDAZ_ORGANIZATION_IDS"`
}

func InitServers() *Service {
	cfg := &Config{}
	
	logger := zap.InitializeLogger()
	
	licenseClient := libLicense.NewLicenseClient(
		cfg.ApplicationName,
		cfg.LicenseKey,
		cfg.MidazOrganizationID,
		&logger,
	)

	httpApp := httpIn.NewRoutes(logger, [...], licenseClient)

	serverAPI := NewServer(cfg, httpApp, logger, [...])

	return &Service{
		serverAPI,
		logger,
	}
```

### 3. Use the middleware in your Fiber application:

```go
func NewRoutes(license *libLicense.LicenseClient, [...]) *fiber.App {
    f := fiber.New(fiber.Config{
        DisableStartupMessage: true,
    })
    
    f.Use(license.Middleware())
    
    // Applications routes
    f.Get("/v1/applications", applicationHandler.GetApplications)
}
```

#### Add graceful shutdown support in your server using the `lib-commons` package function `StartServerWithGracefulShutdown`

```go
libCommons "github.com/LerianStudio/lib-commons/commons/shutdown"

func (s *Server) Run(l *commons.Launcher) error {
	s.Info("Starting server with graceful shutdown support")

	libCommons.StartServerWithGracefulShutdown(
		s.app,
		s.licenseClient,
		&s.Telemetry,
		s.ServerAddress(),
		s.Logger,
	)

	return nil
}
```

## 📧 Contact

For questions or support, contact us at: [contato@lerian.studio](mailto:contato@lerian.studio).
