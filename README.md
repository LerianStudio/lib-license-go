# Plugin License SDK

A lightweight Go SDK with HTTP middleware and gRPC interceptors to validate plugin licenses against the Lerian backend.

## Features

* Ristretto in-memory cache for fast look-ups
* Periodic background refresh (weekly)
* **HTTP Middleware** → Fiber middleware for HTTP routes
* **gRPC Interceptors** → Unary and streaming interceptors for gRPC services
* Fetches license validity & enabled plugins from Gateway (AWS API Gateway)
* Supports both global and multi-organization license validation modes

## 🚀 Quick Start

### 1. Environment Configuration

Set the required environment variables in your `.env` file or environment configuration:

```dotenv
LICENSE_KEY=your-plugin-license-key
ORGANIZATION_IDS=your-organization-id1,your-organization-id2
```

### 2.1 Initialize the License Client

Create an ApplicationName constant in your `pkg/constant/app.go` file:

```go
package constant

const ApplicationName = "your-application-name"
```

### 2.2 Initialize the License Client

Create a license client instance in your application `internal/bootstrap/config.go` file:

```go
import (
    "github.com/LerianStudio/lib-license-go/middleware"
    "github.com/LerianStudio/lib-license-go/pkg/constant"
)

type Config struct {
    LicenseKey      string `env:"LICENSE_KEY"`
    OrganizationIDs string `env:"ORGANIZATION_IDS"`
}

func InitServices() *Service {
    cfg := &Config{}
    
    logger := zap.InitializeLogger()
    
    licenseClient := libLicense.NewLicenseClient(
        constant.ApplicationName,
        cfg.LicenseKey,
        cfg.OrganizationIDs,
        &logger,
    )

    // Use with HTTP routers and server
	httpApp := in.NewRoutes(logger, telemetry, exampleHandler, licenseClient)
	serverAPI := NewHTTPServer(cfg, httpApp, logger, telemetry, licenseClient)

    // Use with gRPC routers and server
	grpcApp := grcpcin.NewRouterGRPC(logger, telemetry, exampleQuery, exampleCommand, licenseClient)
	serverGRPC := NewGRPCServer(cfg, grpcApp, logger, telemetry, licenseClient)

    return &Service{
        httpApp,
        grpcServer,
        logger,
    }
}
```

## 📡 HTTP Middleware Usage

### Basic Fiber Integration

```go
func NewRoutes(license *libLicense.LicenseClient) *fiber.App {
    f := fiber.New(fiber.Config{
        DisableStartupMessage: true,
    })
    
    // Apply license middleware to all routes
    f.Use(license.Middleware())
    
    // Your application routes
    f.Get("/v1/applications", applicationHandler.GetApplications)
    f.Post("/v1/users", userHandler.CreateUser)
    
    return f
}
```

## 🔌 gRPC Interceptor Usage

### Unary RPC Interceptor

```go
import (
    "google.golang.org/grpc"
    libLicense "github.com/LerianStudio/lib-license-go/middleware"
)

func NewGRPCServer(license *libLicense.LicenseClient) *grpc.Server {
    // Create gRPC server with license interceptor
    server := grpc.NewServer(
        grpc.UnaryInterceptor(license.UnaryServerInterceptor()),
    )
    
    // Register your services
    pb.RegisterYourServiceServer(server, &yourServiceImpl{})
    
    return server
}
```

### Streaming RPC Interceptor

```go
func NewGRPCServerWithStreaming(license *libLicense.LicenseClient) *grpc.Server {
    // Create gRPC server with both unary and streaming interceptors
    server := grpc.NewServer(
        grpc.UnaryInterceptor(license.UnaryServerInterceptor()),
        grpc.StreamInterceptor(license.StreamServerInterceptor()),
    )
    
    // Register your services
    pb.RegisterYourServiceServer(server, &yourServiceImpl{})
    
    return server
}
```

### HTTP Multi-Organization Header

For multi-organization mode, ensure your HTTP requests include the organization ID header:

```bash
curl -H "X-Organization-ID: your-org-id" http://localhost:8080/v1/applications
```

### gRPC Multi-Organization Metadata

For multi-organization mode, gRPC clients must include the organization ID in metadata:

```go
import (
    "context"
    "google.golang.org/grpc/metadata"
)

func callGRPCService(client pb.YourServiceClient, orgID string) {
    // Add organization ID to metadata
    ctx := metadata.AppendToOutgoingContext(
        context.Background(),
        "X-Organization-ID", orgID,
    )
    
    // Make the gRPC call
    response, err := client.YourMethod(ctx, &pb.YourRequest{})
    if err != nil {
        log.Fatalf("gRPC call failed: %v", err)
    }
}
```

## 🛡️ Graceful Shutdown Integration

### 📡 HTTP Shutdown

