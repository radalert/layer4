package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

type Config struct {
	ListenBind           string
	Timeout              time.Duration
	SlackWebhookEndpoint string
}

type SlackPayload struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

type Alert struct {
	Org             string   `json:"org"`
	Check           string   `json:"check"`
	AnomalyDuration int      `json:"anomaly_duration"`
	Tags            []string `json:"tags"`
}

func slackHandler(w http.ResponseWriter, r *http.Request) {
}

type pacemakerHandler struct {
	alerts chan Alert
}

func (ph *pacemakerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("[error] pacemakerHandler: %s\n", err)
		return
	}
	var alert Alert
	err = json.Unmarshal(body, &alert)
	if err != nil {
		log.Printf("[error] pacemakerHandler: Couldn't decode json: %s\n", err)
		return
	}
	defer func() {
		ph.alerts <- alert
	}()
	w.Write([]byte("OK"))
}

func Listen(config Config, alerts chan Alert) {
	router := http.NewServeMux()
	router.HandleFunc("/slack", slackHandler)

	ph := &pacemakerHandler{alerts: alerts}
	router.Handle("/pacemaker", ph)

	log.Fatal(http.ListenAndServe(config.ListenBind, router))
}

func SlackSender(config Config, alerts chan Alert) {
	for {
		alert := <-alerts
		log.Printf("%+v\n", alert)
		payload := SlackPayload{Username: "Rad Alert", Text: "Anomaly Detected"}
		value, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[error] Slack: JSON marshal: %s\n", err)
			continue
		}

		client := &http.Client{Timeout: config.Timeout}
		urlStr := config.SlackWebhookEndpoint
		form := url.Values{}
		form.Add("payload", string(value))
		req, err := http.NewRequest("POST", urlStr, bytes.NewReader([]byte(form.Encode())))
		if err != nil {
			log.Printf("[error] Slack: new request: %s\n", err)
			continue
		}
		_, err = client.Do(req)
		if err != nil {
			log.Printf("[error] Slack: client do: %s\n", err)
			continue
		}
	}
}

func main() {
	kingpin.Version("1.0.0")
	kingpin.Parse()

	fmt.Println("tauters gonna taut taut taut taut")

	alerts := make(chan Alert, 100000)
	config := Config{
		ListenBind:           ":8080",
		SlackWebhookEndpoint: "https://hooks.slack.com/services/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",
	}
	go SlackSender(config, alerts)
	Listen(config, alerts)
}
