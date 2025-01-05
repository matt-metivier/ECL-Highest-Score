# Solution: Highest Scores Platform

Hey there! 👋 Here's how to run and understand my solution.

## Quick Start 🚀

Want to see everything in action?
This will run docker-compose.yml file and start the containers.
```bash
docker-compose down -v
docker-compose up -d
```
By default this runs with "example_input_data_3.data" - "12"

Want to see all the tests?
```bash
docker-compose -f docker-compose-small-medium-large.yml down -v
docker-compose -f docker-compose-small-medium-large.yml up -d
```
This will run all tests in sequence:
1. Python (batch mode) - small → medium → large files
2. Python (streaming) - small → medium → large files
3. Go (batch mode) - small → medium → large files
4. Go (streaming) - small → medium → large files

## File Structure 📂
- **Root Directory:**
  - `README.md`: Documentation for the project.
  - `docker-compose.yml`: Configuration for running the application with Docker.
  - `docker-compose-small-medium-large.yml`: Configuration for running tests with different data sizes.

- **Common Directory:**
  - `monitoring/`: Contains configurations for monitoring tools like Grafana and Prometheus.
  - `data-gen/`: Scripts and configurations for generating test data.

- **Go Directory:**
  - `Dockerfile`: Docker configuration for the Go implementation.
  - `src/`: Source code for the Go implementation.

- **Python Directory:**
  - `Dockerfile`: Docker configuration for the Python implementation.
  - `src/`: Source code for the Python implementation.

- **Scripts Directory:**
  - `run.sh`: Script to run the application with specified parameters.

## What's Inside? 🔍

I've built this in both Python and Go, and both versions:
- Use min-heaps for efficient top-N selection
- Support streaming for large files
- Include detailed performance metrics
- Run in both batch and streaming modes

## Running Individual Tests 🏃‍♂️

First, generate some test data:
```bash
docker-compose run --rm data-gen
```

Then run either implementation:
```bash
# Python
docker-compose run --rm python-impl example_input_data_2.data 10
docker-compose run --rm python-impl --stream example_input_data_2.data 10  # streaming mode

# Go
docker-compose run --rm go-impl example_input_data_2.data 10
docker-compose run --rm go-impl --stream example_input_data_2.data 10  # streaming mode
```

## Test Files 📁
I've included three test file sizes:
- Small (example_input_data_1.data): ~4.7K records
- Medium (example_input_data_2.data): ~470K records
- Large (example_input_data_3.data): ~2.7M records

## Performance Monitoring 📊

Want to see how it's performing? Check out:
- Grafana: http://localhost:3000 (for pretty graphs)
- Prometheus: http://localhost:9090 (for raw metrics)

You'll see metrics for:
- Lines processed
- Memory usage
- Processing speed
- Heap operations
- And more!

## Performance Insights 🎯

From my testing:
- Go is generally faster but uses variable memory
- Python is more memory-consistent (~45MB)
- Both handle large files well
- Streaming mode is great for huge files
- Batch mode shines on smaller datasets

## Exit Codes ✨
I've implemented standard exit codes:
- 0: All good!
- 1: Can't find the input file
- 2: Something's wrong with the input data

Need help or have questions? Feel free to ask! 🙋‍♂️
