package database

import (
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	api "github.com/influxdata/influxdb-client-go/v2/api"
)

var (
	Instance    influxdb2.Client
	WriteClient api.WriteAPIBlocking
)

func NewDatabase() {
	token := os.Getenv("INFLUXDB_TOKEN")
	// Store the URL of your InfluxDB instance
	url := os.Getenv("INFLUXDB_URL")
	// Create new client with default option for server url authenticate by token
	Instance = influxdb2.NewClient(url, token)
}

func NewWriteClient() {
	if Instance == nil {
		NewDatabase()
	}
	bucket := os.Getenv("BUCKET_NAME")
	org := os.Getenv("ORG_NAME")
	WriteClient = Instance.WriteAPIBlocking(org, bucket)
}
