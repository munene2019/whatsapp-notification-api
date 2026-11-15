package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsapp-notification-api/config"
	"whatsapp-notification-api/handlers"
	"whatsapp-notification-api/repositories"
	"whatsapp-notification-api/routes"
	"whatsapp-notification-api/services"
)

func main() {
	// Load configuration.
	cfg := config.LoadConfig()

	// Connect to MySQL.
	db, err := config.ConnectMySQL(cfg)
	if err != nil {
		log.Fatal("Database connection failed: ", err)
	}
	defer db.Close()

	log.Println("MySQL database connected successfully")

	// Create application context.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Initialize WhatsApp service.
	whatsappService := services.NewWhatsAppService(cfg)

	// Initialize scheduled message repository.
	scheduledRepository := repositories.NewScheduledRepository(db)

	// Initialize scheduler service.
	schedulerService := services.NewSchedulerService(
		scheduledRepository,
		whatsappService,
	)

	// Start scheduler in the background.
	go schedulerService.Start(ctx)

	// Initialize WhatsApp handler with configuration.
	whatsappHandler := handlers.NewWhatsAppHandler(
		whatsappService,
		scheduledRepository,
		cfg,
	)

	// Register API routes.
	router := routes.RegisterRoutes(whatsappHandler)

	// Configure HTTP server.
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start HTTP server.
	go func() {
		log.Printf(
			"WhatsApp Notification API running on port %s",
			cfg.Port,
		)

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Printf("HTTP server failed: %v", err)
			stop()
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()

	log.Println("Shutting down application...")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Application stopped")
}
