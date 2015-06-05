package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	ListenBind           string
	Timeout              time.Duration
	SlackWebhookEndpoint string
}

// Courtesy of https://github.com/paulhammond/slackcat/blob/master/slackcat.go
type SlackMsg struct {
	Channel   string `json:"channel"`
	Username  string `json:"username,omitempty"`
	Text      string `json:"text"`
	Parse     string `json:"parse"`
	IconEmoji string `json:"icon_emoji,omitempty"`
}

func (m SlackMsg) Encode() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (m SlackMsg) Post(WebhookURL string) error {
	encoded, err := m.Encode()
	if err != nil {
		return err
	}

	resp, err := http.PostForm(WebhookURL, url.Values{"payload": {encoded}})
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New("Not OK")
	}
	return nil
}

type Alert struct {
	Org          string   `json:"org"`
	Check        string   `json:"check"`
	AnomalyStart int64    `json:"anomaly_start"`
	Tags         []string `json:"tags"`
}

func slackHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("[error] slackHandler: reading body: %s\n", err)
		// TODO(auxesis): write outback to slack back?
		return
	}

	log.Println("body", string(body))
	values, err := url.ParseQuery(string(body))
	if err != nil {
		log.Printf("[error] slackHandler: parsing body: %s\n", err)
		// TODO(auxesis): write outback to slack back?
		return
	}
	msg := values["text"][0]
	parts := strings.SplitAfterN(msg, " ", 2)
	if len(parts) != 2 {
		log.Printf("[error] slackHandler: expected 2 parts, got %d\n", len(parts))
		// TODO(auxesis): write outback to slack back?
		return
	}
	command := parts[1]
	resp, err := handleCommand(command)
	if err != nil {
		log.Printf("[error] slackHandler: couldn't handle command: %s", err)
		// TODO(auxesis): write outback to slack back?
		return
	}
	w.Write([]byte(resp))
}

func handleCommand(command string) (msg string, err error) {
	var voteCmd = regexp.MustCompile(`^'(?P<check>[^']+)'\s+(?P<vote>[\+|\-]1)`)
	switch {
	case voteCmd.MatchString(command):
		match := voteCmd.FindStringSubmatch(command)
		result := make(map[string]string)
		for i, name := range voteCmd.SubexpNames() {
			result[name] = match[i]
		}

		msg = "You voted " + result["vote"] + " on '" + result["check"] + "'"
	default:
		msg = "help me"
	}
	return msg, err
}

/*
pacemakerHandler is a HTTP handler for incoming requests from Pacemaker

We define a handler so we can attach a channel of alerts, which we push
requests from Pacemaker onto, for publishing to Slack.
*/
type pacemakerHandler struct {
	alerts chan Alert
}

// ServeHTTP handles requests from the Pacemaker
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

// Listen handles http serving for Pacemaker and Slack inputs.
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
		msg := SlackMsg{
			Username: "Rad Alert",
			Text:     "Anomaly detected for *'" + alert.Check + "'*",
		}
		err := msg.Post(config.SlackWebhookEndpoint)
		if err != nil {
			log.Printf("[error] Slack: msg post: %s\n", err)
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
