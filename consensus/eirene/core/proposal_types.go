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
	"strconv"

	"github.com/zenanetwork/go-zenanet/common"
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

// Validate는 제안 내용의 유효성을 검사합니다.
func (p *ParameterChangeProposal) Validate() error {
	if p.Parameters == nil || len(p.Parameters) == 0 {
		return errors.New("parameters cannot be empty")
	}
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다.
func (p *ParameterChangeProposal) GetParams() map[string]string {
	return p.Parameters
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

// Validate는 제안 내용의 유효성을 검사합니다.
func (p *UpgradeProposal) Validate() error {
	if p.UpgradeInfo.Name == "" {
		return errors.New("upgrade name cannot be empty")
	}
	if p.UpgradeInfo.Height == 0 {
		return errors.New("upgrade height must be greater than 0")
	}
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다.
func (p *UpgradeProposal) GetParams() map[string]string {
	params := make(map[string]string)
	params["name"] = p.UpgradeInfo.Name
	params["height"] = strconv.FormatUint(p.UpgradeInfo.Height, 10)
	params["info"] = p.UpgradeInfo.Info
	params["version"] = p.UpgradeInfo.Version
	params["url"] = p.UpgradeInfo.URL
	return params
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

// Validate는 제안 내용의 유효성을 검사합니다.
func (p *FundingProposal) Validate() error {
	if p.FundingInfo.Recipient == (common.Address{}) {
		return errors.New("recipient address cannot be zero")
	}
	if p.FundingInfo.Amount == nil || p.FundingInfo.Amount.Cmp(big.NewInt(0)) <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if p.FundingInfo.Reason == "" {
		return errors.New("reason cannot be empty")
	}
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다.
func (p *FundingProposal) GetParams() map[string]string {
	params := make(map[string]string)
	params["recipient"] = p.FundingInfo.Recipient.Hex()
	params["amount"] = p.FundingInfo.Amount.String()
	params["reason"] = p.FundingInfo.Reason
	params["purpose"] = p.FundingInfo.Purpose
	return params
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

// Validate는 제안 내용의 유효성을 검사합니다.
func (p *TextProposal) Validate() error {
	// 텍스트 제안은 추가 검증이 필요 없습니다.
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다.
func (p *TextProposal) GetParams() map[string]string {
	// 텍스트 제안은 매개변수가 없습니다.
	return make(map[string]string)
}

// Execute는 제안을 실행합니다.
func (p *TextProposal) Execute(state *state.StateDB) error {
	// 텍스트 제안은 실행할 내용이 없습니다.
	return nil
} 