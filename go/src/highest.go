package main

import (
	"bufio"
	"container/heap"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Record represents a single data record
type Record struct {
	ID      string          `json:"id"`
	RawData json.RawMessage `json:"-"`
	Score   int
}

// MinHeap implementation for maintaining top N scores
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

// Metrics
var (
	labels = prometheus.Labels{
		"implementation": "go",
	}

	processedLines = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "processed_lines_total",
			Help: "Number of lines processed",
		},
		[]string{"implementation", "run_id"},
	)

	heapOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heap_operations_total",
			Help: "Number of heap operations performed",
		},
		[]string{"implementation", "run_id"},
	)

	processingTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "processing_time_seconds",
			Help:    "Time taken to process chunks",
			Buckets: []float64{.001, .005, .01, .025, .05, .075, .1, .25, .5, .75, 1.0, 2.5, 5.0},
		},
		[]string{"implementation", "run_id"},
	)

	memoryUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
		[]string{"implementation", "run_id"},
	)

	threadCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "thread_count",
			Help: "Number of active threads/goroutines",
		},
		[]string{"implementation", "run_id"},
	)

	// New metrics
	invalidRecords = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "invalid_records_total",
			Help: "Number of invalid records encountered",
		},
		[]string{"implementation", "run_id", "error_type"},
	)

	bytesRead = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bytes_read_total",
			Help: "Number of bytes read from input",
		},
		[]string{"implementation", "run_id"},
	)

	gcStats = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gc_stats",
			Help: "Garbage collection statistics",
		},
		[]string{"implementation", "run_id", "stat_type"},
	)

	cpuUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "CPU usage percentage",
		},
		[]string{"implementation", "run_id"},
	)
)

// updateResourceMetrics periodically updates resource usage metrics
func updateResourceMetrics(runID string) {
	labels := prometheus.Labels{
		"implementation": "go",
		"run_id":         runID,
	}

	for {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Memory metrics
		memoryUsage.With(labels).Set(float64(m.Alloc))
		threadCount.With(labels).Set(float64(runtime.NumGoroutine()))

		// GC stats
		gcLabels := prometheus.Labels{
			"implementation": "go",
			"run_id":         runID,
			"stat_type":      "gc_cycles",
		}
		gcStats.With(gcLabels).Set(float64(m.NumGC))

		gcLabels["stat_type"] = "gc_pause_ns"
		gcStats.With(gcLabels).Set(float64(m.PauseTotalNs))

		gcLabels["stat_type"] = "heap_objects"
		gcStats.With(gcLabels).Set(float64(m.HeapObjects))

		// CPU usage (simplified)
		var cpuUsageValue float64
		if info, err := getCPUUsage(); err == nil {
			cpuUsageValue = info
		}
		cpuUsage.With(labels).Set(cpuUsageValue)

		time.Sleep(time.Second)
	}
}

func getCPUUsage() (float64, error) {
	// This is a simplified version - in production you'd want to use something like gopsutil
	// to get more accurate CPU metrics
	return 0.0, nil
}

// processChunk processes a chunk of the file
func processChunk(chunk []string, n int, runID string) []Record {
	h := &MinHeap{}
	heap.Init(h)
	startTime := time.Now()

	labels := prometheus.Labels{
		"implementation": "go",
		"run_id":         runID,
	}

	for _, line := range chunk {
		processedLines.With(labels).Inc()

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
		record.Score = score

		if h.Len() < n {
			heap.Push(h, record)
			heapOperations.With(labels).Inc()
		} else if score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, record)
			heapOperations.With(labels).Add(2) // Count both pop and push
		}
	}

	// Record processing time for the chunk
	processingTime.With(labels).Observe(time.Since(startTime).Seconds())

	result := make([]Record, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(Record)
		heapOperations.With(labels).Inc()
	}
	return result
}

func main() {
	// Generate a unique run ID
	rand.Seed(time.Now().UnixNano())
	runID := fmt.Sprintf("%08x", rand.Int31())

	// Start Prometheus metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":8001", nil))
	}()

	// Start resource metrics collection
	go updateResourceMetrics(runID)

	n := flag.Int("n", 5, "number of top scores to find")
	filename := flag.String("file", "", "input file path")
	flag.Parse()

	if *filename == "" {
		log.Fatal("Please provide an input file")
	}

	file, err := os.Open(*filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Determine chunk size and number of workers
	numWorkers := runtime.NumCPU()
	chunkSize := 10000 // Adjust based on testing

	// Create channels for work distribution
	chunks := make(chan []string, numWorkers)
	results := make(chan []Record, numWorkers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range chunks {
				results <- processChunk(chunk, *n, runID)
			}
		}()
	}

	// Read and distribute chunks
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

	// Collect and merge results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge results using a min-heap
	finalHeap := &MinHeap{}
	heap.Init(finalHeap)

	for result := range results {
		for _, record := range result {
			if finalHeap.Len() < *n {
				heap.Push(finalHeap, record)
			} else if record.Score > (*finalHeap)[0].Score {
				heap.Pop(finalHeap)
				heap.Push(finalHeap, record)
			}
		}
	}

	// Convert to final output format
	type OutputRecord struct {
		Score int    `json:"score"`
		ID    string `json:"id"`
	}

	output := make([]OutputRecord, finalHeap.Len())
	for i := len(output) - 1; i >= 0; i-- {
		record := heap.Pop(finalHeap).(Record)
		output[i] = OutputRecord{Score: record.Score, ID: record.ID}
	}

	// Output JSON
	jsonOutput, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonOutput))
}
