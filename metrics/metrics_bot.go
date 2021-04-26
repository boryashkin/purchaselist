package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TgMsgSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_tg_msg_sent",
			Help: "The total number of telegram messages sent",
		},
		[]string{"result", "msg_type"},
	)
	TgMsgRetrySent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_tg_msg_retry_sent",
			Help: "The total number of telegram messages sent on retry",
		},
		[]string{"result", "msg_type"},
	)
	TgCbAnswer = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_tg_cb_answer",
			Help: "The total number of telegram callback answers",
		},
		[]string{"result"},
	)
	TgCbInlineAnswer = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bot_tg_cb_inline_answer",
			Help: "The total number of telegram inline callback answers",
		},
		[]string{"result"},
	)
)

func InitBotMetrics() {
	prometheus.MustRegister(TgMsgSent)
	prometheus.MustRegister(TgMsgRetrySent)
	prometheus.MustRegister(TgCbAnswer)
	prometheus.MustRegister(TgCbInlineAnswer)
}
