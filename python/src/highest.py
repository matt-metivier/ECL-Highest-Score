import sys
import json
import heapq
import time
import uuid
import psutil
import threading
import argparse
from typing import List, Dict, Tuple
from opentelemetry import metrics
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk.resources import Resource

class Metrics:
    def __init__(self, run_id: str):
        self.run_id = run_id
        self.meter = self._setup_meter()
        self.processed_lines = self._create_processed_lines()
        self.heap_operations = self._create_heap_operations()
        self.processing_time = self._create_processing_time()
        self.bytes_read = self._create_bytes_read()
        self.memory_usage = self._create_memory_usage()
        self.thread_count = self._create_thread_count()
        self.cpu_usage = self._create_cpu_usage()
        self.streaming_latency = self._create_streaming_latency()
        self._processed_lines_count = 0
        self._last_update = time.time()

    def _setup_meter(self):
        resource = Resource.create({
            "service.name": "highest-scores-python",
            "service.instance.id": "python-impl",
            "run.id": self.run_id
        })

        reader = PeriodicExportingMetricReader(
            OTLPMetricExporter(endpoint="otel-collector:4317", insecure=True),
            export_interval_millis=5000
        )
        provider = MeterProvider(resource=resource, metric_readers=[reader])
        metrics.set_meter_provider(provider)
        return metrics.get_meter("highest-scores")

    def _memory_callback(self, options):
        attributes = {
            "implementation": "python",
            "run_id": self.run_id
        }
        yield metrics.Observation(psutil.Process().memory_info().rss, attributes)

    def _thread_callback(self, options):
        attributes = {
            "implementation": "python",
            "run_id": self.run_id
        }
        yield metrics.Observation(threading.active_count(), attributes)

    def _cpu_callback(self, options):
        attributes = {
            "implementation": "python",
            "run_id": self.run_id
        }
        try:
            cpu_percent = psutil.Process().cpu_percent()
            yield metrics.Observation(cpu_percent, attributes)
        except Exception:
            yield metrics.Observation(0.0, attributes)

    def _create_processed_lines(self):
        return self.meter.create_counter(
            "processed_lines_total",
            description="Number of lines processed",
            unit="1"
        )

    def _create_heap_operations(self):
        return self.meter.create_counter(
            "heap_operations_total",
            description="Number of heap operations",
            unit="1"
        )

    def _create_processing_time(self):
        return self.meter.create_histogram(
            "processing_time_seconds",
            description="Time taken to process chunks",
            unit="s"
        )

    def _create_bytes_read(self):
        return self.meter.create_counter(
            "bytes_read_total",
            description="Number of bytes read from input",
            unit="bytes"
        )

    def _create_memory_usage(self):
        return self.meter.create_observable_gauge(
            "memory_usage_bytes",
            description="Current memory usage",
            unit="bytes",
            callbacks=[self._memory_callback]
        )

    def _create_thread_count(self):
        return self.meter.create_observable_gauge(
            "thread_count",
            description="Number of active threads",
            unit="1",
            callbacks=[self._thread_callback]
        )

    def _create_cpu_usage(self):
        return self.meter.create_observable_gauge(
            "cpu_usage_percent",
            description="CPU usage percentage",
            unit="percent",
            callbacks=[self._cpu_callback]
        )

    def _create_streaming_latency(self):
        return self.meter.create_histogram(
            "streaming_latency_seconds",
            description="Time between receiving a line and updating results",
            unit="s"
        )

    def get_attributes(self):
        return {
            "implementation": "python",
            "run_id": self.run_id
        }

def parse_line(line: str, metrics: Metrics) -> Tuple[int, str]:
    try:
        if not line.strip():
            return None
        
        score_str, record_str = line.strip().split(':', 1)
        score = int(score_str.strip())
        
        metrics.bytes_read.add(
            len(line.encode('utf-8')), 
            metrics.get_attributes()
        )
        
        record = json.loads(record_str.strip())
        if 'id' not in record:
            return None
            
        return (score, record['id'])
    except (ValueError, json.JSONDecodeError):
        return None

