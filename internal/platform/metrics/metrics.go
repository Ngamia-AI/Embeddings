package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the Prometheus collectors described in the spec.
type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	Inflight        prometheus.Gauge
	QueueWait       *prometheus.HistogramVec
	EngineDuration  *prometheus.HistogramVec
	TokensTotal     *prometheus.CounterVec
	MediaBytes      prometheus.Gauge
}

// New registers the collectors in the default registry.
func New() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total data-plane requests by app, modality and status.",
		}, []string{"app", "modality", "status"}),
		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "End-to-end request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"modality"}),
		Inflight: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "inflight",
			Help: "Requests currently being served.",
		}),
		QueueWait: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "queue_wait_seconds",
			Help:    "Time spent waiting for an engine slot.",
			Buckets: []float64{.001, .005, .01, .05, .1, .5, 1, 2, 5, 10, 30},
		}, []string{"class"}),
		EngineDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "engine_duration_seconds",
			Help:    "Time spent inside the embedding engine.",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2, 5, 10, 30, 60, 120},
		}, []string{"modality"}),
		TokensTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "tokens_total",
			Help: "Prompt tokens billed per app.",
		}, []string{"app"}),
		MediaBytes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "media_bytes_inflight",
			Help: "Base64-decoded media bytes currently held in memory.",
		}),
	}
}
