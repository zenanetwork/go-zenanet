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

package core_test

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/params"
)

// 테스트 파일에서 필요한 상수를 정의합니다
const (
	extraVanity = 32
	extraSeal   = 65
)

// 테스트 함수에서 core.Eirene 타입의 필드에 접근할 때 필요한 헬퍼 함수를 추가합니다
func getSignerFn(engine *core.Eirene) core.SignerFn {
	// 공개 API를 사용하여 필드에 접근
	return engine.GetSignerFn()
}

func getSigner(engine *core.Eirene) common.Address {
	// 공개 API를 사용하여 필드에 접근
	return engine.GetSigner()
}

// 테스트 계정 생성
func generateTestKey() (*ecdsa.PrivateKey, common.Address) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return key, addr
}

// 테스트용 서명 함수 생성
func signerFn(key *ecdsa.PrivateKey) core.SignerFn {
	return func(signer common.Address, hash []byte) ([]byte, error) {
		return crypto.Sign(hash, key)
	}
}

// TestEireneNew는 Eirene 합의 엔진 생성을 테스트합니다.
func TestEireneNew(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// 테스트 설정 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	_ = core.New(config, db)

	// 기본 검증
	if config.Period != 15 {
		t.Errorf("기대한 Period 값 %d, 실제 값 %d", 15, config.Period)
	}
	if config.Epoch != 30000 {
		t.Errorf("기대한 Epoch 값 %d, 실제 값 %d", 30000, config.Epoch)
	}
}

// TestEireneNewWithZeroPeriod는 Period가 0인 경우 기본값이 설정되는지 테스트합니다.
func TestEireneNewWithZeroPeriod(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// Period가 0인 설정 생성
	config := &params.EireneConfig{
		Period: 0,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := core.New(config, db)

	// 기본값 검증 (defaultPeriod는 15로 가정)
	expectedPeriod := uint64(15)
	actualPeriod := engine.GetConfig().Period
	if actualPeriod != expectedPeriod {
		t.Errorf("Period가 기본값으로 설정되지 않음: 기대값 %d, 실제값 %d", expectedPeriod, actualPeriod)
	}
}

// TestEireneNewWithZeroEpoch는 Epoch가 0인 경우 기본값이 설정되는지 테스트합니다.
func TestEireneNewWithZeroEpoch(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// Epoch가 0인 설정 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  0,
	}

	// Eirene 엔진 생성
	engine := core.New(config, db)

	// 기본값 검증 (defaultEpoch는 30000으로 가정)
	expectedEpoch := uint64(30000)
	actualEpoch := engine.GetConfig().Epoch
	if actualEpoch != expectedEpoch {
		t.Errorf("Epoch가 기본값으로 설정되지 않음: 기대값 %d, 실제값 %d", expectedEpoch, actualEpoch)
	}
}

// TestEireneSealHash는 SealHash 함수를 테스트합니다.
func TestEireneSealHash(t *testing.T) {
	// 테스트 헤더 생성
	header := &types.Header{
		Number:     big.NewInt(1),
		Difficulty: big.NewInt(1),
		Time:       1234,
		Extra:      make([]byte, extraVanity+extraSeal),
	}

	// SealHash 계산
	hash := core.SealHash(header)

	// 결과 검증 - 실제 해시 값은 구현에 따라 다를 수 있음
	emptyHash := common.Hash{}
	if hash == emptyHash {
		t.Error("SealHash가 빈 해시를 반환함")
	}
}

// TestEireneSealHashWithEmptyHeader는 최소한의 필드만 가진 헤더에 대한 SealHash 함수를 테스트합니다.
func TestEireneSealHashWithEmptyHeader(t *testing.T) {
	// 최소한의 필드만 가진 헤더 생성
	header := &types.Header{
		Extra: make([]byte, extraVanity+extraSeal),
	}

	// SealHash 계산
	hash := core.SealHash(header)

	// 결과 검증 - 최소한의 필드만 가진 헤더에 대해서도 유효한 해시가 생성되어야 함
	emptyHash := common.Hash{}
	if hash == emptyHash {
		t.Error("최소한의 필드만 가진 헤더에 대해 SealHash가 빈 해시를 반환함")
	}
}

