#!/bin/env bash
TAG=${1:-"latest"}
docker build -t TODO/haproxy-table-exporter:${TAG} .
