// Package auth validates Google ID tokens server-side and extracts the claims
// the sign-up flow is allowed to trust.
package auth

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/api/idtoken"
)

// ErrInvalidToken means the ID token did not validate. Callers must treat it
// as an authentication failure and must not distinguish the cause to clients.
var ErrInvalidToken = errors.New("invalid ID token")

// Identity is the subset of verified claims the sign-up flow uses.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	HostedDomain  string
}

// Verifier validates ID tokens against Google's JWKS and a fixed audience.
type Verifier struct {
	audience string
	validate func(ctx context.Context, token, audience string) (*idtoken.Payload, error)
}

// NewVerifier returns a verifier that only accepts tokens minted for audience
// (the dedicated OAuth client ID).
func NewVerifier(audience string) *Verifier {
	return &Verifier{audience: audience, validate: idtoken.Validate}
}

// Verify checks signature, issuer, expiry, and audience, then extracts claims.
func (v *Verifier) Verify(ctx context.Context, rawToken string) (Identity, error) {
	payload, err := v.validate(ctx, rawToken, v.audience)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	// idtoken.Validate checks the audience, expiry, and signature but not the
	// issuer, so check it explicitly: only Google-issued tokens are acceptable.
	if !validIssuer(payload.Issuer) {
		return Identity{}, fmt.Errorf("%w: unexpected issuer %q", ErrInvalidToken, payload.Issuer)
	}
	return identityFromClaims(payload.Claims, payload.Subject)
}

// validIssuer accepts both issuer spellings Google uses.
func validIssuer(issuer string) bool {
	return issuer == "https://accounts.google.com" || issuer == "accounts.google.com"
}

// identityFromClaims is separated for unit testing: the token library already
// guarantees the claims are signed and unexpired by this point.
func identityFromClaims(claims map[string]any, subject string) (Identity, error) {
	id := Identity{Subject: subject}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return Identity{}, fmt.Errorf("%w: missing email claim", ErrInvalidToken)
	}
	id.Email = email

	verified, ok := claims["email_verified"].(bool)
	if !ok {
		// Some Google responses encode booleans as strings; accept only the
		// explicit true forms and reject anything ambiguous.
		s, isString := claims["email_verified"].(string)
		verified = isString && s == "true"
	}
	id.EmailVerified = verified
	if !verified {
		return Identity{}, fmt.Errorf("%w: email not verified", ErrInvalidToken)
	}

	if hd, ok := claims["hd"].(string); ok {
		id.HostedDomain = hd
	}
	return id, nil
}
