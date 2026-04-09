// Package main is the entry point for the Future Appointment API.
//
// @title           Future Appointment API
// @version         1.0
// @description     HTTP/JSON API for scheduling 30-minute trainer appointments. All times are RFC3339; business hours are evaluated in America/Los_Angeles (M-F 8am-5pm).
// @BasePath        /
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/pborgen/future-api/docs" // generated swagger spec
	"github.com/pborgen/future-api/internal/appointment"
	"github.com/pborgen/future-api/internal/config"
	"github.com/pborgen/future-api/internal/db"
	"github.com/pborgen/future-api/internal/httputil"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Get()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	apptSvc := appointment.NewService(pool)
	if n, err := apptSvc.SeedFromFile(ctx, cfg.SeedFile); err != nil {
		log.Printf("warning: seed failed: %v", err)
	} else if n > 0 {
		log.Printf("seeded %d appointments from %s", n, cfg.SeedFile)
	}

	apptHandler := appointment.NewHandler(apptSvc)

	// Honor GIN_MODE; default to release so prod logs aren't littered with
	// debug output. Override locally with `GIN_MODE=debug`.
	if cfg.GinMode == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Per-IP rate limiting. Lenient defaults for now (100 rps, burst 200) —
	// tighten once we have real usage data.
	rl := httputil.NewRateLimiter(httputil.DefaultRateLimit())
	defer rl.Close()
	r.Use(rl.Middleware())

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Swagger UI: /swagger/index.html (raw spec at /swagger/doc.json).
	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler)
	r.GET("/swagger/*any", func(c *gin.Context) {
		if p := c.Param("any"); p == "" || p == "/" {
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
			return
		}
		swaggerHandler(c)
	})

	apptHandler.Routes(r)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
