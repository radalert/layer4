package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"net/http"
)

type Config struct {
	Bind string
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

func sendToSlack(alert chan Alert) {
	log.Printf("%+v\n", alert)
}

func Listen(config Config, alerts chan Alert) {
	router := http.NewServeMux()
	router.HandleFunc("/slack", slackHandler)

	ph := &pacemakerHandler{alerts: alerts}
	router.Handle("/pacemaker", ph)

	log.Fatal(http.ListenAndServe(config.Bind, router))
}

func main() {
	kingpin.Version("1.0.0")
	kingpin.Parse()

	fmt.Println("tauters gonna taut taut taut taut")

	alerts := make(chan Alert, 100000)
	config := Config{Bind: ":8080"}
	Listen(config, alerts)
}
