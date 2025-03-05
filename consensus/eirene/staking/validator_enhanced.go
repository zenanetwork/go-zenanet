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

package staking

import (
	"math/big"
	"sort"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// 향상된 검증자 선택 알고리즘 관련 상수
const (
	// 검증자 선택에 사용되는 가중치 (1000 단위)
	enhancedStakingWeightRatio     = 600 // 스테이킹 양 가중치 (60%)
	enhancedPerformanceWeightRatio = 300 // 성능 가중치 (30%)
	enhancedReputationWeightRatio  = 100 // 평판 가중치 (10%)

	// 성능 지표 가중치 (1000 단위)
	enhancedBlocksMissedWeight   = 300 // 놓친 블록 가중치 (30%)
	enhancedBlocksSignedWeight   = 200 // 서명한 블록 가중치 (20%)
	enhancedBlockProposedWeight  = 150 // 제안한 블록 가중치 (15%)
	enhancedUptimeWeight         = 150 // 업타임 가중치 (15%)
	enhancedResponseTimeWeight   = 100 // 응답 시간 가중치 (10%)
	enhancedGovernanceVoteWeight = 100 // 거버넌스 참여 가중치 (10%)

	// 평판 지표 가중치 (1000 단위)
	enhancedSlashingHistoryWeight = 400 // 슬래싱 이력 가중치 (40%)
	enhancedValidatorAgeWeight    = 300 // 검증자 나이 가중치 (30%)
	enhancedCommunityVotesWeight  = 200 // 커뮤니티 투표 가중치 (20%)
	enhancedNetworkContribWeight  = 100 // 네트워크 기여도 가중치 (10%)

	// 평판 시스템 매개변수
	maxValidatorAge           = 365 * 24 * 60 * 4 // 약 1년 (15초 블록 기준)
	maxSlashingHistoryPenalty = 500               // 최대 슬래싱 이력 페널티 (50%)
	maxCommunityVotes         = 1000              // 최대 커뮤니티 투표 수
	maxNetworkContribPoints   = 1000              // 최대 네트워크 기여도 포인트
)

// ValidatorReputationStats는 검증자의 평판 지표를 나타냅니다.
type ValidatorReputationStats struct {
	// 슬래싱 이력
	SlashingHistory     []SlashingEvent `json:"slashingHistory"`     // 슬래싱 이력
	LastSlashingBlock   uint64          `json:"lastSlashingBlock"`   // 마지막 슬래싱 블록
	TotalSlashingAmount *big.Int        `json:"totalSlashingAmount"` // 총 슬래싱 양

	// 검증자 나이
	ActivationBlock   uint64 `json:"activationBlock"`   // 활성화 블록
	TotalActiveBlocks uint64 `json:"totalActiveBlocks"` // 총 활성 블록 수

	// 커뮤니티 투표
	CommunityVotes int64 `json:"communityVotes"` // 커뮤니티 투표 (양수: 긍정, 음수: 부정)

	// 네트워크 기여도
	NetworkContribPoints uint64            `json:"networkContribPoints"` // 네트워크 기여도 포인트
	ContribCategories    map[string]uint64 `json:"contribCategories"`    // 기여 카테고리별 포인트
}

// SlashingEvent는 슬래싱 이벤트를 나타냅니다.
type SlashingEvent struct {
	BlockNumber uint64   `json:"blockNumber"` // 슬래싱 블록 번호
	Type        uint8    `json:"type"`        // 슬래싱 유형
	Amount      *big.Int `json:"amount"`      // 슬래싱 양
	Reason      string   `json:"reason"`      // 슬래싱 이유
}

// EnhancedValidatorStats는 향상된 검증자 성능 지표를 나타냅니다.
type EnhancedValidatorStats struct {
	// 기본 성능 지표
	BlocksProposed  uint64 `json:"blocksProposed"`  // 제안한 블록 수
	BlocksSigned    uint64 `json:"blocksSigned"`    // 서명한 블록 수
	BlocksMissed    uint64 `json:"blocksMissed"`    // 놓친 블록 수
	Uptime          uint64 `json:"uptime"`          // 업타임 (%)
	GovernanceVotes uint64 `json:"governanceVotes"` // 참여한 거버넌스 투표 수

	// 추가 성능 지표
	ResponseTimes   []uint64 `json:"responseTimes"`   // 응답 시간 기록 (밀리초)
	AvgResponseTime uint64   `json:"avgResponseTime"` // 평균 응답 시간 (밀리초)
	BlocksOrphaned  uint64   `json:"blocksOrphaned"`  // 고아가 된 제안 블록 수
	LastActiveBlock uint64   `json:"lastActiveBlock"` // 마지막 활동 블록
}

// EnhancedValidator는 향상된 검증자 정보를 나타냅니다.
type EnhancedValidator struct {
	// 기본 검증자 정보
	Validator *Validator `json:"validator"` // 기본 검증자 정보

	// 향상된 성능 지표
	EnhancedStats EnhancedValidatorStats `json:"enhancedStats"` // 향상된 성능 지표

	// 평판 지표
	Reputation ValidatorReputationStats `json:"reputation"` // 평판 지표
}

// calculateEnhancedValidatorScore는 향상된 검증자 점수를 계산합니다.
func calculateEnhancedValidatorScore(validator *EnhancedValidator, totalBlocks uint64, currentBlock uint64) *big.Int {
	// 1. 스테이킹 양 점수 (60%)
	stakingScore := new(big.Int).Mul(validator.Validator.VotingPower, big.NewInt(enhancedStakingWeightRatio))
	stakingScore = new(big.Int).Div(stakingScore, big.NewInt(1000))

	// 2. 성능 점수 계산 (30%)
	performanceScore := calculateEnhancedPerformanceScore(validator, totalBlocks)

	// 3. 평판 점수 계산 (10%)
	reputationScore := calculateReputationScore(validator, currentBlock)

	// 최종 점수 계산
	totalScore := new(big.Int).Add(stakingScore, performanceScore)
	totalScore = new(big.Int).Add(totalScore, reputationScore)

	return totalScore
}

// calculateEnhancedPerformanceScore는 향상된 성능 점수를 계산합니다.
func calculateEnhancedPerformanceScore(validator *EnhancedValidator, totalBlocks uint64) *big.Int {
	performanceScore := big.NewInt(0)
	stats := validator.EnhancedStats

	// 1. 놓친 블록 점수 (30%)
	missedRatio := float64(stats.BlocksMissed) / float64(totalBlocks)
	missedScore := 1.0 - missedRatio
	if missedScore < 0 {
		missedScore = 0
	}
	missedScoreInt := new(big.Int).SetUint64(uint64(missedScore * 1000))
	missedScoreInt = new(big.Int).Mul(missedScoreInt, big.NewInt(enhancedBlocksMissedWeight))
	missedScoreInt = new(big.Int).Div(missedScoreInt, big.NewInt(1000))

	// 2. 서명한 블록 점수 (20%)
	signedRatio := float64(stats.BlocksSigned) / float64(totalBlocks)
	signedScoreInt := new(big.Int).SetUint64(uint64(signedRatio * 1000))
	signedScoreInt = new(big.Int).Mul(signedScoreInt, big.NewInt(enhancedBlocksSignedWeight))
	signedScoreInt = new(big.Int).Div(signedScoreInt, big.NewInt(1000))

	// 3. 제안한 블록 점수 (15%)
	proposedRatio := float64(stats.BlocksProposed) / float64(totalBlocks/100) // 예상 제안 블록 수의 1%
	if proposedRatio > 1.0 {
		proposedRatio = 1.0
	}
	proposedScoreInt := new(big.Int).SetUint64(uint64(proposedRatio * 1000))
	proposedScoreInt = new(big.Int).Mul(proposedScoreInt, big.NewInt(enhancedBlockProposedWeight))
	proposedScoreInt = new(big.Int).Div(proposedScoreInt, big.NewInt(1000))

	// 4. 업타임 점수 (15%)
	uptimeScoreInt := new(big.Int).SetUint64(stats.Uptime)
	uptimeScoreInt = new(big.Int).Mul(uptimeScoreInt, big.NewInt(enhancedUptimeWeight))
	uptimeScoreInt = new(big.Int).Div(uptimeScoreInt, big.NewInt(1000))

	// 5. 응답 시간 점수 (10%)
	// 응답 시간이 빠를수록 높은 점수
	responseTimeScore := uint64(1000)
	if stats.AvgResponseTime > 0 {
		// 5초(5000ms)를 기준으로 점수 계산
		responseTimeScore = uint64(1000 * (1.0 - float64(stats.AvgResponseTime)/5000.0))
		if responseTimeScore > 1000 {
			responseTimeScore = 1000
		}
		if responseTimeScore < 0 {
			responseTimeScore = 0
		}
	}
	responseTimeScoreInt := new(big.Int).SetUint64(responseTimeScore)
	responseTimeScoreInt = new(big.Int).Mul(responseTimeScoreInt, big.NewInt(enhancedResponseTimeWeight))
	responseTimeScoreInt = new(big.Int).Div(responseTimeScoreInt, big.NewInt(1000))

	// 6. 거버넌스 참여 점수 (10%)
	// 최대 10개의 투표를 고려
	govVotes := stats.GovernanceVotes
	if govVotes > 10 {
		govVotes = 10
	}
	govScoreInt := new(big.Int).SetUint64(govVotes * 100) // 0-1000 범위로 변환
	govScoreInt = new(big.Int).Mul(govScoreInt, big.NewInt(enhancedGovernanceVoteWeight))
	govScoreInt = new(big.Int).Div(govScoreInt, big.NewInt(1000))

	// 성능 점수 합산
	performanceScore = new(big.Int).Add(performanceScore, missedScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, signedScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, proposedScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, uptimeScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, responseTimeScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, govScoreInt)

	// 성능 가중치 적용
	performanceScore = new(big.Int).Mul(performanceScore, big.NewInt(enhancedPerformanceWeightRatio))
	performanceScore = new(big.Int).Div(performanceScore, big.NewInt(1000))

	return performanceScore
}

// calculateReputationScore는 평판 점수를 계산합니다.
func calculateReputationScore(validator *EnhancedValidator, currentBlock uint64) *big.Int {
	reputationScore := big.NewInt(0)
	reputation := validator.Reputation

	// 1. 슬래싱 이력 점수 (40%)
	// 슬래싱이 없을수록 높은 점수
	slashingScore := uint64(1000)
	if len(reputation.SlashingHistory) > 0 {
		// 최근 슬래싱에 더 큰 가중치 부여
		weightedSlashingScore := uint64(0)
		totalWeight := uint64(0)

		for i, event := range reputation.SlashingHistory {
			// 최근 이벤트에 더 큰 가중치 부여
			weight := uint64(len(reputation.SlashingHistory) - i)
			totalWeight += weight

			// 슬래싱 유형에 따른 페널티
			typePenalty := uint64(0)
			switch event.Type {
			case SlashingTypeDoubleSign:
				typePenalty = 1000 // 100% 페널티
			case SlashingTypeDowntime:
				typePenalty = 500 // 50% 페널티
			case SlashingTypeMisbehavior:
				typePenalty = 750 // 75% 페널티
			}

			// 시간 경과에 따른 페널티 감소 (1년 후 50% 감소)
			blocksPassed := currentBlock - event.BlockNumber
			if blocksPassed > maxValidatorAge {
				blocksPassed = maxValidatorAge
			}
			ageFactor := 1.0 - (float64(blocksPassed) / float64(maxValidatorAge) * 0.5)

			// 가중 페널티 계산
			weightedPenalty := uint64(float64(typePenalty)*ageFactor) * weight
			weightedSlashingScore += weightedPenalty
		}

		// 평균 페널티 계산
		if totalWeight > 0 {
			avgPenalty := weightedSlashingScore / totalWeight
			if avgPenalty > maxSlashingHistoryPenalty {
				avgPenalty = maxSlashingHistoryPenalty
			}
			slashingScore = 1000 - avgPenalty
		}
	}

	slashingScoreInt := new(big.Int).SetUint64(slashingScore)
	slashingScoreInt = new(big.Int).Mul(slashingScoreInt, big.NewInt(enhancedSlashingHistoryWeight))
	slashingScoreInt = new(big.Int).Div(slashingScoreInt, big.NewInt(1000))

	// 2. 검증자 나이 점수 (30%)
	// 오래된 검증자일수록 높은 점수
	ageScore := uint64(0)
	if reputation.ActivationBlock > 0 {
		age := currentBlock - reputation.ActivationBlock
		if age > maxValidatorAge {
			age = maxValidatorAge
		}
		ageScore = uint64(float64(age) / float64(maxValidatorAge) * 1000)
	}

	ageScoreInt := new(big.Int).SetUint64(ageScore)
	ageScoreInt = new(big.Int).Mul(ageScoreInt, big.NewInt(enhancedValidatorAgeWeight))
	ageScoreInt = new(big.Int).Div(ageScoreInt, big.NewInt(1000))

	// 3. 커뮤니티 투표 점수 (20%)
	communityVoteScore := uint64(500) // 기본값 (중립)
	if reputation.CommunityVotes != 0 {
		// -1000 ~ 1000 범위의 투표를 0 ~ 1000 범위로 변환
		normalizedVotes := reputation.CommunityVotes
		if normalizedVotes > maxCommunityVotes {
			normalizedVotes = maxCommunityVotes
		}
		if normalizedVotes < -maxCommunityVotes {
			normalizedVotes = -maxCommunityVotes
		}

		communityVoteScore = uint64((float64(normalizedVotes) / float64(maxCommunityVotes) * 500) + 500)
	}

	communityVoteScoreInt := new(big.Int).SetUint64(communityVoteScore)
	communityVoteScoreInt = new(big.Int).Mul(communityVoteScoreInt, big.NewInt(enhancedCommunityVotesWeight))
	communityVoteScoreInt = new(big.Int).Div(communityVoteScoreInt, big.NewInt(1000))

	// 4. 네트워크 기여도 점수 (10%)
	contribScore := uint64(0)
	if reputation.NetworkContribPoints > 0 {
		if reputation.NetworkContribPoints > maxNetworkContribPoints {
			contribScore = 1000
		} else {
			contribScore = uint64(float64(reputation.NetworkContribPoints) / float64(maxNetworkContribPoints) * 1000)
		}
	}

	contribScoreInt := new(big.Int).SetUint64(contribScore)
	contribScoreInt = new(big.Int).Mul(contribScoreInt, big.NewInt(enhancedNetworkContribWeight))
	contribScoreInt = new(big.Int).Div(contribScoreInt, big.NewInt(1000))

	// 평판 점수 합산
	reputationScore = new(big.Int).Add(reputationScore, slashingScoreInt)
	reputationScore = new(big.Int).Add(reputationScore, ageScoreInt)
	reputationScore = new(big.Int).Add(reputationScore, communityVoteScoreInt)
	reputationScore = new(big.Int).Add(reputationScore, contribScoreInt)

	// 평판 가중치 적용
	reputationScore = new(big.Int).Mul(reputationScore, big.NewInt(enhancedReputationWeightRatio))
	reputationScore = new(big.Int).Div(reputationScore, big.NewInt(1000))

	return reputationScore
}

// selectEnhancedValidators는 향상된 알고리즘을 사용하여 다음 에포크의 검증자를 선택합니다.
func (vs *ValidatorSet) selectEnhancedValidators(totalBlocks uint64, currentBlock uint64) []*Validator {
	// 모든 활성 검증자를 슬라이스로 변환
	validators := vs.getActiveValidators()

	// 향상된 검증자 정보 생성
	enhancedValidators := make([]*EnhancedValidator, len(validators))
	for i, validator := range validators {
		// 기본 정보 복사
		enhancedValidators[i] = &EnhancedValidator{
			Validator: validator,
		}

		// 향상된 성능 지표 설정 (실제 구현에서는 DB에서 로드)
		// 여기서는 예시로 기본 지표를 복사
		enhancedValidators[i].EnhancedStats = EnhancedValidatorStats{
			BlocksProposed:  validator.BlocksProposed,
			BlocksSigned:    validator.BlocksSigned,
			BlocksMissed:    validator.BlocksMissed,
			Uptime:          validator.Uptime,
			GovernanceVotes: validator.GovernanceVotes,
			ResponseTimes:   []uint64{500, 600, 450, 700}, // 예시 데이터
			AvgResponseTime: 550,                          // 예시 데이터
			BlocksOrphaned:  0,                            // 예시 데이터
			LastActiveBlock: currentBlock,                 // 예시 데이터
		}

		// 평판 지표 설정 (실제 구현에서는 DB에서 로드)
		enhancedValidators[i].Reputation = ValidatorReputationStats{
			SlashingHistory:      []SlashingEvent{},    // 예시 데이터
			LastSlashingBlock:    0,                    // 예시 데이터
			TotalSlashingAmount:  big.NewInt(0),        // 예시 데이터
			ActivationBlock:      currentBlock - 10000, // 예시 데이터
			TotalActiveBlocks:    10000,                // 예시 데이터
			CommunityVotes:       500,                  // 예시 데이터
			NetworkContribPoints: 300,                  // 예시 데이터
			ContribCategories: map[string]uint64{ // 예시 데이터
				"codeContribution": 100,
				"communitySupport": 100,
				"documentation":    50,
				"testing":          50,
			},
		}
	}

	// 각 검증자의 점수 계산
	type validatorScore struct {
		validator *Validator
		score     *big.Int
	}

	scores := make([]validatorScore, len(enhancedValidators))
	for i, enhancedValidator := range enhancedValidators {
		scores[i] = validatorScore{
			validator: enhancedValidator.Validator,
			score:     calculateEnhancedValidatorScore(enhancedValidator, totalBlocks, currentBlock),
		}
	}

	// 점수에 따라 정렬
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score.Cmp(scores[j].score) > 0
	})

	// 상위 maxValidators개 선택
	result := make([]*Validator, 0, maxValidators)
	for i := 0; i < len(scores) && i < maxValidators; i++ {
		result = append(result, scores[i].validator)
	}

	log.Info("Enhanced validator selection completed",
		"totalCandidates", len(validators),
		"selectedCount", len(result),
		"topScore", scores[0].score,
		"bottomScore", scores[len(result)-1].score)

	return result
}

