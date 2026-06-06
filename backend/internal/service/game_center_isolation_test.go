package service

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameCenterSettlementDoesNotTouchBalanceCollaborators(t *testing.T) {
	serviceType := reflect.TypeOf((*GameCenterService)(nil))
	require.NotNil(t, serviceType, "missing GameCenterService: implement a points-only game center service for checkin/lucky-bonus/lucky-wheel/size-bet settlement")

	serviceElem := serviceType.Elem()
	_, hasExchangeBalanceToPoints := serviceElem.MethodByName("ExchangeBalanceToPoints")
	require.False(t, hasExchangeBalanceToPoints, "GameCenterService should not expose ExchangeBalanceToPoints on a pure points game center")

	_, hasExchangePointsToBalance := serviceElem.MethodByName("ExchangePointsToBalance")
	require.False(t, hasExchangePointsToBalance, "GameCenterService should not expose ExchangePointsToBalance on a pure points game center")

	_, hasAdjustPoints := serviceElem.MethodByName("AdjustPoints")
	_, hasClaimPoints := serviceElem.MethodByName("ClaimPoints")
	require.True(t, hasAdjustPoints || hasClaimPoints, "GameCenterService should expose a points-only settlement capability such as AdjustPoints or ClaimPoints")

	assetsType := reflect.TypeOf(GameCenterAssets{})
	_, hasBalanceField := assetsType.FieldByName("Balance")
	require.False(t, hasBalanceField, "GameCenterAssets should not expose a Balance field on a pure points game center")

	_, hasPointsField := assetsType.FieldByName("Points")
	require.True(t, hasPointsField, "GameCenterAssets should expose a Points field for game-center settlement state")
}
