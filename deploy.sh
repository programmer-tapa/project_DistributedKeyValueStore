#!/usr/bin/env bash
# deploy.sh — Automation script to compile, build, and deploy the DKV cluster.

set -euo pipefail

# Formatting colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color
BLUE='\033[0;34m'
YELLOW='\033[0;33m'

echo -e "${BLUE}==================================================${NC}"
echo -e "${BLUE}        DKV Cluster Deployment Pipeline           ${NC}"
echo -e "${BLUE}==================================================${NC}"

# 1. Check Docker Daemon status
if ! docker info >/dev/null 2>&1; then
    echo -e "${RED}[ERROR] Docker daemon is not running. Please start Docker first.${NC}"
    exit 1
fi

# 2. Initialize environment file if missing
echo -e "\n${YELLOW}[1/5] Checking environment configuration...${NC}"
if [ ! -f .env ]; then
    echo -e "${YELLOW}No .env file found at root. Creating from .env.example...${NC}"
    if [ -f .env.example ]; then
        cp .env.example .env
        echo -e "${GREEN}✓ Created .env successfully.${NC}"
    else
        echo -e "${RED}[ERROR] .env.example is missing. Cannot initialize .env.${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✓ .env file exists.${NC}"
fi

# 3. Build Client CLI
echo -e "\n${YELLOW}[2/5] Compiling local client CLI...${NC}"
if cd DistributedKeyValueStore; then
    CGO_ENABLED=0 GOOS=linux go build -o bin/dkv-client ./cmd/dkv-client
    cd ..
    echo -e "${GREEN}✓ Client compiled successfully at DistributedKeyValueStore/bin/dkv-client${NC}"
else
    echo -e "${RED}[ERROR] Failed to navigate to DistributedKeyValueStore directory.${NC}"
    exit 1
fi

# 4. Stop existing services to avoid collisions
echo -e "\n${YELLOW}[3/5] Cleaning up any existing containers...${NC}"
docker compose down

# 5. Boot up Docker Compose
echo -e "\n${YELLOW}[4/5] Starting Docker Compose cluster...${NC}"
if docker compose up --build -d; then
    echo -e "${GREEN}✓ Containers built and started successfully.${NC}"
else
    echo -e "${RED}[ERROR] Failed to start Docker Compose containers.${NC}"
    exit 1
fi

# 6. Verify status
echo -e "\n${YELLOW}[5/5] Checking container status...${NC}"
docker compose ps

echo -e "\n${GREEN}==================================================${NC}"
echo -e "${GREEN}      DKV Cluster Deployed Successfully!         ${NC}"
echo -e "${GREEN}==================================================${NC}"
echo -e "Services are mapped to the following local ports:"
echo -e "  - ${BLUE}kvsrv (Metadata Config Store)${NC}  : localhost:9000"
echo -e "  - ${BLUE}shardctrler (Shard Controller)${NC}: localhost:9100"
echo -e "  - ${BLUE}Prometheus (Metrics Server)${NC}    : http://localhost:9090"
echo -e "  - ${BLUE}Grafana (Dashboards)${NC}           : http://localhost:3003 (User/Pass: admin/admin)"
echo -e "\nTo test the deployment, try writing and reading a key:"
echo -e "  ${BLUE}docker run --rm --network docker_kvnet -v \"\$PWD\"/DistributedKeyValueStore/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 put hello world${NC}"
echo -e "  ${BLUE}docker run --rm --network docker_kvnet -v \"\$PWD\"/DistributedKeyValueStore/bin/dkv-client:/usr/local/bin/dkv-client alpine:3.20 dkv-client --ctrler-addr kvsrv:9000 get hello${NC}"
echo -e "${BLUE}==================================================${NC}"
