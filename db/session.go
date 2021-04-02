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

type SessionService struct {
	collection *mongo.Collection
}

func NewSessionService(sessionCollection *mongo.Collection) SessionService {
	return SessionService{
		collection: sessionCollection,
	}
}

func (s *SessionService) FindByUserID(id primitive.ObjectID) (Session, error) {
	log.Println("session.FindByUserID")
	var session Session
	err := s.collection.FindOne(context.Background(), bson.M{
		"user_id": id,
	}).Decode(&session)

	return session, err
}

func (s *SessionService) Create(session *Session) error {
	log.Println("session.Create")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	upsert := true
	opts := options.UpdateOptions{
		Upsert: &upsert,
	}
	result, err := s.collection.UpdateOne(
		ctx,
		bson.M{
			"user_id": session.UserId,
		},
		bson.M{
			"$setOnInsert": session,
		},
		&opts,
	)
	if err != nil {
		return err
	}

	if result.UpsertedCount > 0 {
		if oid, ok := result.UpsertedID.(primitive.ObjectID); ok {
			session.Id = oid
		} else {
			log.Println("Failed to extract ObjectId")
			return errors.New("failed to extract id")
		}
	} else {
		return errors.New("already exists")
	}

	return nil
}

func (s *SessionService) UpdateSession(session *Session) error {
	log.Println("session.UpdateSession", session.PreviousState, session.PostingState, session.PurchaseListId)
	_, err := s.collection.UpdateOne(context.Background(), bson.M{"_id": session.Id}, bson.M{
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
