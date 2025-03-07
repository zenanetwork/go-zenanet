// Copyright 2023 The go-zenanet Authors
// This file is part of the go-zenanet library.
//
// The go-zenanet library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-zenanet library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-zenanet library. If not, see <http://www.gnu.org/licenses/>.

package governance

import (
	"math/big"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
)

// VoteTallyResult는 투표 집계 결과를 나타냅니다.
type VoteTallyResult struct {
	ProposalID      uint64      // 제안 ID
	TotalVotes      *big.Int    // 총 투표 수
	YesVotes        *big.Int    // 찬성 투표 수
	NoVotes         *big.Int    // 반대 투표 수
	NoWithVetoVotes *big.Int    // 거부권 투표 수
	AbstainVotes    *big.Int    // 기권 투표 수
	QuorumReached   bool        // 정족수 도달 여부
	VetoReached     bool        // 거부권 도달 여부
	Passed          bool        // 통과 여부
	TallyTime       time.Duration // 집계 소요 시간
}

// VoteTallyOptimizer는 투표 집계 과정을 최적화하는 구조체입니다.
type VoteTallyOptimizer struct {
	// 설정
	workerCount     int         // 병렬 처리 워커 수
	batchSize       int         // 배치 크기
	cacheSize       int         // 캐시 크기
	cacheExpiration time.Duration // 캐시 만료 시간

	// 캐시
	resultCache     map[uint64]*VoteTallyResult // 결과 캐시 (제안 ID -> 결과)
	voteCountCache  map[uint64]map[GovVoteOption]*big.Int // 투표 수 캐시 (제안 ID -> 옵션 -> 수)
	cacheLock       sync.RWMutex // 캐시 락

	// 로거
	logger log.Logger
}

// NewVoteTallyOptimizer는 새로운 VoteTallyOptimizer 인스턴스를 생성합니다.
func NewVoteTallyOptimizer(workerCount, batchSize, cacheSize int, cacheExpiration time.Duration) *VoteTallyOptimizer {
	return &VoteTallyOptimizer{
		workerCount:     workerCount,
		batchSize:       batchSize,
		cacheSize:       cacheSize,
		cacheExpiration: cacheExpiration,
		resultCache:     make(map[uint64]*VoteTallyResult),
		voteCountCache:  make(map[uint64]map[GovVoteOption]*big.Int),
		logger:          log.New("module", "vote_tally_optimizer"),
	}
}

