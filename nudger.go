package main

import (
	"log"
	"net/http"
	"fmt"
	"strings"
	"io/ioutil"
	"encoding/json"
	"time"
)

type ApplicationResponse struct {
	Application	Application
}

type Application struct {
	Id					int		`json:"id"`
	Name				string	`json:"name"`
	Reporting			bool	`json:"reporting"`
	ApplicationSummary	ApplicationSummary	`json:"application_summary"`
}

type ApplicationSummary struct {
	ResponseTime	float64 `json:"response_time"`
	Throughput		float64	`json:"throughput"`
	ErrorRate		float64	`json:"error_rate"`
	ApdexTarget		float64	`json:"apdex_target"`
	ApdexScore		float64	`json:"apdex_score"`
	HostCount		float64	`json:"host_count"`
	InstanceCount	float64	`json:"instance_count"`
}

type Sample struct {
	Appid 	string
	Name	string
	Value	float64
}

type Metric struct {
	ApiKey	string		`json:"api_key"`
	Check	string		`json:"check"`
	Metric	float64		`json:"metric"`
	TTL		int			`json:"ttl"`
	Tags	[]string	`json:"tags"`
}

func poll(appid string, apikey string, samples chan Sample) {
	parts := []string{"https://api.newrelic.com/v2/applications/", appid, ".json"}
	url := strings.Join(parts, "")

	client := &http.Client{Timeout: time.Second * 5}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("error: new request: %s\n", err)
		return
	}
	req.Header.Set("X-Api-Key", apikey)

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

	var sample Sample
	sample = Sample{Appid:appid,Name:"response_time",Value:app.Application.ApplicationSummary.ResponseTime,}
	samples <- sample
	sample = Sample{Appid:appid,Name:"throughput",Value:app.Application.ApplicationSummary.Throughput,}
	samples <- sample
	sample = Sample{Appid:appid,Name:"error_rate",Value:app.Application.ApplicationSummary.ErrorRate,}
	samples <- sample
}

func dispatch(samples chan Sample) {
	for {
		sample := <- samples
		log.Printf("dispatch: %+v", sample)
	}
}

func main() {
	fmt.Println("nudgers gonna nudge")

	samples := make(chan Sample)
	go dispatch(samples)

	tick := time.NewTicker(time.Second * 30).C
	for {
		select {
		case <- tick:
			log.Println("tick")
			appid := "6337276"
			apikey := "a42db8f0d605f19835ca9cc1c535adba9bfa003b3e75dd2"
			go poll(appid, apikey, samples)
		}
	}
}
