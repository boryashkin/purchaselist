package db

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	TgId      int                `json:"tg_id" bson:"tg_id"`
	Name      string             `json:"name" bson:"name"`
	Phone     string             `json:"phone" bson:"phone"`
	Lang      string             `json:"lang" bson:"lang"`
	CreatedAt primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
}
