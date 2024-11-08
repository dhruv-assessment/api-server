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
	// Check if the main client instance is nil
	if Instance == nil {
		NewDatabase()
	}
	// Retrieve bucket and organization information
	bucket := os.Getenv("INFLUXDB_BUCKET_NAME")
	org := os.Getenv("INFLUXDB_ORG_NAME")
	// Create a new blocking write client
	WriteClient = Instance.WriteAPIBlocking(org, bucket)
}
