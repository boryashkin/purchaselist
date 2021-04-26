package dialog

import (
	"errors"
	"github.com/boryashkin/purchaselist/metrics"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/prometheus/client_golang/prometheus"
	"log"
)

type ChatMessageID struct {
	ChatID          *int64
	MessageID       *int
	InlineMessageID *string
}

type BotReply func(bot *tgbotapi.BotAPI, chatMsgID ChatMessageID, forReply MessageForReply) (*tgbotapi.Message, error)

func Reply(bot *tgbotapi.BotAPI, chatMsgID ChatMessageID, forReply MessageForReply) (*tgbotapi.Message, error) {
	if bot == nil {
		log.Println("[No bot] ", forReply.Text)
		return nil, errors.New("No bot")
	}

	var msg tgbotapi.Chattable
	msgLabel := "empty"
	if forReply.NewMessage {
		msgLabel = "new"
		if forReply.Text == "" {
			log.Println("not sent")
			return nil, errors.New("Not sent")
		}
		msgNew := tgbotapi.NewMessage(*chatMsgID.ChatID, forReply.Text)
		//msgNew.ReplyToMessageID = messageID
		if forReply.Markdown != nil {
			msgNew.ParseMode = *forReply.Markdown
		}
		if forReply.InlineKeyboard != nil {
			msgNew.ReplyMarkup = forReply.InlineKeyboard
		}
		if forReply.ReplyKeyboard != nil {
			msgNew.ReplyMarkup = forReply.ReplyKeyboard
		}
		msg = msgNew
	} else {
		msgLabel = "edit"
		log.Println("EditMessage")
		var msgEdit tgbotapi.EditMessageTextConfig
		if chatMsgID.InlineMessageID != nil {
			msgEdit = tgbotapi.EditMessageTextConfig{
				BaseEdit: tgbotapi.BaseEdit{
					InlineMessageID: *chatMsgID.InlineMessageID,
				},
				Text: forReply.Text,
			}
		} else {
			msgEdit = tgbotapi.NewEditMessageText(*chatMsgID.ChatID, *chatMsgID.MessageID, forReply.Text)
		}
		if forReply.Markdown != nil {
			msgEdit.ParseMode = *forReply.Markdown
		}

		if forReply.InlineKeyboard != nil {
			msgEdit.ReplyMarkup = forReply.InlineKeyboard
		}
		msg = msgEdit
	}
	if forReply.AnswerCallback != nil {
		_, err := bot.AnswerCallbackQuery(*forReply.AnswerCallback)
		if err != nil {
			metrics.TgCbAnswer.With(prometheus.Labels{"result": "error"}).Inc()
			log.Println("err while answering CallbackQuery " + err.Error())
		} else {
			metrics.TgCbAnswer.With(prometheus.Labels{"result": "success"}).Inc()
		}
	}

	sent, err := bot.Send(msg)
	if err != nil {
		metrics.TgMsgSent.With(prometheus.Labels{"result": "error", "msg_type": msgLabel}).Inc()
		log.Println("err while sending " + err.Error())
		msgNew := tgbotapi.NewMessage(*chatMsgID.ChatID, "Произошла ошибка при отправке. Попробуйте ещё раз или нажмите /clear")
		_, err = bot.Send(msgNew)
		if err != nil {
			metrics.TgMsgRetrySent.With(prometheus.Labels{"result": "error", "msg_type": msgLabel}).Inc()
		} else {
			metrics.TgMsgRetrySent.With(prometheus.Labels{"result": "success", "msg_type": msgLabel}).Inc()
		}
	} else {
		metrics.TgMsgSent.With(prometheus.Labels{"result": "success", "msg_type": msgLabel}).Inc()
		log.Println("bot.Send() ok")
	}
	return &sent, err
}
