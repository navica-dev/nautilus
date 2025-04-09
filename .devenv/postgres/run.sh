#!/bin/bash

IMAGE_NAME="nautilus-postgres"
CONTAINER_NAME="postgres-instance"

# Check if container already exists
if [ "docker ps -aq -f name=$CONTAINER_NAME" ]; then
    echo "Stopping and removing existing container: $CONTAINER_NAME"
    docker stop $CONTAINER_NAME
    docker rm $CONTAINER_NAME
fi
echo "Starting PostgreSQL container..."
docker run -d \
    --name $CONTAINER_NAME \
    -p 5432:5432 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=postgres \
    -e POSTGRES_DB=postgres \
    $IMAGE_NAME
echo "Container started successfully."
echo "PostgreSQL is now accessible at: localhost:5432"
echo "Username: postgres"
echo "Password: postgres"
echo "Database: postgres"
