#!/usr/bin/env python3
import sys
import json
import heapq
import time
import uuid
import psutil
import gc
import threading
from typing import List, Dict, Tuple
from opentelemetry import metrics
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk.resources import Resource

# Generate a unique run ID
run_id = str(uuid.uuid4())[:8]

# Initialize OpenTelemetry metrics
resource = Resource.create({
    "service.name": "highest-scores-python",
    "service.instance.id": "python-impl",
    "run.id": run_id
})

reader = PeriodicExportingMetricReader(
    OTLPMetricExporter(endpoint="otel-collector:4317", insecure=True)
)
provider = MeterProvider(resource=resource, metric_readers=[reader])
metrics.set_meter_provider(provider)
meter = metrics.get_meter("highest-scores")

# Create metrics
processed_lines = meter.create_counter(
    "processed_lines_total",
    description="Number of lines processed",
    unit="1"
)

heap_operations = meter.create_counter(
    "heap_operations_total",
    description="Number of heap operations",
    unit="1"
)

processing_time = meter.create_histogram(
    "processing_time_seconds",
    description="Time taken to process chunks",
    unit="s"
)

# New metrics to match Go implementation
invalid_records = meter.create_counter(
    "invalid_records_total",
    description="Number of invalid records encountered",
    unit="1"
)

bytes_read = meter.create_counter(
    "bytes_read_total",
    description="Number of bytes read from input",
    unit="bytes"
)

gc_stats = meter.create_observable_gauge(
    "gc_stats",
    description="Garbage collection statistics",
    unit="1"
)

cpu_usage = meter.create_observable_gauge(
    "cpu_usage_percent",
    description="CPU usage percentage",
    unit="percent"
)

memory_usage = meter.create_observable_gauge(
    "memory_usage_bytes",
    description="Current memory usage",
    unit="bytes"
)

thread_count = meter.create_observable_gauge(
    "thread_count",
    description="Number of active threads",
    unit="1"
)

def memory_callback(options):
    """Callback for memory usage metric"""
    attributes = {
        "implementation": "python",
        "run_id": run_id
    }
    yield metrics.Observation(psutil.Process().memory_info().rss, attributes)

def thread_callback(options):
    """Callback for thread count metric"""
    attributes = {
        "implementation": "python",
        "run_id": run_id
    }
    yield metrics.Observation(threading.active_count(), attributes)

def gc_callback(options):
    """Callback for GC statistics"""
    stats = gc.get_stats()
    total_collections = sum(s["collections"] for s in stats)
    total_time = sum(s["collected"] for s in stats)  # approximate time based on objects collected
    total_objects = sum(s["collected"] for s in stats)

    base_attributes = {
        "implementation": "python",
        "run_id": run_id,
    }
    
    # Match Go's gc_stats metrics
    yield metrics.Observation(total_collections, {**base_attributes, "stat_type": "gc_cycles"})
    yield metrics.Observation(total_time, {**base_attributes, "stat_type": "gc_pause_ns"})
    yield metrics.Observation(total_objects, {**base_attributes, "stat_type": "heap_objects"})

def cpu_callback(options):
    """Callback for CPU usage"""
    attributes = {
        "implementation": "python",
        "run_id": run_id
    }
    try:
        cpu_percent = psutil.Process().cpu_percent()
        yield metrics.Observation(cpu_percent, attributes)
    except Exception:
        yield metrics.Observation(0.0, attributes)

# Register callbacks
memory_usage.add_callback(memory_callback)
thread_count.add_callback(thread_callback)
gc_stats.add_callback(gc_callback)
cpu_usage.add_callback(cpu_callback)

def parse_line(line: str) -> Tuple[int, str]:
    """Parse a single line of input, returning (score, id)."""
    try:
        # Skip empty lines
        if not line.strip():
            return None
        
        # Split and parse score
        score_str, record_str = line.strip().split(':', 1)
        score = int(score_str.strip())
        
        # Track bytes read
        bytes_read.add(
            len(line.encode('utf-8')), 
            {"implementation": "python", "run_id": run_id}
        )
        
        # Parse and validate record
        record = json.loads(record_str.strip())
        if 'id' not in record:
            invalid_records.add(
                1, 
                {"implementation": "python", "run_id": run_id, "error_type": "missing_id"}
            )
            raise ValueError("Record missing 'id' field")
            
        return (score, record['id'])
    except (ValueError, json.JSONDecodeError) as e:
        error_type = "json_error" if isinstance(e, json.JSONDecodeError) else "value_error"
        invalid_records.add(
            1, 
            {"implementation": "python", "run_id": run_id, "error_type": error_type}
        )
        return None

def get_highest_scores(filepath: str, n: int) -> List[Dict[str, any]]:
    """Process file and return n highest scores using a min-heap."""
    # Use min-heap to keep track of n highest scores
    heap: List[Tuple[int, str]] = []
    chunk_start_time = time.time()
    chunk_size = 0
    
    try:
        with open(filepath, 'r') as f:
            for line in f:
                attributes = {
                    "implementation": "python",
                    "run_id": run_id
                }
                
                processed_lines.add(1, attributes)
                chunk_size += 1
                
                result = parse_line(line)
                if result is None:
                    continue
                    
                score, record_id = result
                
                if len(heap) < n:
                    # Heap not full, add new item
                    heapq.heappush(heap, (score, record_id))
                    heap_operations.add(1, attributes)
                elif score > heap[0][0]:
                    # Score is higher than smallest in heap
                    heapq.heapreplace(heap, (score, record_id))
                    heap_operations.add(2, attributes)
                
                # Record processing time every 10000 lines
                if chunk_size >= 10000:
                    chunk_time = time.time() - chunk_start_time  # Keep in seconds
                    processing_time.record(chunk_time, attributes)
                    chunk_size = 0
                    chunk_start_time = time.time()
                    
    except FileNotFoundError:
        print(f"Error: File '{filepath}' not found", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error processing file: {str(e)}", file=sys.stderr)
        sys.exit(2)
    
    # Record final chunk if any
    if chunk_size > 0:
        chunk_time = time.time() - chunk_start_time
        processing_time.record(chunk_time, attributes)
    
    # Convert heap to sorted list of dictionaries
    result = [{"score": score, "id": id_} for score, id_ in heap]
    result.sort(key=lambda x: x["score"], reverse=True)
    return result

def main():
    # Validate command line arguments
    if len(sys.argv) != 3:
        print("Usage: python highest.py <input_file> <n>", file=sys.stderr)
        sys.exit(2)
    
    try:
        n = int(sys.argv[2])
        if n <= 0:
            raise ValueError("N must be positive")
    except ValueError:
        print("Error: Second argument must be a positive integer", file=sys.stderr)
        sys.exit(2)
    
    # Get and output results
    result = get_highest_scores(sys.argv[1], n)
    print(json.dumps(result, indent=4))

if __name__ == "__main__":
    main() 