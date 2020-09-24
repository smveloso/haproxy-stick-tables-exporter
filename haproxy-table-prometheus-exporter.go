package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	_ "os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type tableMetrics struct {
	name                     string
	entries                  float64 // Gauge
	tcpConnRates             []float64
	tcpConnRatesInterval     string
	httpRequestRates         []float64
	httpRequestRatesInterval string
}

var (
	// https://github.com/prometheus/prometheus/wiki/Default-port-allocations
	addr     = flag.String("listen-address", ":9752", "Address and port to listen to.")
	socket   = flag.String("socket", "/var/haproxy/admin.sock", "Unix socket to reach haproxy.")
	interval = flag.Int64("interval", 5, "Sleep interval between haproxy data colletctions.")

	namespace = flag.String("namespace", "smveloso", "Namespace for prometheus metrics.")
	subsystem = flag.String("subsystem", "haproxy", "Subsystem for prometheus metrics.")

	entries     *prometheus.GaugeVec
	tcpConnRate *prometheus.HistogramVec
	httpReqRate *prometheus.HistogramVec

	tableMetricsSlice = make([]tableMetrics, 0, 8)
)

func register() {

	// numero de entradas em uma tabela stick-table
	entries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: *namespace,
		Subsystem: *subsystem,
		Name:      "sticktable_curr_entries",
		Help:      "Number of entries in the stick-table.",
	},
		[]string{"table"},
	)

	tcpConnRate = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: *namespace,
		Subsystem: *subsystem,
		Name:      "sticktable_tcp_conn_rate",
		Help:      "TCP connection rates.",
		Buckets:   prometheus.LinearBuckets(0, 10, 40), // TODO parametrizar os buckets
	},
		[]string{"table", "interval"},
	)

	httpReqRate = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: *namespace,
		Subsystem: *subsystem,
		Name:      "sticktable_http_req_rate",
		Help:      "HTTP request rates.",
		Buckets:   prometheus.LinearBuckets(0, 10, 40), // TODO parametrizar os buckets
	},
		[]string{"table", "interval"},
	)

	prometheus.MustRegister(entries)
	prometheus.MustRegister(tcpConnRate)
	prometheus.MustRegister(httpReqRate)
}

func connectToHaproxy() (net.Conn, error) {
	fmt.Println(">> connectToHaproxy")
	conn, err := net.Dial("unix", *socket)
	if err != nil {
		fmt.Println("Error connecting to haproxy: ", err)
	}
	return conn, err
}

func askHaproxy(question string) (string, error) {
	fmt.Println(">>> askHaproxy")

	var answer string

	connection, err := connectToHaproxy()

	if err != nil {
		return answer, err
	}

	defer connection.Close()

	err = writeToSocket(connection, question)

	if err != nil {
		return answer, err
	}

	answer, err = readFromSocket(connection)

	if err != nil {
		return answer, err
	}

	return answer, err

}

// will append a line feed (0x0A) to the txt
func writeToSocket(connection net.Conn, txt string) error {
	fmt.Println(">>> writeToSocket")
	fmt.Println("writing: ", txt)
	txt = txt + "\n"
	bytesWritten, err := connection.Write([]byte(txt))
	fmt.Printf("wrote %d bytes\n", bytesWritten)
	fmt.Printf("error ? %t\n", err != nil)
	return err
}

func readFromSocket(connection net.Conn) (string, error) {
	fmt.Println(">>> readFromSocket")

	var buffer bytes.Buffer
	bytesCopied, err := io.Copy(&buffer, connection)

	// https://golang.org/pkg/io/#Copy
	//
	// A successful Copy returns err == nil, not err == EOF.
	// Because Copy is defined to read from src until EOF, it does not treat an EOF from Read as an error to be reported.
	//

	fmt.Printf("read %d bytes\n", bytesCopied)

	if err == nil {
		txt := buffer.String()
		return txt, nil
	} else {
		return "", err
	}
}

/*
# table: monitoring, type: ip, size:32, used:0
# table: fe_https, type: ip, size:1048576, used:0
# table: fe_http, type: ip, size:1048576, used:0
*/
func getTablesFromFromAnswer(rawtext string) []string {
	fmt.Println(">>> getTablesFromShowTable")

	const prefix string = "# table: "
	var tableNames []string

	for _, line := range strings.Split(rawtext, "\n") {
		if strings.HasPrefix(line, prefix) {
			tableNames = append(tableNames, line[len(prefix):strings.Index(line, ",")])
		}
	}

	return tableNames
}

/*

Pega isso:
0x564104c72d60: key=192.168.128.1 use=0 exp=8183 gpc0=0 conn_rate(3000)=40 conn_cur=0 http_req_rate(10000)=19

E gera um map para facilitar a consulta.

Chaves que contenham intervalos entre parênteses resultam em duas chaves; exemplo:

some_metric(iiii)=nnnn
===> some_metric=nnnn
===> some_metric_interval=iiii

Você é responsável por converter as strings para o que quiser.

*/
func lineToMap(line string) map[string]string {

	m := make(map[string]string)

	for _, kvPair := range strings.Split(line[strings.Index(line, "key"):], " ") {
		fmt.Println("kvPair: ", kvPair)

		var key, value string

		tmp := strings.Split(kvPair, "=")
		value = tmp[1]

		idxAbrePar := strings.Index(tmp[0], "(")
		idxFechaPar := strings.Index(tmp[0], ")")
		if idxAbrePar == -1 {
			key = tmp[0]
		} else {
			key = tmp[0][:idxAbrePar]
			keyInterval := key + "_interval"
			valueInterval := tmp[0][(idxAbrePar + 1):idxFechaPar]
			m[keyInterval] = valueInterval
		}

		m[key] = value
	}

	for k, v := range m {
		fmt.Printf("TRACE> |%s| : |%s|\n", k, v)
	}

	return m

}

