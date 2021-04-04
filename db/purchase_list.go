package db

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"strings"
	"time"
)

type PurchaseItemName string
type PurchaseItemHash string

type PurchaseItem struct {
	Name PurchaseItemName
	Hash PurchaseItemHash
}

type TgMsgID struct {
	TgChatID    int64 `json:"tg_chat_id" bson:"tg_chat_id"`
	TgMessageID int   `json:"tg_message_id" bson:"tg_message_id"`
}
type PurchaseList struct {
	Id                primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	UserID            primitive.ObjectID `json:"user_id" bson:"user_id"`
	ItemsDictionary   []PurchaseItem     `json:"items_dictionary" bson:"items_dictionary"`
	Items             []PurchaseItemHash `json:"purchase_items" bson:"purchase_items"`
	DeletedItemHashes []PurchaseItemHash `json:"deleted_purchase_items" bson:"deleted_purchase_items"`
	TgMsgID           []TgMsgID          `json:"tg_msg_id" bson:"tg_msg_id"`
	CreatedAt         primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt         primitive.DateTime `json:"updated_at" bson:"updated_at,omitempty"`
}

type PurchaseListService struct {
	collection *mongo.Collection
}

func NewPurchaseListService(purchaseListCollection *mongo.Collection) PurchaseListService {
	return PurchaseListService{
		collection: purchaseListCollection,
	}
}

func (s *PurchaseListService) Create(list *PurchaseList) error {
	log.Println("pl.Create")
	result, err := s.collection.InsertOne(context.Background(), list)
	if err != nil {
		return err
	}
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		list.Id = oid
	} else {
		log.Println("Failed to extract ObjectId")
		return errors.New("failed to extract id")
	}
	return err
}

func (s *PurchaseListService) AddMsgID(id primitive.ObjectID, msgID TgMsgID) error {
	log.Println("pl.AddMsgID")
	_, err := s.collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id}, bson.M{"$push": bson.M{"tg_msg_id": msgID}},
	)

	return err
}

func (s *PurchaseListService) DeleteMsgID(id primitive.ObjectID, msgID TgMsgID) error {
	log.Println("pl.DeleteMsgID")
	_, err := s.collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id}, bson.M{"$pull": bson.M{"tg_msg_id": msgID}},
	)

	return err
}

func (s *PurchaseListService) FindByID(id primitive.ObjectID) (PurchaseList, error) {
	log.Println("pl.FindByID", id)
	var pList PurchaseList
	err := s.collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&pList)

	return pList, err
}

func (s *PurchaseListService) CrossOutItemFromPurchaseList(id primitive.ObjectID, itemHash string) error {
	log.Println("pl.CrossOut")
	_, err := s.collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{
			"$addToSet": bson.M{"deleted_purchase_items": itemHash},
			"$pull":     bson.M{"purchase_items": itemHash},
		},
	)
	return err
}

func (s *PurchaseListService) AddItemToPurchaseList(id primitive.ObjectID, item PurchaseItemName) error {
	log.Println("pl.AddItemToPurchaseList")
	hash := PurchaseItemHash(GetMD5Hash(string(item)))
	_, err := s.collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{
			"$addToSet": bson.M{
				"items_dictionary": PurchaseItem{Name: item, Hash: hash},
				"purchase_items":   hash,
			},
		},
	)
	return err
}

func (s *PurchaseListService) CreateEmptyList(id primitive.ObjectID) (*PurchaseList, error) {
	purchaseList := PurchaseList{
		UserID:    id,
		TgMsgID:   []TgMsgID{},
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}
	purchaseList.UserID = id
	purchaseList.CreatedAt = primitive.NewDateTimeFromTime(time.Now())
	purchaseList.ItemsDictionary = []PurchaseItem{}
	purchaseList.Items = []PurchaseItemHash{}
	purchaseList.DeletedItemHashes = []PurchaseItemHash{}
	err := s.Create(&purchaseList)
	if err != nil {
		log.Println("Failed to insert a purchaseList", err)
		return nil, errors.New("failed to save a purchaseList")
	}
	return &purchaseList, err
}

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(strings.ToLower(text)))
	return hex.EncodeToString(hash[:])
}
