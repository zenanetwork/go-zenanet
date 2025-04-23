package irisgrpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/zenanetwork/go-zenanet/consensus/zena/iris"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/milestone"
	"github.com/zenanetwork/go-zenanet/log"

	proto "github.com/zenanetwork/zenaproto/iris"
	protoutils "github.com/zenanetwork/zenaproto/utils"
)

const (
	milestoneCountCachePrefix     = "milestone_count"
	milestoneCachePrefix          = "milestone"
	milestoneLastNoAckCachePrefix = "milestone_last_no_ack"
	milestoneNoAckCachePrefix     = "milestone_no_ack"
	milestoneIDCachePrefix        = "milestone_id"
)

// FetchMilestoneCount는 마일스톤 수를 가져옵니다.
func (h *IrisGRPCClient) FetchMilestoneCount(ctx context.Context) (int64, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인
	cacheKey := getCacheKey(milestoneCountCachePrefix)
	if cachedCount, ok := h.cache.Get(cacheKey); ok {
		log.Debug("Using cached milestone count")
		return cachedCount.(int64), nil
	}

	log.Info("Fetching milestone count")

	var count int64

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker("milestone_count", func() error {
		res, err := h.client.FetchMilestoneCount(ctx, nil)
		if err != nil {
			return err
		}

		count = res.Result.Count
		return nil
	})

	if err != nil {
		return 0, err
	}

	// 결과 캐싱
	h.cache.Add(cacheKey, count)

	log.Info("Fetched milestone count successfully", "count", count)

	return count, nil
}

// FetchMilestone는 최신 마일스톤을 가져옵니다.
func (h *IrisGRPCClient) FetchMilestone(ctx context.Context) (*milestone.Milestone, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인 (최신 마일스톤은 변경될 수 있으므로 캐싱하지 않음)

	log.Info("Fetching milestone")

	var result *milestone.Milestone

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker("milestone", func() error {
		res, err := h.client.FetchMilestone(ctx, nil)
		if err != nil {
			return err
		}

		// 응답 변환
		result = &milestone.Milestone{
			StartBlock:  new(big.Int).SetUint64(res.Result.StartBlock),
			EndBlock:    new(big.Int).SetUint64(res.Result.EndBlock),
			Hash:        protoutils.ConvertH256ToHash(res.Result.RootHash),
			Proposer:    protoutils.ConvertH160toAddress(res.Result.Proposer),
			ZenaChainID: res.Result.ZenaChainID,
			Timestamp:   uint64(res.Result.Timestamp.GetSeconds()),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Info("Fetched milestone successfully")

	return result, nil
}

// FetchLastNoAckMilestone는 최근 거부된 마일스톤 ID를 가져옵니다.
func (h *IrisGRPCClient) FetchLastNoAckMilestone(ctx context.Context) (string, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	log.Debug("Fetching latest no ack milestone ID")

	var result string

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker("last_no_ack_milestone", func() error {
		res, err := h.client.FetchLastNoAckMilestone(ctx, nil)
		if err != nil {
			return err
		}

		result = res.Result.Result
		return nil
	})

	if err != nil {
		return "", err
	}

	log.Debug("Fetched last no-ack milestone", "result", result)

	return result, nil
}

// FetchNoAckMilestone는 지정된 마일스톤 ID가 거부되었는지 확인합니다.
func (h *IrisGRPCClient) FetchNoAckMilestone(ctx context.Context, milestoneID string) error {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인
	cacheKey := getCacheKey(milestoneNoAckCachePrefix, milestoneID)
	if cachedResult, ok := h.cache.Get(cacheKey); ok {
		if !cachedResult.(bool) {
			return fmt.Errorf("%w: milestoneID %q", iris.ErrNotInRejectedList, milestoneID)
		}
		log.Debug("Using cached no ack milestone result", "milestoneID", milestoneID)
		return nil
	}

	log.Debug("Fetching no ack milestone", "milestoneID", milestoneID)

	var success bool

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker(fmt.Sprintf("no_ack_milestone %s", milestoneID), func() error {
		req := &proto.FetchMilestoneNoAckRequest{
			MilestoneID: milestoneID,
		}

		res, err := h.client.FetchNoAckMilestone(ctx, req)
		if err != nil {
			return err
		}

		success = res.Result.Result
		return nil
	})

	if err != nil {
		return err
	}

	// 결과 캐싱
	h.cache.Add(cacheKey, success)

	if !success {
		return fmt.Errorf("%w: milestoneID %q", iris.ErrNotInRejectedList, milestoneID)
	}

	log.Debug("Fetched no ack milestone", "milestoneID", milestoneID)

	return nil
}

// FetchMilestoneID는 지정된 마일스톤 ID가 유효한지 확인합니다.
func (h *IrisGRPCClient) FetchMilestoneID(ctx context.Context, milestoneID string) error {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인
	cacheKey := getCacheKey(milestoneIDCachePrefix, milestoneID)
	if cachedResult, ok := h.cache.Get(cacheKey); ok {
		if !cachedResult.(bool) {
			return fmt.Errorf("%w: milestoneID %q", iris.ErrNotInMilestoneList, milestoneID)
		}
		log.Debug("Using cached milestone ID result", "milestoneID", milestoneID)
		return nil
	}

	log.Debug("Fetching milestone id", "milestoneID", milestoneID)

	var success bool

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker(fmt.Sprintf("milestone_id %s", milestoneID), func() error {
		req := &proto.FetchMilestoneIDRequest{
			MilestoneID: milestoneID,
		}

		res, err := h.client.FetchMilestoneID(ctx, req)
		if err != nil {
			return err
		}

		success = res.Result.Result
		return nil
	})

	if err != nil {
		return err
	}

	// 결과 캐싱
	h.cache.Add(cacheKey, success)

	if !success {
		return fmt.Errorf("%w: milestoneID %q", iris.ErrNotInMilestoneList, milestoneID)
	}

	log.Debug("Fetched milestone id", "milestoneID", milestoneID)

	return nil
}
