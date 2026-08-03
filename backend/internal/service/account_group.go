package service

import "time"

type AccountGroup struct {
	AccountID int64
	GroupID   int64
	Priority  int
	// PriceGroupingLocked keeps this membership when OpenAI channel-price grouping changes.
	PriceGroupingLocked bool
	CreatedAt           time.Time

	Account *Account
	Group   *Group
}
