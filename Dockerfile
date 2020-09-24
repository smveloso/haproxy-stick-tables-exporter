FROM golang:1.14.6-stretch
RUN mkdir -p /go/src/app
WORKDIR /go/src/app
# todo: instalar os modulos e deps e somente depois fazer o build do meu fonte
COPY haproxy-table-prometheus-exporter.go .
COPY go.mod .
COPY go.sum .
RUN go install -v
FROM debian:stretch
WORKDIR .
COPY --from=0 /go/bin/haproxy-table-prometheus-exporter.go .
ENTRYPOINT ["./haproxy-table-prometheus-exporter.go"]

