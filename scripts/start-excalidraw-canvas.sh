#!/bin/bash
# Start Excalidraw canvas server for MCP integration
# Run this BEFORE starting opencode

cd /tmp/mcp_excalidraw && PORT=3001 node dist/server.js
