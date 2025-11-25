#!/bin/bash

# Start Temporal development server using Temporal CLI
# This creates a local SQLite database for persistence

echo "Starting Temporal development server..."
echo "Database file: ./temporal.db"
echo "Temporal UI will be available at: http://localhost:8233"
echo "Temporal server will be available at: localhost:7233"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

temporal server start-dev --db-filename ./temporal.db