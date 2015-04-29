package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Interval     time.Duration
	MasterApiKey string
	Api          string
	Pacemaker    string
	Timeout      time.Duration
}

type ApplicationResponse struct {
	Application Application
}

type Application struct {
	Id                 int                `json:"id"`
	Name               string             `json:"name"`
	Reporting          bool               `json:"reporting"`
	ApplicationSummary ApplicationSummary `json:"application_summary"`
}

type ApplicationSummary struct {
	ResponseTime  float64 `json:"response_time"`
	Throughput    float64 `json:"throughput"`
	ErrorRate     float64 `json:"error_rate"`
	ApdexTarget   float64 `json:"apdex_target"`
	ApdexScore    float64 `json:"apdex_score"`
	HostCount     float64 `json:"host_count"`
	InstanceCount float64 `json:"instance_count"`
}

type Check struct {
	NRAppId  int      `json:"nr_app_id"`
	NRApiKey string   `json:"nr_api_key"`
	ApiKey   string   `json:"api_key"`
	Tags     []string `json:"tags"`
}

type Metric struct {
	ApiKey string   `json:"api_key"`
	Check  string   `json:"check"`
	Metric float64  `json:"metric"`
	TTL    int      `json:"ttl"`
	Tags   []string `json:"tags"`
}

func PollNR(check Check, metrics chan Metric) {
	appid := strconv.Itoa(check.NRAppId)
	parts := []string{"https://api.newrelic.com/v2/applications/", appid, ".json"}
	url := strings.Join(parts, "")

	client := &http.Client{Timeout: time.Second * 5}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[error] PollNR: new request: %s\n", err)
		return
	}
	req.Header.Set("X-Api-Key", check.NRApiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[error] PollNR: client do: %s\n", err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[error] PollNR: couldn't read body: %s\n", err)
		return
	}

	var app ApplicationResponse
	err = json.Unmarshal(body, &app)
	if err != nil {
		log.Printf("[error] PollNR: couldn't decode json: %s", err)
		return
	}

	m := Metric{Tags: check.Tags, ApiKey: check.ApiKey}
	m.Check = app.Application.Name + ": response time"
	m.Metric = app.Application.ApplicationSummary.ResponseTime
	metrics <- m
	m.Check = app.Application.Name + ": throughput"
	m.Metric = app.Application.ApplicationSummary.Throughput
	metrics <- m
	m.Check = app.Application.Name + ": error rate"
	m.Metric = app.Application.ApplicationSummary.ErrorRate
	metrics <- m
}

func PollChecks(config Config, checks *[]Check) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[error] PollChecks: unhandled panic when polling for checks:", r)
		}
	}()

	tick := time.NewTicker(config.Interval).C
	for {
		select {
		case <-tick:
			log.Println("[info] PollChecks: tick")
			client := &http.Client{Timeout: config.Timeout}
			req, err := http.NewRequest("GET", config.Api, nil)
			if err != nil {
				log.Printf("[error] PollChecks: new request: %s\n", err)
				continue
			}
			req.SetBasicAuth(config.MasterApiKey, "")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[error] PollChecks: client do: %s\n", err)
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[error] PollChecks: couldn't read body: %s\n", err)
				continue
			}
			err = json.Unmarshal(body, &checks)
			if err != nil {
				log.Printf("[error] PollChecks: couldn't decode checks: %s\n", err)
				log.Printf("[error] PollChecks: response body: %s\n", string(body))
				continue
			}
		}
	}
}

func Dispatch(config Config, metrics chan Metric) {
	url := config.Pacemaker
	for {
		metric := <-metrics
		log.Printf("[debug] Dispatch: %+v", metric)

		body, err := json.Marshal(metric)
		if err != nil {
			log.Printf("[error] Dispatch: JSON marshal: %s\n", err)
			continue
		}
		log.Printf("[debug] Dispatch: JSON marshal: %s", string(body))

		client := &http.Client{Timeout: time.Second * 5}
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			log.Printf("[error] Dispatch: new request: %s\n", err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[error] Dispatch: client do: %s\n", err)
			continue
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[error] Dispatch: couldn't read body: %s\n", err)
			continue
		}

		if resp.StatusCode != 200 {
			log.Printf("[error] Dispatch: Pacemaker returned HTTP %d: %s\n", resp.StatusCode, string(body))
		}
	}
}

var (
	apikey    = kingpin.Flag("apikey", "Master API key for authenticating to console").Default("r4d4l3rt").OverrideDefaultFromEnvar("APIKEY").String()
	api       = kingpin.Flag("endpoint", "API endpoint to fetch checks").Default("https://radalert.io/api/v1/checks/new_relic.nudger").OverrideDefaultFromEnvar("API").String()
	pacemaker = kingpin.Flag("pacemaker", "Pacemaker instance to submit heartbeats to").Default("http://130.211.158.50:7223").OverrideDefaultFromEnvar("PACEMAKER").String()
)

func main() {
	kingpin.Version("1.0.0")
	kingpin.Parse()

	fmt.Println("nudgers gonna nudge nudge nudge nudge")

	config := Config{
		Interval:     time.Second * 30,
		MasterApiKey: *apikey,
		Api:          *api,
		Pacemaker:    *pacemaker,
		Timeout:      time.Second * 5,
	}
	log.Printf("[debug] Main: config: %+v\n", config)

	var checks []Check
	go PollChecks(config, &checks)

	metrics := make(chan Metric)
	go Dispatch(config, metrics)

	tick := time.NewTicker(time.Second * 30).C
	for {
		select {
		case <-tick:
			log.Println("[info] Main: tick")
			log.Println("[info] Main: number of checks:", len(checks))
			for _, c := range checks {
				go PollNR(c, metrics)
			}
		}
	}
}
