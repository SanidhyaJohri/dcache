#!/bin/bash

echo "=== Starting Load Test ==="
echo "Inserting 1000 keys across cluster..."

START=$(date +%s)

# Insert keys
for i in {1..1000}; do
    PORT=$((8001 + (i % 3)))
    curl -s -X POST http://localhost:$PORT/cache/loadtest-$i \
         -d "value-$i" > /dev/null
    
    if [ $((i % 100)) -eq 0 ]; then
        echo "Progress: $i/1000 keys inserted"
    fi
done

END=$(date +%s)
DURATION=$((END - START))

echo ""
echo "=== Load Test Complete ==="
echo "Time taken: ${DURATION} seconds"
echo "Rate: $((1000 / DURATION)) keys/second"
echo ""
echo "=== Distribution Check ==="

for PORT in 8001 8002 8003; do
    NODE=$((PORT - 8000))
    COUNT=$(curl -s http://localhost:$PORT/stats | jq '.items')
    echo "Node$NODE (port $PORT): $COUNT items"
done

echo ""
echo "=== Sample Key Locations ==="
for i in {1..10}; do
    KEY="loadtest-$i"
    NODE=$(curl -s http://localhost:8001/cluster/locate/$KEY | jq -r '.node_id')
    echo "$KEY -> $NODE"
done
