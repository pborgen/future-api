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
	"github.com/pborgen/future-api/internal/config"
	appointmentdao "github.com/pborgen/future-api/internal/dao/appointment"
	"github.com/pborgen/future-api/internal/db"
	appointmenthttp "github.com/pborgen/future-api/internal/handler/appointment"
	appointmentsvc "github.com/pborgen/future-api/internal/service/appointment"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	apptDAO := appointmentdao.NewDAO(pool)
	if n, err := appointmentsvc.SeedFromFile(ctx, apptDAO, cfg.SeedFile); err != nil {
		log.Printf("warning: seed failed: %v", err)
	} else if n > 0 {
		log.Printf("seeded %d appointments from %s", n, cfg.SeedFile)
	}

	apptSvc := appointmentsvc.NewService(apptDAO)
	apptHandler := appointmenthttp.NewHandler(apptSvc)

	// Honor GIN_MODE; default to release so prod logs aren't littered with
	// debug output. Override locally with `GIN_MODE=debug`.
	if cfg.GinMode == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Swagger UI: /swagger/index.html (raw spec at /swagger/doc.json).
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
