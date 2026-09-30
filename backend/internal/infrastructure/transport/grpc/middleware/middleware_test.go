package middleware

import (
	"context"
	"errors"
	"net"
	"testing"

	"notes/internal/infrastructure/observability"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// contextWithPeer builds a context that looks like one arriving from addr.
func contextWithPeer(addr string) context.Context {
	return peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP(addr), Port: 1000},
	})
}

const testMethod = "/notes.NoteService/GetNote"

// counterValue reads a labelled counter from the default registry. The
// Prometheus counter is read rather than the label tuple, so a wrong label
// name or a missing .Inc() shows up here instead of passing silently.
func counterValue(t *testing.T, method, code string) float64 {
	t.Helper()
	return testutil.ToFloat64(observability.GrpcRequestsTotal.WithLabelValues(method, code))
}

func TestGrpcMetricsInterceptorRecordsOK(t *testing.T) {
	before := counterValue(t, testMethod, codes.OK.String())

	interceptor := GrpcMetricsInterceptor()
	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return "response", nil
		},
	)

	if err != nil {
		t.Fatalf("interceptor() = %v, want nil", err)
	}
	if got := counterValue(t, testMethod, codes.OK.String()); got != before+1 {
		t.Errorf("grpc_requests_total for OK = %v, want %v", got, before+1)
	}
}

// The status code must come from the error the handler returned, not from a
// default, so that failed RPCs are distinguishable in the metrics.
func TestGrpcMetricsInterceptorRecordsHandlerError(t *testing.T) {
	beforeError := counterValue(t, testMethod, codes.NotFound.String())
	beforeOK := counterValue(t, testMethod, codes.OK.String())

	interceptor := GrpcMetricsInterceptor()
	wantErr := status.Error(codes.NotFound, "note not found")

	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, wantErr
		},
	)

	if !errors.Is(err, wantErr) {
		t.Errorf("interceptor() = %v, want %v", err, wantErr)
	}
	if got := counterValue(t, testMethod, codes.NotFound.String()); got != beforeError+1 {
		t.Errorf("grpc_requests_total for NotFound = %v, want %v", got, beforeError+1)
	}
	if got := counterValue(t, testMethod, codes.OK.String()); got != beforeOK {
		t.Errorf("an error was also recorded as OK: %v, want %v", got, beforeOK)
	}
}

// A plain (non-status) error must still be classified rather than crashing the
// interceptor, otherwise the panic would take the whole RPC down.
func TestGrpcMetricsInterceptorHandlesPlainError(t *testing.T) {
	before := counterValue(t, testMethod, codes.Unknown.String())

	interceptor := GrpcMetricsInterceptor()
	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, errors.New("boom")
		},
	)

	if err == nil {
		t.Fatal("interceptor() = nil, want the handler error")
	}
	if got := counterValue(t, testMethod, codes.Unknown.String()); got != before+1 {
		t.Errorf("grpc_requests_total for Unknown = %v, want %v", got, before+1)
	}
}

func TestGrpcMetricsInterceptorPassesThroughResponse(t *testing.T) {
	type payload struct{ ID int64 }

	interceptor := GrpcMetricsInterceptor()
	got, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return &payload{ID: 7}, nil
		},
	)

	if err != nil {
		t.Fatalf("interceptor() = %v, want nil", err)
	}
	if got.(*payload).ID != 7 {
		t.Errorf("response ID = %d, want 7", got.(*payload).ID)
	}
}

func TestGrpcAuthRejectsMissingKey(t *testing.T) {
	auth := NewAuthorizer([]string{"secret"}, false)
	called := false

	_, err := auth.AuthorizeUnaryInterceptor()(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			called = true
			return nil, nil
		},
	)

	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
	if called {
		t.Error("the handler ran despite the request being unauthenticated")
	}
}

func TestGrpcAuthAcceptsMetadataKey(t *testing.T) {
	cases := []struct {
		name string
		md   metadata.MD
	}{
		{"x-api-key", metadata.Pairs(APIKeyMetadata, "secret")},
		{"bearer", metadata.Pairs("authorization", "Bearer secret")},
		{"surrounding whitespace", metadata.Pairs(APIKeyMetadata, "  secret  ")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := NewAuthorizer([]string{"secret"}, false)
			called := false

			_, err := auth.AuthorizeUnaryInterceptor()(
				metadata.NewIncomingContext(context.Background(), tc.md),
				nil,
				&grpc.UnaryServerInfo{FullMethod: testMethod},
				func(ctx context.Context, req interface{}) (interface{}, error) {
					called = true
					return "ok", nil
				},
			)

			if err != nil {
				t.Fatalf("interceptor() = %v, want nil", err)
			}
			if !called {
				t.Error("the handler did not run for an authenticated request")
			}
		})
	}
}

func TestGrpcAuthRejectsWrongKey(t *testing.T) {
	auth := NewAuthorizer([]string{"secret", "other"}, false)

	_, err := auth.AuthorizeUnaryInterceptor()(
		metadata.NewIncomingContext(context.Background(), metadata.Pairs(APIKeyMetadata, "wrong")),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil },
	)

	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", status.Code(err))
	}
}

// A trailing comma in API_KEYS must not create an empty key that authenticates
// anyone who omits the header entirely.
func TestGrpcAuthIgnoresEmptyKey(t *testing.T) {
	auth := NewAuthorizer([]string{"", "secret"}, false)

	_, err := auth.AuthorizeUnaryInterceptor()(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil },
	)

	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated for a request with no key", status.Code(err))
	}
}

// With no keys and no explicit opt-in, the service must fail loudly rather
// than accept everything.
func TestGrpcAuthRefusesWhenUnconfigured(t *testing.T) {
	auth := NewAuthorizer(nil, false)

	_, err := auth.AuthorizeUnaryInterceptor()(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testMethod},
		func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil },
	)

	if status.Code(err) != codes.Internal {
		t.Errorf("code = %v, want Internal", status.Code(err))
	}
}

func TestGrpcRateLimiterRejectsExcess(t *testing.T) {
	limiter := NewGrpcRateLimiter(1, 1)
	interceptor := limiter.RateLimitUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}
	ok := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }

	// Burst of 1 permits exactly one call.
	if _, err := interceptor(context.Background(), nil, info, ok); err != nil {
		t.Fatalf("first call = %v, want nil", err)
	}
	_, err := interceptor(context.Background(), nil, info, ok)
	if status.Code(err) != codes.ResourceExhausted {
		t.Errorf("second call code = %v, want ResourceExhausted", status.Code(err))
	}
}

func TestGrpcRateLimiterSeparatesClients(t *testing.T) {
	limiter := NewGrpcRateLimiter(1, 1)
	interceptor := limiter.RateLimitUnaryInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}
	ok := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }

	first := contextWithPeer("10.0.0.1")
	second := contextWithPeer("10.0.0.2")

	if _, err := interceptor(first, nil, info, ok); err != nil {
		t.Fatalf("first client = %v, want nil", err)
	}
	// A different address has its own budget, so it must still pass.
	if _, err := interceptor(second, nil, info, ok); err != nil {
		t.Errorf("second client = %v, want nil", err)
	}
}
