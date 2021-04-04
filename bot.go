package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/boryashkin/purchaselist/db"
	"github.com/boryashkin/purchaselist/dialog"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/writeas/go-strip-markdown"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// struct to test command handling
type MessageEnvelope struct {
	Text      string
	IsCommand bool
	Update    *tgbotapi.Update
}

const (
	DbName      = "purchaselist"
	ColUsers    = "users"
	ColSessions = "sessions"
	ColProducts = "purchaseLists"

	MaxCountOfItemsInList = 50
)

var (
	client        *mongo.Client
	users         *mongo.Collection
	sessions      *mongo.Collection
	purchaseLists *mongo.Collection
	bot           *tgbotapi.BotAPI

	userService         db.UserService
	sessionService      db.SessionService
	purchaseListService db.PurchaseListService
	testMemoryMap       map[string][]string
)

func generateStdinUpdates(ch chan *MessageEnvelope) {
	wg := sync.WaitGroup{}
	for {
		wg.Add(1)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter text: ")
		envelope := MessageEnvelope{}
		text, _ := reader.ReadString('\n')

		envelope.Text = strings.Trim(text, "\n")
		envelope.IsCommand = strings.Index(text, "/") == 0
		if text == "" {
			time.Sleep(3 * time.Second)
			wg.Done()
			continue
		}
		ch <- &envelope
		wg.Done()
	}
}

// Handle them via
// go generateSingleThreadedTgUpdates(ch)
//	for {
//		select {
//		case envelope := <-ch:
//			go handleAsync(envelope)
//		}
//	}
func generateSingleThreadedTgUpdates(ch chan *MessageEnvelope) {
	tgtoken := os.Getenv("TGTOKEN")

	bot1, err := tgbotapi.NewBotAPI(tgtoken)
	if err != nil {
		log.Panic(err)
	}
	bot = bot1
	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}
	for update := range updates {
		log.Println("Received update: ", update)
		envelope := MessageEnvelope{}
		if update.Message != nil {
			envelope.Text = update.Message.Text
		}
		envelope.Update = &update
		ch <- &envelope
	}
}
func generateTgUpdates() *tgbotapi.UpdatesChannel {
	tgtoken := os.Getenv("TGTOKEN")

	bot1, err := tgbotapi.NewBotAPI(tgtoken)
	if err != nil {
		log.Panic(err)
	}
	bot = bot1
	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}
	return &updates
}

func main() {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://" + os.Getenv("MONGODB") + ":" + os.Getenv("MONGOPORT")))
	if err != nil {
		log.Println("Mongo instantiation err", err)
	}
	dbCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = client.Connect(dbCtx)
	if err != nil {
		log.Println("Mongo connection err", err)
	}
	users = client.Database(DbName).Collection(ColUsers)
	sessions = client.Database(DbName).Collection(ColSessions)
	purchaseLists = client.Database(DbName).Collection(ColProducts)
	userService = db.NewUserService(users)
	sessionService = db.NewSessionService(sessions)
	purchaseListService = db.NewPurchaseListService(purchaseLists)
	testMemoryMap = map[string][]string{
		"": {""},
	}
	//ch := make(chan *MessageEnvelope)
	//go generateStdinUpdates(ch)
	//go generateSingleThreadedTgUpdates(ch)//side effect: duplicate messages on race conditions
	updates := generateTgUpdates()
	for update := range *updates {
		update := update // trying to make a local copy to prevent races
		envelope := MessageEnvelope{}
		if update.Message != nil {
			envelope.Text = update.Message.Text
		} else {
			envelope.Text = ""
		}
		envelope.Update = &update
		go handleAsync(&envelope)
	}

	//for {
	//	select {
	//	case envelope := <-ch:
	//		go handleAsync(envelope)
	//	}
	//}
}

