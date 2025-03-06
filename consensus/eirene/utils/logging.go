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
	"github.com/zenanetwork/go-zenanet/log"
)

// LogLevel은 로그 레벨을 정의합니다.
type LogLevel int

// 로그 레벨 상수
const (
	LogLevelTrace LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelCrit
)

// Logger는 Eirene 합의 알고리즘에서 사용하는 로거 인터페이스입니다.
type Logger interface {
	// 기본 로깅 메서드
	Trace(msg string, ctx ...interface{})
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
	
	// 컨텍스트 추가 메서드
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
	
	// 로그 레벨 설정
	SetLevel(level LogLevel)
	GetLevel() LogLevel
}

// EireneLogger는 Eirene 합의 알고리즘에서 사용하는 로거 구현체입니다.
type EireneLogger struct {
	logger log.Logger
	level  LogLevel
}

// NewLogger는 새로운 EireneLogger 인스턴스를 생성합니다.
func NewLogger(module string) *EireneLogger {
	return &EireneLogger{
		logger: log.New("module", module),
		level:  LogLevelInfo, // 기본 로그 레벨은 Info
	}
}

// Trace는 TRACE 레벨 로그를 기록합니다.
func (l *EireneLogger) Trace(msg string, ctx ...interface{}) {
	if l.level <= LogLevelTrace {
		l.logger.Trace(msg, ctx...)
	}
}

// Debug는 DEBUG 레벨 로그를 기록합니다.
func (l *EireneLogger) Debug(msg string, ctx ...interface{}) {
	if l.level <= LogLevelDebug {
		l.logger.Debug(msg, ctx...)
	}
}

// Info는 INFO 레벨 로그를 기록합니다.
func (l *EireneLogger) Info(msg string, ctx ...interface{}) {
	if l.level <= LogLevelInfo {
		l.logger.Info(msg, ctx...)
	}
}

// Warn은 WARN 레벨 로그를 기록합니다.
func (l *EireneLogger) Warn(msg string, ctx ...interface{}) {
	if l.level <= LogLevelWarn {
		l.logger.Warn(msg, ctx...)
	}
}

// Error는 ERROR 레벨 로그를 기록합니다.
func (l *EireneLogger) Error(msg string, ctx ...interface{}) {
	if l.level <= LogLevelError {
		l.logger.Error(msg, ctx...)
	}
}

// Crit은 CRIT 레벨 로그를 기록합니다.
func (l *EireneLogger) Crit(msg string, ctx ...interface{}) {
	if l.level <= LogLevelCrit {
		l.logger.Crit(msg, ctx...)
	}
}

// WithField는 로그 컨텍스트에 필드를 추가합니다.
func (l *EireneLogger) WithField(key string, value interface{}) Logger {
	newLogger := &EireneLogger{
		logger: l.logger.New(key, value),
		level:  l.level,
	}
	return newLogger
}

// WithFields는 로그 컨텍스트에 여러 필드를 추가합니다.
func (l *EireneLogger) WithFields(fields map[string]interface{}) Logger {
	ctx := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		ctx = append(ctx, k, v)
	}
	
	newLogger := &EireneLogger{
		logger: l.logger.New(ctx...),
		level:  l.level,
	}
	return newLogger
}

// SetLevel은 로그 레벨을 설정합니다.
func (l *EireneLogger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel은 현재 로그 레벨을 반환합니다.
func (l *EireneLogger) GetLevel() LogLevel {
	return l.level
}

// LogMethodEntry는 메서드 진입 시 로그를 기록합니다.
func LogMethodEntry(logger Logger, method string, args ...interface{}) {
	if len(args) > 0 {
		fields := make(map[string]interface{}, len(args)/2)
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				fields[args[i].(string)] = args[i+1]
			}
		}
		logger.WithFields(fields).Debug("Entering " + method)
	} else {
		logger.Debug("Entering " + method)
	}
}

// LogMethodExit는 메서드 종료 시 로그를 기록합니다.
func LogMethodExit(logger Logger, method string, err error) {
	if err != nil {
		logger.WithField("error", err.Error()).Debug("Exiting " + method + " with error")
	} else {
		logger.Debug("Exiting " + method)
	}
} 