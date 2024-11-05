package main

import (
	"github.com/dhruv-assessment/api-server/handler"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/", handler.HelloWorldHandler)
	e.Logger.Fatal(e.Start(":1323"))
}