func handleAsync(envelope *MessageEnvelope) {
	var chatID int64
	var msgID int
	if envelope.Update == nil {
		// tests
		createFakeUpdate(envelope)
	}
	var m dialog.MessageDto
	var message *tgbotapi.Message
	var err error
	var msg dialog.MessageForReply
	var dState *dialog.DialogState
	update := envelope.Update

	c := dialog.NewMessageHandler(bot, &purchaseListService)

	if update.CallbackQuery != nil {
		chatID, msgID = getCallbackChatId(envelope)
		msg = readCallbackQuery(update.CallbackQuery, &c)
		reply(chatID, msgID, msg)
		return
	} else if update.Message != nil {
		message = update.Message
		chatID, msgID = getMessageChatId(envelope)
		log.Printf("[RECEIVED][%d] %s", message.From.ID, message.Text)

		m = c.ReadMessage(message)
		dState, err = createDialogStateFromMessage(&m, message)
		if err != nil {
			reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
			return
		}

	} else {
		log.Println("[noop] Unknown request")
		return
	}

	prevMsgID := dState.PurchaseList.TgMsgID
	st := c.GetNewStateByMessage(&m, dState)
	err = updateSession(st.Session)
	if err != nil {
		reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
		return
	}
	msg = c.GetMessageForReply(&m, dState.Session, dState.User, dState.PurchaseList)
	if msg.DeletePrevious != nil && *msg.DeletePrevious == true {
		deleteMessage(dState.Session.PurchaseListId, prevMsgID)
	}
	sent, err := reply(chatID, msgID, msg)
	if err == nil {
		msgID := db.TgMsgID{
			TgChatID:    sent.Chat.ID,
			TgMessageID: sent.MessageID,
		}
		purchaseListService.AddMsgID(dState.PurchaseList.Id, msgID)
	}
}

func createDialogStateFromMessage(m *dialog.MessageDto, message *tgbotapi.Message) (*dialog.DialogState, error) {
	user, err := getOrRegisterUser(message)
	if err != nil {
		return nil, err
	}
	session, err := getOrCreateSession(user)
	if err != nil {
		return nil, err
	}

	purchaseList, err := createOrUpdateList(m, session)
	if err != nil {
		return nil, err
	}
	session.PurchaseListId = purchaseList.Id
	dState := dialog.DialogState{
		User:         user,
		Session:      session,
		PurchaseList: purchaseList,
	}
	return &dState, nil
}

