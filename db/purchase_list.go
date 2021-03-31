package db

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type PurchaseList struct {
	Id          primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	UserID      primitive.ObjectID `json:"user_id" bson:"user_id"`
	Items       []PurchaseItem     `json:"purchase_items" bson:"purchase_items"`
	TgChatID    int64              `json:"tg_chat_id" bson:"tg_chat_id"`
	TgMessageID int                `json:"tg_message_id" bson:"tg_message_id"`
	CreatedAt   primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt   primitive.DateTime `json:"updated_at" bson:"updated_at,omitempty"`
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

func (s *PurchaseListService) UpdateFields(id primitive.ObjectID, fields interface{}) error {
	log.Println("pl.UpdateFields")
	_, err := s.collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id}, bson.M{"$set": fields},
	)

	return err
}

func (s *PurchaseListService) FindByID(id primitive.ObjectID) (PurchaseList, error) {
	log.Println("pl.FindByID", id)
	var pList PurchaseList
	err := s.collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&pList)

	return pList, err
}

func (s *PurchaseListService) CrossOutItemFromPurchaseList(id primitive.ObjectID, itemHash string) (*PurchaseList, error) {
	log.Println("pl.CrossOut")
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
	_, err := s.collection.UpdateOne(
		context.Background(),
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"purchase_items.$[elem].state": PiStateBought}},
		&updOpts,
	)
	purchaseList := PurchaseList{}
	err = s.collection.FindOne(context.Background(), bson.M{
		"_id": id,
	}).Decode(&purchaseList)
	if err != nil {
		return nil, errors.New("failed to find a purchaseList")
	}

	return &purchaseList, err
}
