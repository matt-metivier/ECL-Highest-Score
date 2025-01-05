# Highest Scores - Solution Overview

## The Challenge & My Approach

The core challenge is finding the N highest scores from a large dataset efficiently. This is a common problem in real-world scenarios, like processing high-throughput laboratory data or analyzing streaming sensor data.

### Why Min-Heap?
I chose a min-heap data structure because:
- It only keeps N items in memory at any time
- New items are compared with the smallest item in our top-N list
- Perfect for finding top-N elements in a single pass
- Memory usage stays constant regardless of input size

## Performance Comparison

Based on our monitoring, here's how the implementations compare:

### Processing Speed
- Go: ~24.5ms per chunk
- Python: ~128-148ms per chunk
- Go is approximately 5x faster in processing

### Memory Efficiency
- Go: ~567 KiB
- Python: ~43-45 MiB
- Go uses about 80x less memory

### Operation Efficiency
- Go: 378 heap operations for 20K lines
- Python: 218 heap operations for 60K lines
- Different trade-offs in heap management strategy

## Core Implementation (Python)

The Python implementation demonstrates the basic solution using a min-heap. It's clean, straightforward, and memory-efficient.

```bash
# Run Python version directly
python python/src/highest.py example_input_data_3.data 10
```

### Memory Efficiency
Instead of loading the entire file into memory, I:
- Process the file line by line
- Keep only N records in the heap
- Discard records that won't make it to the top N

## Going Further (Go Implementation)

I created a Go version to showcase how I can push performance further using:
- Concurrent processing with goroutines
- Efficient memory management
- Parallel chunk processing

```bash
# Run Go version directly
./go/src/highest-scores -file example_input_data_3.data -n 10
```

### Why Go?
Go was chosen for the high-performance because:
- Native support for concurrency (goroutines)
- Efficient memory management
- Fast JSON parsing
- Excellent for processing large files

## Advanced Monitoring Solution

I've implemented comprehensive performance monitoring using:
- OpenTelemetry for metrics collection
- Prometheus for metrics storage
- Grafana for visualization

### Metrics Tracked
1. **Core Performance**
   - Total lines processed
   - Heap operations
   - Processing time per chunk
   - Memory usage

2. **Error Handling**
   - Invalid records by type
   - JSON parsing errors
   - Missing ID errors

3. **Resource Usage**
   - Memory allocation patterns
   - GC statistics
   - Thread/Goroutine count
   - CPU usage

4. **I/O Performance**
   - Bytes read
   - Processing throughput

### Monitoring Dashboard
The Grafana dashboard provides real-time insights into:
- Processing efficiency
- Memory utilization
- Error rates
- Resource consumption

## Running the Solutions

I have provided multiple ways to run the implementations:

### Quick Comparison (Single Container)
Best for quick comparison of implementations:
```bash
# Run Python version
docker run highest-scores-multi python example_input_data_3.data 10

# Run Go version
docker run highest-scores-multi go example_input_data_3.data 10
```

### Development Setup (Separate Containers)
Best to showcase detailed performance analysis:
```bash
# Start only the monitoring stack
docker-compose up -d otel-collector prometheus grafana

# Start everything including monitoring
docker-compose up -d

# Run implementations
docker-compose up python-impl go-impl
```

### Generate Test Data
```bash
# Using data generator container
docker-compose up data-gen
```

### View Monitoring
```bash
# Access dashboards
open http://localhost:3000  # Grafana
open http://localhost:9090  # Prometheus

# View logs
docker-compose logs -f python-impl go-impl
docker-compose logs -f otel-collector
```

## Project Structure

```
ECL Highest Scores Platform/
├── python/                    # Python implementation
│   ├── src/                  # Source code
│   └── Dockerfile            # Python container build
├── go/                       # Go implementation
│   ├── src/                  # Source code
│   └── Dockerfile            # Go container build
├── common/
│   ├── data-gen/            # Test data generation
│   └── monitoring/          # Performance monitoring
│       ├── grafana/         # Dashboards and config
│       ├── prometheus/      # Prometheus config
│       └── otel-collector/  # OpenTelemetry config
└── scripts/                 # Helper scripts
```

### Why This Structure?
- Separates concerns clearly
- Makes it easy to add new implementations
- Keeps monitoring and data generation independent
- Simplifies maintenance

## Development Notes

### Key Design Decisions
1. **Memory Efficiency First**: Both implementations prioritize memory efficiency over raw speed
2. **Streaming Processing**: Process data as it comes, don't load everything at once
3. **Proper Error Handling**: Graceful handling of invalid input and edge cases
4. **Observable**: Built-in metrics for performance analysis
5. **Comparable**: Consistent metrics between implementations

### Future Improvements
- Add streaming input support
- Implement distributed processing for very large files
- Add more test cases and benchmarks
- Support different input/output formats
- Add REST API endpoint for querying results
- Add support for compressed files
- Implement parallel processing of multiple files

## Running Tests
```bash
# Test with different dataset sizes
docker run highest-scores-multi python example_input_data_1.data 5  # Small
docker run highest-scores-multi python example_input_data_2.data 10 # Medium
docker run highest-scores-multi python example_input_data_3.data 20 # Large

# Same for Go
docker run highest-scores-multi go example_input_data_3.data 20
```
