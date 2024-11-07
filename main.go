package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dhruv-assessment/api-server/handler"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/health", handler.HealthHandler)
	e.POST("/facerecognition", handler.FaceRecognition)
	e.POST("/temperature", handler.PostTemperature)

	go handler.WaitForSQSResponseMessageTest()

	// e.Logger.Fatal(e.Start(":1323"))
	go func() {
		if err := e.Start(":1323"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Wait for a termination signal

	log.Println("Shutting down gracefully...")

	// Context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
