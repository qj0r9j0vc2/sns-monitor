package main

import (
	"log"
	"os"
	"sns-monitor/internal/lambdaclient"

	"sns-monitor/internal/server"
)

func main() {
	mode := os.Getenv("MODE")
	switch mode {
	case "server":
		server.Run()
	case "lambda-client":
		lambdaclient.Run()
	default:
		log.Fatalf("Unknown or unset MODE: %s. Use 'client' or 'server'", mode)
	}
}
