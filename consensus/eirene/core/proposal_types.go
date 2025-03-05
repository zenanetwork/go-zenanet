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
	"errors"
	"math/big"

	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
)

// ParameterChangeProposal은 매개변수 변경 제안을 나타냅니다.
type ParameterChangeProposal struct {
	Parameters map[string]string // 변경할 매개변수
}

// GetType은 제안 유형을 반환합니다.
func (p *ParameterChangeProposal) GetType() string {
	return utils.ProposalTypeParameter
}

// Execute는 제안을 실행합니다.
func (p *ParameterChangeProposal) Execute(state *state.StateDB) error {
	// 매개변수 변경 로직 구현
	return nil
}

// UpgradeProposal은 업그레이드 제안을 나타냅니다.
type UpgradeProposal struct {
	UpgradeInfo utils.UpgradeInfo // 업그레이드 정보
}

// GetType은 제안 유형을 반환합니다.
func (p *UpgradeProposal) GetType() string {
	return utils.ProposalTypeUpgrade
}

// Execute는 제안을 실행합니다.
func (p *UpgradeProposal) Execute(state *state.StateDB) error {
	// 업그레이드 로직 구현
	return nil
}

// FundingProposal은 자금 지원 제안을 나타냅니다.
type FundingProposal struct {
	FundingInfo utils.FundingInfo // 자금 지원 정보
}

// GetType은 제안 유형을 반환합니다.
func (p *FundingProposal) GetType() string {
	return utils.ProposalTypeFunding
}

// Execute는 제안을 실행합니다.
func (p *FundingProposal) Execute(state *state.StateDB) error {
	// 자금 지원 로직 구현
	balance := state.GetBalance(p.FundingInfo.Recipient)
	balanceBig := new(big.Int).Set(balance.ToBig())
	
	if balanceBig.Cmp(p.FundingInfo.Amount) < 0 {
		return errors.New("insufficient funds")
	}
	
	// 자금 전송
	// state.SubBalance(communityPoolAddress, p.FundingInfo.Amount)
	// state.AddBalance(p.FundingInfo.Recipient, p.FundingInfo.Amount)
	
	return nil
}

// TextProposal은 텍스트 제안을 나타냅니다.
type TextProposal struct {
	// 텍스트 제안은 추가 필드가 없습니다.
}

// GetType은 제안 유형을 반환합니다.
func (p *TextProposal) GetType() string {
	return utils.ProposalTypeText
}

// Execute는 제안을 실행합니다.
func (p *TextProposal) Execute(state *state.StateDB) error {
	// 텍스트 제안은 실행할 내용이 없습니다.
	return nil
} 