package dialog

import "github.com/boryashkin/purchaselist/db"

type DialogState struct {
	User         *db.User
	Session      *db.Session
	PurchaseList *db.PurchaseList
}
