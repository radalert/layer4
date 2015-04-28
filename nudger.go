package main

import (
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
	Url          string
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
		log.Printf("[error] new request: %s\n", err)
		return
	}
	req.Header.Set("X-Api-Key", check.NRApiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[error] client do: %s\n", err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[error] couldn't read body: %s\n", err)
		return
	}

	var app ApplicationResponse
	err = json.Unmarshal(body, &app)
	if err != nil {
		log.Printf("[error] couldn't decode json: %s", err)
		return
	}

	m := Metric{Tags: check.Tags, ApiKey: check.ApiKey}
	m.Check = appid + " response_time"
	m.Metric = app.Application.ApplicationSummary.ResponseTime
	metrics <- m
	m.Check = appid + " throughput"
	m.Metric = app.Application.ApplicationSummary.Throughput
	metrics <- m
	m.Check = appid + " error_rate"
	m.Metric = app.Application.ApplicationSummary.ErrorRate
	metrics <- m
}

func PollChecks(config Config, checks *[]Check) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[error] unhandled panic when polling for checks:", r)
		}
	}()

	tick := time.NewTicker(config.Interval).C
	for {
		select {
		case <-tick:
			log.Println("[info] tick: checks")
			client := &http.Client{Timeout: config.Timeout}
			req, err := http.NewRequest("GET", config.Url, nil)
			if err != nil {
				log.Printf("[error] new request: %s\n", err)
				continue
			}
			req.SetBasicAuth(config.MasterApiKey, "")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[error] client do: %s\n", err)
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf("[error] couldn't read body: %s\n", err)
				continue
			}
			err = json.Unmarshal(body, &checks)
			if err != nil {
				log.Printf("[error] couldn't decode checks: %s\n", err)
				log.Printf("[error] response body: %s\n", string(body))
				continue
			}
		}
	}
}

func dispatch(metrics chan Metric) {
	for {
		metric := <-metrics
		log.Printf("[debug] dispatch: %+v", metric)
	}
}

var (
	apikey = kingpin.Flag("apikey", "Master API key for authenticating to console").Default("r4d4l3rt").OverrideDefaultFromEnvar("APIKEY").String()
	url    = kingpin.Flag("endpoint", "URL endpoint to fetch checks").Default("https://radalert.io/api/v1/checks/new_relic.nudger").OverrideDefaultFromEnvar("ENDPOINT").String()
)

func main() {
	kingpin.Version("1.0.0")
	kingpin.Parse()

	fmt.Println("nudgers gonna nudge nudge nudge nudge")

	config := Config{
		Interval:     time.Second * 30,
		MasterApiKey: *apikey,
		Url:          *url,
		Timeout:      time.Second * 5,
	}
	log.Printf("[debug] config: %+v\n", config)

	var checks []Check
	go PollChecks(config, &checks)

	metrics := make(chan Metric)
	go dispatch(metrics)

	tick := time.NewTicker(time.Second * 30).C
	for {
		select {
		case <-tick:
			log.Println("[info] tick: new relic")
			log.Println("[info] number of checks:", len(checks))
			for _, c := range checks {
				go PollNR(c, metrics)
			}
		}
	}
}
