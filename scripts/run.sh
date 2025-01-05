#!/bin/bash

# Generate test data if it doesn't exist
if [ ! -f example_input_data_1.data ]; then
    echo "Generating test data..."
    node gen.js
fi

# Default values
IMPLEMENTATION=${1:-"python"}  # Default to python if not specified
N=${2:-5}                     # Default to 5 if not specified
FILE=${3:-"example_input_data_1.data"}  # Default to smallest dataset if not specified

echo "Running $IMPLEMENTATION implementation with n=$N on file $FILE"

case $IMPLEMENTATION in
    "python")
        python3 /app/python/highest.py "$FILE" "$N"
        ;;
    "go")
        /app/go/highest-scores -file "$FILE" -n "$N"
        ;;
    *)
        echo "Invalid implementation. Use 'python' or 'go'"
        exit 1
        ;;
esac 