package db

import "go.mongodb.org/mongo-driver/bson/primitive"

const (
	PiStatePlanned   PurchaseItemState = "0"
	PiStateBought    PurchaseItemState = "1"
	PiStateCancelled PurchaseItemState = "2"
)

type PurchaseItemState string

type PurchaseItem struct {
	Id        primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	Name      string             `json:"name" bson:"name"`
	Hash      string             `json:"hash" bson:"hash"`
	State     PurchaseItemState  `json:"state" bson:"state"`
	CreatedAt primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at" bson:"updated_at,omitempty"`
}
