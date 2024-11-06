package main

import (
	"github.com/dhruv-assessment/api-server/handler"
	_ "github.com/joho/godotenv/autoload"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/helloworld", handler.HelloWorldHandler)
	e.POST("/facerecognition", handler.FaceRecognition)
	e.POST("/temperature", handler.PostTemperature)
	e.Logger.Fatal(e.Start(":1323"))
}
