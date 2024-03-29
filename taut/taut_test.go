package main

import (
	"bytes"
	"encoding/json"
	"github.com/nlopes/slack"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	serverAddr string
	once       sync.Once
)

func startServer() {
	server := httptest.NewServer(nil)
	serverAddr = server.Listener.Addr().String()
	log.Print("Test WebSocket server listening on ", serverAddr)
}

// MockSlack is a fake Slack incoming webhook endpoint, for testing posts to Slack
func MockSlack(t *testing.T, bind string, success chan bool) {
	http.HandleFunc("/services/ABC/123", func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		t.Logf("body: %s", string(body))
		values, _ := url.ParseQuery(string(body))
		if len(values.Get("payload")) > 0 {
			success <- true
			w.Write([]byte("ok"))
		} else {
			w.Write([]byte("not ok"))
		}
	})
	t.Fatal(http.ListenAndServe(bind, nil))
}

func TestAlertFromPacemaker(t *testing.T) {
	// Setup listen
	alerts := make(chan Alert, 10)
	config := Config{ListenBind: ":8081"}
	go Listen(config, alerts)

	// Build alert
	alert := Alert{
		State:        "CRITICAL",
		Org:          "MyCo",
		Check:        "shizzle.com/health",
		AnomalyStart: 1434972584,
		Tags:         []string{"shizzle", "health"},
	}
	body, err := json.Marshal(alert)
	if err != nil {
		t.Fatalf("Error encoding alert as JSON: %s\n", err)
	}

	// Make request
	url := "http://localhost" + config.ListenBind + "/integrations/pacemaker"
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
	t.Logf("Pacemaker POST response body: %s\n", body)

	if len(alerts) != 1 {
		t.Fatalf("Expected %d alert, got %d", 1, len(alerts))
	}
}

func getSearchMessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := []byte(`
		{"ok": true, "query": "hello", "messages": {
			"total":0,
			"paging": {},
			"matches": []
		}}`)
	w.Write(response)
}

func TestSlackSend(t *testing.T) {
	// Setup mock Slack Web API
	http.HandleFunc("/search.messages", getSearchMessagesHandler)
	once.Do(startServer)
	slack.SLACK_API = "http://" + serverAddr + "/"
	api := slack.New("testing-token")
	api.SetDebug(true)

	// Setup mock Slack webhook endpoint
	success := make(chan bool, 10)
	go MockSlack(t, ":3456", success)

	// Dispatch an alert
	config := Config{
		SlackWebhookEndpoint: "http://localhost:3456/services/ABC/123",
		SlackApi:             api,
	}
	alerts := make(chan Alert, 10)
	go SlackSender(config, alerts)
	alerts <- Alert{
		Org:          "MyCo",
		Check:        "shizzle.com/health",
		AnomalyStart: 1434972584,
		Tags:         []string{"shizzle", "health"},
	}

	// Test Slack webhook endpoint was called
	time.Sleep(10 * time.Millisecond)
	if len(success) != 1 {
		t.Fatalf("Expected Slack webhook to be called, was not\n")
	}
}

func TestSlackReceiveVote(t *testing.T) {
	// Setup
	alerts := make(chan Alert, 10)
	config := Config{ListenBind: ":8082"}
	go Listen(config, alerts)

	// http://requestb.in/1fl5kji1

	// Test
	text := slackPost(t, config, "radalert: 'spoons of doom' -1")
	expected := "You voted -1 on 'spoons of doom'"
	contains := strings.Contains(text, expected)
	if contains != true {
		t.Fatalf("Expected response to include:\n\n%s\n\nGot:\n\n%s", expected, text)
	}

	// TODO(auxesis): test passing votes back to pacemaker
}

func TestSlackReceiveHelp(t *testing.T) {
	// Setup
	alerts := make(chan Alert, 10)
	config := Config{ListenBind: ":8083"}
	go Listen(config, alerts)

	// http://requestb.in/1fl5kji1
	text := slackPost(t, config, "radalert: help")
	expected := "usage: radalert: <command> [<args>]"
	contains := strings.Contains(text, expected)
	if contains != true {
		t.Fatalf("Expected response to include:\n\n%s\n\nGot:\n\n%s", expected, text)
	}
}

// TODO(auxesis): test authentication token matches known token

func slackPost(t *testing.T, config Config, msg string) string {
	// Test
	values := url.Values{"text": {msg}}
	url := "http://localhost" + config.ListenBind + "/integrations/slack"
	resp, err := http.PostForm(url, values)
	if err != nil {
		t.Fatalf("HTTP POST should not have failed! Got: %s\n", err)
	}
	var ack map[string]string
	body, _ := ioutil.ReadAll(resp.Body)
	t.Logf("Slack POST response body: %s\n", body)

	err = json.Unmarshal(body, &ack)
	text := ack["text"]
	if len(text) == 0 {
		t.Fatalf("No 'text' field in response from Slack endpoint")
	}
	t.Logf("Text: %s\n", text)
	return text
}
