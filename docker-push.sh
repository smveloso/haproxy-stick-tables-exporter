#!/bin/env bash
TAG=${1:-"latest"}
docker push TODO/haproxy-table-exporter:${TAG}
