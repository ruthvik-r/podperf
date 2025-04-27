package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logrus "github.com/sirupsen/logrus"
)

var log = &logrus.Logger{
	Formatter: new(logrus.JSONFormatter),
	Hooks:     make(logrus.LevelHooks),
	Out:       os.Stdout,
	Level:     logrus.DebugLevel,
}

type Response struct {
	RequestID string `json:"requestID"`
	Status    string `json:"status"`
	Duration  string `json:"duration,omitempty"`
}

var numCPU = runtime.NumCPU()

// Prometheus metrics
var (
	sortRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "podperf_sort_requests_total",
		Help: "The total number of sort requests",
	})

	sortDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "podperf_sort_duration_seconds",
		Help:    "The duration of sort operations in seconds",
		Buckets: prometheus.LinearBuckets(1, 1, 10), // 1s to 10s in 1s increments
	})

	arraySize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "podperf_array_size",
		Help: "The size of array being sorted",
	})

	errorCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "podperf_errors_total",
		Help: "Total number of errors",
	})
)

func main() {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(numCPU)

	// Register metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/sort", sortHandler)

	log.Info("Starting on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func sortHandler(w http.ResponseWriter, r *http.Request) {
	requestID := fmt.Sprintf("%d", time.Now().UnixNano())
	logger := log.WithField("requestID", requestID)
	logger.Info("received request")

	// Record request in Prometheus
	sortRequests.Inc()

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		logger.WithField("status", http.StatusMethodNotAllowed).Warn("Method not allowed")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			RequestID: requestID,
			Status:    "error",
		})
		errorCounter.Inc()
		return
	}

	requestIDInt, _ := strconv.ParseInt(requestID, 10, 64)
	if requestIDInt%2 == 0 {
		w.Header().Set("Content-Type", "application/json")
		logger.WithField("status", http.StatusBadRequest).Warn("Request is even")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			RequestID: requestID,
			Status:    "error",
		})
		errorCounter.Inc()
		return
	}

	const size = 20000000
	// Update array size metric
	arraySize.Set(float64(size))

	numbers := generateRandomNumbers(size)

	logger.Debugf("Unsorted (first 10): %v", numbers[:10])

	startTime := time.Now()
	sortedNumbers := parallelMergeSort(numbers)
	duration := time.Since(startTime)

	// Record sort duration in Prometheus
	sortDuration.Observe(duration.Seconds())

	logger.Warnf("Sorted (first 10): %v", sortedNumbers[:10])
	logger.Infof("Finished sorting in %s", duration)

	w.Header().Set("Content-Type", "application/json")
	logger.WithField("status", http.StatusOK).Info("Request completed")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		RequestID: requestID,
		Status:    "success",
		Duration:  duration.String(),
	})
}

func generateRandomNumbers(size int) []int {
	numbers := make([]int, size)
	for i := 0; i < size; i++ {
		numbers[i] = rand.Intn(20000000)
	}
	return numbers
}

func parallelMergeSort(items []int) []int {
	n := len(items)
	if n <= 1 {
		return items
	}

	threshold := 100000
	if n <= threshold {
		return mergeSort(items)
	}

	middle := n / 2

	var wg sync.WaitGroup
	wg.Add(2)

	var left, right []int

	go func() {
		defer wg.Done()
		left = parallelMergeSort(items[:middle])
	}()

	go func() {
		defer wg.Done()
		right = parallelMergeSort(items[middle:])
	}()

	wg.Wait()
	return merge(left, right)
}

func mergeSort(items []int) []int {
	n := len(items)
	if n <= 1 {
		return items
	}

	middle := n / 2
	return merge(mergeSort(items[:middle]), mergeSort(items[middle:]))
}

func merge(left, right []int) []int {
	result := make([]int, 0, len(left)+len(right))
	i, j := 0, 0

	for i < len(left) && j < len(right) {
		if left[i] <= right[j] {
			result = append(result, left[i])
			i++
		} else {
			result = append(result, right[j])
			j++
		}
	}

	result = append(result, left[i:]...)
	result = append(result, right[j:]...)
	return result
}
