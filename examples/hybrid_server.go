package main

import (
	"log"
	"net"

	commonLog "github.com/LerianStudio/lib-commons/commons/log"
	libLicense "github.com/LerianStudio/lib-license-go/middleware"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Example demonstrating how to use the same LicenseClient
// for both HTTP Fiber server and gRPC server
func main() {
	// Initialize logger (replace with your actual logger initialization)
	// For this example, we'll use a nil logger - replace with your actual logger
	var logger *commonLog.Logger

	// Create a single license client instance
	// This will be shared between HTTP and gRPC servers
	licenseClient := libLicense.NewLicenseClient(
		"your-app-id",
		"your-license-key",
		"org1,org2",
		logger,
	)

	if licenseClient == nil {
		log.Fatal("Failed to create license client")
	}

	// Explicitly perform startup validation once
	// This validates the license and starts background refresh
	licenseClient.StartupValidation()

	// Start both servers concurrently
	go startHTTPServer(licenseClient)
	go startGRPCServer(licenseClient)

	// Keep the main goroutine alive
	select {}
}

// startHTTPServer demonstrates HTTP Fiber server setup
func startHTTPServer(licenseClient *libLicense.LicenseClient) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Apply license middleware - this will trigger startup validation ONCE
	app.Use(licenseClient.Middleware())

	// Example routes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	app.Get("/api/v1/users", func(c *fiber.Ctx) error {
		// This route is protected by license validation
		return c.JSON(fiber.Map{"users": []string{"user1", "user2"}})
	})

	log.Println("Starting HTTP server on 127.0.0.1:8080")
	if err := app.Listen("127.0.0.1:8080"); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// startGRPCServer demonstrates gRPC server setup
func startGRPCServer(licenseClient *libLicense.LicenseClient) {
	lis, err := net.Listen("tcp", "127.0.0.1:9090")
	if err != nil {
		log.Fatalf("Failed to listen on port 9090: %v", err)
	}

	// Create gRPC server with license interceptors
	// This will NOT trigger startup validation again (thanks to sync.Once)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(licenseClient.UnaryServerInterceptor()),
		grpc.StreamInterceptor(licenseClient.StreamServerInterceptor()),
	)

	// Register your gRPC services here
	// pb.RegisterYourServiceServer(server, &yourServiceImpl{})

	// Enable reflection for testing with grpcurl
	reflection.Register(server)

	log.Println("Starting gRPC server on 127.0.0.1:9090")
	if err := server.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
