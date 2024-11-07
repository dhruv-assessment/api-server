package service

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func UploadToReqS3(fileName string, file io.Reader) (string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", err
	}

	client := s3.NewFromConfig(cfg)

	uploader := manager.NewUploader(client)
	if result, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("AWS_IN_BUCKET_NAME")),
		Key:    aws.String(fileName),
		Body:   file,
	}); err != nil {
		return "", err
	} else {
		return *result.Key, nil
	}
}

func UploadToRespS3(fileName string, prediction string) error {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg)
	r := strings.NewReader(prediction)

	uploader := manager.NewUploader(client)
	if _, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("AWS_OUT_BUCKET_NAME")),
		Key:    aws.String(fileName),
		Body:   r,
	}); err != nil {
		return err
	} else {
		return nil
	}
}

func SendMessageToSQS(fileName string) (string, error) {
	log.Println("Sending message to req queue")
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", nil
	}
	client := sqs.NewFromConfig(cfg)

	if sender, err := client.SendMessage(context.TODO(), &sqs.SendMessageInput{
		MessageBody:  aws.String(fileName),
		QueueUrl:     aws.String(os.Getenv("AWS_REQ_URL")),
		DelaySeconds: 0,
	}); err != nil {
		return "", err
	} else {
		return *sender.MessageId, nil
	}
}

func WaitForSQSResponseMessage(responseMessageID string) (string, string, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", "", err
	}
	client := sqs.NewFromConfig(cfg)
	for {
		result, err := client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(os.Getenv("AWS_RESP_URL")),
			VisibilityTimeout:     1,
			MessageAttributeNames: []string{"Request-Queue-Message-ID"},
			WaitTimeSeconds:       0,
			MaxNumberOfMessages:   10,
		})
		if err != nil {
			return "", "", err
		}
		for _, message := range result.Messages {
			if *message.MessageAttributes["Request-Queue-Message-ID"].StringValue == responseMessageID {
				_, err = client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
					QueueUrl:      aws.String(os.Getenv("AWS_RESP_URL")),
					ReceiptHandle: aws.String(*message.ReceiptHandle),
				})
				if err != nil {
					return "", "", err
				}
				return *message.Body, *message.ReceiptHandle, nil
			}
		}
		time.Sleep(time.Millisecond * 500)
	}
}