// updateEnhancedValidatorPerformance는 검증자의 향상된 성능 지표를 업데이트합니다.
func (vs *ValidatorSet) updateEnhancedValidatorPerformance(header *types.Header, proposer common.Address, signers []common.Address, responseTime uint64) {
	// 기본 성능 지표 업데이트
	vs.updateValidatorPerformance(header, proposer, signers)

	// 향상된 성능 지표 업데이트 (실제 구현에서는 DB에 저장)
	// 여기서는 로깅만 수행
	log.Debug("Enhanced validator performance updated",
		"blockNumber", header.Number.Uint64(),
		"proposer", proposer,
		"signerCount", len(signers),
		"responseTime", responseTime)
}

// addNetworkContribution은 검증자의 네트워크 기여도를 추가합니다.
func (vs *ValidatorSet) addNetworkContribution(validator common.Address, category string, points uint64) {
	log.Info("Network contribution added",
		"validator", validator,
		"category", category,
		"points", points)

	// 실제 구현에서는 DB에 저장
}

// addCommunityVote는 검증자에 대한 커뮤니티 투표를 추가합니다.
func (vs *ValidatorSet) addCommunityVote(validator common.Address, vote int64) {
	log.Info("Community vote added",
		"validator", validator,
		"vote", vote)

	// 실제 구현에서는 DB에 저장
}
