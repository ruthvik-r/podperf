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

func main() {
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(numCPU)

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

	if r.Method != http.MethodGet {
		logger.Warn("Method not allowed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			RequestID: requestID,
			Status:    "error",
		})
		return
	}

	requestIDInt, _ := strconv.ParseInt(requestID, 10, 64)
	if requestIDInt%2 == 0 {
		logger.Error("Request is even")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			RequestID: requestID,
			Status:    "error",
		})
		return
	}

	const size = 20000000
	numbers := generateRandomNumbers(size)

	logger.Debugf("Unsorted (first 10): %v", numbers[:10])

	startTime := time.Now()
	sortedNumbers := parallelMergeSort(numbers)
	duration := time.Since(startTime)

	logger.Warnf("Sorted (first 10): %v", sortedNumbers[:10])
	logger.Infof("Finished sorting in %s", duration)
	logger.Info("Request completed")

	w.Header().Set("Content-Type", "application/json")
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
