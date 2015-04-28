package main

import (
	"encoding/json"
	"log"
	"net/http"
	"testing"
	"time"
)

func MockApi(bind string) {
	http.HandleFunc("/api/v1/checks/new_relic.nudger", func(w http.ResponseWriter, r *http.Request) {
		checks := []Check{
			Check{NRAppId: 123, NRApiKey: "abc", ApiKey: "def"},
			Check{NRAppId: 123, NRApiKey: "ghi", ApiKey: "jkl"},
			Check{NRAppId: 123, NRApiKey: "mno", ApiKey: "qrs"},
		}
		b, _ := json.Marshal(checks)
		w.Write(b)
	})
	log.Fatal(http.ListenAndServe(bind, nil))
}

func TestPollChecks(t *testing.T) {
	go MockApi("127.0.0.1:42424")
	config := Config{
		Interval:     1 * time.Millisecond,
		MasterApiKey: "r4d4l3rt",
		Url:          "http://127.0.0.1:42424/api/v1/checks/new_relic.nudger",
		Timeout:      5 * time.Second,
	}
	var checks []Check
	go PollChecks(config, &checks)
	time.Sleep(10 * time.Millisecond)

	if len(checks) == 0 {
		t.Errorf("No checks, got %+v\n", config)
	}

	t.Logf("checks: %d\n", len(checks))
}
