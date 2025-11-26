#!/bin/bash
echo "Starting load test..."
START=$(date +%s)

# Insert 1000 keys
for i in {1..1000}; do
  curl -s -X POST http://localhost:8080/cache/key$i -d "value$i" > /dev/null &
  if [ $((i % 100)) -eq 0 ]; then
    echo "Inserted $i keys..."
    wait
  fi
done
wait

END=$(date +%s)
echo "Completed in $((END-START)) seconds"

# Check final stats
curl -s http://localhost:8080/stats | jq '.items, .hits, .misses, .evictions'
