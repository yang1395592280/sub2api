package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type CheckinDistributionBucket struct {
	StartPercent float64 `json:"start_percent"`
	EndPercent   float64 `json:"end_percent"`
	Weight       int     `json:"weight"`
}

func ParseCheckinDistributionConfig(raw string) ([]CheckinDistributionBucket, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return nil, fmt.Errorf("签到分档配置不能为空")
	}

	var buckets []CheckinDistributionBucket
	if err := json.Unmarshal([]byte(trimmed), &buckets); err != nil {
		return nil, fmt.Errorf("签到分档配置不是合法 JSON: %w", err)
	}
	if err := validateCheckinDistributionBuckets(buckets); err != nil {
		return nil, err
	}
	return buckets, nil
}

func validateCheckinDistributionBuckets(buckets []CheckinDistributionBucket) error {
	if len(buckets) == 0 {
		return fmt.Errorf("签到分档配置至少需要一个分档")
	}

	expectedStart := 0.0
	for index, bucket := range buckets {
		if bucket.Weight <= 0 {
			return fmt.Errorf("签到分档第 %d 档权重必须大于 0", index+1)
		}
		if bucket.StartPercent < 0 || bucket.EndPercent > 100 || bucket.StartPercent >= bucket.EndPercent {
			return fmt.Errorf("签到分档第 %d 档区间非法", index+1)
		}
		if math.Abs(bucket.StartPercent-expectedStart) > 0.000001 {
			return fmt.Errorf("签到分档第 %d 档起点必须紧接上一档终点 %.6g", index+1, expectedStart)
		}
		expectedStart = bucket.EndPercent
	}

	if math.Abs(expectedStart-100.0) > 0.000001 {
		return fmt.Errorf("签到分档配置必须完整覆盖 0 到 100")
	}
	return nil
}
