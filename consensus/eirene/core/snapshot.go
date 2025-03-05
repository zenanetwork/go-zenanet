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
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/params"
)

// sigLRU는 서명 캐시를 위한 LRU 캐시 타입입니다.
type sigLRU = *lru.ARCCache

// Snapshot은 특정 시점의 검증자 상태를 나타냅니다
type Snapshot struct {
	config *params.EireneConfig // 합의 엔진 구성

	Number     uint64                      // 스냅샷이 생성된 블록 번호
	Hash       common.Hash                 // 스냅샷이 생성된 블록 해시
	Validators map[common.Address]uint64   // 검증자 집합 (주소 -> 투표 가중치)
	Recents    map[uint64]common.Address   // 최근 블록 서명자 집합 (블록 번호 -> 서명자)
	Votes      map[common.Address][]Vote   // 검증자 추가/제거를 위한 투표 집합
	Tally      map[common.Address]Tally    // 현재 투표 결과

	// 스테이킹 및 성능 정보
	Stakes         map[common.Address]*big.Int                           // 검증자별 스테이킹 양
	Performance    map[common.Address]*ValidatorPerformance              // 검증자별 성능 지표
	SlashingPoints map[common.Address]uint64                             // 검증자별 슬래싱 포인트
	Delegations    map[common.Address]map[common.Address]*ValidatorDelegation // 검증자별 위임 정보 (검증자 -> 위임자 -> 위임 정보)

	// 캐시
	validatorList []common.Address // 정렬된 검증자 목록 (캐시)
	lock          sync.RWMutex     // 동시성 제어를 위한 잠금
}

// Vote는 검증자 추가/제거를 위한 단일 투표를 나타냅니다
type Vote struct {
	Validator common.Address // 투표 대상 검증자
	Block     uint64         // 투표가 발생한 블록 번호
	Authorize bool           // 추가(true) 또는 제거(false)
}

// Tally는 투표 결과를 나타냅니다
type Tally struct {
	Authorize bool            // 추가(true) 또는 제거(false)
	Votes     uint64          // 투표 수
	Voters    []common.Address // 투표자 목록
}

// newSnapshot은 새로운 스냅샷 인스턴스를 생성합니다
func newSnapshot(config *params.EireneConfig, number uint64, hash common.Hash, validators map[common.Address]uint64) *Snapshot {
	snap := &Snapshot{
		config:         config,
		Number:         number,
		Hash:           hash,
		Validators:     validators,
		Recents:        make(map[uint64]common.Address),
		Votes:          make(map[common.Address][]Vote),
		Tally:          make(map[common.Address]Tally),
		Stakes:         make(map[common.Address]*big.Int),
		Performance:    make(map[common.Address]*ValidatorPerformance),
		SlashingPoints: make(map[common.Address]uint64),
		Delegations:    make(map[common.Address]map[common.Address]*ValidatorDelegation),
	}
	return snap
}

// loadSnapshot은 데이터베이스에서 스냅샷을 로드합니다
func loadSnapshot(config *params.EireneConfig, db ethdb.Database, hash common.Hash) (*Snapshot, error) {
	blob, err := db.Get(append([]byte("eirene-"), hash[:]...))
	if err != nil {
		return nil, err
	}
	snap := new(Snapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	snap.config = config

	return snap, nil
}

// store는 스냅샷을 데이터베이스에 저장합니다
func (s *Snapshot) store(db ethdb.Database) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.Put(append([]byte("eirene-"), s.Hash[:]...), blob)
}

// copy는 스냅샷의 복사본을 생성합니다
func (s *Snapshot) copy() *Snapshot {
	s.lock.RLock()
	defer s.lock.RUnlock()

	cpy := &Snapshot{
		config:     s.config,
		Number:     s.Number,
		Hash:       s.Hash,
		Validators: make(map[common.Address]uint64),
		Recents:    make(map[uint64]common.Address),
		Votes:      make(map[common.Address][]Vote),
		Tally:      make(map[common.Address]Tally),
	}

	for validator, weight := range s.Validators {
		cpy.Validators[validator] = weight
	}
	for block, validator := range s.Recents {
		cpy.Recents[block] = validator
	}
	for validator, votes := range s.Votes {
		cpy.Votes[validator] = make([]Vote, len(votes))
		copy(cpy.Votes[validator], votes)
	}
	for validator, tally := range s.Tally {
		cpy.Tally[validator] = Tally{
			Authorize: tally.Authorize,
			Votes:     tally.Votes,
			Voters:    make([]common.Address, len(tally.Voters)),
		}
		copy(cpy.Tally[validator].Voters, tally.Voters)
	}
	if s.validatorList != nil {
		cpy.validatorList = make([]common.Address, len(s.validatorList))
		copy(cpy.validatorList, s.validatorList)
	}
	return cpy
}

