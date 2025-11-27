#!/bin/bash

echo " Testing P2P Collaboration Engine"
echo "===================================="

# Clean up previous test files
echo " Cleaning up previous test files..."
rm -f test-*.txt
rm -rf .collab

# Build the test harness
echo "🔨 Building test harness..."
go build -o bin/test-collab ./cmd/test-collab
if [ $? -ne 0 ]; then
    echo "❌ Failed to build test harness"
    exit 1
fi

# Build the watcher test
echo "🔨 Building watcher test..."
go build -o bin/test-watcher ./cmd/test-watcher
if [ $? -ne 0 ]; then
    echo "❌ Failed to build watcher test"
    exit 1
fi

echo ""
echo " Available Tests:"
echo "1. Run collaboration engine tests: ./bin/test-collab"
echo "2. Run file watcher test: ./bin/test-watcher"
echo "3. Run original watcher: ./bin/p2pflow-watcher"
echo ""

# Run the collaboration engine tests
echo "Running collaboration engine tests..."
./bin/test-collab

echo ""
echo "Collaboration engine tests completed!"
echo ""
echo "Check the .collab/ directory for persisted session data:"
ls -la .collab/ 2>/dev/null || echo "No .collab directory found"

echo ""
echo "To test the file watcher with collaboration engine:"
echo "   1. Run: ./bin/test-watcher"
echo "   2. In another terminal, edit the test-watcher.txt file"
echo "   3. Watch for patch generation and session updates"
echo "   4. Check .collab/ directory for session persistence"
