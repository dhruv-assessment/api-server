package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dhruv-assessment/api-server/handler"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.GET("/", func(c echo.Context) error {
		log.Println("Received GET request on /")
		data, err := json.MarshalIndent(e.Routes(), "", "  ")
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, string(data))
	})
	e.GET("/health", handler.HealthHandler)
	e.POST("/facerecognition", handler.FaceRecognition)
	e.POST("/temperature", handler.PostTemperature)
	e.POST("/log", handler.LogError)
	e.Use(echoprometheus.NewMiddleware("myapp"))   // adds middleware to gather metrics
	e.GET("/metrics", echoprometheus.NewHandler()) // adds route to serve gathered metrics

	// Start a background goroutine to handle incoming SQS messages
	go handler.WaitForSQSResponseMessage()

	// Start the Echo server in a separate goroutine
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
