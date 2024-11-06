package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/dhruv-assessment/api-server/database"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
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

	prediction, err := exec.Command("python3", os.Getenv("PATH_TO_MODEL"), inputFile.Filename).Output()
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Not able to run the model: %v", err))
	}

	os.Remove(inputFile.Filename)

	return c.String(http.StatusOK, string(prediction))
}

type TemperatureData struct {
	Measurement string                 `json:"measurement"`
	Tags        map[string]string      `json:"tags"`
	Fields      map[string]interface{} `json:"fields"`
}

func PostTemperature(c echo.Context) error {
	database.NewWriteClient()
	temperature := new(TemperatureData)
	if err := c.Bind(temperature); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("error in binding json: %v", err))
	}

	// Convert fields to the appropriate types (e.g., float64)
	for key, value := range temperature.Fields {
		if strValue, ok := value.(string); ok {
			// Attempt to convert string fields to float64
			if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
				temperature.Fields[key] = floatValue
			} else {
				return c.String(http.StatusBadRequest, fmt.Sprintf("error in parsing json: %v", err))
			}
		}
	}

	p := influxdb2.NewPoint(temperature.Measurement,
		temperature.Tags,
		temperature.Fields,
		time.Now())
	// Write point immediately
	if err := database.WriteClient.WritePoint(context.Background(), p); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("error in writing data: %v", err))
	}
	// Ensures background processes finishes
	database.Instance.Close()
	return c.String(http.StatusOK, "Successfull")
}
