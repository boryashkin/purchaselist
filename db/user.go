package db

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

type User struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	TgId      int                `json:"tg_id" bson:"tg_id"`
	Name      string             `json:"name" bson:"name"`
	Phone     string             `json:"phone" bson:"phone"`
	Lang      string             `json:"lang" bson:"lang"`
	CreatedAt primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
}

type UserService struct {
	collection *mongo.Collection
}

func NewUserService(userCollection *mongo.Collection) UserService {
	unq := true
	idxOpts := options.IndexOptions{Unique: &unq}
	_, err := userCollection.Indexes().CreateOne(
		context.Background(),
		mongo.IndexModel{Keys: bson.M{"tg_id": 1}, Options: &idxOpts},
	)
	if err != nil {
		log.Println("failed to create index tg_id", err)
	}
	return UserService{
		collection: userCollection,
	}
}

func (s *UserService) Upsert(user *User) error {
	log.Println("user.upsert")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	upsert := true
	opts := options.UpdateOptions{
		Upsert: &upsert,
	}
	result, err := s.collection.UpdateOne(
		ctx,
		bson.M{
			"tg_id": user.TgId,
		},
		bson.M{
			"$set": bson.M{
				"name":  user.Name,
				"lang":  user.Lang,
				"phone": user.Phone,
			},
			"$setOnInsert": bson.M{
				"created_at": user.CreatedAt,
				"tg_id":      user.TgId,
			},
		},
		&opts,
	)
	if err != nil {
		return err
	}

	if result.UpsertedCount > 0 {
		if oid, ok := result.UpsertedID.(primitive.ObjectID); ok {
			log.Println("Upserted")
			user.Id = oid
		} else {
			log.Println("Failed to extract ObjectId")
			return errors.New("failed to extract id")
		}
	} else {
		return errors.New("user already exists")
	}

	return nil
}

func (s *UserService) FindByID(id primitive.ObjectID) (User, error) {
	log.Println("user.findByID")
	var user User
	err := s.collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user)

	return user, err
}

func (s *UserService) FindByTgID(id int) (User, error) {
	log.Println("user.findByTgID")
	var user User
	err := s.collection.FindOne(context.Background(), bson.M{"tg_id": id}).Decode(&user)

	return user, err
}
