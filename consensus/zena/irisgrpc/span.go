package irisgrpc

import (
	"context"
	"fmt"

	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
	"github.com/zenanetwork/go-zenanet/consensus/zena/valset"
	"github.com/zenanetwork/go-zenanet/log"

	proto "github.com/zenanetwork/zenaproto/iris"
	protoutils "github.com/zenanetwork/zenaproto/utils"
)

const (
	spanCachePrefix = "span"
)

// Span은 지정된 스팬 ID에 대한 정보를 가져옵니다.
func (h *IrisGRPCClient) Span(ctx context.Context, spanID uint64) (*span.IrisSpan, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인
	cacheKey := getCacheKey(spanCachePrefix, spanID)
	if cachedSpan, ok := h.cache.Get(cacheKey); ok {
		log.Debug("Using cached span", "spanID", spanID)
		return cachedSpan.(*span.IrisSpan), nil
	}

	log.Info("Fetching span", "spanID", spanID)

	// 서킷 브레이커 패턴을 사용하여 요청
	var result *span.IrisSpan
	err := h.executeWithCircuitBreaker(fmt.Sprintf("span %d", spanID), func() error {
		req := &proto.SpanRequest{
			ID: spanID,
		}

		res, err := h.client.Span(ctx, req)
		if err != nil {
			return err
		}

		result = parseSpan(res.Result)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 결과 캐싱
	h.cache.Add(cacheKey, result)

	log.Info("Fetched span successfully", "spanID", spanID)

	return result, nil
}

// parseSpan은 protobuf Span을 도메인 모델 Span으로 변환합니다.
func parseSpan(protoSpan *proto.Span) *span.IrisSpan {
	if protoSpan == nil {
		return nil
	}

	resp := &span.IrisSpan{
		Span: span.Span{
			ID:         protoSpan.ID,
			StartBlock: protoSpan.StartBlock,
			EndBlock:   protoSpan.EndBlock,
		},
		ValidatorSet:      valset.ValidatorSet{},
		SelectedProducers: []valset.Validator{},
		ChainID:           protoSpan.ChainID,
	}

	if protoSpan.ValidatorSet != nil {
		for _, validator := range protoSpan.ValidatorSet.Validators {
			resp.ValidatorSet.Validators = append(resp.ValidatorSet.Validators, parseValidator(validator))
		}

		if protoSpan.ValidatorSet.Proposer != nil {
			resp.ValidatorSet.Proposer = parseValidator(protoSpan.ValidatorSet.Proposer)
		}
	}

	for _, validator := range protoSpan.SelectedProducers {
		if validator != nil {
			resp.SelectedProducers = append(resp.SelectedProducers, *parseValidator(validator))
		}
	}

	return resp
}

// parseValidator는 protobuf Validator를 도메인 모델 Validator로 변환합니다.
func parseValidator(validator *proto.Validator) *valset.Validator {
	if validator == nil {
		return nil
	}

	return &valset.Validator{
		ID:               validator.ID,
		Address:          protoutils.ConvertH160toAddress(validator.Address),
		VotingPower:      validator.VotingPower,
		ProposerPriority: validator.ProposerPriority,
	}
}
