package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	QueueExecItem = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_exec_item",
			Help: "Queue exec item",
		},
		[]string{"action"},
	)
)

func InitDbQueue() {
	prometheus.MustRegister(QueueExecItem)
}
