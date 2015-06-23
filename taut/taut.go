package main

import (
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/nlopes/slack"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	ListenBind           string
	Timeout              time.Duration
	SlackWebhookEndpoint string
	SlackApi             *slack.Slack
}

// Courtesy of https://github.com/paulhammond/slackcat/blob/master/slackcat.go
type SlackMsg struct {
	Channel     string             `json:"channel"`
	Username    string             `json:"username,omitempty"`
	Text        string             `json:"text"`
	Parse       string             `json:"parse"`
	IconEmoji   string             `json:"icon_emoji,omitempty"`
	Attachments []slack.Attachment `json:"attachments,omitempty"`
	UnfurlLinks bool               `json:"unfurl_links,omitempty"`
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
	State        string   `json:"state"`
	Org          string   `json:"org"`
	Check        string   `json:"check"`
	AnomalyStart int64    `json:"anomaly_start"`
	Tags         []string `json:"tags"`
}

func slackHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			trace := make([]byte, 1024)
			runtime.Stack(trace, false)
			log.Printf("[panic] Slack Handler: %s\n", e)
			log.Printf("[panic] Backtrace: %s\n", trace)
			log.Printf("[panic] Slack Handler: %s\n", r.Body)
		}
	}()

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
	text, err := handleCommand(command)
	if err != nil {
		log.Printf("[error] slackHandler: couldn't handle command: %s", err)
		// TODO(auxesis): write outback to slack back?
		return
	}
	ack := SlackMsg{Text: text}
	b, err := ack.Encode()
	if err != nil {
		return
	}
	w.Write([]byte(b))
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
		msg = `usage: radalert: <command> [<args>]

		:books: commands:

		spoons		of doom
		another		line of text
		`
	}
	re := regexp.MustCompile("\n\t*")
	msg = re.ReplaceAllLiteralString(msg, "\n")
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
	fmt.Println("pacemaker body:", string(body))
	var alert Alert
	err = json.Unmarshal(body, &alert)
	if err != nil {
		log.Printf("[error] pacemakerHandler: Couldn't decode json: %s\n", err)
		return
	}
	defer func() {
		if len(alert.State) == 0 {
			log.Printf("[error]: Not posting to Slack, there is no state: %+v\n", alert)
			return
		}
		if alert.State == "OK" {
			log.Println("[info]: Not posting to Slack, state is:", alert.State)
			return
		}
		ph.alerts <- alert
	}()
	w.Write([]byte("OK"))
}

// a copy of expvar.expvarHandler
func ExpvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

// Listen handles http serving for Pacemaker and Slack inputs.
func Listen(config Config, alerts chan Alert) {
	router := http.NewServeMux()
	router.HandleFunc("/integrations/slack", slackHandler)

	ph := &pacemakerHandler{alerts: alerts}
	router.Handle("/integrations/pacemaker", ph)
	router.Handle("/", ph)

	router.HandleFunc("/debug/vars", ExpvarHandler)
	router.HandleFunc("/integrations/slack/ping", ExpvarHandler)

	log.Fatal(http.ListenAndServe(config.ListenBind, router))
}

// slackAlertHistory searches Slack history for previous occurances of this the alert
func slackAlertHistory(config Config, alert Alert) slack.Attachment {
	// FIXME(auxesis): should wrap this in a function that times out after 10 seconds
	query := `"` + alert.Check + `"`
	messages, _ := config.SlackApi.SearchMessages(query, slack.SearchParameters{})
	var text string
	if len(messages.Matches) > 0 {
		items := []string{}
		for i, match := range messages.Matches {
			if i > 2 {
				break
			}
			var item string
			item += "*<" + match.Permalink + "|"
			item += "2 hours ago> "
			item += "_" + match.Username + "_"
			item += " in #" + match.Channel.Name + ":* "
			item += match.Text[3:len(match.Text)] // somehow emoji are being inserted here. trim them
			items = append(items, item)
		}
		text = strings.Join(items, "\n")
	} else {
		text = "No history found for this alert."
	}

	attachment := slack.Attachment{
		Color:      "#373736",
		Title:      "History: " + alert.Check,
		Text:       text,
		MarkdownIn: []string{"text"},
	}

	return attachment
}

func slackAnomalyAlert(config Config, alert Alert) slack.Attachment {
	t := time.Unix(alert.AnomalyStart, 0)

	attachment := slack.Attachment{
		Fallback: "Anomaly detected: " + alert.Check,
		Color:    "#f9006c",
		Title:    "Anomaly detected: " + alert.Check,
		// TODO(auxesis): include graph of anomaly
		// ImageURL: "",
		Fields: []slack.AttachmentField{
			slack.AttachmentField{
				Title: "Started :running:",
				Value: humanize.Time(t),
				Short: true,
			},
			slack.AttachmentField{
				Title: "Last alerted :repeat:",
				// FIXME(auxesis): actually look up when the alert last fired
				Value: "3 days ago",
				Short: true,
			},
		},
	}

	return attachment
}

func SlackSender(config Config, alerts chan Alert) {
	for {
		alert := <-alerts
		log.Printf("%+v\n", alert)

		attachments := []slack.Attachment{}
		anomalyAttachment := slackAnomalyAlert(config, alert)
		attachments = append(attachments, anomalyAttachment)
		historyAttachment := slackAlertHistory(config, alert)
		attachments = append(attachments, historyAttachment)

		msg := SlackMsg{
			Username:    "Rad Alert",
			Attachments: attachments,
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
		SlackApi:             slack.New("xoxp-3032859045-3032859047-6683267184-9e01e6"),
	}
	go SlackSender(config, alerts)
	Listen(config, alerts)
}
