#!/bin/bash

# Generate test data files
echo "Generating test data files..."
node gen.js

# Get the number of records to process (default to 5 if not provided)
N=${1:-5}
# Get the input file to process (default to example_input_data_1.data if not provided)
INPUT_FILE=${2:-"example_input_data_1.data"}

echo "Processing $INPUT_FILE for top $N records..."
python3 highest.py "$INPUT_FILE" "$N" 