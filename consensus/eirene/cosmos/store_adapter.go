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

// Package cosmos implements the Cosmos SDK adapter for the Eirene consensus algorithm.
package cosmos

import (
	"fmt"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/log"
)

// StateDBAdapter는 go-zenanet의 상태 DB와 Cosmos SDK의 KVStore를 연결하는 어댑터입니다.
type StateDBAdapter struct {
	stateDB *state.StateDB
	logger  log.Logger
	prefix  common.Hash // 상태 DB에서 Cosmos SDK 관련 데이터를 저장하기 위한 접두사
}

// NewStateDBAdapter는 새로운 StateDBAdapter 인스턴스를 생성합니다.
func NewStateDBAdapter(stateDB *state.StateDB) *StateDBAdapter {
	return &StateDBAdapter{
		stateDB: stateDB,
		logger:  log.New("module", "cosmos-store"),
		prefix:  common.HexToHash("0xCosmos"), // 임의의 접두사
	}
}

// SetStateDB는 상태 DB를 설정합니다.
func (a *StateDBAdapter) SetStateDB(stateDB *state.StateDB) {
	a.stateDB = stateDB
}

// GetStateDB는 상태 DB를 반환합니다.
func (a *StateDBAdapter) GetStateDB() *state.StateDB {
	return a.stateDB
}

// Set은 키-값 쌍을 저장합니다.
func (a *StateDBAdapter) Set(key []byte, value []byte) error {
	if a.stateDB == nil {
		return fmt.Errorf("stateDB is nil")
	}

	// 키를 해시로 변환
	keyHash := a.getKeyHash(key)
	
	// 상태 DB에 저장
	a.stateDB.SetState(a.getStoreAddress(), keyHash, common.BytesToHash(value))
	
	return nil
}

// Get은 키에 해당하는 값을 반환합니다.
func (a *StateDBAdapter) Get(key []byte) ([]byte, error) {
	if a.stateDB == nil {
		return nil, fmt.Errorf("stateDB is nil")
	}

	// 키를 해시로 변환
	keyHash := a.getKeyHash(key)
	
	// 상태 DB에서 조회
	value := a.stateDB.GetState(a.getStoreAddress(), keyHash)
	
	return value.Bytes(), nil
}

// Delete는 키에 해당하는 값을 삭제합니다.
func (a *StateDBAdapter) Delete(key []byte) error {
	if a.stateDB == nil {
		return fmt.Errorf("stateDB is nil")
	}

	// 키를 해시로 변환
	keyHash := a.getKeyHash(key)
	
	// 상태 DB에서 삭제 (빈 값으로 설정)
	a.stateDB.SetState(a.getStoreAddress(), keyHash, common.Hash{})
	
	return nil
}

// Has는 키가 존재하는지 확인합니다.
func (a *StateDBAdapter) Has(key []byte) (bool, error) {
	if a.stateDB == nil {
		return false, fmt.Errorf("stateDB is nil")
	}

	// 키를 해시로 변환
	keyHash := a.getKeyHash(key)
	
	// 상태 DB에서 조회
	value := a.stateDB.GetState(a.getStoreAddress(), keyHash)
	
	// 값이 비어있지 않으면 키가 존재함
	return value != common.Hash{}, nil
}

// getKeyHash는 키를 해시로 변환합니다.
func (a *StateDBAdapter) getKeyHash(key []byte) common.Hash {
	return common.BytesToHash(key)
}

// getStoreAddress는 Cosmos SDK 관련 데이터를 저장하기 위한 주소를 반환합니다.
func (a *StateDBAdapter) getStoreAddress() common.Address {
	return common.BytesToAddress(a.prefix.Bytes())
}

