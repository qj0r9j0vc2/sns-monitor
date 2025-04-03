package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func SendAlert(subject, msg string) error {
	slackURL := os.Getenv("SLACK_WEBHOOK_URL")
	if slackURL != "" {
		err := PostJSON(slackURL, map[string]string{"text": msg})
		if err != nil {
			log.Printf("Slack alert failed: %v", err)
		}
	}

	pdKey := os.Getenv("PAGERDUTY_ROUTING_KEY")
	if pdKey != "" {
		payload := map[string]interface{}{
			"routing_key":  pdKey,
			"event_action": "trigger",
			"payload": map[string]interface{}{
				"summary":   subject,
				"source":    "sns-monitor",
				"severity":  "error",
				"timestamp": time.Now().Format(time.RFC3339),
			},
		}
		err := PostJSON("https://events.pagerduty.com/v2/enqueue", payload)
		if err != nil {
			log.Printf("PagerDuty alert failed: %v", err)
		}
	}
	return nil
}

func PostJSON(url string, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP request returned status: %d", resp.StatusCode)
	}
	return nil
}

func GetEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if val, err := strconv.Atoi(v); err == nil {
		return val
	}
	return defaultVal
}
