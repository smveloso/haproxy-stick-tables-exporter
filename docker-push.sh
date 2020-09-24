#!/bin/env bash
TAG=${1:-"latest"}
docker push smveloso/haproxy-stick-tables-exporter:${TAG}
