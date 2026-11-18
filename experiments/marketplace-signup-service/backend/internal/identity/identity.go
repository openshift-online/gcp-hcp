// Package identity implements the identity rules the sign-up backend must
// enforce: email normalization identical to the platform's RoleBinding subject
// normalization, and the Red Hat hosted-domain policy.
package identity

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Errors returned by Authorize. Callers map them to the authorization failure
// category; none of them may be treated as retryable.
var (
	// ErrInvalidEmail means the claim is not a usable email address.
	ErrInvalidEmail = errors.New("invalid email claim")
	// ErrHostedDomainMissing means the token carries no hd claim, i.e. the
	// account is not part of a Google Workspace directory.
	ErrHostedDomainMissing = errors.New("hosted domain claim missing")
	// ErrHostedDomainNotAllowed means hd names a domain outside the allowlist.
	ErrHostedDomainNotAllowed = errors.New("hosted domain not allowed")
	// ErrDomainMismatch means the email domain disagrees with hd.
	ErrDomainMismatch = errors.New("email domain does not match hosted domain")
)

// NormalizeEmail applies the platform's RoleBinding subject normalization:
// NFC, lowercase domain, preserved local-part case.
func NormalizeEmail(raw string) (string, error) {
	email := norm.NFC.String(strings.TrimSpace(raw))
	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		// Deliberately no raw value: this error is logged.
		return "", fmt.Errorf("%w: missing local part or domain", ErrInvalidEmail)
	}

	local, domain := email[:at], strings.ToLower(email[at+1:])
	if strings.ContainsAny(local, "@ \t\r\n") || strings.ContainsAny(domain, " \t\r\n@") {
		return "", fmt.Errorf("%w: illegal character", ErrInvalidEmail)
	}

	normalized := local + "@" + domain
	if len(normalized) > 254 {
		return "", fmt.Errorf("%w: address too long", ErrInvalidEmail)
	}
	return normalized, nil
}

// Policy is the hosted-domain allowlist.
type Policy struct {
	allowed []string
	index   map[string]struct{}
}

// NewPolicy builds a policy from domain names. Domains are compared
// case-insensitively. An empty list is a programming error: deny by default
// must never mean "allow everything".
func NewPolicy(domains []string) (*Policy, error) {
	p := &Policy{index: make(map[string]struct{}, len(domains))}
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || strings.ContainsAny(d, "@ \t\r\n") {
			return nil, fmt.Errorf("invalid allowed domain %q", d)
		}
		if _, dup := p.index[d]; dup {
			continue
		}
		p.index[d] = struct{}{}
		p.allowed = append(p.allowed, d)
	}
	if len(p.allowed) == 0 {
		return nil, errors.New("allowed domain list must not be empty")
	}
	return p, nil
}

// Allowed returns a copy of the configured allowlist, for logging.
func (p *Policy) Allowed() []string {
	out := make([]string, len(p.allowed))
	copy(out, p.allowed)
	return out
}

// Authorize enforces the Red Hat-only policy against a validated ID token's
// hosted-domain and email claims. The hd claim is set by Google for Workspace
// accounts and cannot be influenced by the client; the email domain must agree
// with it.
func (p *Policy) Authorize(hostedDomain, email string) error {
	hd := strings.ToLower(strings.TrimSpace(hostedDomain))
	if hd == "" {
		return ErrHostedDomainMissing
	}
	if _, ok := p.index[hd]; !ok {
		return fmt.Errorf("%w: %q", ErrHostedDomainNotAllowed, hd)
	}

	at := strings.LastIndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return ErrInvalidEmail
	}
	if domain := strings.ToLower(email[at+1:]); domain != hd {
		return fmt.Errorf("%w: email domain %q, hd %q", ErrDomainMismatch, domain, hd)
	}
	return nil
}
