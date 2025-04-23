package irisgrpc

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"
	"github.com/zenanetwork/go-zenanet/log"

	proto "github.com/zenanetwork/zenaproto/iris"
)

const (
	stateSyncCachePrefix = "state_sync"
)

// StateSyncEvents는 이벤트 레코드를 가져옵니다.
func (h *IrisGRPCClient) StateSyncEvents(ctx context.Context, fromID uint64, to int64) ([]*clerk.EventRecordWithTime, error) {
	// 컨텍스트에 타임아웃 설정
	ctx, cancel := h.contextWithTimeout(ctx)
	defer cancel()

	// 캐시 확인 (부분 요청에 대한 캐시)
	cacheKey := getCacheKey(stateSyncCachePrefix, fromID, to)
	if cachedEvents, ok := h.cache.Get(cacheKey); ok {
		log.Debug("Using cached state sync events", "fromID", fromID, "to", to)
		return cachedEvents.([]*clerk.EventRecordWithTime), nil
	}

	log.Info("Fetching state sync events", "fromID", fromID, "to", to)

	eventRecords := make([]*clerk.EventRecordWithTime, 0)

	// 서킷 브레이커 패턴을 사용하여 요청
	err := h.executeWithCircuitBreaker(fmt.Sprintf("state_sync_events %d-%d", fromID, to), func() error {
		req := &proto.StateSyncEventsRequest{
			FromID: fromID,
			ToTime: uint64(to),
			Limit:  uint64(stateFetchLimit),
		}

		var (
			res    proto.Iris_StateSyncEventsClient
			events *proto.StateSyncEventsResponse
			err    error
		)

		res, err = h.client.StateSyncEvents(ctx, req)
		if err != nil {
			return err
		}

		for {
			// 스트리밍 방식으로 데이터 수신
			events, err = res.Recv()
			if errors.Is(err, io.EOF) {
				return nil
			}

			if err != nil {
				return err
			}

			// 결과 처리 및 이벤트 레코드 변환
			for _, event := range events.Result {
				eventRecord := &clerk.EventRecordWithTime{
					EventRecord: clerk.EventRecord{
						ID:       event.ID,
						Contract: common.HexToAddress(event.Contract),
						Data:     common.Hex2Bytes(event.Data[2:]),
						TxHash:   common.HexToHash(event.TxHash),
						LogIndex: event.LogIndex,
						ChainID:  event.ChainID,
					},
					Time: event.Time.AsTime(),
				}
				eventRecords = append(eventRecords, eventRecord)
			}

			// 정기적으로 컨텍스트 취소 확인
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// 계속 진행
			}
		}
	})

	if err != nil {
		return nil, err
	}

	// 결과가 너무 크지 않다면 캐싱 (응답이 작은 경우만)
	if len(eventRecords) <= stateFetchLimit {
		h.cache.Add(cacheKey, eventRecords)
	}

	log.Info("Fetched state sync events successfully",
		"fromID", fromID,
		"to", to,
		"count", len(eventRecords))

	return eventRecords, nil
}
