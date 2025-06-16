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
ORGANIZATION_IDS=your-organization-id1,your-organization-id2
```

### 2. Create a new instance of the middleware:

In your `config.go` file, configure the environment variables for the Auth Service:

```go
import libLicense "github.com/LerianStudio/lib-license-go/middleware"

type Config struct {
    ApplicationName        string   `env:"APPLICATION_NAME"`
    LicenseKey             string   `env:"LICENSE_KEY"`
    OrganizationIDs        string   `env:"ORGANIZATION_IDS"`
}

func InitServers() *Service {
	cfg := &Config{}
	
	logger := zap.InitializeLogger()
	
	licenseClient := libLicense.NewLicenseClient(
		cfg.ApplicationName,
		cfg.LicenseKey,
		cfg.OrganizationIDs,
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
"github.com/LerianStudio/lib-commons/commons"
"github.com/LerianStudio/lib-commons/commons/log"
"github.com/LerianStudio/lib-commons/commons/opentelemetry"
"github.com/LerianStudio/lib-commons/commons/shutdown"
libLicense "github.com/LerianStudio/lib-license-go/middleware"
"github.com/gofiber/fiber/v2"

type Server struct {
	app           *fiber.App
	serverAddress string
	license			  *shutdown.LicenseManagerShutdown
	logger        log.Logger
	telemetry     opentelemetry.Telemetry
}

func (s *Server) ServerAddress() string {
	return s.serverAddress
}

func NewServer(cfg *Config, app *fiber.App, logger log.Logger, telemetry *opentelemetry.Telemetry, licenseClient *libLicense.LicenseClient) *Server {
	return &Server{
		app:           app,
		serverAddress: cfg.ServerAddress,
		license: 			 licenseClient.GetLicenseManagerShutdown(),
		logger:        logger,
		telemetry:     *telemetry,
	}
}

func (s *Server) Run(l *commons.Launcher) error {
	shutdown.StartServerWithGracefulShutdown(
		s.app,
		s.license,
		&s.telemetry,
		s.ServerAddress(),
		s.logger,
	)

	return nil
}
```

## 📧 Contact

For questions or support, contact us at: [contato@lerian.studio](mailto:contato@lerian.studio).
