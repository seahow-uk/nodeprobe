#!/bin/bash

set -e

echo "Deploying NodeProbe cluster..."

# Build and start the cluster
docker-compose down --volumes
docker-compose build --no-cache
docker-compose up -d

echo "Waiting for services to start..."
sleep 10

# Check health of all nodes
echo "Checking node health..."
for port in 8443 8444 8445 8446; do
    echo "Checking node on port $port..."
    curl -k -f "https://localhost:$port/health" || echo "Node on port $port not ready yet"
done

echo ""
echo "NodeProbe cluster deployed successfully!"
echo ""
echo "Access points:"
echo "  Node 1 Dashboard: https://localhost:8443/dashboard"
echo "  Node 2 Dashboard: https://localhost:8444/dashboard" 
echo "  Node 3 Dashboard: https://localhost:8445/dashboard"
echo "  Node 4 Dashboard: https://localhost:8446/dashboard"
echo ""
echo "Monitor logs with: docker-compose logs -f" 