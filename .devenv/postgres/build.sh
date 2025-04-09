#!/bin/bash

IMAGE_NAME="nautilus-postgres"

echo "Building Docker image: $IMAGE_NAME"
docker build -t $IMAGE_NAME .

echo "Build completed successfully."
echo "To run the container, execute: ./run.sh"