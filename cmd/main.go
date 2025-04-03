package main

import (
	"log"
	"os"
	"sns-monitor/internal/common"
	"sns-monitor/internal/lambda"

	"sns-monitor/internal/server"
)

func main() {
	common.Mode = os.Getenv("MODE")
	switch common.Mode {
	case "server":
		server.Run()
	case "lambda":
		lambda.Run()
	default:
		log.Fatalf("Unknown or unset MODE: %s. Use 'lambda' or 'server'", common.Mode)
	}
}