```go
libCommons "github.com/LerianStudio/lib-commons/commons"
libCommonsLog "github.com/LerianStudio/lib-commons/commons/log"
libCommonsOtel "github.com/LerianStudio/lib-commons/commons/opentelemetry"
libCommonsLicense "github.com/LerianStudio/lib-commons/commons/license"
libCommonsServer "github.com/LerianStudio/lib-commons/commons/server"
libLicense "github.com/LerianStudio/lib-license-go/middleware"
"github.com/gofiber/fiber/v2"

type HTTPServer struct {
	app             *fiber.App
	serverAddress   string
	license		    *libCommonsLicense.ManagerShutdown
	logger          libCommonsLog.Logger
	telemetry       libCommonsOtel.Telemetry
}

func (s *HTTPServer) ServerAddress() string {
	return s.serverAddress
}

func NewHTTPServer(cfg *Config, app *fiber.App, logger libCommonsLog.Logger, telemetry *libCommonsOtel.Telemetry, licenseClient *libLicense.LicenseClient) *HTTPServer {
	return &HTTPServer{
		app:            app,
		serverAddress:  cfg.ServerAddress,
		license:        licenseClient.GetLicenseManagerShutdown(),
		logger:         logger,
		telemetry:      *telemetry,
	}
}

func (s *HTTPServer) Run(l *libCommons.Launcher) error {
	libCommonsServer.NewServerManager(s.license, &s.telemetry, s.logger).
        WithHTTPServer(s.app, s.serverAddress).
        StartWithGracefulShutdown()

    return nil
}
```
## 🔌 GRPC Shutdown

```go
import (
	"net"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	libCommons "github.com/LerianStudio/lib-commons/commons"
	libCommonsLicense "github.com/LerianStudio/lib-commons/commons/license"
	libCommonsLog "github.com/LerianStudio/lib-commons/commons/log"
	libCommonsOtel "github.com/LerianStudio/lib-commons/commons/opentelemetry"
	libCommonsServer "github.com/LerianStudio/lib-commons/commons/server"
	libLicense "github.com/LerianStudio/lib-license-go/middleware"
)

// GRPCServer represents the gRPC server for Ledger service.
type GRPCServer struct {
	grpcServer   *grpc.Server
	protoAddress string
	license      *libCommonsLicense.ManagerShutdown
	logger       libCommonsLog.Logger
	telemetry    libCommonsOtel.Telemetry
}

// ProtoAddress returns is a convenience method to return the proto server address.
func (s *GRPCServer) ProtoAddress() string {
	return s.protoAddress
}

// NewGRPCServer creates an instance of gRPC Server.
func NewGRPCServer(cfg *Config, grpcServer *grpc.Server, logger libCommonsLog.Logger, telemetry *libCommonsOtel.Telemetry, lc *libLicense.LicenseClient) *GRPCServer {
	return &GRPCServer{
		grpcServer:   grpcServer,
		protoAddress: cfg.ProtoAddress,
		license:      lc.GetLicenseManagerShutdown(),
		logger:       logger,
		telemetry:    *telemetry,
	}
}

// Run gRPC server.
func (s *GRPCServer) Run(l *libCommons.Launcher) error {
	libCommonsServer.NewServerManager(s.license, &s.telemetry, s.logger).
        WithGRPCServer(s.grpcServer, s.protoAddress).
        StartWithGracefulShutdown()

    return nil
}
```

## 🔧 Advanced Configuration

### Custom Termination Handler

```go
// Set custom behavior when license validation fails
licenseClient.SetTerminationHandler(func(reason string) {
    log.Fatalf("License validation failed: %s", reason)
    os.Exit(1)
})
```

### Manual Shutdown

```go
// Manually stop background refresh process
defer licenseClient.ShutdownBackgroundRefresh()
```

## 🏗️ Architecture

The SDK is organized into separate files for better maintainability:

- `client.go` - Core license client and shared validation logic
- `http.go` - HTTP Fiber middleware implementation  
- `grpc.go` - gRPC unary and streaming interceptors
- `middleware.go` - Package documentation and overview

### Benefits of Single Client

- ✅ **Single startup validation** - No duplicate license checks
- ✅ **One background refresh** - Shared cache and refresh cycle  
- ✅ **Consistent state** - Both servers see the same license status
- ✅ **Resource efficiency** - Reduced memory and network usage
- ✅ **Synchronized shutdown** - Single point for cleanup

### Per-Request Validation

While startup and background validation happen once, per-request validation works independently for each protocol:

- **HTTP requests** validate organization ID from `X-Organization-ID` header
- **gRPC requests** validate organization ID from metadata
- Each request is validated separately (no shared state per request)

## 🚨 Error Handling

### HTTP Errors
- `400 Bad Request`
  - `LCS-0010` - Missing organization ID header
  - `LCS-0011` - Unknown organization ID
  - `LCS-0002` - No organization IDs configured
- `403 Forbidden`
  - `LCS-0013` - Organization license is invalid or expired
  - `LCS-0012` - Failed to validate organization license
  - `LCS-0003` - No valid licenses found for any organization
- `500 Internal Server Error`
  - `LCS-0001` - Internal server error during license validation

### gRPC Errors  
- `INVALID_ARGUMENT`
  - `LCS-0010` - Missing organization ID header in metadata
  - `LCS-0011` - Unknown organization ID
- `PERMISSION_DENIED`
  - `LCS-0013` - Organization license is invalid or expired
  - `LCS-0012` - Failed to validate organization license
  - `LCS-0003` - No valid licenses found for any organization
- `INTERNAL`
  - `LCS-0001` - Internal server error during license validation
  - Missing metadata in gRPC context

## 📧 Contact

For questions or support, contact us at: [contato@lerian.studio](mailto:contato@lerian.studio).
