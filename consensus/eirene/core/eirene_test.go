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
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/params"
)

// 테스트 계정 생성
func generateTestKey() (*ecdsa.PrivateKey, common.Address) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return key, addr
}

// 테스트용 서명 함수 생성
func signerFn(key *ecdsa.PrivateKey) SignerFn {
	return func(signer common.Address, hash []byte) ([]byte, error) {
		return crypto.Sign(hash, key)
	}
}

// TestEireneNew는 Eirene 합의 엔진 생성을 테스트합니다.
func TestEireneNew(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// Eirene 구성 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := New(config, db)

	// 기본 검증
	if engine.config.Period != 15 {
		t.Errorf("기대한 Period 값 %d, 실제 값 %d", 15, engine.config.Period)
	}
	if engine.config.Epoch != 30000 {
		t.Errorf("기대한 Epoch 값 %d, 실제 값 %d", 30000, engine.config.Epoch)
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
	hash := SealHash(header)

	// 동일한 헤더에 대해 동일한 해시가 생성되는지 확인
	if hash2 := SealHash(header); hash != hash2 {
		t.Errorf("동일한 헤더에 대해 다른 해시가 생성됨: %x != %x", hash, hash2)
	}

	// 헤더 변경 시 해시가 변경되는지 확인
	header.Number = big.NewInt(2)
	if hash3 := SealHash(header); hash == hash3 {
		t.Errorf("다른 헤더에 대해 동일한 해시가 생성됨: %x == %x", hash, hash3)
	}
}

// TestEireneAuthor는 Author 함수를 테스트합니다.
func TestEireneAuthor(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// Eirene 구성 생성
	config := &params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}

	// Eirene 엔진 생성
	engine := New(config, db)

	// 테스트 키 생성
	key, addr := generateTestKey()

	// 서명 함수 설정
	engine.signer = addr
	engine.signFn = signerFn(key)

	// 테스트 헤더 생성
	header := &types.Header{
		Number:     big.NewInt(1),
		Difficulty: big.NewInt(1),
		Time:       1234,
		Extra:      make([]byte, extraVanity+extraSeal),
	}

	// 헤더 서명
	sighash, _ := engine.signFn(engine.signer, SealHash(header).Bytes())
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)

	// Author 확인
	author, err := engine.Author(header)
	if err != nil {
		t.Fatalf("Author 함수 오류: %v", err)
	}

	// 서명자와 Author가 일치하는지 확인
	if author != addr {
		t.Errorf("기대한 Author %x, 실제 값 %x", addr, author)
	}
}
