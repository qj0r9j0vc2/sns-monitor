package lambdaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"sns-monitor/internal/common"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func Run() {
	lambda.Start(lambdaHandler)
}

func lambdaHandler(ctx context.Context, event events.SNSEvent) (string, error) {
	for _, record := range event.Records {
		message := record.SNS.Message

		// Try to parse the SNS message as JSON
		var parsed common.SNSMessage
		err := json.Unmarshal([]byte(message), &parsed)
		if err == nil && parsed.Timestamp > 0 {
			processSNSTimestamp(parsed)
		} else {
			log.Println("EventBridge SNS message detected - checking server health")
			checkServerHealth()
		}
	}
	return "SNS message processed", nil
}

func processSNSTimestamp(parsed common.SNSMessage) {
	publishedTS := parsed.Timestamp
	currentTS := time.Now().UnixMilli()
	latency := float64(currentTS-publishedTS) / 1000.0

	log.Printf("Received SNS timestamp: %s, current time: %s, latency: %.2f seconds", time.UnixMilli(publishedTS), time.UnixMilli(currentTS), latency)

	if callbackURL := os.Getenv("SNS_CHECKER_CALLBACK_URL"); callbackURL != "" {
		payload := common.CallbackPayload{
			Timestamp:      publishedTS,
			Received:       currentTS,
			LatencySeconds: latency,
		}

		common.PostJSON(callbackURL, payload)
	} else {
		log.Println("SNS_CHECKER_CALLBACK_URL is not set")
	}

	threshold := common.GetEnvInt("LATENCY_THRESHOLD_SECONDS", 10)
	if int(latency) > threshold {
		sendAlert(fmt.Sprintf("High latency detected: %.2f seconds", latency))
	} else {
		log.Println("Latency within acceptable range")
	}
}

func checkServerHealth() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		log.Println("ADDR is not set")
		return
	}

	waitMinutes := common.GetEnvInt("WAIT_MINUTES", 5)
	totalWait := time.Duration(waitMinutes) * time.Minute
	checkInterval := 10 * time.Second
	start := time.Now()
	normalResponse := false

	for time.Since(start) < totalWait {
		resp, err := http.Get(addr)
		if err == nil && resp.StatusCode == 200 {
			log.Println("✅ Server responded successfully")
			normalResponse = true
			break
		}
		log.Printf("Server response failed or not 200. Retrying in %s...", checkInterval)
		time.Sleep(checkInterval)
	}

	if !normalResponse {
		sendAlert(fmt.Sprintf("🚨 Server unresponsive for %d minutes", waitMinutes))
	}
}

func sendAlert(msg string) {
	alarm := fmt.Sprintf("🚨 Mon_bharvest_monitor_kr alert: %s", msg)
	_ = common.SendAlert(alarm)
}
