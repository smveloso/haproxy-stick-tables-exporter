#!/bin/env sh
go run haproxy-table-prometheus-exporter.go --socket $PWD/misc/prospeccao/haproxy/socket/admin.sock
