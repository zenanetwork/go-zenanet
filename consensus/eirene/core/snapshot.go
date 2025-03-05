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

package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"sort"

	lru "github.com/hashicorp/golang-lru"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// sigLRU는 서명 캐시를 위한 LRU 캐시 타입입니다.
type sigLRU = *lru.ARCCache

// Snapshot은 특정 시점의 검증자 집합 상태를 나타냅니다.
type Snapshot struct {
	config   *params.EireneConfig // 합의 엔진 구성
	sigcache sigLRU               // 서명 검증 결과 캐시

	Number     uint64                       `json:"number"`     // 스냅샷이 생성된 블록 번호
	Hash       common.Hash                  `json:"hash"`       // 스냅샷이 생성된 블록 해시
	Validators map[common.Address]uint64    `json:"validators"` // 검증자 집합과 각각의 투표 가중치
	Recents    map[uint64]common.Address    `json:"recents"`    // 최근 블록 서명자 집합
	Votes      []*Vote                      `json:"votes"`      // 검증자 집합 변경을 위한 투표 목록
	Tally      map[common.Address]VoteTally `json:"tally"`      // 현재 투표 상태

	// 검증자 선택 알고리즘 개선을 위한 추가 필드
	Stakes         map[common.Address]*big.Int       `json:"stakes"`         // 각 검증자의 스테이킹 양
	Delegations    map[common.Address][]Delegation   `json:"delegations"`    // 각 검증자에게 위임된 스테이킹
	Performance    map[common.Address]ValidatorStats `json:"performance"`    // 검증자 성능 지표
	SlashingPoints map[common.Address]uint64         `json:"slashingPoints"` // 검증자 슬래싱 포인트
}

// VoteTally는 특정 검증자에 대한 투표 상태를 추적합니다.
type VoteTally struct {
	Authorize bool   `json:"authorize"` // 검증자 추가 또는 제거 여부
	Votes     uint64 `json:"votes"`     // 투표 수
}

// Vote는 검증자 집합 변경을 위한 단일 투표를 나타냅니다.
type Vote struct {
	Validator common.Address `json:"validator"` // 투표 대상 검증자 주소
	Block     uint64         `json:"block"`     // 투표가 발생한 블록 번호
	Address   common.Address `json:"address"`   // 투표한 검증자의 주소
	Authorize bool           `json:"authorize"` // 검증자 추가 또는 제거 여부
}

// Delegation은 토큰 소유자가 검증자에게 위임한 스테이킹을 나타냅니다.
type Delegation struct {
	Delegator common.Address `json:"delegator"` // 위임자 주소
	Amount    *big.Int       `json:"amount"`    // 위임 금액
	Since     uint64         `json:"since"`     // 위임 시작 블록 번호
}

// ValidatorStats는 검증자의 성능 지표를 추적합니다.
type ValidatorStats struct {
	BlocksProposed uint64  `json:"blocksProposed"` // 제안한 블록 수
	BlocksMissed   uint64  `json:"blocksMissed"`   // 놓친 블록 수
	Uptime         float64 `json:"uptime"`         // 업타임 비율 (0.0-1.0)
	LastActive     uint64  `json:"lastActive"`     // 마지막 활동 블록 번호
}

// newSnapshot은 주어진 헤더로부터 새로운 스냅샷을 생성합니다.
func newSnapshot(config *params.EireneConfig, sigcache sigLRU, number uint64, hash common.Hash, validators []common.Address) *Snapshot {
	snap := &Snapshot{
		config:         config,
		sigcache:       sigcache,
		Number:         number,
		Hash:           hash,
		Validators:     make(map[common.Address]uint64),
		Recents:        make(map[uint64]common.Address),
		Tally:          make(map[common.Address]VoteTally),
		Stakes:         make(map[common.Address]*big.Int),
		Delegations:    make(map[common.Address][]Delegation),
		Performance:    make(map[common.Address]ValidatorStats),
		SlashingPoints: make(map[common.Address]uint64),
	}

	// 초기 검증자 설정
	for _, validator := range validators {
		snap.Validators[validator] = 1                     // 모든 검증자에게 동일한 가중치 부여
		snap.Stakes[validator] = new(big.Int).SetUint64(1) // 초기 스테이킹 값 설정
		snap.Performance[validator] = ValidatorStats{
			BlocksProposed: 0,
			BlocksMissed:   0,
			Uptime:         1.0,
			LastActive:     number,
		}
		snap.SlashingPoints[validator] = 0
	}

	return snap
}

