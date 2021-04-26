package dialog

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

type MessageDto struct {
	ChatMsgID      ChatMessageID
	PhotoUrls      []string
	Command        string
	Text           string
	UnknownContent bool
	TgUser         *tgbotapi.User
	TgContact      *tgbotapi.Contact
}
