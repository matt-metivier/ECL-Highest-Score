package main

import (
	"bufio"
	"container/heap"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Record struct {
	ID      string          `json:"id"`
	RawData json.RawMessage `json:"-"`
	Score   int
}

type OutputRecord struct {
	Score int    `json:"score"`
	ID    string `json:"id"`
}

type MinHeap []Record

func (h MinHeap) Len() int            { return len(h) }
func (h MinHeap) Less(i, j int) bool  { return h[i].Score < h[j].Score }
func (h MinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *MinHeap) Push(x interface{}) { *h = append(*h, x.(Record)) }
func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type Metrics struct {
	processedLines   metric.Int64Counter
	heapOperations   metric.Int64Counter
	processingTime   metric.Float64Histogram
	memoryUsage      metric.Float64ObservableGauge
	threadCount      metric.Int64ObservableGauge
	bytesRead        metric.Int64Counter
	cpuUsage         metric.Float64ObservableGauge
	streamingLatency metric.Float64Histogram
	meter            metric.Meter
	addOpts          []metric.AddOption
	recordOpts       []metric.RecordOption
	observeOpts      []metric.ObserveOption
}

func newMetrics(runID string) (*Metrics, error) {
	ctx := context.Background()

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("highest-scores-go"),
		semconv.ServiceInstanceID("go-impl"),
	)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var conn *grpc.ClientConn
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		conn, err = grpc.DialContext(ctx, "otel-collector:4317",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to collector (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Second * time.Duration(2<<uint(i)))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to collector after %d attempts: %v", maxRetries, err)
	}

	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
		otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     5 * time.Second,
			MaxElapsedTime:  30 * time.Second,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %v", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(5*time.Second))),
	)
	otel.SetMeterProvider(meterProvider)

	meter := meterProvider.Meter("highest-scores")
	attrs := []attribute.KeyValue{
		attribute.String("implementation", "go"),
		attribute.String("run_id", runID),
	}

	m := &Metrics{
		meter:       meter,
		addOpts:     []metric.AddOption{metric.WithAttributes(attrs...)},
		recordOpts:  []metric.RecordOption{metric.WithAttributes(attrs...)},
		observeOpts: []metric.ObserveOption{metric.WithAttributes(attrs...)},
	}

	var err1, err2, err3, err4, err5, err6, err7, err8 error

	m.processedLines, err1 = meter.Int64Counter(
		"processed_lines_total",
		metric.WithDescription("Number of lines processed"),
		metric.WithUnit("1"),
	)

	m.heapOperations, err2 = meter.Int64Counter(
		"heap_operations_total",
		metric.WithDescription("Number of heap operations"),
		metric.WithUnit("1"),
	)

	m.processingTime, err3 = meter.Float64Histogram(
		"processing_time_seconds",
		metric.WithDescription("Time taken to process chunks"),
		metric.WithUnit("s"),
	)

	m.bytesRead, err4 = meter.Int64Counter(
		"bytes_read_total",
		metric.WithDescription("Number of bytes read from input"),
		metric.WithUnit("bytes"),
	)

	m.streamingLatency, err5 = meter.Float64Histogram(
		"streaming_latency_seconds",
		metric.WithDescription("Time between receiving a line and updating results"),
		metric.WithUnit("s"),
	)

	m.memoryUsage, err6 = meter.Float64ObservableGauge(
		"memory_usage_bytes",
		metric.WithDescription("Current memory usage"),
		metric.WithUnit("bytes"),
	)

	m.threadCount, err7 = meter.Int64ObservableGauge(
		"thread_count",
		metric.WithDescription("Number of active threads"),
		metric.WithUnit("1"),
	)

	m.cpuUsage, err8 = meter.Float64ObservableGauge(
		"cpu_usage_percent",
		metric.WithDescription("CPU usage percentage"),
		metric.WithUnit("percent"),
	)

	if err := firstError(err1, err2, err3, err4, err5, err6, err7, err8); err != nil {
		return nil, fmt.Errorf("failed to create metrics: %v", err)
	}

	return m, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Metrics) startResourceMetrics(ctx context.Context) {
	_, err := m.meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			o.ObserveFloat64(m.memoryUsage, float64(memStats.Alloc), m.observeOpts...)
			o.ObserveInt64(m.threadCount, int64(runtime.NumGoroutine()), m.observeOpts...)
			o.ObserveFloat64(m.cpuUsage, 0.0, m.observeOpts...)
			return nil
		},
		m.memoryUsage, m.threadCount, m.cpuUsage,
	)
	if err != nil {
		log.Printf("Failed to register callback: %v", err)
	}
}

