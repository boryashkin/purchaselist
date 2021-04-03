package dialog

import (
	"github.com/boryashkin/purchaselist/db"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"strings"
)

const (
	ComStartBot         = "start"
	ComHelp             = "help"
	ComCreatePost       = "create"
	ComConfirm          = "ok"
	ComClear            = "clear"
	ComCancel           = "cancel"
	ComDone             = "Гoтовo"
	ComFinishedCrossout = "Нoвый списoк"
)

type MessageHandler struct {
	Bot                 *tgbotapi.BotAPI
	PurchaseListService *db.PurchaseListService
	commands            map[string]bool
}

func NewMessageHandler(api *tgbotapi.BotAPI, purchaseListService *db.PurchaseListService) MessageHandler {
	list := map[string]bool{
		ComStartBot:         true,
		ComHelp:             true,
		ComCreatePost:       true,
		ComConfirm:          true,
		ComClear:            true,
		ComCancel:           true,
		ComDone:             true,
		ComFinishedCrossout: true,
	}
	return MessageHandler{Bot: api, commands: list, PurchaseListService: purchaseListService}
}

func (h *MessageHandler) ReadMessage(message *tgbotapi.Message) MessageDto {
	m := MessageDto{UnknownContent: false, ID: message.MessageID, ChatID: message.Chat.ID}
	if message == nil {
		return m
	}
	if message.IsCommand() {
		m.Command = strings.ToLower(message.Command())
		if _, found := h.commands[m.Command]; !found {
			m.Command = ComHelp
		}
	} else if message.Text == ComDone || message.Text == ComFinishedCrossout {
		m.Command = ComConfirm
		m.Text = ""
	} else if message.Photo != nil && *message.Photo != nil && len(*message.Photo) > 0 {
		m.PhotoUrls = h.readPhoto(message.Photo)
	} else if message.Text != "" {
		m.Text = message.Text
	} else {
		m.UnknownContent = true
	}
	return m
}

func (h *MessageHandler) GetNewStateByMessage(message *MessageDto, dState *DialogState) *DialogState {
	currState := dState.Session.PostingState
	prevState := dState.Session.PreviousState
	newSessState := h.getNewSessionStateByCommand(message.Command, currState)
	log.Println("[STATES] ", prevState, currState, newSessState)

	if newSessState == currState {
		if currState == db.SessPStateDone {
			newSessState = db.SessPStateCreation
		}
	}
	if newSessState == db.SessPStateDone {
		purchaseList, err := h.PurchaseListService.CreateEmptyList(dState.Session.UserId)
		if err != nil {
			log.Println("failed to create p list", err)
			dState.Session.PurchaseListId = primitive.NilObjectID
		} else {
			dState.Session.PurchaseListId = purchaseList.Id
		}
	}
	dState.Session.PreviousState = currState
	dState.Session.PostingState = newSessState

	return dState
}

func (h *MessageHandler) getNewSessionStateByCommand(command string, currState db.SessState) db.SessState {
	switch command {
	case ComStartBot:
		if currState == db.SessPStateNew {
			return db.SessPStateCreation
		}
		break
	case ComConfirm:
		if currState == db.SessPStateCreation {
			return db.SessPStateCreation
		} else if currState == db.SessPStateDone {
			return db.SessPStateCreation
		} else if currState == db.SessPStateNew {
			return db.SessPStateCreation
		}
		break
	case ComCreatePost:
		if currState == db.SessPStateNew || currState == db.SessPStateRegistered {
			return db.SessPStateCreation
		}
	case ComClear:
		return db.SessPStateDone

	}

	return currState
}

type MessageForReply struct {
	NewMessage     bool
	DeletePrevious *bool
	Text           string
	InlineKeyboard *tgbotapi.InlineKeyboardMarkup
	AnswerCallback *tgbotapi.CallbackConfig
	ReplyKeyboard  *tgbotapi.ReplyKeyboardMarkup
	Markdown       *string
}

func (h *MessageHandler) GetMessageForReply(
	m *MessageDto,
	session *db.Session, user *db.User, purchaseList *db.PurchaseList,
) MessageForReply {
	defaultMkdwn := tgbotapi.ModeMarkdown + "V2"
	msg := MessageForReply{NewMessage: true, Markdown: &defaultMkdwn}
	if session == nil {
		msg.NewMessage = false
		msg = createMessageForPurchaseList(msg, purchaseList)
		return msg
	}
	if m.Command != "" {
		switch m.Command {
		case ComHelp:
			msg.Markdown = nil
			msg.Text = " Чтобы составить список, записывайте товары сюда\n" +
				" - Отдельными сообщениями\n" +
				" - Одним сообщением, каждый товар с новой строки\n" +
				" - Пересылайте сообщения из других чатов\n\n"
			return msg
		case ComClear:
			msg.Text = "Список закрыт\n"
			msg.NewMessage = true
			return msg
		}
	}
	switch session.PostingState {
	case db.SessPStateCreation:
		if m.Command != "" && session.PreviousState == db.SessPStateNew {
			msg.Markdown = nil
			msg.Text = "Приветствую! \n" +
				"Чтобы составить список, записывайте товары сюда\n" +
				" - Отдельными сообщениями\n" +
				" - Одним сообщением, каждый товар с новой строки\n" +
				" - Пересылайте сообщения из других чатов\n\n\n" +
				"Введите название товара или список"
		} else {
			dmsg := len(purchaseList.Items) > 0
			msg.DeletePrevious = &dmsg
			msg = createMessageForPurchaseList(msg, purchaseList)
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
			log.Println("[GMFR] 3")
			msg = createMessageForPurchaseList(msg, purchaseList)
		}
		break
	default:
		msg.Text = "Не знаю, что ответить"
	}

	return msg
}

func (h *MessageHandler) readPhoto(photo *[]tgbotapi.PhotoSize) []string {
	var urls []string
	for _, photo := range *photo {
		url, err := h.Bot.GetFileDirectURL(photo.FileID)
		if err != nil {
			log.Println(err)
		}
		urls = append(urls, url)
	}

	return urls
}

func createMessageForPurchaseList(msg MessageForReply, purchaseList *db.PurchaseList) MessageForReply {
	log.Println("createMessageForPurchaseList")
	rows := [][]tgbotapi.InlineKeyboardButton{}
	msg.Text = ""
	stylePre := ""
	stylePost := ""
	for _, key := range purchaseList.Items {
		keys := []tgbotapi.InlineKeyboardButton{}
		keyS := string(key)
		//todo: add hash
		keys = append(keys, tgbotapi.NewInlineKeyboardButtonData(keyS, purchaseList.Id.Hex()+":"+keyS))
		rows = append(rows, keys)
		msg.Text += stylePre + keyS + stylePost + "\n"
	}
	for _, key := range purchaseList.DeletedItems {
		stylePre = "✔ ~"
		stylePost = "~"
		keyS := string(key)
		msg.Text += stylePre + keyS + stylePost + "️\n"
	}
	if len(rows) > 0 {
		log.Println("[BUTT]1")
		keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
		msg.InlineKeyboard = &keyboard
	} else {
		log.Println("[BUTT]2")
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
