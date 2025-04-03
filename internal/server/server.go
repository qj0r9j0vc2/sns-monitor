package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"sns-monitor/internal/common"
)

var (
	snsClient        *sns.Client
	pendingMessages  = make(map[int64]time.Time)
	pendingMessagesM sync.Mutex
)

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

	cfg.Region = os.Getenv("AWS_REGION")

	// STS identity check
	identityClient := sts.NewFromConfig(cfg)
	identity, err := identityClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Printf("⚠️ Failed to get caller identity: %v", err)
	} else {
		log.Printf("🔐 Running as AWS Account: %s, UserId: %s, ARN: %s", aws.ToString(identity.Account), aws.ToString(identity.UserId), aws.ToString(identity.Arn))
	}

	snsClient = sns.NewFromConfig(cfg)

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

	healthTimeout := common.GetEnvInt("HEALTHCHECK_TIMEOUT", 20) // seconds
	go monitorPendingMessages(healthTimeout)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			if err = publishTimestamp(ctx); err != nil {
				log.Printf("Failed to publish: %v", err)
				subject := "Error publishing timestamp"
				_ = common.SendAlert(subject, subject+": "+err.Error())
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

	ts := time.Now().UnixMilli()
	msg := common.SNSMessage{Timestamp: ts}
	body, _ := json.Marshal(msg)

	_, err := snsClient.Publish(ctx, &sns.PublishInput{
		Message:  aws.String(string(body)),
		TopicArn: aws.String(topicArn),
	})
	if err == nil {
		log.Printf("Published timestamp: %s", time.UnixMilli(ts).String())
		pendingMessagesM.Lock()
		pendingMessages[ts] = time.Now()
		pendingMessagesM.Unlock()
	}
	return err
}

func monitorPendingMessages(timeoutSec int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-time.Duration(timeoutSec) * time.Second)
		var expired []int64

		pendingMessagesM.Lock()
		for ts, publishedAt := range pendingMessages {
			if publishedAt.Before(cutoff) {
				expired = append(expired, ts)
			}
		}
		for _, ts := range expired {
			log.Printf("🚨 No callback received for timestamp %s within %d seconds", time.UnixMilli(ts).String(), timeoutSec)
			subject := "🚨 No callback received"
			_ = common.SendAlert(subject, fmt.Sprintf("%s within %d seconds for timestamp %s", subject, timeoutSec, time.UnixMilli(ts).String()))
			delete(pendingMessages, ts)
		}
		pendingMessagesM.Unlock()
	}
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload common.CallbackPayload
	if err = json.Unmarshal(body, &payload); err != nil {
		log.Printf("Invalid callback JSON: %v", err)
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	log.Printf("📥 Received callback: published=%s, received=%s, latency=%.2fs", time.UnixMilli(payload.Timestamp).String(), time.UnixMilli(payload.Received).String(), payload.LatencySeconds)

	pendingMessagesM.Lock()
	delete(pendingMessages, payload.Timestamp)
	pendingMessagesM.Unlock()

	threshold := common.GetEnvInt("LATENCY_THRESHOLD_SECONDS", 10)
	if int(payload.LatencySeconds) > threshold {
		log.Printf("🚨 Latency %.2fs exceeds threshold %ds", payload.LatencySeconds, threshold)
		subject := "🚨 High latency detected"
		err = common.SendAlert(subject, fmt.Sprintf("%s: %.2f sec", subject, payload.LatencySeconds))
		if err != nil {
			log.Printf("❌ Alert sending failed: %v", err)
		}
	} else {
		log.Println("✅ Latency within acceptable range")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"callback processed"}`))
}