// TallyVotes는 제안에 대한 투표를 최적화된 방식으로 집계합니다.
func (o *VoteTallyOptimizer) TallyVotes(
	proposal *GovProposal,
	quorumThreshold, vetoThreshold, passThreshold float64,
	totalStaked *big.Int,
) *VoteTallyResult {
	startTime := time.Now()

	// 캐시에서 결과 확인
	o.cacheLock.RLock()
	cachedResult, exists := o.resultCache[proposal.ID]
	o.cacheLock.RUnlock()

	// 캐시된 결과가 있고 투표 수가 변경되지 않았으면 캐시된 결과 반환
	if exists && len(cachedResult.TotalVotes.Bits()) > 0 && int64(len(proposal.Votes)) == cachedResult.TotalVotes.Int64() {
		return cachedResult
	}

	// 투표 수 집계 (병렬 처리)
	result := &VoteTallyResult{
		ProposalID:      proposal.ID,
		TotalVotes:      big.NewInt(0),
		YesVotes:        big.NewInt(0),
		NoVotes:         big.NewInt(0),
		NoWithVetoVotes: big.NewInt(0),
		AbstainVotes:    big.NewInt(0),
	}

	// 투표가 없으면 빠르게 결과 반환
	if len(proposal.Votes) == 0 {
		result.TallyTime = time.Since(startTime)
		return result
	}

	// 투표 배치 처리
	votes := proposal.Votes
	batchCount := (len(votes) + o.batchSize - 1) / o.batchSize
	workerCount := o.workerCount
	if workerCount > batchCount {
		workerCount = batchCount
	}

	// 결과 채널
	resultCh := make(chan map[GovVoteOption]*big.Int, workerCount)
	var wg sync.WaitGroup

	// 워커 시작
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 각 워커의 담당 범위 계산
			startIdx := workerID * (len(votes) / workerCount)
			endIdx := (workerID + 1) * (len(votes) / workerCount)
			if workerID == workerCount-1 {
				endIdx = len(votes)
			}

			// 로컬 카운터
			localCounts := map[GovVoteOption]*big.Int{
				GovOptionYes:        big.NewInt(0),
				GovOptionNo:         big.NewInt(0),
				GovOptionNoWithVeto: big.NewInt(0),
				GovOptionAbstain:    big.NewInt(0),
			}

			// 배치 처리
			votesSlice := votes[startIdx:endIdx]
			for _, vote := range votesSlice {
				// 투표자의 스테이킹 양 가져오기 (실제로는 스테이킹 어댑터에서 가져와야 함)
				// 여기서는 간단히 모든 투표자가 동일한 가중치를 가진다고 가정
				weight := big.NewInt(1)

				// 투표 옵션에 따라 카운트 증가
				localCounts[vote.Option].Add(localCounts[vote.Option], weight)
			}

			// 결과 전송
			resultCh <- localCounts
		}(i)
	}

	// 모든 워커 완료 대기
	wg.Wait()
	close(resultCh)

	// 결과 집계
	for localCounts := range resultCh {
		result.YesVotes.Add(result.YesVotes, localCounts[GovOptionYes])
		result.NoVotes.Add(result.NoVotes, localCounts[GovOptionNo])
		result.NoWithVetoVotes.Add(result.NoWithVetoVotes, localCounts[GovOptionNoWithVeto])
		result.AbstainVotes.Add(result.AbstainVotes, localCounts[GovOptionAbstain])
	}

	// 총 투표 수 계산
	result.TotalVotes.Add(result.TotalVotes, result.YesVotes)
	result.TotalVotes.Add(result.TotalVotes, result.NoVotes)
	result.TotalVotes.Add(result.TotalVotes, result.NoWithVetoVotes)
	result.TotalVotes.Add(result.TotalVotes, result.AbstainVotes)

	// 쿼럼 확인
	if totalStaked.Sign() > 0 {
		quorumRatio := new(big.Float).Quo(
			new(big.Float).SetInt(result.TotalVotes),
			new(big.Float).SetInt(totalStaked),
		)
		quorumThresholdFloat := big.NewFloat(quorumThreshold)

		if quorumRatio.Cmp(quorumThresholdFloat) >= 0 {
			result.QuorumReached = true
		}
	}

	// 거부권 확인
	if result.TotalVotes.Sign() > 0 {
		vetoRatio := new(big.Float).Quo(
			new(big.Float).SetInt(result.NoWithVetoVotes),
			new(big.Float).SetInt(result.TotalVotes),
		)
		vetoThresholdFloat := big.NewFloat(vetoThreshold)

		if vetoRatio.Cmp(vetoThresholdFloat) >= 0 {
			result.VetoReached = true
		}
	}

	// 통과 임계값 확인
	if result.TotalVotes.Sign() > 0 {
		// 기권표는 총 투표 수에서 제외
		nonAbstainVotes := new(big.Int).Sub(result.TotalVotes, result.AbstainVotes)
		if nonAbstainVotes.Sign() > 0 {
			yesRatio := new(big.Float).Quo(
				new(big.Float).SetInt(result.YesVotes),
				new(big.Float).SetInt(nonAbstainVotes),
			)
			passThresholdFloat := big.NewFloat(passThreshold)

			if yesRatio.Cmp(passThresholdFloat) >= 0 {
				result.Passed = true
			}
		}
	}

	// 거부권이 도달했으면 통과하지 않음
	if result.VetoReached {
		result.Passed = false
	}

	// 정족수에 도달하지 않았으면 통과하지 않음
	if !result.QuorumReached {
		result.Passed = false
	}

	// 소요 시간 기록
	result.TallyTime = time.Since(startTime)

	// 결과 캐싱
	o.cacheLock.Lock()
	o.resultCache[proposal.ID] = result
	o.cacheLock.Unlock()

	return result
}

// ClearCache는 캐시를 초기화합니다.
func (o *VoteTallyOptimizer) ClearCache() {
	o.cacheLock.Lock()
	defer o.cacheLock.Unlock()

	o.resultCache = make(map[uint64]*VoteTallyResult)
	o.voteCountCache = make(map[uint64]map[GovVoteOption]*big.Int)
}

// RemoveFromCache는 특정 제안의 캐시를 제거합니다.
func (o *VoteTallyOptimizer) RemoveFromCache(proposalID uint64) {
	o.cacheLock.Lock()
	defer o.cacheLock.Unlock()

	delete(o.resultCache, proposalID)
	delete(o.voteCountCache, proposalID)
}

// GetCacheStats는 캐시 통계를 반환합니다.
func (o *VoteTallyOptimizer) GetCacheStats() map[string]interface{} {
	o.cacheLock.RLock()
	defer o.cacheLock.RUnlock()

	return map[string]interface{}{
		"result_cache_size":    len(o.resultCache),
		"vote_count_cache_size": len(o.voteCountCache),
		"worker_count":         o.workerCount,
		"batch_size":           o.batchSize,
		"cache_size":           o.cacheSize,
		"cache_expiration":     o.cacheExpiration,
	}
} 