package dialog

import (
	"github.com/boryashkin/purchaselist/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
)

const (
	ComStartBot         = "start"
	ComStartHelp        = "help"
	ComCreatePost       = "create"
	ComConfirm          = "ok"
	ComCancel           = "cancel"
	ComDone             = "Гoтовo"
	ComFinishedCrossout = "Нoвый списoк"
)

type Dialog struct {
	Bot *tgbotapi.BotAPI
}

func (c *Dialog) ReadMessage(message *tgbotapi.Message) MessageDto {
	m := MessageDto{UnknownContent: false, ID: message.MessageID, ChatID: message.Chat.ID}
	if message == nil {
		return m
	}
	if message.IsCommand() {
		m.Command = strings.ToLower(message.Command())
		switch m.Command {
		case "ok":
			m.Command = ComConfirm
			break
		case "create":
			m.Command = ComCreatePost
			break
		default:
			m.Command = ComConfirm
			break
		}
	} else if message.Text == ComDone || message.Text == ComFinishedCrossout {
		m.Command = ComConfirm
		m.Text = ""
	} else if message.Photo != nil && *message.Photo != nil && len(*message.Photo) > 0 {
		m.PhotoUrls = c.readPhoto(message.Photo)
	} else if message.Text != "" {
		m.Text = message.Text
	} else {
		m.UnknownContent = true
	}
	return m
}

func (c *Dialog) GetNewStateByMessage(message *MessageDto, session *db.Session) db.SessState {
	switch message.Command {
	case ComStartBot:
		if session.PostingState == db.SessPStateNew {
			return db.SessPStateCreation
		}
		break
	case ComConfirm:
		if session.PostingState == db.SessPStateCreation {
			return db.SessPStateDone
		} else if session.PostingState == db.SessPStateDone {
			return db.SessPStateCreation
		} else if session.PostingState == db.SessPStateNew {
			return db.SessPStateCreation
		}
		break
	case ComCreatePost:
		if session.PostingState == db.SessPStateNew || session.PostingState == db.SessPStateRegistered {
			return db.SessPStateCreation
		}
	case ComCancel:
		return db.SessPStateCreation

	}

	return session.PostingState
}

type MessageForReply struct {
	NewMessage     bool
	Text           string
	InlineKeyboard *tgbotapi.InlineKeyboardMarkup
	AnswerCallback *tgbotapi.CallbackConfig
	ReplyKeyboard  *tgbotapi.ReplyKeyboardMarkup
}

func (c *Dialog) GetMessageForReply(
	m *MessageDto, session *db.Session,
	user *db.User,
	purchaseList *db.PurchaseList,
) MessageForReply {
	msg := MessageForReply{NewMessage: true}
	if session == nil {
		msg.NewMessage = false
		msg = createMessageForPurchaseList(msg, purchaseList)
		return msg
	}
	switch session.PostingState {
	case db.SessPStateRegistered:
		msg.Text = "Приветствуем, " + user.Name + "! \n" +
			" Чтобы составить список, записывайте товары сюда\n" +
			" - Отдельными сообщениями\n" +
			" - Одним сообщением, каждый товар с новой строки\n" +
			" - Пересылайте сообщения из других чатов\n"
		break
	case db.SessPStateCreation:
		if m.Text == "" {
			msg.Text = "Введите название товара или список"
			bb := []tgbotapi.KeyboardButton{
				tgbotapi.NewKeyboardButton(ComDone),
			}
			kb := tgbotapi.NewReplyKeyboard(bb)
			msg.ReplyKeyboard = &kb
		} else {
			msg.Text = ""
		}
		break
	case db.SessPStateInProgress:
		if m.Text == "" {
			msg.Text = "Добавьте название товара или список, если нужно"
		} else {
			msg.Text = "Введите /ok, чтобы потдвердить"
		}
		break
	case db.SessPStateDone:
		if m.Text == "" {
			msg = createMessageForPurchaseList(msg, purchaseList)
		}
		break
	default:
		if m.Command == ComConfirm {
			if session.PreviousState == db.SessPStateDone && session.PostingState != db.SessPStateRegistered {
				msg.Text = "Поздравляем"
				return msg
			}
		} else if m.Command == ComStartHelp {
			msg.Text = "Инструкция по боту:\n" +
				"/ok - подтвердить шаг при изменениях"
			return msg
		} else if m.Command == ComCancel {
			msg.Text = "Вы отменили свои действия"
			return msg
		}
		msg.Text = "Введите название товара"
	}

	return msg
}

func (c *Dialog) readPhoto(photo *[]tgbotapi.PhotoSize) []string {
	var urls []string
	for _, photo := range *photo {
		url, err := c.Bot.GetFileDirectURL(photo.FileID)
		if err != nil {
			log.Println(err)
		}
		urls = append(urls, url)
	}

	return urls
}

func createMessageForPurchaseList(msg MessageForReply, purchaseList *db.PurchaseList) MessageForReply {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	msg.Text = ""
	for _, key := range purchaseList.Items {
		keys := []tgbotapi.InlineKeyboardButton{}
		stylePre := ""
		stylePost := ""
		if key.State == db.PiStateBought {
			stylePre = "~"
			stylePost = "~"
		} else {
			keys = append(keys, tgbotapi.NewInlineKeyboardButtonData(key.Name, purchaseList.Id.Hex()+":"+key.Hash))
			rows = append(rows, keys)

		}
		msg.Text += stylePre + key.Name + stylePost + "\n"
	}
	log.Println("[kb] ROWS len", len(rows))
	if len(rows) > 0 {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg.InlineKeyboard = &keyboard
	} else {
		if msg.NewMessage != true { //hack to fix unremovable inline button for items like ~~2~~
			msg.NewMessage = false
		}
		keys := []tgbotapi.InlineKeyboardButton{}
		keys = append(keys, tgbotapi.NewInlineKeyboardButtonData(ComFinishedCrossout, purchaseList.Id.Hex()+":"+ComFinishedCrossout))
		keyboard := tgbotapi.NewInlineKeyboardMarkup(keys)
		msg.InlineKeyboard = &keyboard
	}

	return msg
}
