package queue

import (
	"github.com/boryashkin/purchaselist/db"
	"github.com/boryashkin/purchaselist/dialog"
	"github.com/boryashkin/purchaselist/metrics"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"
)

type DelayMessage struct {
	messages     map[primitive.ObjectID]int
	fn           dialog.BotReply
	pListService *db.PurchaseListService
}

func NewDelayMessage(fn dialog.BotReply, pListService *db.PurchaseListService) DelayMessage {
	return DelayMessage{
		messages:     make(map[primitive.ObjectID]int),
		fn:           fn,
		pListService: pListService,
	}
}

func (d *DelayMessage) SetLastDate(id primitive.ObjectID, random int) {
	d.messages[id] = random
}

func (d *DelayMessage) ExecItem(bot *tgbotapi.BotAPI, chatID int64, messageID int, reply dialog.MessageForReply) {
	if reply.CreatedAt != nil {
		time.Sleep(time.Millisecond * 500)
		if reply.Rand == d.messages[reply.PListID] {
			metrics.QueueExecItem.With(prometheus.Labels{"action": "exec_delayed"}).Inc()
			sent, err := d.fn(bot, chatID, messageID, reply)
			if err == nil {
				msgID := db.TgMsgID{
					TgChatID:    sent.Chat.ID,
					TgMessageID: sent.MessageID,
				}
				d.pListService.AddMsgID(reply.PListID, msgID)
			}
		} else {
			metrics.QueueExecItem.With(prometheus.Labels{"action": "skip"}).Inc()
			log.Println("[queue] skip")
		}
	} else {
		metrics.QueueExecItem.With(prometheus.Labels{"action": "no_delay"}).Inc()
		d.fn(bot, chatID, messageID, reply)
	}
}