func (m *Metrics) processChunk(ctx context.Context, chunk []string, n int) []Record {
	h := &MinHeap{}
	heap.Init(h)
	startTime := time.Now()

	for _, line := range chunk {
		lineStartTime := time.Now()
		m.processedLines.Add(ctx, 1, m.addOpts...)
		m.bytesRead.Add(ctx, int64(len(line)), m.addOpts...)

		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		var score int
		if _, err := fmt.Sscanf(parts[0], "%d", &score); err != nil {
			continue
		}

		var record Record
		if err := json.Unmarshal([]byte(parts[1]), &record); err != nil {
			continue
		}

		if record.ID == "" {
			continue
		}

		record.Score = score

		if h.Len() < n {
			heap.Push(h, record)
			m.heapOperations.Add(ctx, 1, m.addOpts...)
		} else if score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, record)
			m.heapOperations.Add(ctx, 2, m.addOpts...)
		}

		m.streamingLatency.Record(ctx, time.Since(lineStartTime).Seconds(), m.recordOpts...)
	}

	m.processingTime.Record(ctx, time.Since(startTime).Seconds(), m.recordOpts...)

	result := make([]Record, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(Record)
		m.heapOperations.Add(ctx, 1, m.addOpts...)
	}
	return result
}

func processFile(ctx context.Context, metrics *Metrics) error {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [--stream] input_file n\n", os.Args[0])
		flag.PrintDefaults()
	}

	streamMode := flag.Bool("stream", false, "enable streaming mode")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		return fmt.Errorf("usage: %s [--stream] input_file n", os.Args[0])
	}

	filename := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid value for n: %v", err)
	}

	if n <= 0 {
		return fmt.Errorf("n must be positive")
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if *streamMode {
		return processStream(ctx, file, n, metrics)
	}
	return processBatch(ctx, file, n, metrics)
}

func processStream(ctx context.Context, file *os.File, n int, metrics *Metrics) error {
	h := &MinHeap{}
	heap.Init(h)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB buffer, 1MB max line size

	for scanner.Scan() {
		lineStartTime := time.Now()
		line := scanner.Text()
		metrics.processedLines.Add(ctx, 1, metrics.addOpts...)
		metrics.bytesRead.Add(ctx, int64(len(line)), metrics.addOpts...)

		if strings.TrimSpace(line) == "" {
			continue
		}

		metrics.streamingLatency.Record(ctx, time.Since(lineStartTime).Seconds(), metrics.recordOpts...)

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		var score int
		if _, err := fmt.Sscanf(parts[0], "%d", &score); err != nil {
			continue
		}

		var record Record
		if err := json.Unmarshal([]byte(parts[1]), &record); err != nil {
			continue
		}

		if record.ID == "" {
			continue
		}

		record.Score = score

		if h.Len() < n {
			heap.Push(h, record)
			metrics.heapOperations.Add(ctx, 1, metrics.addOpts...)
		} else if score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, record)
			metrics.heapOperations.Add(ctx, 2, metrics.addOpts...)
		}

		metrics.streamingLatency.Record(ctx, time.Since(lineStartTime).Seconds(), metrics.recordOpts...)
	}

	output := make([]OutputRecord, h.Len())
	for i := len(output) - 1; i >= 0; i-- {
		record := heap.Pop(h).(Record)
		output[i] = OutputRecord{Score: record.Score, ID: record.ID}
		metrics.heapOperations.Add(ctx, 1, metrics.addOpts...)
	}

	jsonOutput, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %v", err)
	}
	fmt.Println(string(jsonOutput))

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

func processBatch(ctx context.Context, file *os.File, n int, metrics *Metrics) error {
	numWorkers := runtime.NumCPU()
	chunkSize := 50000

	chunks := make(chan []string, numWorkers)
	results := make(chan []Record, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range chunks {
				results <- metrics.processChunk(ctx, chunk, n)
			}
		}()
	}

	go func() {
		scanner := bufio.NewScanner(file)
		currentChunk := make([]string, 0, chunkSize)

		for scanner.Scan() {
			currentChunk = append(currentChunk, scanner.Text())
			if len(currentChunk) == chunkSize {
				chunks <- currentChunk
				currentChunk = make([]string, 0, chunkSize)
			}
		}

		if len(currentChunk) > 0 {
			chunks <- currentChunk
		}
		close(chunks)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	finalHeap := &MinHeap{}
	heap.Init(finalHeap)

	for result := range results {
		for _, record := range result {
			if finalHeap.Len() < n {
				heap.Push(finalHeap, record)
			} else if record.Score > (*finalHeap)[0].Score {
				heap.Pop(finalHeap)
				heap.Push(finalHeap, record)
			}
		}
	}

	type OutputRecord struct {
		Score int    `json:"score"`
		ID    string `json:"id"`
	}

	output := make([]OutputRecord, finalHeap.Len())
	for i := len(output) - 1; i >= 0; i-- {
		record := heap.Pop(finalHeap).(Record)
		output[i] = OutputRecord{Score: record.Score, ID: record.ID}
	}

	jsonOutput, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonOutput))
	return nil
}

func main() {
	timestamp := time.Now().Format("20060102-150405")
	runID := fmt.Sprintf("go-%s-%x", timestamp, rand.Int31())

	metrics, err := newMetrics(runID)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	metrics.startResourceMetrics(ctx)

	if err := processFile(ctx, metrics); err != nil {
		log.Fatal(err)
	}

	log.Println("Processing complete. Waiting for metrics to be scraped...")
	time.Sleep(5 * time.Second)
}
