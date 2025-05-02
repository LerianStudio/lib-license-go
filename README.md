# Plugin License SDK

A lightweight Go SDK + Fiber middleware to validate plugin licenses against the Lerian backend.

## Features

* Ristretto in-memory cache for fast look-ups
* Periodic background refresh (weekly)
* Fiber middleware → drop-in guard for any route
* Fetches license validity & enabled plugins from Gateway (AWS API Gateway)

## Quickstart

```go
import (
    sdk "github.com/LerianStudio/plugin-license-sdk"
    "github.com/gofiber/fiber/v2"
)

func main() {
    lClient := sdk.NewLicenseClient(sdk.LoadFromEnv())

    app := fiber.New()
    app.Use(lClient.Middleware())

    app.Get("/hello", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"msg": "ok"})
    })

    app.Shutdown(func() {
        lClient.ShutdownBackgroundRefresh()
    })

    app.Listen(":8080")
}
```

## Env Vars

| Var | Description |
| --- | ----------- |
| `LICENSE_KEY` | Your Keygen license key |
| `ORG_ID` | Your Midaz Org ID (Part of fingerprint algorithm) |
| `LERIAN_API_GATEWAY_URL` | Gateway base URL (without trailing slash) |

## TODO

* Unit tests (gomock)
* Metrics / tracing hooks
