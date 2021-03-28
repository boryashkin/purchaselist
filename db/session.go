package db

import "go.mongodb.org/mongo-driver/bson/primitive"

const (
	SessPStateNew        SessState = "0"
	SessPStateRegistered SessState = "1"
	SessPStateCreation   SessState = "100"
	SessPStateInProgress SessState = "101"
	SessPStateDone       SessState = "102"
)

type SessState string

type Session struct {
	Id             primitive.ObjectID `json:"_id" bson:"_id,omitempty"`
	UserId         primitive.ObjectID `json:"user_id" bson:"user_id"`
	PostingState   SessState          `json:"posting_state" bson:"posting_state"`
	PreviousState  SessState          `json:"previous_state" bson:"previous_state"`
	PurchaseListId primitive.ObjectID `json:"purchase_list_id" bson:"purchase_list_id"`
	CreatedAt      primitive.DateTime `json:"created_at" bson:"created_at,omitempty"`
}
