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

type Config struct {
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

func poll(config Config, metrics chan Metric) {
	appid := strconv.Itoa(config.NRAppId)
	parts := []string{"https://api.newrelic.com/v2/applications/", appid, ".json"}
	url := strings.Join(parts, "")

	client := &http.Client{Timeout: time.Second * 5}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("error: new request: %s\n", err)
		return
	}
	req.Header.Set("X-Api-Key", config.NRApiKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error: client do: %s\n", err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error: Couldn't read body: %s\n", err)
		return
	}

	var app ApplicationResponse
	err = json.Unmarshal(body, &app)
	if err != nil {
		log.Printf("error: Couldn't decode json: %s", err)
		return
	}

	m := Metric{Tags: config.Tags, ApiKey: config.ApiKey}
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

func dispatch(metrics chan Metric) {
	for {
		metric := <-metrics
		log.Printf("dispatch: %+v", metric)
	}
}

var (
	configPath = kingpin.Arg("config", "Path to nudger config").Default("nudger.json").String()
)

func main() {
	kingpin.Version("1.0.0")
	kingpin.Parse()

	file, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("fatal: Couldn't read config %s: %s\n", *configPath, err)
	}

	var config []Config
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatalf("fatal: Couldn't decode config %s: %s\n", *configPath, err)
	}

	fmt.Println("nudgers gonna nudge nudge nudge nudge")

	metrics := make(chan Metric)
	go dispatch(metrics)

	tick := time.NewTicker(time.Second * 30).C
	for {
		select {
		case <-tick:
			log.Println("tick")
			for _, c := range config {
				go poll(c, metrics)
			}
		}
	}
}
