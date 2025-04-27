package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var log = &logrus.Logger{
	Formatter: new(logrus.JSONFormatter),
	Hooks:     make(logrus.LevelHooks),
	Out:       os.Stdout,
	Level:     logrus.InfoLevel,
}

type Response struct {
	RequestID string `json:"requestID"`
	Status    string `json:"status"`
	Duration  string `json:"duration,omitempty"`
}

var numCPU = runtime.NumCPU()
var tracer trace.Tracer

// Custom Prometheus metrics
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

func init() {
	// Create log directory if it doesn't exist
	logDir := "/var/log/app"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logDir+"/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
	}

	// Configure logrus to write to both stdout and file
	mw := io.MultiWriter(os.Stdout, logFile)
	log = &logrus.Logger{
		Formatter: new(logrus.JSONFormatter),
		Hooks:     make(logrus.LevelHooks),
		Out:       mw,
		Level:     logrus.TraceLevel,
	}
}

// initTracer initializes an OTLP exporter
func initTracer() (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the collector endpoint from environment or default to localhost
	collectorURL := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if collectorURL == "" {
		collectorURL = "otel-collector:4317"
	}

	log.Infof("Using OTLP endpoint: %s", collectorURL)

	// Set up a connection to the collector
	conn, err := grpc.DialContext(ctx, collectorURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		log.Errorf("Failed to create gRPC connection to collector: %v", err)
		return nil, err
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		log.Errorf("Failed to create trace exporter: %v", err)
		return nil, err
	}

	// Register the trace exporter with a TracerProvider
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("podperf-zipkin-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		log.Errorf("Failed to create resource: %v", err)
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)
	tracer = otel.Tracer("podperf-app")

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Shutdown will flush any remaining spans
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Errorf("Failed to shutdown TracerProvider: %v", err)
		}
	}, nil
}

func main() {
	log.Info("Application starting up")
	rand.Seed(time.Now().UnixNano())
	runtime.GOMAXPROCS(numCPU)

	// Initialize tracer
	shutdown, err := initTracer()
	if err != nil {
		log.Warnf("Failed to initialize tracer: %v. Continuing without tracing.", err)
	} else {
		defer shutdown()
	}

	// Register endpoints
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/sort", sortHandler)
	http.HandleFunc("/health", healthHandler)

	log.Info("Starting on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Health check request received")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func sortHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var span trace.Span

	// Only create spans if tracer is initialized
	if tracer != nil {
		ctx, span = tracer.Start(ctx, "sort-handler")
		defer span.End()
	}

	requestID := fmt.Sprintf("%d", time.Now().UnixNano())
	logger := log.WithField("requestID", requestID)
	logger.Info("Received sort request")

	if span != nil {
		span.SetAttributes(attribute.String("request.id", requestID))
	}

	// Record request in Prometheus
	sortRequests.Inc()

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		logger.WithField("status", http.StatusMethodNotAllowed).Warn("Method not allowed")
		if span != nil {
			span.SetAttributes(attribute.Int("http.status_code", http.StatusMethodNotAllowed))
			span.SetAttributes(attribute.String("error", "method not allowed"))
		}
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
		if span != nil {
			span.SetAttributes(attribute.Int("http.status_code", http.StatusBadRequest))
			span.SetAttributes(attribute.String("error", "request id is even"))
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			RequestID: requestID,
			Status:    "error",
		})
		errorCounter.Inc()
		return
	}

	const size = 20000
	// Update array size metric
	arraySize.Set(float64(size))
	if span != nil {
		span.SetAttributes(attribute.Int("array.size", size))
	}

	// Create a child span for array generation
	var genSpan trace.Span
	if tracer != nil {
		_, genSpan = tracer.Start(ctx, "generate-random-numbers")
	}
	numbers := generateRandomNumbers(size)
	if genSpan != nil {
		genSpan.End()
	}

	logger.Infof("Unsorted (first 10): %v", numbers[:10])
	if span != nil {
		span.AddEvent("unsorted-array-ready", trace.WithAttributes(
			attribute.String("first_10_elements", fmt.Sprintf("%v", numbers[:10])),
		))
	}

	// Create a child span for sorting
	var sortSpan trace.Span
	if tracer != nil {
		ctx, sortSpan = tracer.Start(ctx, "parallel-merge-sort")
	}
	startTime := time.Now()
	sortedNumbers := parallelMergeSort(numbers)
	duration := time.Since(startTime)
	if sortSpan != nil {
		sortSpan.SetAttributes(attribute.Float64("duration_seconds", duration.Seconds()))
		sortSpan.End()
	}

	// Record sort duration in Prometheus
	sortDuration.Observe(duration.Seconds())

	logger.Infof("Sorted (first 10): %v", sortedNumbers[:10])
	logger.Infof("Finished sorting in %s", duration)
	if span != nil {
		span.AddEvent("sorted-array-ready", trace.WithAttributes(
			attribute.String("first_10_elements", fmt.Sprintf("%v", sortedNumbers[:10])),
			attribute.String("duration", duration.String()),
		))
	}

	w.Header().Set("Content-Type", "application/json")
	logger.WithField("status", http.StatusOK).Info("Request completed")
	if span != nil {
		span.SetAttributes(attribute.Int("http.status_code", http.StatusOK))
	}
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
		numbers[i] = rand.Intn(20000)
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
