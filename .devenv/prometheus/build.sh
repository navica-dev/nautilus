#!/bin/bash

# Script to build the Prometheus Docker image

# Set the image name
IMAGE_NAME="nautilus-prometheus"

echo "Building Docker image: $IMAGE_NAME"
docker build -t $IMAGE_NAME .

echo "Build completed successfully."
echo "To run the container, execute: ./run.sh"