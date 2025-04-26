package main

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func main() {
	rate := vegeta.Rate{Freq: 2, Per: time.Second} // change frequency of API calls as required
	duration := 5 * time.Second                    // change duration of test as required
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8080/sort", // endpoint to test
	})
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Testing /sort!") {
		metrics.Add(res)
	}
	metrics.Close()

	fmt.Printf("99th percentile: %s\n", metrics.Latencies.P99)
	fmt.Printf("Total requests: %d\n", metrics.Requests)
	fmt.Printf("Status codes: %v\n", metrics.StatusCodes)
}
