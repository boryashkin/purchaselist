package dialog

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
)

type BotReply func(bot *tgbotapi.BotAPI, chatID int64, messageID int, forReply MessageForReply) (*tgbotapi.Message, error)

func Reply(bot *tgbotapi.BotAPI, chatID int64, messageID int, forReply MessageForReply) (*tgbotapi.Message, error) {
	if bot == nil {
		log.Println("[No bot] ", forReply.Text)
		return nil, errors.New("No bot")
	}

	var msg tgbotapi.Chattable
	if forReply.NewMessage {
		if forReply.Text == "" {
			log.Println("not sent")
			return nil, errors.New("Not sent")
		}
		msgNew := tgbotapi.NewMessage(chatID, forReply.Text)
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
		log.Println("EditMessage")
		msgEdit := tgbotapi.NewEditMessageText(chatID, messageID, forReply.Text)
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
			log.Println("err while answering CallbackQuery " + err.Error())
		}
	}

	sent, err := bot.Send(msg)
	if err != nil {
		log.Println("err while sending " + err.Error())
		msgNew := tgbotapi.NewMessage(chatID, "Произошла ошибка при отправке")
		_, _ = bot.Send(msgNew)
	} else {
		log.Println("bot.Send() ok")
	}
	return &sent, err
}
