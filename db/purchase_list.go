package db

import "go.mongodb.org/mongo-driver/bson/primitive"

type PurchaseList struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	UserID    primitive.ObjectID `json:"user_id" bson:"user_id"`
	Items     []PurchaseItem     `json:"purchase_items" bson:"purchase_items"`
	CreatedAt primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at" bson:"updated_at,omitempty"`
}
