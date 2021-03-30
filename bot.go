package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/boryashkin/purchaselist/db"
	"github.com/boryashkin/purchaselist/dialog"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/writeas/go-strip-markdown"
	"go.mongodb.org/mongo-driver/bson"
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
	dbCtx         context.Context
	bot           *tgbotapi.BotAPI
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
	ch := make(chan *MessageEnvelope)
	//go generateStdinUpdates(ch)
	go generateSingleThreadedTgUpdates(ch)
	//updates := generateTgUpdates()
	//for update := range *updates {
	//	update := update // trying to make a local copy to prevent races
	//	envelope := MessageEnvelope{}
	//	if update.Message != nil {
	//		envelope.Text = update.Message.Text
	//	} else {
	//		envelope.Text = ""
	//	}
	//	envelope.Update = &update
	//	go handleAsync(&envelope)
	//}

	for {
		select {
		case envelope := <-ch:
			go handleAsync(envelope)
		}
	}

	log.Printf("After the loop")
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
	var session *db.Session
	var user *db.User
	var purchaseList *db.PurchaseList
	var err error
	var msg dialog.MessageForReply
	update := envelope.Update

	c := dialog.Dialog{Bot: bot}

	if update.CallbackQuery != nil {
		chatID, msgID = getCallbackChatId(envelope)
		msg = readCallbackQuery(update.CallbackQuery, &c)
		reply(chatID, msgID, msg)
		return
	} else if update.Message != nil {
		message = update.Message
		log.Printf("[%d] %s", message.From.ID, message.Text)
		m = c.ReadMessage(message)
		user, err = getOrRegisterUser(message)
		if err != nil {
			chatID, msgID = getMessageChatId(envelope)
			reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
			return
		}
		session, err = getOrCreateSession(user)
		if err != nil {
			chatID, msgID = getMessageChatId(envelope)
			reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
			return
		}

		purchaseList, err = createOrUpdateList(&m, session)
		if err != nil {
			chatID, msgID = getMessageChatId(envelope)
			reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
		} else if purchaseList != nil {
			session.PurchaseListId = purchaseList.Id
		}
		if session.PostingState == db.SessPStateDone && purchaseList == nil && m.Command == dialog.ComConfirm {
			chatID, msgID = getMessageChatId(envelope)
			reply(chatID, msgID, dialog.MessageForReply{Text: "failed to save a purchaseList, try again"})
		}

	} else {
		log.Println("[noop] Unknown request")
		return
	}

	st := c.GetNewStateByMessage(&m, session)
	if st != session.PostingState {
		if session.PostingState == db.SessPStateCreation && session.PreviousState == db.SessPStateDone {
			session.PurchaseListId = primitive.NilObjectID
		}
		session.PreviousState = session.PostingState
		session.PostingState = st
		err = updateSession(session)
		if err != nil {
			chatID, msgID = getMessageChatId(envelope)
			reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
			return
		}
	} else if purchaseList != nil {
		err = updateSession(session)
		if err != nil {
			chatID, msgID = getMessageChatId(envelope)
			reply(chatID, msgID, dialog.MessageForReply{Text: err.Error()})
			return
		}
	}
	msg = c.GetMessageForReply(&m, session, user, purchaseList)

	chatID, msgID = getMessageChatId(envelope)
	reply(chatID, msgID, msg)
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
	err := users.FindOne(context.Background(), bson.M{
		"tg_id": user.TgId,
	}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			res, err := users.InsertOne(dbCtx, user)
			if err != nil {
				log.Println("Failed to insert a user", err)
				return nil, errors.New("failed to save a user")
			}
			if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
				user.Id = oid
			} else {
				log.Println("Failed to extract ObjectId")
				return nil, errors.New("failed to extract id")
			}
		} else {
			log.Println("Find user err", err)
			return nil, errors.New("failed to find a user")
		}
	}

	return &user, nil
}
func getOrCreateSession(user *db.User) (*db.Session, error) {

	session := db.Session{
		UserId:         user.Id,
		PostingState:   db.SessPStateNew,
		PurchaseListId: primitive.NilObjectID,
	}
	err := sessions.FindOne(context.Background(), bson.M{
		"user_id": session.UserId,
	}).Decode(&session)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			res, err := sessions.InsertOne(dbCtx, session)
			if err != nil {
				log.Println("Failed to insert a session", err)
				return nil, errors.New("failed to save a session")
			}
			if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
				session.Id = oid
			} else {
				log.Println("Failed to extract sess ObjectId")
				return nil, errors.New("failed to extract sess id")
			}
		} else {
			log.Println("Find session err", err)
			return nil, errors.New("failed to find a session")
		}
	}

	return &session, nil
}
func updateSession(session *db.Session) error {
	_, err := sessions.UpdateOne(dbCtx, bson.M{"_id": session.Id}, bson.M{
		"$set": bson.M{
			"posting_state":    session.PostingState,
			"previous_state":   session.PreviousState,
			"purchase_list_id": session.PurchaseListId,
		},
	})
	if err != nil {
		return errors.New("failed to update a session")
	}
	return nil
}
func reply(chatID int64, messageID int, forReply dialog.MessageForReply) error {
	if bot == nil {
		log.Println("[No bot] ", forReply.Text)
		return nil
	} else {
		var msg tgbotapi.Chattable
		if forReply.NewMessage {
			log.Println("NewMessage", forReply.Text)
			if forReply.Text == "" {
				log.Println("not sent")
				return nil
			}
			msgNew := tgbotapi.NewMessage(chatID, forReply.Text)
			msgNew.ReplyToMessageID = messageID
			msgNew.ParseMode = tgbotapi.ModeMarkdown + "V2"
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

		_, err := bot.Send(msg)
		if err != nil {
			log.Println("err while sending " + err.Error())
		} else {
			log.Println("bot.Send() ok")
		}
		return err
	}
	return nil
}
func createOrUpdateList(m *dialog.MessageDto, session *db.Session) (*db.PurchaseList, error) {

	purchaseList := db.PurchaseList{
		TgMessageID: m.ID,
		TgChatID:    m.ChatID,
		UpdatedAt:   primitive.NewDateTimeFromTime(time.Now()),
	}
	if session.PurchaseListId == primitive.NilObjectID {
		purchaseList.UserID = session.UserId
		purchaseList.CreatedAt = primitive.NewDateTimeFromTime(time.Now())
		res, err := purchaseLists.InsertOne(dbCtx, purchaseList)
		if err != nil {
			log.Println("Failed to insert a purchaseList", err)
			return nil, errors.New("failed to save a purchaseList")
		}
		if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
			purchaseList.Id = oid
		} else {
			return nil, errors.New("failed to extract pro id")
		}
	} else {
		err := purchaseLists.FindOne(context.Background(), bson.M{
			"_id": session.PurchaseListId,
		}).Decode(&purchaseList)
		if err != nil {
			return nil, errors.New("failed to find a purchaseList")
		}
	}
	switch session.PostingState {
	case db.SessPStateCreation:
		log.Println("sanitizedList", m.Text)
		textItems := createListFromText(m.Text)
		textItems = sanitizeList(textItems)
		log.Println("sanitizedList", textItems)
		items := purchaseList.Items
		for _, textItem := range textItems {
			items = append(items, db.PurchaseItem{
				Name:      textItem,
				Hash:      GetMD5Hash(textItem),
				State:     db.PiStatePlanned,
				CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
			})
		}
		purchaseList.Items = items

		break
	}

	_, err := purchaseLists.UpdateOne(dbCtx, bson.M{"_id": purchaseList.Id}, bson.M{
		"$set": bson.M{
			"purchase_items": purchaseList.Items,
			"created_at":     purchaseList.CreatedAt,
			"updated_at":     purchaseList.UpdatedAt,
		},
	})
	if err != nil {
		return nil, errors.New("failed to save a purchaseList")
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
	/*
		db.getCollection('purchaseLists').update(
		    {"_id" : ObjectId("605f9d11cfd0e8a0ad111a7d")},
		    {$set: {"purchase_items.$[elem].state": 1}},
		    {
		        multi: false,
		        arrayFilters: [{"elem.name": "dk"}]
		    }
		)
	*/
	filters := []interface{}{
		bson.M{"elem.hash": itemHash},
	}
	arrFilter := options.ArrayFilters{
		bson.DefaultRegistry,
		filters,
	}
	updOpts := options.UpdateOptions{
		ArrayFilters: &arrFilter,
	}
	_, err := purchaseLists.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"purchase_items.$[elem].state": db.PiStateBought}},
		&updOpts,
	)
	purchaseList := db.PurchaseList{}
	err = purchaseLists.FindOne(context.Background(), bson.M{
		"_id": id,
	}).Decode(&purchaseList)
	if err != nil {
		return nil, errors.New("failed to find a purchaseList")
	}

	return &purchaseList, err
}

