#!/bin/bash

# Build script for frontend assets
set -e

echo "Building frontend assets..."

# Check if we can build with webpack or need to use existing assets
if command -v node >/dev/null 2>&1; then
    # Change to app directory
    cd app

    # Install dependencies if node_modules doesn't exist
    if [ ! -d "node_modules" ]; then
        echo "Installing npm dependencies..."
        npm install
    fi

    echo "Building with webpack..."
    npm run build
    echo "Webpack built assets to ../assets/static/build/"

    cd ..
else
    echo "Node.js not found, using fallback assets..."
fi

# Ensure we have the basic static files
echo "Ensuring static files are available..."
mkdir -p assets/static/build

# Copy other static files if they exist
if [ -f "assets/static/favicon.ico" ]; then
    echo "favicon.ico already exists in assets/static/"
fi

if [ -f "assets/static/sharetechmono.woff2" ]; then
    echo "sharetechmono.woff2 already exists in assets/static/"
fi

if ! ls assets/static/build/app*.js >/dev/null 2>&1; then
    echo "No frontend bundle found in assets/static/build/" >&2
    exit 1
fi

echo "Frontend assets prepared successfully!"
