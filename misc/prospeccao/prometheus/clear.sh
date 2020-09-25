#/usr/bin/env bash
curl -v -XPOST -g 'http://localhost:9090/api/v1/admin/tsdb/delete_series?match[]={foo=""}'
curl -v -XPOST http://localhost:9090/api/v1/admin/tsdb/clean_tombstones
