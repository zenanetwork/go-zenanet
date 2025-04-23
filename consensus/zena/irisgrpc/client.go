package irisgrpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/checkpoint"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/milestone"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
	"github.com/zenanetwork/go-zenanet/log"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	lru "github.com/hashicorp/golang-lru"
	proto "github.com/zenanetwork/zenaproto/iris"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	stateFetchLimit  = 50
	defaultCacheSize = 100
)

// CircuitBreaker는 서킷 브레이커 패턴을 구현합니다.
type CircuitBreaker struct {
	failures     int
	threshold    int
	open         bool
	resetTimeout time.Duration
	lastFailure  time.Time
	mu           sync.RWMutex
}

// NewCircuitBreaker는 새로운 CircuitBreaker를 생성합니다.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Execute는 제공된 함수를 실행하고 서킷 상태를 관리합니다.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.RLock()
	if cb.open {
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.open = false
			cb.failures = 0
			cb.mu.Unlock()
		} else {
			cb.mu.RUnlock()
			return fmt.Errorf("circuit breaker is open")
		}
	} else {
		cb.mu.RUnlock()
	}

	err := fn()
	if err != nil {
		cb.mu.Lock()
		cb.failures++
		if cb.failures >= cb.threshold {
			cb.open = true
			cb.lastFailure = time.Now()
		}
		cb.mu.Unlock()
	}
	return err
}

// IrisClient는 IRIS와의 통신을 담당하는 인터페이스입니다.
type IrisClient interface {
	// Span 관련
	Span(ctx context.Context, spanID uint64) (*span.IrisSpan, error)

	// 이벤트 관련
	StateSyncEvents(ctx context.Context, fromID uint64, to int64) ([]*clerk.EventRecordWithTime, error)

	// 체크포인트 관련
	FetchCheckpointCount(ctx context.Context) (int64, error)
	FetchCheckpoint(ctx context.Context, number int64) (*checkpoint.Checkpoint, error)

	// 마일스톤 관련
	FetchMilestoneCount(ctx context.Context) (int64, error)
	FetchMilestone(ctx context.Context) (*milestone.Milestone, error)
	FetchLastNoAckMilestone(ctx context.Context) (string, error)
	FetchNoAckMilestone(ctx context.Context, milestoneID string) error
	FetchMilestoneID(ctx context.Context, milestoneID string) error

	// 비동기 호출
	SpanAsync(spanID uint64) <-chan SpanResult

	// 연결 관리
	Close()
}

// SpanResult는 비동기 스팬 요청의 결과를 담는 구조체입니다.
type SpanResult struct {
	Span *span.IrisSpan
	Err  error
}

// IrisGRPCClient는 IRIS gRPC 클라이언트 구현입니다.
type IrisGRPCClient struct {
	conn           *grpc.ClientConn
	client         proto.IrisClient
	defaultTimeout time.Duration
	retryConfig    RetryConfig
	circuitBreaker *CircuitBreaker
	cache          *lru.Cache

	// 종료 관리
	mu     sync.Mutex
	closed bool
}

// RetryConfig는 재시도 설정을 담는 구조체입니다.
type RetryConfig struct {
	MaxRetries  uint
	BackoffTime time.Duration
	RetryCodes  []codes.Code
}

// ClientOption은 클라이언트 옵션 함수 타입입니다.
type ClientOption func(*IrisGRPCClient)

// WithTimeout은 기본 타임아웃 설정 옵션을 제공합니다.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *IrisGRPCClient) {
		c.defaultTimeout = timeout
	}
}

// WithRetryConfig는 재시도 설정 옵션을 제공합니다.
func WithRetryConfig(maxRetries uint, backoffTime time.Duration, retryCodes []codes.Code) ClientOption {
	return func(c *IrisGRPCClient) {
		c.retryConfig = RetryConfig{
			MaxRetries:  maxRetries,
			BackoffTime: backoffTime,
			RetryCodes:  retryCodes,
		}
	}
}

// WithCircuitBreaker는 서킷 브레이커 설정 옵션을 제공합니다.
func WithCircuitBreaker(threshold int, resetTimeout time.Duration) ClientOption {
	return func(c *IrisGRPCClient) {
		c.circuitBreaker = NewCircuitBreaker(threshold, resetTimeout)
	}
}

// WithCache는 캐시 설정 옵션을 제공합니다.
func WithCache(size int) ClientOption {
	return func(c *IrisGRPCClient) {
		cache, _ := lru.New(size)
		c.cache = cache
	}
}

// NewIrisGRPCClient는 새 IRIS gRPC 클라이언트를 생성합니다.
func NewIrisGRPCClient(address string, options ...ClientOption) *IrisGRPCClient {
	// 기본 설정으로 클라이언트 생성
	client := &IrisGRPCClient{
		defaultTimeout: 30 * time.Second,
		retryConfig: RetryConfig{
			MaxRetries:  10,
			BackoffTime: 5 * time.Second,
			RetryCodes:  []codes.Code{codes.Internal, codes.Unavailable, codes.Aborted, codes.NotFound},
		},
		circuitBreaker: NewCircuitBreaker(5, 1*time.Minute),
	}

	// 캐시 초기화
	cache, _ := lru.New(defaultCacheSize)
	client.cache = cache

	// 옵션 적용
	for _, option := range options {
		option(client)
	}

	// gRPC 재시도 옵션 구성
	opts := []grpc_retry.CallOption{
		grpc_retry.WithMax(client.retryConfig.MaxRetries),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(client.retryConfig.BackoffTime)),
		grpc_retry.WithCodes(client.retryConfig.RetryCodes...),
	}

	// gRPC 연결 설정
	conn, err := grpc.NewClient(address,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Crit("Failed to connect to Iris gRPC", "error", err, "address", address)
	}

	log.Info("Connected to Iris gRPC server", "address", address)

	client.conn = conn
	client.client = proto.NewIrisClient(conn)

	return client
}

// contextWithTimeout은 컨텍스트에 타임아웃을 추가합니다.
func (h *IrisGRPCClient) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, h.defaultTimeout)
}

// wrapError는 에러를 래핑하여 더 상세한 정보를 제공합니다.
func (h *IrisGRPCClient) wrapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("iris client: %s: %w", operation, err)
	}

	return fmt.Errorf("iris client: %s: code=%s message=%s",
		operation, st.Code(), st.Message())
}

// executeWithMetrics는 함수를 실행하고 오류를 처리합니다.
func (h *IrisGRPCClient) executeWithCircuitBreaker(operation string, fn func() error) error {
	err := h.circuitBreaker.Execute(fn)
	if err != nil {
		return h.wrapError(err, operation)
	}

	return nil
}

// getCacheKey는 캐시 키를 생성합니다.
func getCacheKey(prefix string, params ...interface{}) string {
	return fmt.Sprintf("%s:%v", prefix, params)
}

// SpanAsync는 비동기적으로 스팬을 요청합니다.
func (h *IrisGRPCClient) SpanAsync(spanID uint64) <-chan SpanResult {
	resultCh := make(chan SpanResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.defaultTimeout)
		defer cancel()

		span, err := h.Span(ctx, spanID)
		resultCh <- SpanResult{Span: span, Err: err}
		close(resultCh)
	}()
	return resultCh
}

// Close는 클라이언트 연결을 종료합니다.
func (h *IrisGRPCClient) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}

	log.Debug("Shutdown detected, Closing Iris gRPC client")
	h.conn.Close()
	h.closed = true
}