func getOrRegisterUser(message *tgbotapi.Message) (*db.User, error) {
	phone := ""
	if message.Contact != nil {
		phone = message.Contact.PhoneNumber
	}
	user := db.User{
		TgId:  message.From.ID,
		Name:  message.From.FirstName,
		Phone: phone,
		Lang:  message.From.LanguageCode,
	}
	err := userService.Upsert(&user)
	if err != nil {
		user, err = userService.FindByTgID(message.From.ID)
	}

	return &user, err
}
func getOrCreateSession(user *db.User) (*db.Session, error) {
	session := db.Session{
		UserId:         user.Id,
		PostingState:   db.SessPStateNew,
		PurchaseListId: primitive.NilObjectID,
	}
	log.Println("getOrCreateSession")
	err := sessionService.Create(&session)
	if err != nil {
		log.Println("failed to create session", err)
		session, err = sessionService.FindByUserID(user.Id)
		if err != nil {
			return nil, err
		}
	}

	return &session, err
}
func updateSession(session *db.Session) error {
	return sessionService.UpdateSession(session)
}
func reply(chatID int64, messageID int, forReply dialog.MessageForReply) (*tgbotapi.Message, error) {
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
		msgEdit.ParseMode = tgbotapi.ModeMarkdown + "V2"
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
func createEmptyList(session *db.Session) (*db.PurchaseList, error) {
	return purchaseListService.CreateEmptyList(session.UserId)
}

func createOrUpdateList(m *dialog.MessageDto, session *db.Session) (*db.PurchaseList, error) {
	var purchaseList db.PurchaseList
	var err error
	if session.PurchaseListId == primitive.NilObjectID {
		purchaseList = db.PurchaseList{
			UserID: session.UserId,
			TgMsgID: []db.TgMsgID{
				{
					TgMessageID: m.ID,
					TgChatID:    m.ChatID,
				},
			},
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
		}
		purchaseList.UserID = session.UserId
		purchaseList.CreatedAt = primitive.NewDateTimeFromTime(time.Now())
		purchaseList.ItemsDictionary = []db.PurchaseItem{}
		purchaseList.Items = []db.PurchaseItemHash{}
		purchaseList.DeletedItemHashes = []db.PurchaseItemHash{}
		err = purchaseListService.Create(&purchaseList)
		if err != nil {
			log.Println("Failed to insert a purchaseList", err)
			return nil, errors.New("failed to save a purchaseList")
		}
		session.PurchaseListId = purchaseList.Id
	} else {
		purchaseList, err = purchaseListService.FindByID(session.PurchaseListId)
		if err != nil {
			return nil, err
		}
	}
	switch session.PostingState {
	case db.SessPStateCreation, db.SessPStateDone:
		textItems := []string{}
		textItems = append(textItems, createListFromText(m.Text)...)
		textItems = sanitizeList(textItems)
		for _, textItem := range textItems {
			err = purchaseListService.AddItemToPurchaseList(
				purchaseList.Id,
				db.PurchaseItemName(textItem),
			)
			if err != nil {
				err = purchaseListService.AddItemToPurchaseList(
					purchaseList.Id,
					db.PurchaseItemName(textItem),
				)
				log.Println("Failed to add an item", err)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	purchaseList, err = purchaseListService.FindByID(purchaseList.Id)
	if err != nil {
		return nil, errors.New("failed to find a purchaseList " + err.Error())
	}
	return &purchaseList, nil
}

func createFakeUpdate(envelope *MessageEnvelope) {
	envelope.Update = &tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: int64(0)},
			Text: envelope.Text,
			From: &tgbotapi.User{
				ID:           0,
				FirstName:    "TestName",
				LastName:     "TestLname",
				LanguageCode: "en",
				IsBot:        false,
			},
		},
	}
	if envelope.IsCommand {
		envelope.Update.Message.Entities = &[]tgbotapi.MessageEntity{
			{
				Type:   "bot_command",
				Offset: 0,
				Length: len(envelope.Text),
			},
		}
	}
}

func crossOutItemFromPurchaseList(id primitive.ObjectID, itemHash string) (*db.PurchaseList, error) {
	err := purchaseListService.CrossOutItemFromPurchaseList(id, itemHash)
	if err != nil {
		log.Println("failed to cross out", err)
	}
	pList, err := purchaseListService.FindByID(id)
	return &pList, err
}

func getMessageChatId(envelope *MessageEnvelope) (int64, int) {
	return envelope.Update.Message.Chat.ID, envelope.Update.Message.MessageID
}
func getCallbackChatId(envelope *MessageEnvelope) (int64, int) {
	return envelope.Update.CallbackQuery.Message.Chat.ID, envelope.Update.CallbackQuery.Message.MessageID
}

func readCallbackQuery(query *tgbotapi.CallbackQuery, c *dialog.MessageHandler) dialog.MessageForReply {
	cbQueryData := query.Data
	m := dialog.MessageDto{UnknownContent: true}
	log.Println("query data", cbQueryData)
	if len(cbQueryData) < 24 {
		log.Println("invalid CallbackQuery data length")

		return dialog.MessageForReply{Text: "[Ошибка] Недостаточно данных"}
	}
	strListID := cbQueryData[:24]
	log.Println("strListID", strListID)
	itemHash := cbQueryData[25:]
	listID, err := primitive.ObjectIDFromHex(strListID)
	if err != nil {
		log.Println("failed to read ID from CallbackQuery", err)

		return dialog.MessageForReply{Text: "[Ошибка] Некорректные данные"}
	}
	log.Println("listID", listID, "text", itemHash)
	cbAnswer := tgbotapi.CallbackConfig{CallbackQueryID: query.ID, Text: ""}
	if itemHash == dialog.ComFinishedCrossout {
		_, session, _, err := getStateByList(listID)
		if err != nil {
			cbAnswer.Text = "Ошибка"
			log.Println(err)
			return dialog.MessageForReply{NewMessage: false, Text: "", AnswerCallback: &cbAnswer}
		}
		msg := dialog.MessageForReply{NewMessage: true, Text: "Введите название товара или список", AnswerCallback: &cbAnswer}
		session.PreviousState = session.PostingState
		session.PostingState = db.SessPStateCreation
		session.PurchaseListId = primitive.NilObjectID
		purchaseList, err := createEmptyList(session)
		if err != nil {
			cbAnswer.Text = "Ошибка обновления сессии"
			log.Println(err)
			return dialog.MessageForReply{NewMessage: false, Text: "", AnswerCallback: &cbAnswer}
		}
		session.PurchaseListId = purchaseList.Id
		err = updateSession(session)
		if err != nil {
			cbAnswer.Text = "Ошибка обновления сессии"
			log.Println(err)
			return dialog.MessageForReply{NewMessage: false, Text: "", AnswerCallback: &cbAnswer}
		}
		return msg
	} else { //element is crossed out
		purchaseList, err := crossOutItemFromPurchaseList(listID, itemHash)
		if err != nil {
			cbAnswer.Text = "Ошибка"
			log.Println(err)
			return dialog.MessageForReply{NewMessage: false, Text: "failed to cross out an item", AnswerCallback: &cbAnswer}
		}
		msg := c.GetMessageForReply(&m, nil, nil, purchaseList)
		msg.AnswerCallback = &cbAnswer
		log.Println("crossed out")

		return msg
	}
}

func createListFromText(text string) []string {
	list := strings.Split(text, "\n")
	return getUniqueItemsFromListBoundedToMax(list)
}

func sanitizeList(list []string) []string {
	var result []string
	for _, text := range list {
		text = stripmd.Strip(text)
		text = strings.ReplaceAll(text, ".", "․")
		text = strings.Trim(text, "`~\n\t")
		//todo It doesnt work!
		//crazy way to deal with long strings with emojis
		if len(text) > 30 {
			text = string([]rune(text)[:30])
			text = strings.Trim(text, "\u0000")

			text += "…"
		}
		if text == "" {
			continue
		}
		result = append(result, text)
	}
	return getUniqueItemsFromListBoundedToMax(result)
}

func getUniqueItemsFromListBoundedToMax(list []string) []string {
	var result []string
	keys := make(map[string]string)
	i := 0
	for _, key := range list {
		hash := db.GetMD5Hash(key)
		if _, found := keys[hash]; !found {
			keys[hash] = key
			result = append(result, key)

			i++
			if i >= MaxCountOfItemsInList {
				break
			}
		}
	}

	return result
}

func getStateByList(listID primitive.ObjectID) (*db.PurchaseList, *db.Session, *db.User, error) {
	var pList db.PurchaseList
	var user db.User
	pList, err := purchaseListService.FindByID(listID)
	if err != nil {
		return nil, nil, nil, errors.New("failed to find a purchaseList" + listID.Hex())
	}
	user, err = userService.FindByID(pList.UserID)
	if err != nil {
		return nil, nil, nil, errors.New("failed to find a user")
	}
	session, err := getOrCreateSession(&user)
	if err != nil {
		return nil, nil, nil, errors.New("failed to find a session")
	}

	return &pList, session, &user, err
}

func deleteMessage(listID primitive.ObjectID, ids []db.TgMsgID) error {
	if bot == nil {
		log.Println("[No bot] ")
		return errors.New("No bot")
	}
	log.Println("[message] DELETE")

	var err error
	for _, id := range ids {
		_, err = bot.DeleteMessage(tgbotapi.NewDeleteMessage(id.TgChatID, id.TgMessageID))
		log.Println("[message] DELETE one", err)
		_ = purchaseListService.DeleteMsgID(listID, id)
	}
	return err
}
