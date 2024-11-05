package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

func HelloWorldHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func FaceRecognition(c echo.Context) error {
	inputFile, err := c.FormFile("inputFile")
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("unable to get file: %v", err))
	}

	src, err := inputFile.Open()
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("unable to open file: %v", err))
	}
	defer src.Close()

	dst, err := os.Create(inputFile.Filename)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("unable to create file: %v", err))
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("unable to copy file: %v", err))
	}

	return c.String(http.StatusOK, "File created")
}
