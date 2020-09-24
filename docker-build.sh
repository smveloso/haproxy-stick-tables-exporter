#!/bin/env bash
TAG=${1:-"latest"}
docker build -t smveloso/haproxy-stick-tables-exporter:${TAG} .
