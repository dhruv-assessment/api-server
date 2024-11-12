package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/dhruv-assessment/api-server/database"
	"github.com/dhruv-assessment/api-server/service"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/labstack/echo/v4"
)

// HealthHandler handles requests to the health check endpoint
func HealthHandler(c echo.Context) error {
	log.Println("Received GET request on /health")
	return c.String(http.StatusOK, "Up and running!")
}

type mapValueSQS struct {
	prediction    string
	receiptHandle string
}

var (
	mapSQSMessages = make(map[string]mapValueSQS)
	mapMutex       sync.RWMutex
)

func WaitForSQSResponseMessage() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return
	}
	// Create SQS client from config
	client := sqs.NewFromConfig(cfg)
	for {
		result, err := client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(os.Getenv("AWS_RESP_URL")),
			VisibilityTimeout:     360,
			MessageAttributeNames: []string{"Request-Queue-Message-ID"},
			WaitTimeSeconds:       0,
			MaxNumberOfMessages:   10,
		})
		if err != nil {
			return
		}

		mapMutex.Lock()
		// Store received messages in mapSQSMessages
		for _, message := range result.Messages {
			mapSQSMessages[*message.MessageAttributes["Request-Queue-Message-ID"].StringValue] = mapValueSQS{
				prediction:    *message.Body,
				receiptHandle: *message.ReceiptHandle,
			}
		}
		mapMutex.Unlock()

		time.Sleep(time.Millisecond * 500)
	}
}

// FaceRecognition handles the face recognition process using S3 and SQS
func FaceRecognition(c echo.Context) error {
	log.Println("Received POST request on /facerecognition")
	inputFile, err := c.FormFile("inputFile")
	if err != nil || inputFile == nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("unable to get file: %v", err))
	}

	src, err := inputFile.Open()
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("unable to open file: %v", err))
	}
	defer src.Close()

	// FaceRecognition handles the face recognition process using S3 and SQS
	if _, err := service.UploadToReqS3(inputFile.Filename, src); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Unable to upload image to S3: %v", err))
	}

	// Send a message to the request SQS queue with the file name
	responseMessageID, err := service.SendMessageToSQS(inputFile.Filename)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Unable to send message to request queue: %v", err))
	}

	// Poll mapSQSMessages until the prediction result is available
	var prediction string
	for {
		flag := 0
		var tempKey string
		mapMutex.RLock()
		for key, value := range mapSQSMessages {
			if key == responseMessageID {
				prediction = value.prediction
				flag = 1

				cfg, err := config.LoadDefaultConfig(context.TODO())
				if err != nil {
					return c.String(http.StatusInternalServerError, fmt.Sprintf("unable to load aws config: %v", err))
				}

				client := sqs.NewFromConfig(cfg)
				if _, err = client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(os.Getenv("AWS_RESP_URL")),
					ReceiptHandle: aws.String(value.receiptHandle),
				}); err != nil {
					return c.String(http.StatusInternalServerError, fmt.Sprintf("unable to delete sqs message: %v", err))
				}

				tempKey = key
				break
			}
		}
		mapMutex.RUnlock()
		if flag == 1 {
			mapMutex.Lock()
			delete(mapSQSMessages, tempKey)
			mapMutex.Unlock()
			break
		}
		// time.Sleep(time.Millisecond * 500)
	}

	filenameWithoutExt := strings.TrimSuffix(inputFile.Filename, filepath.Ext(inputFile.Filename))
	if err = service.UploadToRespS3(filenameWithoutExt, prediction); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Unable to upload the prediction to resp S3: %v", err))
	}

	// Return the response as filename:prediction
	responseData := fmt.Sprintf("%s:%s", filenameWithoutExt, strings.TrimSpace(prediction))
	return c.String(http.StatusOK, string(responseData))
}

type TemperatureData struct {
	Measurement string                 `json:"measurement"`
	Tags        map[string]string      `json:"tags"`
	Fields      map[string]interface{} `json:"fields"`
}

// PostTemperature handles temperature data submission to InfluxDB
func PostTemperature(c echo.Context) error {
	log.Println("Received POST request on /temperature")
	database.NewWriteClient()
	temperature := new(TemperatureData)
	if err := c.Bind(temperature); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("error in binding json: %v", err))
	}

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

	// Write point
	if err := database.WriteClient.WritePoint(context.Background(), p); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("error in writing data: %v", err))
	}
	// Ensures background processes finishes
	database.Instance.Close()
	return c.String(http.StatusOK, "Successfull")
}

type Log struct {
	Service   string `json:"service"`
	Endpoint  string `json:"endpoint"`
	Error     string `json:"error"`
	TraceBack string `json:"traceback"`
}

func LogError(c echo.Context) error {
	log.Println("Received POST request on /log")
	database.NewWriteClient()
	newLog := new(Log)
	if err := c.Bind(newLog); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("error in binding json: %v", err))
	}

	if newLog.Endpoint == "" || newLog.Service == "" {
		return c.String(http.StatusBadRequest, "missing required fields: endpoint and service")
	}

	p := influxdb2.NewPoint("logs",
		map[string]string{
			"service":  newLog.Service,
			"endpoint": newLog.Endpoint,
		},
		map[string]interface{}{
			"error_msg": newLog.Error,
			"traceback": newLog.TraceBack,
		},
		time.Now())

	// Write point
	if err := database.WriteClient.WritePoint(context.Background(), p); err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("error in writing data: %v", err))
	}
	// Ensures background processes finishes
	database.Instance.Close()
	return c.String(http.StatusOK, "Successfull")
}