// loadSnapshot은 데이터베이스에서 스냅샷을 로드합니다.
func loadSnapshot(config *params.EireneConfig, sigcache sigLRU, db ethdb.Database, hash common.Hash) (*Snapshot, error) {
	blob, err := db.Get(append([]byte("eirene-"), hash[:]...))
	if err != nil {
		return nil, err
	}

	snap := new(Snapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config
	snap.sigcache = sigcache

	return snap, nil
}

// store는 스냅샷을 데이터베이스에 저장합니다.
func (s *Snapshot) store(db ethdb.Database) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("eirene-"), s.Hash[:]...), blob)
}

// copy는 스냅샷의 깊은 복사본을 생성합니다.
func (s *Snapshot) copy() *Snapshot {
	cpy := &Snapshot{
		config:         s.config,
		sigcache:       s.sigcache,
		Number:         s.Number,
		Hash:           s.Hash,
		Validators:     make(map[common.Address]uint64),
		Recents:        make(map[uint64]common.Address),
		Votes:          make([]*Vote, len(s.Votes)),
		Tally:          make(map[common.Address]VoteTally),
		Stakes:         make(map[common.Address]*big.Int),
		Delegations:    make(map[common.Address][]Delegation),
		Performance:    make(map[common.Address]ValidatorStats),
		SlashingPoints: make(map[common.Address]uint64),
	}

	for validator, weight := range s.Validators {
		cpy.Validators[validator] = weight
	}
	for block, validator := range s.Recents {
		cpy.Recents[block] = validator
	}
	for address, tally := range s.Tally {
		cpy.Tally[address] = tally
	}
	for validator, stake := range s.Stakes {
		cpy.Stakes[validator] = new(big.Int).Set(stake)
	}
	for validator, delegations := range s.Delegations {
		cpy.Delegations[validator] = make([]Delegation, len(delegations))
		for i, delegation := range delegations {
			cpy.Delegations[validator][i] = Delegation{
				Delegator: delegation.Delegator,
				Amount:    new(big.Int).Set(delegation.Amount),
				Since:     delegation.Since,
			}
		}
	}
	for validator, stats := range s.Performance {
		cpy.Performance[validator] = stats
	}
	for validator, points := range s.SlashingPoints {
		cpy.SlashingPoints[validator] = points
	}
	copy(cpy.Votes, s.Votes)

	return cpy
}

