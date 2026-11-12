package repository

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const walletLockSQL = `SELECT balance, checkin_balance, liyu_balance, gift_balance FROM users WHERE id = \$1 AND deleted_at IS NULL FOR UPDATE`
const walletDeltaSQL = `(?s)UPDATE users SET balance = balance \+ \$1,.*checkin_balance = checkin_balance \+ \$2, liyu_balance = liyu_balance \+ \$3,.*gift_balance = gift_balance \+ \$4, updated_at = NOW\(\).*WHERE id = \$5 AND deleted_at IS NULL.*RETURNING balance, checkin_balance, liyu_balance, gift_balance`

func walletRows(w service.WalletBalance) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"balance", "checkin_balance", "liyu_balance", "gift_balance"}).AddRow(w.Paid, w.Checkin, w.Liyu, w.Gift)
}
func expectWalletLock(mock sqlmock.Sqlmock, id int64, w service.WalletBalance) {
	mock.ExpectQuery(walletLockSQL).WithArgs(id).WillReturnRows(walletRows(w))
}
func expectWalletDelta(mock sqlmock.Sqlmock, id int64, delta, after service.WalletBalance, event, op string, apiKeyID int64) {
	mock.ExpectQuery(walletDeltaSQL).WithArgs(delta.Paid, delta.Checkin, delta.Liyu, delta.Gift, id).WillReturnRows(walletRows(after))
	mock.ExpectExec(`INSERT INTO wallet_transactions`).WithArgs(id, event, op, apiKeyID, delta.Paid, delta.Checkin, delta.Liyu, delta.Gift, after.Paid, after.Checkin, after.Liyu, after.Gift, nil, "").WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectDefaultWalletPolicy(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT value FROM settings WHERE key=").WithArgs(service.SettingKeyWalletUsagePolicy).WillReturnRows(sqlmock.NewRows([]string{"value"}))
}