def process_stream(filepath: str, n: int, metrics: Metrics) -> List[Dict[str, any]]:
    heap = []
    attributes = metrics.get_attributes()
    chunk_start_time = time.time()
    chunk_size = 0
    
    try:
        with open(filepath, 'r') as f:
            for line in f:
                line_start_time = time.time()
                metrics.processed_lines.add(1, attributes)
                metrics._processed_lines_count += 1
                
                metrics.streaming_latency.record(
                    time.time() - line_start_time,
                    attributes
                )
                
                result = parse_line(line, metrics)
                if result is None:
                    continue
                    
                score, record_id = result
                
                if len(heap) < n:
                    heapq.heappush(heap, (score, record_id))
                    metrics.heap_operations.add(1, attributes)
                elif score > heap[0][0]:
                    heapq.heapreplace(heap, (score, record_id))
                    metrics.heap_operations.add(2, attributes)
                
                if chunk_size >= 1000:
                    chunk_time = time.time() - chunk_start_time
                    metrics.processing_time.record(chunk_time, attributes)
                    chunk_size = 0
                    chunk_start_time = time.time()
                chunk_size += 1

    except FileNotFoundError:
        print(f"Error: File '{filepath}' not found", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error processing file: {str(e)}", file=sys.stderr)
        sys.exit(2)
    
    if chunk_size > 0:
        chunk_time = time.time() - chunk_start_time
        metrics.processing_time.record(chunk_time, attributes)
    
    result = [{"score": score, "id": id_} for score, id_ in heap]
    result.sort(key=lambda x: x["score"], reverse=True)
    return result

def process_batch(filepath: str, n: int, metrics: Metrics) -> List[Dict[str, any]]:
    heap = []
    chunk_start_time = time.time()
    chunk_size = 0
    attributes = metrics.get_attributes()
    
    try:
        with open(filepath, 'r') as f:
            for line in f:
                line_start_time = time.time()
                metrics.processed_lines.add(1, attributes)
                metrics._processed_lines_count += 1
                chunk_size += 1
                
                result = parse_line(line, metrics)
                if result is None:
                    continue
                    
                score, record_id = result
                
                if len(heap) < n:
                    heapq.heappush(heap, (score, record_id))
                    metrics.heap_operations.add(1, attributes)
                elif score > heap[0][0]:
                    heapq.heapreplace(heap, (score, record_id))
                    metrics.heap_operations.add(2, attributes)
                
                if chunk_size >= 10000:
                    chunk_time = time.time() - chunk_start_time
                    metrics.processing_time.record(chunk_time, attributes)
                    chunk_size = 0
                    chunk_start_time = time.time()
                    
    except FileNotFoundError:
        print(f"Error: File '{filepath}' not found", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Error processing file: {str(e)}", file=sys.stderr)
        sys.exit(2)
    
    if chunk_size > 0:
        chunk_time = time.time() - chunk_start_time
        metrics.processing_time.record(chunk_time, attributes)
    
    result = [{"score": score, "id": id_} for score, id_ in heap]
    result.sort(key=lambda x: x["score"], reverse=True)
    return result

def main():
    parser = argparse.ArgumentParser(description='Process scored records to find N highest scores')
    parser.add_argument('input_file', help='Input file path')
    parser.add_argument('n', type=int, help='Number of top scores to find')
    parser.add_argument('--stream', action='store_true', help='Enable streaming mode')
    args = parser.parse_args()
    
    if args.n <= 0:
        print("Error: N must be positive", file=sys.stderr)
        sys.exit(2)

    timestamp = time.strftime("%Y%m%d-%H%M%S")
    run_id = f"python-{timestamp}-{str(uuid.uuid4())[:8]}"
    metrics = Metrics(run_id)
    
    if args.stream:
        result = process_stream(args.input_file, args.n, metrics)
    else:
        result = process_batch(args.input_file, args.n, metrics)
        
    print(json.dumps(result, indent=4))
    time.sleep(5)

if __name__ == "__main__":
    main() 