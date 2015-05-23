package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestMessageFromPacemaker(t *testing.T) {
	// Setup listen
	alerts := make(chan Alert, 10)
	config := Config{Bind: ":8081"}
	go Listen(config, alerts)

	// Build alert
	alert := Alert{
		Org:             "MyCo",
		Check:           "shizzle.com/health",
		AnomalyDuration: 180,
		Tags:            []string{"shizzle", "health"},
	}
	body, err := json.Marshal(alert)
	if err != nil {
		t.Fatalf("Error encoding alert as JSON: %s\n", err)
	}

	// Make request
	url := "http://localhost" + config.Bind + "/pacemaker"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Could not create request: %s\n", err)
	}

	client := &http.Client{Timeout: time.Millisecond * 50}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP POST should not have failed! Got: %s\n", err)
	}

	body, _ = ioutil.ReadAll(resp.Body)
	t.Logf("Pacemaker POST response: %+v\n", resp)
	t.Logf("Pacemaker POST response body: %s\n", body)

	if len(alerts) != 1 {
		t.Fatalf("Expected %d alert, got %d", 1, len(alerts))
	}
}

func TestMessageFromSlack(t *testing.T) {
	// Setup
	alerts := make(chan Alert, 10)
	config := Config{Bind: ":8082"}
	go Listen(config, alerts)

	// http://requestb.in/1fl5kji1

	// Test
	values := url.Values{"text": {"text"}, "channel_id": {"C030YR91P"}}
	url := "http://localhost" + config.Bind + "/slack"
	resp, err := http.PostForm(url, values)
	if err != nil {
		t.Fatalf("HTTP POST should not have failed! Got: %s\n", err)
	}

	t.Logf("%+v\n", resp)
}
