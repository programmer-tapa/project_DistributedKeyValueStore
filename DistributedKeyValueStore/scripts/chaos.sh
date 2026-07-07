#!/usr/bin/env bash
# chaos.sh — Chaos engineering script for the DKV cluster.
#
# Simulates network partitions, leader kills, and random container pauses
# to test fault tolerance and linearizability of the system.

set -euo pipefail

COMPOSE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "Starting Chaos Injector against Docker Compose cluster..."

random_target() {
    local nodes=("kvraft-0" "kvraft-1" "kvraft-2" "shardgrp2-0" "shardgrp2-1" "shardgrp2-2")
    echo "${nodes[RANDOM % ${#nodes[@]}]}"
}

case "${1:-help}" in
    kill-leader)
        TARGET=$(random_target)
        echo "Killing random replica node: $TARGET"
        docker --compose-project-dir "$COMPOSE_DIR" compose kill "$TARGET"
        sleep 5
        echo "Starting $TARGET back up..."
        docker --compose-project-dir "$COMPOSE_DIR" compose start "$TARGET"
        ;;
    partition)
        echo "Creating a network partition separating dkv-kvraft-2..."
        docker network disconnect docker_kvnet dkv-kvraft-2
        sleep 10
        echo "Healing network partition..."
        docker network connect docker_kvnet dkv-kvraft-2
        ;;
    pause)
        TARGET=$(random_target)
        echo "Pausing container: $TARGET"
        docker --compose-project-dir "$COMPOSE_DIR" compose pause "$TARGET"
        sleep 7
        echo "Unpausing container: $TARGET"
        docker --compose-project-dir "$COMPOSE_DIR" compose unpause "$TARGET"
        ;;
    *)
        echo "Usage: $0 {kill-leader|partition|pause}"
        exit 1
        ;;
esac