// apply는 새 헤더를 기존 스냅샷에 적용하여 새 스냅샷을 생성합니다.
func (s *Snapshot) apply(headers []*types.Header) (*Snapshot, error) {
	// 헤더가 없으면 원본 스냅샷 반환
	if len(headers) == 0 {
		return s, nil
	}

	// 헤더가 연속적인지 확인
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, errInvalidVotingChain
		}
	}
	if headers[0].Number.Uint64() != s.Number+1 {
		return nil, errInvalidVotingChain
	}

	// 모든 헤더를 순차적으로 적용
	snap := s.copy()

	for _, header := range headers {
		// 서명자 추출
		signer, err := ecrecover(header, snap.sigcache)
		if err != nil {
			return nil, err
		}

		// 서명자가 검증자인지 확인
		if _, ok := snap.Validators[signer]; !ok {
			return nil, errUnauthorizedSigner
		}

		// 최근 서명자 목록에 추가
		for i := 0; i < int(snap.config.Epoch) && i < int(header.Number.Uint64()); i++ {
			if recent, ok := snap.Recents[header.Number.Uint64()-uint64(i)]; ok && recent == signer {
				return nil, errRecentlySigned
			}
		}
		snap.Recents[header.Number.Uint64()] = signer

		// 검증자 성능 지표 업데이트
		if stats, ok := snap.Performance[signer]; ok {
			stats.BlocksProposed++
			stats.LastActive = header.Number.Uint64()
			snap.Performance[signer] = stats
		}

		// 놓친 블록 처리 (차례였지만 블록을 생성하지 않은 검증자)
		expectedSigner := snap.expectedSigner(header.Number.Uint64() - 1)
		if expectedSigner != signer && expectedSigner != (common.Address{}) {
			if stats, ok := snap.Performance[expectedSigner]; ok {
				stats.BlocksMissed++
				// 업타임 계산 업데이트
				totalBlocks := stats.BlocksProposed + stats.BlocksMissed
				if totalBlocks > 0 {
					stats.Uptime = float64(stats.BlocksProposed) / float64(totalBlocks)
				}
				snap.Performance[expectedSigner] = stats

				// 놓친 블록에 대한 슬래싱 포인트 추가
				if snap.config.MissedBlockPenalty > 0 {
					snap.SlashingPoints[expectedSigner] += snap.config.MissedBlockPenalty

					// 슬래싱 포인트가 임계값을 초과하면 검증자에게 슬래싱 적용
					if snap.config.SlashingThreshold > 0 && snap.SlashingPoints[expectedSigner] > snap.config.SlashingThreshold {
						snap.slashValidator(expectedSigner, snap.SlashingPoints[expectedSigner]/snap.config.SlashingThreshold)
					}
				}
			}
		}

		// 에포크 경계에서 투표 처리
		if header.Number.Uint64()%snap.config.Epoch == 0 {
			// 투표 처리 및 검증자 집합 업데이트
			snap.Votes = nil
			snap.Tally = make(map[common.Address]VoteTally)

			// 오래된 최근 서명자 정보 제거
			for i := uint64(0); i < snap.config.Epoch; i++ {
				delete(snap.Recents, header.Number.Uint64()-i)
			}

			// 검증자 성능 기반 재정렬
			snap.reorderValidators()

			// 에포크마다 슬래싱 포인트 일부 감소 (복구 기회 제공)
			for validator := range snap.SlashingPoints {
				if snap.SlashingPoints[validator] > 0 {
					// 슬래싱 포인트의 10%를 감소
					snap.SlashingPoints[validator] = snap.SlashingPoints[validator] * 9 / 10
				}
			}
		}

		// 스냅샷 정보 업데이트
		snap.Number = header.Number.Uint64()
		snap.Hash = header.Hash()
	}

	return snap, nil
}

// validators는 현재 활성 검증자 목록을 반환합니다.
func (s *Snapshot) validators() []common.Address {
	validators := make([]common.Address, 0, len(s.Validators))
	for validator := range s.Validators {
		validators = append(validators, validator)
	}

	// 주소 순으로 정렬
	sort.Sort(addressByHash(validators))
	return validators
}

// inturn은 주어진 블록 번호에서 지정된 서명자가 차례인지 확인합니다.
func (s *Snapshot) inturn(number uint64, signer common.Address) bool {
	validators := s.validators()
	if len(validators) == 0 {
		return false
	}

	// 검증자 순서 결정
	idx := (number % uint64(len(validators)))
	return validators[idx] == signer
}

// addressByHash는 주소 정렬을 위한 헬퍼 타입입니다.
type addressByHash []common.Address

func (a addressByHash) Len() int           { return len(a) }
func (a addressByHash) Less(i, j int) bool { return bytes.Compare(a[i][:], a[j][:]) < 0 }
func (a addressByHash) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// errInvalidVotingChain은 투표 체인이 유효하지 않을 때 반환됩니다.
var errInvalidVotingChain = errors.New("invalid voting chain")

// errUnauthorizedSigner는 서명자가 검증자가 아닐 때 반환됩니다.
var errUnauthorizedSigner = errors.New("unauthorized signer")

