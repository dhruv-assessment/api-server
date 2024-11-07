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

func HelloWorldHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

type mapValueSQS struct {
	prediction    string
	receiptHandle string
}

var (
	mapSQSMessages = make(map[string]mapValueSQS)
	mapMutex       sync.RWMutex
)

func WaitForSQSResponseMessageTest() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return
	}
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

	if _, err := service.UploadToReqS3(inputFile.Filename, src); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Unable to upload image to S3: %v", err))
	}
	responseMessageID, err := service.SendMessageToSQS(inputFile.Filename)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Unable to send message to request queue: %v", err))
	}

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
					return c.String(http.StatusBadRequest, fmt.Sprintf("unable to load aws config: %v", err))
				}

				client := sqs.NewFromConfig(cfg)
				if _, err = client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(os.Getenv("AWS_RESP_URL")),
					ReceiptHandle: aws.String(value.receiptHandle),
				}); err != nil {
					return c.String(http.StatusBadRequest, fmt.Sprintf("unable to create sqs config: %v", err))
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
		time.Sleep(time.Millisecond * 500)
	}

	filenameWithoutExt := strings.TrimSuffix(inputFile.Filename, filepath.Ext(inputFile.Filename))
	if err = service.UploadToRespS3(filenameWithoutExt, prediction); err != nil {
		log.Fatalf("Unable to upload the prediction to resp S3: %v", err)
	}

	responseData := fmt.Sprintf("%s:%s", filenameWithoutExt, strings.TrimSpace(prediction))
	return c.String(http.StatusOK, string(responseData))
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
