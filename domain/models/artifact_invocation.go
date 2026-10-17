package models

import "errors"

// ErrInvalidArtifactInvocation reports missing authorization/idempotency
// context for a Core-to-service artifact stream without exposing supplied values.
var ErrInvalidArtifactInvocation = errors.New("invalid Service SDK artifact invocation")

// ArtifactInvocation contains only the authenticated, non-secret context Core
// authorizes for one artifact action. TLS credentials remain in the transport.
type ArtifactInvocation struct {
	CallerID       string
	InstanceID     string
	PageID         string
	ActionID       string
	SurfaceDigest  string
	IdempotencyKey string
	RequestID      string
	IfMatch        string
}

// Validate requires the action identity and replay-protection context needed by
// a service to apply its own product contract safely.
func (invocation ArtifactInvocation) Validate() error {
	if invocation.CallerID == "" || invocation.InstanceID == "" || invocation.PageID == "" ||
		invocation.ActionID == "" || invocation.SurfaceDigest == "" ||
		invocation.IdempotencyKey == "" || invocation.RequestID == "" {
		return ErrInvalidArtifactInvocation
	}
	return nil
}