// expectedSigner는 주어진 블록 번호에서 예상되는 서명자를 반환합니다.
func (s *Snapshot) expectedSigner(number uint64) common.Address {
	validators := s.weightedValidators()
	if len(validators) == 0 {
		return common.Address{}
	}

	// 검증자 순서 결정 (가중치 기반)
	var totalWeight uint64
	for _, validator := range validators {
		totalWeight += s.Validators[validator]
	}

	// 블록 번호를 시드로 사용하여 검증자 선택
	// 가중치가 높은 검증자가 더 자주 선택됨
	seed := number
	pick := seed % totalWeight

	var cumulative uint64
	for _, validator := range validators {
		cumulative += s.Validators[validator]
		if pick < cumulative {
			return validator
		}
	}

	// 기본값으로 첫 번째 검증자 반환
	return validators[0]
}

// weightedValidators는 가중치 기반으로 정렬된 검증자 목록을 반환합니다.
func (s *Snapshot) weightedValidators() []common.Address {
	validators := make([]common.Address, 0, len(s.Validators))
	for validator := range s.Validators {
		validators = append(validators, validator)
	}

	// 스테이킹 양과 성능 지표를 기반으로 정렬
	sort.Slice(validators, func(i, j int) bool {
		vi, vj := validators[i], validators[j]

		// 스테이킹 양 비교
		stakeI := s.Stakes[vi]
		stakeJ := s.Stakes[vj]

		// 스테이킹 양이 다르면 더 많은 스테이킹을 우선시
		if stakeI.Cmp(stakeJ) != 0 {
			return stakeI.Cmp(stakeJ) > 0
		}

		// 스테이킹 양이 같으면 성능 지표 비교
		perfI := s.Performance[vi]
		perfJ := s.Performance[vj]

		// 업타임이 다르면 더 높은 업타임을 우선시
		if perfI.Uptime != perfJ.Uptime {
			return perfI.Uptime > perfJ.Uptime
		}

		// 모든 조건이 같으면 주소로 정렬
		return bytes.Compare(vi[:], vj[:]) < 0
	})

	return validators
}

// reorderValidators는 검증자 가중치를 성능 지표에 따라 재조정합니다.
func (s *Snapshot) reorderValidators() {
	validators := s.weightedValidators()

	// 성능 지표에 따라 가중치 재조정
	for _, validator := range validators {
		stats := s.Performance[validator]
		stake := s.Stakes[validator]

		// 기본 가중치는 1
		weight := uint64(1)

		// 스테이킹 양에 따른 가중치 증가
		stakeWeight := stake.Uint64() / 1e18 // 1 ETH 단위로 나눔
		if stakeWeight > 0 {
			weight += stakeWeight
		}

		// 성능 지표에 따른 가중치 조정
		if stats.Uptime >= 0.99 {
			weight += 2 // 높은 업타임에 대한 보너스
		} else if stats.Uptime >= 0.95 {
			weight += 1 // 양호한 업타임에 대한 보너스
		} else if stats.Uptime < 0.8 {
			weight = weight / 2 // 낮은 업타임에 대한 페널티
		}

		// 슬래싱 포인트에 따른 가중치 감소
		if s.SlashingPoints[validator] > 0 {
			penalty := s.SlashingPoints[validator]
			if penalty >= weight {
				weight = 1 // 최소 가중치는 1
			} else {
				weight -= penalty
			}
		}

		// 가중치 업데이트
		s.Validators[validator] = weight
	}
}

// addStake는 검증자의 스테이킹 양을 증가시킵니다.
func (s *Snapshot) addStake(validator common.Address, amount *big.Int) {
	if _, ok := s.Stakes[validator]; !ok {
		s.Stakes[validator] = new(big.Int)
	}
	s.Stakes[validator] = new(big.Int).Add(s.Stakes[validator], amount)
}

// removeStake는 검증자의 스테이킹 양을 감소시킵니다.
func (s *Snapshot) removeStake(validator common.Address, amount *big.Int) error {
	if _, ok := s.Stakes[validator]; !ok {
		return errors.New("validator not found")
	}

	if s.Stakes[validator].Cmp(amount) < 0 {
		return errors.New("insufficient stake")
	}

	s.Stakes[validator] = new(big.Int).Sub(s.Stakes[validator], amount)
	return nil
}