// apply는 헤더를 스냅샷에 적용하여 새로운 스냅샷을 생성합니다
func (s *Snapshot) apply(header *types.Header) (*Snapshot, error) {
	// 헤더 검증
	if header.Number.Uint64() != s.Number+1 {
		return nil, errors.New("invalid header number")
	}
	if header.ParentHash != s.Hash {
		return nil, errors.New("invalid header parent hash")
	}

	// 새 스냅샷 생성
	snap := s.copy()
	snap.Number = header.Number.Uint64()
	snap.Hash = header.Hash()

	// 서명자 가져오기
	signer, err := ecrecover(header, nil)
	if err != nil {
		return nil, err
	}

	// 최근 서명자 목록 업데이트
	// 실제 구현에서는 적절한 방식으로 최근 서명자 목록을 관리해야 합니다.
	// 여기서는 임시로 간단하게 구현합니다.
	snap.Recents[header.Number.Uint64()] = signer

	// 투표 처리
	// 실제 구현에서는 헤더의 엑스트라 데이터에서 투표 정보를 추출하여 처리해야 합니다.
	// 여기서는 임시로 이 부분을 생략합니다.

	return snap, nil
}

// validators는 정렬된 검증자 목록을 반환합니다
func (s *Snapshot) validators() []common.Address {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.validatorList != nil {
		return s.validatorList
	}

	s.validatorList = make([]common.Address, 0, len(s.Validators))
	for validator := range s.Validators {
		s.validatorList = append(s.validatorList, validator)
	}
	sort.Sort(validatorsAscending(s.validatorList))
	return s.validatorList
}

// inturn은 지정된 블록 번호에서 주어진 검증자가 블록을 생성할 차례인지 확인합니다
func (s *Snapshot) inturn(number uint64, validator common.Address) bool {
	validators := s.validators()
	if len(validators) == 0 {
		return false
	}
	idx := number % uint64(len(validators))
	return validators[idx] == validator
}

// validatorsAscending은 검증자 주소를 오름차순으로 정렬하기 위한 타입입니다
type validatorsAscending []common.Address

func (s validatorsAscending) Len() int           { return len(s) }
func (s validatorsAscending) Less(i, j int) bool { return bytes.Compare(s[i][:], s[j][:]) < 0 }
func (s validatorsAscending) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

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
		s.Delegations[validator] = make(map[common.Address]*ValidatorDelegation)
	}

	// 기존 위임이 있는지 확인
	if delegation, exists := s.Delegations[validator][delegator]; exists {
		// 기존 위임 업데이트
		delegation.Amount = new(big.Int).Add(delegation.Amount, amount)
	} else {
		// 새 위임 추가
		s.Delegations[validator][delegator] = &ValidatorDelegation{
			Amount: new(big.Int).Set(amount),
			Since:  blockNumber,
		}
	}

	// 검증자의 총 스테이킹 양 증가
	s.addStake(validator, amount)
}

// removeDelegation은 검증자로부터 위임을 제거합니다.
func (s *Snapshot) removeDelegation(validator, delegator common.Address) error {
	if _, ok := s.Delegations[validator]; !ok {
		return errors.New("validator has no delegations")
	}

	if _, exists := s.Delegations[validator][delegator]; !exists {
		return errors.New("delegation not found")
	}

	// 위임 제거
	delete(s.Delegations[validator], delegator)

	// 검증자의 총 스테이킹 양 감소
	return s.removeStake(validator, s.Delegations[validator][delegator].Amount)
}

// slashValidator는 검증자에게 슬래싱 포인트를 부여합니다
func (s *Snapshot) slashValidator(validator common.Address, severity uint64) {
	// 슬래싱 포인트 추가
	if _, ok := s.SlashingPoints[validator]; !ok {
		s.SlashingPoints[validator] = 0
	}
	s.SlashingPoints[validator] += severity

	// 심각도가 높은 경우 추가 조치
	if severity > 10 {
		// 검증자 제거 로직 (투표 시스템을 통해 처리)
		// 실제 구현에서는 적절한 방식으로 검증자 제거 투표를 생성해야 합니다.
		// 여기서는 임시로 이 부분을 생략합니다.
	}
}
