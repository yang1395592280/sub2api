package service

import (
	"context"
	"fmt"
	"math"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type gamePointsCodeRepo struct {
	RedeemCodeRepository
	codes []RedeemCode
}

func (r *gamePointsCodeRepo) Create(_ context.Context, code *RedeemCode) error {
	r.codes = append(r.codes, *code)
	return nil
}

func (r *gamePointsCodeRepo) CreateBatch(_ context.Context, codes []RedeemCode) error {
	r.codes = append(r.codes, codes...)
	return nil
}

func TestGamePointsCodesValidateAllCreationPaths(t *testing.T) {
	for _, value := range []float64{0, -1, 1.5, 1e12, math.NaN(), math.Inf(1)} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			repo := &gamePointsCodeRepo{}
			svc := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil)
			_, err := svc.GenerateCodes(context.Background(), GenerateCodesRequest{Count: 1, Type: RedeemTypeGamePoints, Value: value})
			require.ErrorIs(t, err, ErrRedeemGamePointsInvalid)
			require.ErrorIs(t, svc.CreateCode(context.Background(), &RedeemCode{Code: "POINTS", Type: RedeemTypeGamePoints, Value: value}), ErrRedeemGamePointsInvalid)
			admin := &adminServiceImpl{redeemCodeRepo: repo}
			_, err = admin.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{Count: 1, Type: RedeemTypeGamePoints, Value: value})
			require.ErrorIs(t, err, ErrRedeemGamePointsInvalid)
			require.Empty(t, repo.codes)
		})
	}
	for _, value := range []float64{1, 250, 999999999999} {
		t.Run(fmt.Sprintf("valid-%v", value), func(t *testing.T) {
			repo := &gamePointsCodeRepo{}
			svc := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil)
			codes, err := svc.GenerateCodes(context.Background(), GenerateCodesRequest{Count: 2, Type: RedeemTypeGamePoints, Value: value})
			require.NoError(t, err)
			require.Len(t, codes, 2)
			for _, code := range codes {
				require.Equal(t, RedeemTypeGamePoints, code.Type)
				require.Equal(t, value, code.Value)
				require.Equal(t, StatusUnused, code.Status)
			}
			require.NoError(t, svc.CreateCode(context.Background(), &RedeemCode{Code: "POINTS", Type: RedeemTypeGamePoints, Value: value}))
			admin := &adminServiceImpl{redeemCodeRepo: repo}
			generated, err := admin.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{Count: 1, Type: RedeemTypeGamePoints, Value: value})
			require.NoError(t, err)
			require.Len(t, generated, 1)
		})
	}
}

func TestRedeemRejectsMalformedGamePointsBeforeUse(t *testing.T) {
	for _, value := range []float64{0, -1, 0.5, 1e12, math.NaN(), math.Inf(1)} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			repo := &redeemRejectRepo{code: RedeemCode{ID: 1, Code: "POINTS", Type: RedeemTypeGamePoints, Value: value, Status: StatusUnused}}
			svc := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil)
			_, err := svc.Redeem(context.Background(), 42, "POINTS")
			require.Equal(t, "REDEEM_GAME_POINTS_INVALID", infraerrors.Reason(err))
			require.False(t, repo.useCalled)
		})
	}
}
