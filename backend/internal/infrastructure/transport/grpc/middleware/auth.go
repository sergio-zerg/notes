package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// APIKeyMetadata is the gRPC metadata key carrying the API key. It matches the
// HTTP X-API-Key header so that one credential works over both transports.
const APIKeyMetadata = "x-api-key"

// Authorizer validates the API key carried in incoming gRPC metadata.
//
// Keys are stored as SHA-256 digests and compared in constant time, mirroring
// the HTTP authenticator; see
// internal/infrastructure/transport/http/middleware/auth.go for the rationale.
type Authorizer struct {
	digests    [][sha256.Size]byte
	allowEmpty bool
}

// NewAuthorizer builds an Authorizer from the configured keys. An empty key is
// skipped, since accepting one would authenticate anyone who omits the header.
func NewAuthorizer(keys []string, allowEmpty bool) *Authorizer {
	a := &Authorizer{allowEmpty: allowEmpty}
	for _, key := range keys {
		if key == "" {
			continue
		}
		a.digests = append(a.digests, sha256.Sum256([]byte(key)))
	}
	return a
}

// AuthorizeUnaryInterceptor rejects RPCs without an accepted API key.
func (a *Authorizer) AuthorizeUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := a.authorize(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (a *Authorizer) authorize(ctx context.Context) error {
	if len(a.digests) == 0 {
		// No keys configured. Honouring an explicit opt-in is the only way
		// through, so a missing environment variable cannot silently open the
		// API.
		if a.allowEmpty {
			return nil
		}
		return status.Error(codes.Internal, "authentication is not configured")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "valid API key required")
	}

	presented := presentedKey(md)
	if !a.matches(presented) {
		return status.Error(codes.Unauthenticated, "valid API key required")
	}
	return nil
}

// presentedKey reads the key from metadata, accepting either the dedicated
// x-api-key entry or an "authorization: Bearer <key>" one.
func presentedKey(md metadata.MD) string {
	if values := md.Get(APIKeyMetadata); len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	if values := md.Get("authorization"); len(values) > 0 {
		if after, found := strings.CutPrefix(values[0], "Bearer "); found {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// matches compares against every configured digest without returning early, so
// that response timing does not depend on which key matched.
func (a *Authorizer) matches(presented string) bool {
	if presented == "" {
		return false
	}
	candidate := sha256.Sum256([]byte(presented))

	var found int
	for _, digest := range a.digests {
		found |= subtle.ConstantTimeCompare(candidate[:], digest[:])
	}
	return found == 1
}