// addDelegation은 검증자에게 위임을 추가합니다.
func (s *Snapshot) addDelegation(validator, delegator common.Address, amount *big.Int, blockNumber uint64) {
	if _, ok := s.Delegations[validator]; !ok {
		s.Delegations[validator] = make([]Delegation, 0)
	}

	// 기존 위임이 있는지 확인
	for i, delegation := range s.Delegations[validator] {
		if delegation.Delegator == delegator {
			// 기존 위임 업데이트
			s.Delegations[validator][i].Amount = new(big.Int).Add(delegation.Amount, amount)
			return
		}
	}

	// 새 위임 추가
	s.Delegations[validator] = append(s.Delegations[validator], Delegation{
		Delegator: delegator,
		Amount:    new(big.Int).Set(amount),
		Since:     blockNumber,
	})

	// 검증자의 총 스테이킹 양 증가
	s.addStake(validator, amount)
}

// removeDelegation은 검증자로부터 위임을 제거합니다.
func (s *Snapshot) removeDelegation(validator, delegator common.Address, amount *big.Int) error {
	if _, ok := s.Delegations[validator]; !ok {
		return errors.New("validator has no delegations")
	}

	for i, delegation := range s.Delegations[validator] {
		if delegation.Delegator == delegator {
			if delegation.Amount.Cmp(amount) < 0 {
				return errors.New("insufficient delegation amount")
			}

			// 위임 금액 감소
			newAmount := new(big.Int).Sub(delegation.Amount, amount)

			if newAmount.Sign() == 0 {
				// 위임 완전 제거
				s.Delegations[validator] = append(s.Delegations[validator][:i], s.Delegations[validator][i+1:]...)
			} else {
				// 위임 금액 업데이트
				s.Delegations[validator][i].Amount = newAmount
			}

			// 검증자의 총 스테이킹 양 감소
			return s.removeStake(validator, amount)
		}
	}

	return errors.New("delegation not found")
}

// slashValidator는 검증자에게 슬래싱 페널티를 적용합니다.
func (s *Snapshot) slashValidator(validator common.Address, severity uint64) {
	// 슬래싱 포인트 증가
	s.SlashingPoints[validator] += severity

	// 심각도에 따라 스테이킹 양 감소
	if stake, ok := s.Stakes[validator]; ok && stake.Sign() > 0 {
		var slashRate uint64 = 1000 // 기본값: 0.1% (1/1000)

		// 구성에서 슬래싱 비율 가져오기
		if s.config.SlashingRate > 0 {
			slashRate = s.config.SlashingRate
		}

		// 스테이킹의 일정 비율을 슬래싱
		slashAmount := new(big.Int).Div(stake, new(big.Int).SetUint64(slashRate))
		slashAmount = new(big.Int).Mul(slashAmount, new(big.Int).SetUint64(severity))

		if slashAmount.Sign() > 0 && slashAmount.Cmp(stake) < 0 {
			s.Stakes[validator] = new(big.Int).Sub(stake, slashAmount)

			// 슬래싱 이벤트 로깅 (실제 구현에서는 이벤트 시스템 사용)
			log.Info("Validator slashed",
				"validator", validator.Hex(),
				"severity", severity,
				"amount", slashAmount,
				"remaining_stake", s.Stakes[validator])
		}
	}

	// 가중치 감소
	if s.Validators[validator] > 1 {
		s.Validators[validator]--
	}

	// 심각한 위반의 경우 검증자 제거 고려
	if severity > 10 {
		// 검증자 제거 로직 (투표 시스템을 통해 처리)
		s.Votes = append(s.Votes, &Vote{
			Validator: validator,
			Block:     s.Number,
			Address:   validator, // 자체 투표 (시스템에 의한)
			Authorize: false,     // 제거 투표
		})

		// 투표 집계 업데이트
		tally, ok := s.Tally[validator]
		if !ok {
			tally = VoteTally{Authorize: false, Votes: 0}
		}
		tally.Votes++
		s.Tally[validator] = tally
	}
}
