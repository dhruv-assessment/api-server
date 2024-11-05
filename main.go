package main

import (
	"github.com/dhruv-assessment/api-server/handler"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/helloworld", handler.HelloWorldHandler)
	e.POST("/facerecognition", handler.FaceRecognition)
	e.Logger.Fatal(e.Start(":1323"))
}
