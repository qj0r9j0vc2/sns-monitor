package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"sns-monitor/internal/common"
)

type Message struct {
	Timestamp int64 `json:"timestamp"`
}

var snsClient *sns.Client

func Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithCredentialsProvider(
		credentials.NewStaticCredentialsProvider(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		)))
	if err != nil {
		log.Fatalf("AWS config error: %v", err)
	}

	snsClient = sns.NewFromConfig(cfg)

	// HTTP server for receiving callbacks
	http.HandleFunc("/callback", callbackHandler)
	go func() {
		log.Println("Listening on :8080 for callbacks")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	interval := 30
	if v := os.Getenv("PUBLISH_INTERVAL_SECONDS"); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			interval = val
		}
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			if err := publishTimestamp(ctx); err != nil {
				log.Printf("Failed to publish: %v", err)
				_ = common.SendAlert("Error publishing timestamp: " + err.Error())
			}
		case <-stop:
			log.Println("Stopping client...")
			return
		}
	}
}

func publishTimestamp(ctx context.Context) error {
	topicArn := os.Getenv("SNS_TOPIC_ARN")
	if topicArn == "" {
		return fmt.Errorf("SNS_TOPIC_ARN not set")
	}

	msg := Message{Timestamp: time.Now().UnixMilli()}
	body, _ := json.Marshal(msg)

	_, err := snsClient.Publish(ctx, &sns.PublishInput{
		Message:  aws.String(string(body)),
		TopicArn: aws.String(topicArn),
	})
	if err == nil {
		log.Printf("Published timestamp: %d", msg.Timestamp)
	}
	return err
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	latency := time.Since(time.UnixMilli(msg.Timestamp))
	log.Printf("Received callback latency: %s", latency)

	threshold := time.Duration(common.GetEnvInt("LATENCY_THRESHOLD_SECONDS", 10)) * time.Second
	if latency > threshold {
		_ = common.SendAlert(fmt.Sprintf("High latency detected: %s", latency))
	}

	w.WriteHeader(http.StatusOK)
}