func getMessageChatId(envelope *MessageEnvelope) (int64, int) {
	return envelope.Update.Message.Chat.ID, envelope.Update.Message.MessageID
}
func getCallbackChatId(envelope *MessageEnvelope) (int64, int) {
	return envelope.Update.CallbackQuery.Message.Chat.ID, envelope.Update.CallbackQuery.Message.MessageID
}

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(strings.ToLower(text)))
	return hex.EncodeToString(hash[:])
}

func readCallbackQuery(query *tgbotapi.CallbackQuery, c *dialog.Dialog) dialog.MessageForReply {
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
		_, session, _, err := changeStateToNewListCreation(listID)
		if err != nil {
			cbAnswer.Text = "Ошибка"
			log.Println(err)
			return dialog.MessageForReply{NewMessage: false, Text: "", AnswerCallback: &cbAnswer}
		}
		msg := dialog.MessageForReply{NewMessage: false, Text: "Введите название товара или список", AnswerCallback: &cbAnswer}
		session.PreviousState = session.PostingState
		session.PostingState = db.SessPStateCreation
		err = updateSession(session)
		if err != nil {
			cbAnswer.Text = "Ошибка обновления сессии"
			log.Println(err)
			return dialog.MessageForReply{NewMessage: false, Text: "", AnswerCallback: &cbAnswer}
		}
		return msg
	} else {
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
		log.Println("[sanitize] before", text)
		text = stripmd.Strip(text)
		log.Println("[sanitize] a1", text)
		text = strings.ReplaceAll(text, ".", " ")
		log.Println("[sanitize] a2", text)
		text = strings.Trim(text, "`~\n\t")
		log.Println("[sanitize] a3", text)
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
		hash := GetMD5Hash(key)
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

func changeStateToNewListCreation(listID primitive.ObjectID) (*db.PurchaseList, *db.Session, *db.User, error) {
	var pList db.PurchaseList
	var user db.User
	err := purchaseLists.FindOne(context.Background(), bson.M{"_id": listID}).Decode(&pList)
	if err != nil {
		return nil, nil, nil, errors.New("failed to find a purchaseList" + listID.Hex())
	}
	err = users.FindOne(context.Background(), bson.M{"_id": pList.UserID}).Decode(&user)
	if err != nil {
		return nil, nil, nil, errors.New("failed to find a user")
	}
	session, err := getOrCreateSession(&user)
	if err != nil {
		return nil, nil, nil, errors.New("failed to find a session")
	}

	return &pList, session, &user, err
}
