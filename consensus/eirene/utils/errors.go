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

package utils

import (
	"errors"
	"fmt"
)

// 공통 에러 상수 정의
var (
	// 일반 에러
	ErrInvalidParameter     = errors.New("invalid parameter")
	ErrInternalError        = errors.New("internal error")
	ErrNotImplemented       = errors.New("not implemented")
	ErrOperationFailed      = errors.New("operation failed")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrUnauthorized         = errors.New("unauthorized operation")
	
	// 검증자 관련 에러
	ErrValidatorNotFound    = errors.New("validator not found")
	ErrValidatorExists      = errors.New("validator already exists")
	ErrValidatorJailed      = errors.New("validator is jailed")
	ErrInvalidValidatorPower = errors.New("invalid validator power")
	
	// 스테이킹 관련 에러
	ErrStakingFailed        = errors.New("staking operation failed")
	ErrDelegationNotFound   = errors.New("delegation not found")
	ErrInsufficientStake    = errors.New("insufficient stake")
	ErrInvalidStakeAmount   = errors.New("invalid stake amount")
	
	// 거버넌스 관련 에러
	ErrProposalNotFound       = errors.New("proposal not found")
	ErrInvalidProposalStatus  = errors.New("invalid proposal status")
	ErrInvalidProposalType    = errors.New("invalid proposal type")
	ErrDepositPeriodEnded     = errors.New("deposit period ended")
	ErrVotingPeriodEnded      = errors.New("voting period ended")
	ErrDuplicateVote          = errors.New("duplicate vote")
	ErrProposalAlreadyExecuted = errors.New("proposal already executed")
	ErrInvalidVoteOption      = errors.New("invalid vote option")
	ErrInvalidDeposit         = errors.New("invalid deposit")
	
	// ABCI 관련 에러
	ErrInvalidTransaction   = errors.New("invalid transaction")
	ErrTransactionFailed    = errors.New("transaction execution failed")
	
	// P2P 관련 에러
	ErrPeerNotFound         = errors.New("peer not found")
	ErrConnectionFailed     = errors.New("connection failed")
	ErrMessageTooLarge      = errors.New("message too large")
	ErrInvalidMessage       = errors.New("invalid message")
	
	// IBC 관련 에러
	ErrClientNotFound       = errors.New("client not found")
	ErrConnectionNotFound   = errors.New("connection not found")
	ErrChannelNotFound      = errors.New("channel not found")
	ErrInvalidPacket        = errors.New("invalid packet")
)

// FormatError는 에러 메시지를 포맷팅합니다.
// 기본 에러에 추가 정보를 포함하여 더 자세한 에러 메시지를 생성합니다.
func FormatError(baseErr error, format string, args ...interface{}) error {
	if baseErr == nil {
		return fmt.Errorf(format, args...)
	}
	
	details := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", details, baseErr)
}

// WrapError는 기존 에러를 새로운 에러로 감싸서 컨텍스트를 추가합니다.
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// IsErrorType은 주어진 에러가 특정 타입의 에러인지 확인합니다.
func IsErrorType(err, target error) bool {
	return errors.Is(err, target)
} 