package irisgrpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/checkpoint"
	"github.com/zenanetwork/go-zenanet/log"

	proto "github.com/zenanetwork/zenaproto/iris"
	protoutils "github.com/zenanetwork/zenaproto/utils"
)

const (
	checkpointCachePrefix      = "checkpoint"
	checkpointCountCachePrefix = "checkpoint_count"
)

// FetchCheckpointCount는 체크포인트 수를 가져옵니다.
func (h *IrisGRPCClient) FetchCheckpointCount(ctx context.Context) (int64, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인
	cacheKey := getCacheKey(checkpointCountCachePrefix)
	if cachedCount, ok := h.cache.Get(cacheKey); ok {
		log.Debug("Using cached checkpoint count")
		return cachedCount.(int64), nil
	}

	log.Info("Fetching checkpoint count")

	var count int64

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker("checkpoint_count", func() error {
		res, err := h.client.FetchCheckpointCount(ctx, nil)
		if err != nil {
			return err
		}

		count = res.Result.Result
		return nil
	})

	if err != nil {
		return 0, err
	}

	// 결과 캐싱 (TTL 설정 없음, 캐시 사이즈로 관리)
	h.cache.Add(cacheKey, count)

	log.Info("Fetched checkpoint count successfully", "count", count)

	return count, nil
}

// FetchCheckpoint는 체크포인트를 가져옵니다.
func (h *IrisGRPCClient) FetchCheckpoint(ctx context.Context, number int64) (*checkpoint.Checkpoint, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인
	cacheKey := getCacheKey(checkpointCachePrefix, number)
	if cachedCheckpoint, ok := h.cache.Get(cacheKey); ok {
		log.Debug("Using cached checkpoint", "number", number)
		return cachedCheckpoint.(*checkpoint.Checkpoint), nil
	}

	log.Info("Fetching checkpoint", "number", number)

	var result *checkpoint.Checkpoint

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker(fmt.Sprintf("checkpoint %d", number), func() error {
		req := &proto.FetchCheckpointRequest{
			ID: number,
		}

		res, err := h.client.FetchCheckpoint(ctx, req)
		if err != nil {
			return err
		}

		// 응답 변환
		result = &checkpoint.Checkpoint{
			StartBlock:  new(big.Int).SetUint64(res.Result.StartBlock),
			EndBlock:    new(big.Int).SetUint64(res.Result.EndBlock),
			RootHash:    protoutils.ConvertH256ToHash(res.Result.RootHash),
			Proposer:    protoutils.ConvertH160toAddress(res.Result.Proposer),
			ZenaChainID: res.Result.ZenaChainID,
			Timestamp:   uint64(res.Result.Timestamp.GetSeconds()),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 결과 캐싱
	h.cache.Add(cacheKey, result)

	log.Info("Fetched checkpoint successfully", "number", number)

	return result, nil
}
