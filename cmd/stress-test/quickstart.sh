#!/bin/bash

# Quick Start Script for Stress Test Tool
# This script builds and runs a basic stress test

set -e

echo "=========================================="
echo "IM System Stress Test Tool - Quick Start"
echo "=========================================="
echo ""

# Check if we're in the right directory
if [ ! -f "main.go" ]; then
    echo "Error: Please run this script from backend/cmd/stress-test directory"
    exit 1
fi

# Build the tool
echo "Building stress test tool..."
go build -o stress-test
echo "✓ Build complete"
echo ""

# Show help
echo "Available commands:"
echo ""
./stress-test --help
echo ""

# Offer to run a quick test
echo "=========================================="
echo "Quick Test Options:"
echo "=========================================="
echo ""
echo "1. Run interactive mode"
echo "2. Run small user creation test (10 users)"
echo "3. Run small friend request test (5 users, 2 requests each)"
echo "4. Run small group chat test (2 groups, 5 members, 10 messages)"
echo "5. Exit"
echo ""
read -p "Enter your choice (1-5): " choice

case $choice in
    1)
        echo "Starting interactive mode..."
        ./stress-test interactive
        ;;
    2)
        echo "Running user creation test with 10 users..."
        ./stress-test scenario user-creation --count 10
        ;;
    3)
        echo "Running friend request test with 5 users..."
        ./stress-test scenario friend-requests --users 5 --requests 2
        ;;
    4)
        echo "Running group chat test..."
        ./stress-test scenario group-chat --groups 2 --members 5 --messages 10
        ;;
    5)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid choice. Exiting..."
        exit 1
        ;;
esac

echo ""
echo "=========================================="
echo "Test complete!"
echo "=========================================="
echo ""
echo "To clean up test data, run:"
echo "  ./stress-test cleanup --config ../../config.yaml"
echo ""
