package queue

import (
	"github.com/boryashkin/purchaselist/db"
	"github.com/boryashkin/purchaselist/dialog"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
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
			sent, err := d.fn(bot, chatID, messageID, reply)
			if err == nil {
				msgID := db.TgMsgID{
					TgChatID:    sent.Chat.ID,
					TgMessageID: sent.MessageID,
				}
				d.pListService.AddMsgID(reply.PListID, msgID)
			}
		} else {
			log.Println("[queue] skip")
		}
	} else {
		d.fn(bot, chatID, messageID, reply)
	}
}
