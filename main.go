package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	MetricsNamespace = "cloudnative"
)

var (
	functionLatency = CreateExecutionTimeMetric(MetricsNamespace,
		"Time spent.")
)

// NewExecutionTimer provides a timer for Updater's RunOnce execution
func NewTimer() *ExecutionTimer {
	return NewExecutionTimer(functionLatency)
}

func NewExecutionTimer(histo *prometheus.HistogramVec) *ExecutionTimer {
	now := time.Now()
	return &ExecutionTimer{
		histo: histo,
		start: now,
		last:  now,
	}
}

// ObserveTotal measures the execution time from the creation of the ExecutionTimer
func (t *ExecutionTimer) ObserveTotal() {
	(*t.histo).WithLabelValues("total").Observe(time.Now().Sub(t.start).Seconds())
}

// CreateExecutionTimeMetric prepares a new histogram labeled with execution step
func CreateExecutionTimeMetric(namespace string, help string) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "execution_latency_seconds",
			Help:      help,
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 15),
		}, []string{"step"},
	)
}

// ExecutionTimer measures execution time of a computation, split into major steps
// usual usage pattern is: timer := NewExecutionTimer(...) ; compute ; timer.ObserveStep() ; ... ; timer.ObserveTotal()
type ExecutionTimer struct {
	histo *prometheus.HistogramVec
	start time.Time
	last  time.Time
}

func Register() {
	err := prometheus.Register(functionLatency)
	if err != nil {
		fmt.Println(err)
	}
}

func GracefullExit() {
	fmt.Println("Start Exit...")
	fmt.Println("Execute Clean...")
	fmt.Println("End Exit...")
	os.Exit(0)
}

func Msg(w http.ResponseWriter, r *http.Request) {

	for k, v := range r.Header {
		for _, value := range v {
			w.Header().Set(k, value)
		}
	}

	timer := NewTimer()
	defer timer.ObserveTotal()

	w.WriteHeader(http.StatusOK)
	fmt.Println("Client IP:", r.Host)
	fmt.Println("Return Code:", http.StatusOK)
	len := r.ContentLength
	fmt.Println(len)
	msg := make([]byte, len)
	r.Body.Read(msg)
	fmt.Println(msg)
	s, _ := ioutil.ReadAll(r.Body)
	fmt.Println(s)
	w.Write(msg)
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	io.WriteString(w, "ok\n")
}

func main() {
	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/msg", Msg)
	http.Handle("/metrics", promhttp.Handler())

	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer close(sigs)
	go func() {
		for s := range sigs {
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				fmt.Println("Program Exit...", s)
				GracefullExit()
			default:
				fmt.Println("other signal", s)
			}
		}
	}()

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		log.Fatal(err)
	}
}
