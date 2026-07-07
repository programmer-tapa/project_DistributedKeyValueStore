#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# Set text colors for logging
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Locate the .env file in the same directory as the script
ENV_FILE="$(dirname "$0")/.env"

# 1. Load the .env file if it exists
if [ -f "$ENV_FILE" ]; then
    echo "=== Loading GCP configuration from $ENV_FILE ==="
    # Export variables starting with GCP_ to avoid polluting environment
    eval $(grep '^GCP_' "$ENV_FILE" | xargs)
else
    echo -e "${RED}Error: .env file not found at $ENV_FILE${NC}"
    exit 1
fi

# 2. Validate that the required GCP variables are set
if [ -z "$GCP_PROJECT_ID" ] || [ -z "$GCP_REGION" ] || [ -z "$GCP_REPO_NAME" ]; then
    echo -e "${RED}Error: Missing required GCP variables in .env${NC}"
    echo "Please ensure the following variables are defined in your .env file:"
    echo "  GCP_PROJECT_ID=<your-project-id>"
    echo "  GCP_REGION=<your-region>"
    echo "  GCP_REPO_NAME=<your-registry-repo-name>"
    exit 1
fi

echo -e "${GREEN}Configuration Loaded:${NC}"
echo "  Project ID:  $GCP_PROJECT_ID"
echo "  Region:      $GCP_REGION"
echo "  Repository:  $GCP_REPO_NAME"
echo "------------------------------------------------"

# 3. Authenticate Docker with the GCP Artifact Registry
echo -e "${GREEN}=== Authenticating Docker with Artifact Registry ===${NC}"
gcloud auth configure-docker "$GCP_REGION-docker.pkg.dev" --quiet

# Define image base registry path
REGISTRY_PATH="$GCP_REGION-docker.pkg.dev/$GCP_PROJECT_ID/$GCP_REPO_NAME"

# Function to build and push an image
build_and_push() {
    local target="$1"
    local image_tag="$REGISTRY_PATH/$target:latest"
    
    echo -e "\n${GREEN}=== Building $target ===${NC}"
    DOCKER_BUILDKIT=1 docker build -t "$image_tag" \
        --build-arg BUILD_TARGET="$target" \
        -f DistributedKeyValueStore/deployments/docker/Dockerfile DistributedKeyValueStore
        
    echo -e "${GREEN}=== Pushing $target to Artifact Registry ===${NC}"
    docker push "$image_tag"
    
    echo -e "${GREEN}=== Successfully deployed $target ===${NC}"
}

# 3. Build and push all DKV core components
build_and_push "kvsrvd"
build_and_push "kvraftd"
build_and_push "shardctrlrd"

echo -e "\n${GREEN}================================================${NC}"
echo -e "${GREEN}All DKV images built and pushed successfully!${NC}"
echo -e "${GREEN}================================================${NC}"