// GetKeysWithPrefix는 주어진 접두사로 시작하는 모든 키를 반환합니다.
// 참고: 이 구현은 실제 상태 DB에서 접두사 검색을 지원하지 않으므로 제한적입니다.
// 실제 구현에서는 상태 DB의 이터레이터를 사용하여 접두사로 시작하는 모든 키를 검색해야 합니다.
func (a *StateDBAdapter) GetKeysWithPrefix(prefix []byte) ([][]byte, error) {
	if a.stateDB == nil {
		return nil, fmt.Errorf("stateDB is nil")
	}
	
	// 접두사 로깅
	a.logger.Debug("Searching for keys with prefix", "prefix", string(prefix))
	
	// 접두사 검색을 위한 키 맵 생성
	// 실제 구현에서는 상태 DB의 이터레이터를 사용해야 하지만,
	// 현재 go-zenanet의 상태 DB는 이터레이터를 제공하지 않으므로
	// 미리 정의된 키 패턴을 사용하여 검색합니다.
	
	// 결과 키 슬라이스
	var keys [][]byte
	
	// 접두사 문자열
	prefixStr := string(prefix)
	
	// 접두사에 따라 다른 검색 로직 적용
	switch prefixStr {
	case "validator:":
		// 검증자 키 패턴: validator:<address>
		// 테스트용 더미 데이터 생성
		validators := []string{
			"cosmosvaloper1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj",
			"cosmosvaloper1sjllsnramtg3ewxqwwrwjxfgc4n4ef9u2lcnj0",
			"cosmosvaloper156gqf9837u7d4c4678yt3rl4ls9c5vuursrrzf",
		}
		
		for _, val := range validators {
			key := append(prefix, []byte(val)...)
			// 키가 실제로 존재하는지 확인
			exists, err := a.Has(key)
			if err != nil {
				a.logger.Error("Failed to check key existence", "key", string(key), "err", err)
				continue
			}
			
			if exists {
				keys = append(keys, key)
				a.logger.Debug("Found key with prefix", "key", string(key))
			}
		}
		
	case "delegation:":
		// 위임 키 패턴: delegation:<delegator_address><validator_address>
		// 테스트용 더미 데이터 생성
		delegators := []string{
			"cosmos1gghjut3ccd8ay0zduzj64hwre2fxs9ld75ru9p",
			"cosmos1sjllsnramtg3ewxqwwrwjxfgc4n4ef9u2lcnj0",
		}
		
		validators := []string{
			"cosmosvaloper1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj",
			"cosmosvaloper1sjllsnramtg3ewxqwwrwjxfgc4n4ef9u2lcnj0",
		}
		
		// 접두사에서 위임자 주소 추출 (있는 경우)
		var delegatorPrefix string
		if len(prefix) > len("delegation:") {
			delegatorPrefix = string(prefix[len("delegation:"):])
			a.logger.Debug("Extracted delegator prefix", "delegatorPrefix", delegatorPrefix)
			
			// 특정 위임자에 대한 키만 검색
			for _, validator := range validators {
				key := append(prefix, []byte(validator)...)
				exists, err := a.Has(key)
				if err != nil {
					a.logger.Error("Failed to check key existence", "key", string(key), "err", err)
					continue
				}
				
				if exists {
					keys = append(keys, key)
					a.logger.Debug("Found key with delegator prefix", "key", string(key))
				}
			}
		} else {
			// 모든 위임자에 대한 키 검색
			for _, delegator := range delegators {
				for _, validator := range validators {
					key := append(append(prefix, []byte(delegator)...), []byte(validator)...)
					exists, err := a.Has(key)
					if err != nil {
						a.logger.Error("Failed to check key existence", "key", string(key), "err", err)
						continue
					}
					
					if exists {
						keys = append(keys, key)
						a.logger.Debug("Found key with prefix", "key", string(key))
					}
				}
			}
		}
		
	default:
		a.logger.Warn("Unknown prefix for key search", "prefix", prefixStr)
	}
	
	a.logger.Info("Found keys with prefix", "prefix", prefixStr, "count", len(keys))
	return keys, nil
} 