// TestEireneAuthor는 Author 메서드를 테스트합니다.
func TestEireneAuthor(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// 테스트 설정 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := core.New(config, db)

	// 테스트 키 생성
	key, addr := generateTestKey()

	// 서명 함수 설정 - 이제 공개 API를 사용할 수 있습니다
	engine.SetSigner(addr)
	engine.SetSignerFn(signerFn(key))

	// 테스트 헤더 생성
	header := &types.Header{
		Number:     big.NewInt(1),
		Difficulty: big.NewInt(1),
		Time:       1,
		Extra:      make([]byte, extraVanity+extraSeal),
	}

	// 헤더 서명
	sealHash := core.SealHash(header)
	signature, err := crypto.Sign(sealHash.Bytes(), key)
	if err != nil {
		t.Fatalf("서명 생성 실패: %v", err)
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], signature)

	// Author 메서드 테스트
	author, err := engine.Author(header)
	if err != nil {
		t.Fatalf("Author 메서드 호출 실패: %v", err)
	}

	if author != addr {
		t.Errorf("Author 주소가 일치하지 않음: 기대값 %s, 실제값 %s", addr.Hex(), author.Hex())
	}
}

// TestEireneAuthorWithInvalidSignature는 잘못된 서명이 있는 헤더에 대한 Author 메서드를 테스트합니다.
func TestEireneAuthorWithInvalidSignature(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// 테스트 설정 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := core.New(config, db)

	// 테스트 헤더 생성 (잘못된 서명)
	header := &types.Header{
		Number:     big.NewInt(1),
		Difficulty: big.NewInt(1),
		Time:       1,
		Extra:      make([]byte, extraVanity+extraSeal),
	}

	// 잘못된 서명 설정 (모두 0으로 채움)
	invalidSignature := make([]byte, extraSeal)
	copy(header.Extra[len(header.Extra)-extraSeal:], invalidSignature)

	// Author 메서드 테스트 - 오류가 발생해야 함
	_, err := engine.Author(header)
	if err == nil {
		t.Error("잘못된 서명에 대해 Author 메서드가 오류를 반환하지 않음")
	}
}

// TestEireneSetAndGetSigner는 SetSigner와 GetSigner 메서드를 테스트합니다.
func TestEireneSetAndGetSigner(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// 테스트 설정 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := core.New(config, db)

	// 테스트 주소 생성
	_, addr := generateTestKey()

	// 서명자 설정
	engine.SetSigner(addr)

	// 서명자 가져오기
	signer := engine.GetSigner()

	// 결과 검증
	if signer != addr {
		t.Errorf("서명자 주소가 일치하지 않음: 기대값 %s, 실제값 %s", addr.Hex(), signer.Hex())
	}
}

// TestEireneSetAndGetSignerFn는 SetSignerFn와 GetSignerFn 메서드를 테스트합니다.
func TestEireneSetAndGetSignerFn(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// 테스트 설정 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := core.New(config, db)

	// 테스트 키 생성
	key, addr := generateTestKey()

	// 서명 함수 생성
	fn := signerFn(key)

	// 서명 함수 설정
	engine.SetSignerFn(fn)

	// 서명 함수 가져오기
	retrievedFn := engine.GetSignerFn()

	// 결과 검증 - 함수 자체를 비교할 수 없으므로 함수 호출 결과를 비교
	// 32바이트 해시 생성
	testHash := crypto.Keccak256([]byte("test data"))
	
	sig1, err1 := fn(addr, testHash)
	sig2, err2 := retrievedFn(addr, testHash)

	if err1 != nil || err2 != nil {
		t.Fatalf("서명 함수 호출 실패: %v, %v", err1, err2)
	}

	// 두 서명이 동일해야 함
	for i := range sig1 {
		if sig1[i] != sig2[i] {
			t.Errorf("서명이 일치하지 않음: 인덱스 %d에서 %d != %d", i, sig1[i], sig2[i])
			break
		}
	}
}
