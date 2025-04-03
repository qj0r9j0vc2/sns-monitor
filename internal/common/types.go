package common

var Mode string

type SNSMessage struct {
	Timestamp int64 `json:"timestamp"`
}

type CallbackPayload struct {
	Timestamp      int64   `json:"timestamp"`
	Received       int64   `json:"received"`
	LatencySeconds float64 `json:"latency_seconds"`
}
