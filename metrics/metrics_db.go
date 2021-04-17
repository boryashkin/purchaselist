package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	DbPlistCreate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_plist_create",
			Help: "Purchase list create",
		},
		[]string{"result"},
	)
	DbPlistAddMsgID = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_plist_add_msg_id",
			Help: "Purchase AddMsgID",
		},
		[]string{"result"},
	)
	DbPlistDeleteMsgID = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_plist_del_msg_id",
			Help: "Purchase DeleteMsgID",
		},
		[]string{"result"},
	)
	DbPlistFindByID = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_plist_find_by_id",
			Help: "Purchase FindByID",
		},
		[]string{"result"},
	)
	DbPlistCrossOutItemFromPurchaseList = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_plist_crossout_item",
			Help: "Purchase CrossOutItemFromPurchaseList",
		},
		[]string{"result"},
	)
	DbPlistAddItemToPurchaseList = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_plist_add_item_to_plist",
			Help: "Purchase AddItemToPurchaseList",
		},
		[]string{"result"},
	)

	DbSessionFindByUserID = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_session_find_by_user_id",
			Help: "Session FindByUserID",
		},
		[]string{"result"},
	)
	DbSessionCreate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_session_create",
			Help: "Session Create",
		},
		[]string{"result"},
	)
	DbSessionUpdate = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_session_update",
			Help: "Session UpdateSession",
		},
		[]string{"result"},
	)

	DbUserUpsert = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_user_upsert",
			Help: "User Upsert",
		},
		[]string{"result"},
	)
	DbUserFindByID = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_user_find_by_id",
			Help: "User FindByID",
		},
		[]string{"result"},
	)
	DbUserFindByTgID = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_user_find_by_tg_id",
			Help: "User FindByTgID",
		},
		[]string{"result"},
	)
)

func InitDbMetrics() {
	prometheus.MustRegister(DbPlistCreate)
	prometheus.MustRegister(DbPlistFindByID)
	prometheus.MustRegister(DbPlistAddMsgID)
	prometheus.MustRegister(DbPlistAddItemToPurchaseList)
	prometheus.MustRegister(DbPlistDeleteMsgID)
	prometheus.MustRegister(DbPlistCrossOutItemFromPurchaseList)
}