/*

Pega isso:
# table: fe_http, type: ip, size:1048576, used:1

E gera um mapa para facilitar a consulta.

Você é responsável por converter as strings para o que quiser.

*/
func firstLineToMap(firstLine string) map[string]string {

	m := make(map[string]string)

	for _, kvPair := range strings.Split(firstLine[strings.Index(firstLine, "#")+1:], ",") {
		fmt.Println("F ======>: ", kvPair)
		tmp := strings.Split(kvPair, ":")
		k := strings.TrimSpace(tmp[0])
		v := strings.TrimSpace(tmp[1])
		m[k] = v
		fmt.Printf("FK ======>: |%s| : |%s|\n", k, v)
	}

	return m

}

/*

Formato depende da configuração do haproxy (mas está hard-coded nesta versão).

TODO: map associando nome da stick-table ao formato/configuração no haproxy

# table: fe_http, type: ip, size:1048576, used:1
0x564104c72d60: key=192.168.128.1 use=0 exp=8183 gpc0=0 conn_rate(3000)=40 conn_cur=0 http_req_rate(10000)=19

*/
func getTableMetricsFromAnswer(tableName string, rawtext string) tableMetrics {
	fmt.Println(">>> getTableMetricsFromShowTableName")

	tm := tableMetrics{name: tableName}

	// TODO métrica (gauge) para capacidade da stick-table (via primeira linha 'size:n')
	// TODO métrica (? ou label ?) para tipo da stick-table (via primeira linha 'type: ip')

	// TODO esse split provavelmente não escala. se houver 1 milhão de entradas ?
	lines := strings.Split(rawtext, "\n")

	// TODO conferir se used:n bate com o comprimento de 'lines' (desconsiderando header e empty lines, claro)

	firstLineMap := firstLineToMap(lines[0])

	// sanidade:  # table: xxxx (...)
	actualTableName := firstLineMap["table"]
	if actualTableName != tableName {
		fmt.Printf("SANITY ERROR: %s != %s\n", tableName, actualTableName)
		return tm
	}

	tm.entries, _ = strconv.ParseFloat(firstLineMap["used"], 64) // TODO considerar erros de conversão.

	for _, line := range lines[1:] { // pula a primeira linha
		if len(strings.TrimSpace(line)) == 0 { // ignora a linha em branco ao final
			break
		}
		lineMap := lineToMap(line)

		connRate, _ := strconv.ParseFloat(lineMap["conn_rate"], 64) // TODO considerar erros de conversão
		tm.tcpConnRates = append(tm.tcpConnRates, connRate)
		if tm.tcpConnRatesInterval == "" {
			tm.tcpConnRatesInterval = lineMap["conn_rate_interval"]
		}

		httpReqRate, _ := strconv.ParseFloat(lineMap["http_req_rate"], 64) // TODO considerar erros de conversão
		tm.httpRequestRates = append(tm.httpRequestRates, httpReqRate)
		if tm.httpRequestRatesInterval == "" {
			tm.httpRequestRatesInterval = lineMap["http_req_rate_interval"]
		}

	}

	return tm
}

// colateral
func collect() error {
	fmt.Println(">> collect")
	tableMetricsSlice = tableMetricsSlice[:0] // ZERANDO DADOS CORRENTES
	fmt.Println("gathering tables")

	var answer string
	var err error

	answer, err = askHaproxy("show table")

	if err != nil {
		return err
	}

	fmt.Printf("\n<ANSWER>%s\n</ANSWER>\n", answer)

	for _, tableName := range getTablesFromFromAnswer(answer) {

		fmt.Println("found table: ", tableName)
		answer, err = askHaproxy("show table " + tableName)

		if err != nil {
			return err
		}

		fmt.Printf("\n<ANSWER>%s\n</ANSWER>\n", answer)
		tableMetricsSlice = append(tableMetricsSlice, getTableMetricsFromAnswer(tableName, answer))

	} // iteration on slice of tables

	return nil
}

func updateMetrics() {
	fmt.Println(">> updateMetrics")
	for {
		fmt.Println("update metrics iteration")

		collectError := collect()
		if collectError == nil {
			// aproveita o colateral
			fmt.Println("updateMetrics: got ", len(tableMetricsSlice), " ", cap(tableMetricsSlice))
			for _, tm := range tableMetricsSlice {
				entries.With(prometheus.Labels{"table": tm.name}).Set(tm.entries)
				// TODO alguma maneira de iterar apenas uma vez, dado que o nro de entradas é igual ?
				for _, cr := range tm.tcpConnRates {
					tcpConnRate.With(prometheus.Labels{"table": tm.name, "interval": tm.tcpConnRatesInterval}).Observe(cr)
				}
				for _, hr := range tm.httpRequestRates {
					httpReqRate.With(prometheus.Labels{"table": tm.name, "interval": tm.httpRequestRatesInterval}).Observe(hr)
				}
			}
		} else {
			fmt.Println("error [collect]. not updating. desc= ", collectError)
		}

		time.Sleep(time.Duration(*interval) * time.Second)
	}
}

func main() {
	fmt.Println(">>>> main")
	flag.Parse()
	register()
	go updateMetrics()
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
	fmt.Println("<<< main")
}
