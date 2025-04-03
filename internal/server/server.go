package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sns-monitor/internal/common"
	"time"
)

type SNSEvent struct {
	Records []struct {
		Sns struct {
			Message string `json:"Message"`
		} `json:"Sns"`
	} `json:"Records"`
}

func Run() {
	http.HandleFunc("/sns", handleSNS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server mode listening on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleSNS(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var event SNSEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Invalid SNS event JSON: %v", err)
		http.Error(w, "Invalid event format", http.StatusBadRequest)
		return
	}

	for _, rec := range event.Records {
		if err := processMessage(rec.Sns.Message); err != nil {
			log.Printf("Error processing message: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"processed"}`))
}

func processMessage(raw string) error {
	// Try to parse as JSON with timestamp
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.Println("🧪 Treating as plain healthcheck ping")
		return checkServerHealth()
	}

	tsFloat, ok := parsed["timestamp"].(float64)
	if !ok {
		log.Println("No 'timestamp' key found. Skipping.")
		return nil
	}

	// Latency logic
	published := int64(tsFloat)
	now := time.Now().UnixMilli()
	latency := float64(now-published) / 1000.0
	log.Printf("🕒 SNS Timestamp: %d, Now: %d, Latency: %.2f sec", published, now, latency)

	// Optional callback to client
	callbackURL := os.Getenv("SNS_CHECKER_CALLBACK_URL")
	if callbackURL != "" {
		payload := map[string]interface{}{
			"timestamp":       published,
			"received":        now,
			"latency_seconds": latency,
		}
		err := common.PostJSON(callbackURL, payload)
		if err != nil {
			log.Printf("❌ Callback failed: %v", err)
		} else {
			log.Println("📡 Callback sent successfully")
		}
	}

	// Send alert if over threshold
	threshold := common.GetEnvInt("LATENCY_THRESHOLD_SECONDS", 10)
	if int(latency) > threshold {
		return common.SendAlert(fmt.Sprintf("🚨 High latency detected: %.2f sec", latency))
	}

	log.Println("✅ Latency within acceptable range")
	return nil
}

func checkServerHealth() error {
	addr := os.Getenv("ADDR")
	if addr == "" {
		log.Println("❌ ADDR not set")
		return fmt.Errorf("ADDR env var is not set")
	}
	wait := time.Duration(common.GetEnvInt("WAIT_MINUTES", 5)) * time.Minute
	deadline := time.Now().Add(wait)

	for time.Now().Before(deadline) {
		resp, err := http.Get(addr)
		if err == nil && resp.StatusCode == 200 {
			log.Println("✅ Server healthy")
			return nil
		}
		log.Printf("🚨 Server unhealthy or unreachable. Retrying...")
		time.Sleep(10 * time.Second)
	}

	return common.SendAlert(fmt.Sprintf("🚨 Server unresponsive for %v", wait))
}
