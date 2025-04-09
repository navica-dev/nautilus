#!/bin/bash

# Script to run the Prometheus Docker container

# Set the image name and container name
IMAGE_NAME="nautilus-prometheus"
CONTAINER_NAME="prometheus-instance"

# Check if container already exists
if [ "$(docker ps -aq -f name=$CONTAINER_NAME)" ]; then
    echo "Stopping and removing existing container: $CONTAINER_NAME"
    docker stop $CONTAINER_NAME
    docker rm $CONTAINER_NAME
fi

echo "Starting Prometheus container..."
docker run -d \
    --name $CONTAINER_NAME \
    -p 9090:9090 \
    $IMAGE_NAME

echo "Container started successfully."
echo "Prometheus is now accessible at: http://localhost:9090"
