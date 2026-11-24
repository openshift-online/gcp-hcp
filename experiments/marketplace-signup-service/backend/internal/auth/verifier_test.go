package auth

import (
	"errors"
	"testing"
)

func TestIdentityFromClaims(t *testing.T) {
	tests := []struct {
		name    string
		claims  map[string]any
		want    Identity
		wantErr bool
	}{
		{
			name:   "red hat workspace account",
			claims: map[string]any{"email": "alice@redhat.com", "email_verified": true, "hd": "redhat.com"},
			want:   Identity{Subject: "s1", Email: "alice@redhat.com", EmailVerified: true, HostedDomain: "redhat.com"},
		},
		{
			name:   "consumer account has no hd",
			claims: map[string]any{"email": "alice@gmail.com", "email_verified": true},
			want:   Identity{Subject: "s1", Email: "alice@gmail.com", EmailVerified: true},
		},
		{
			name:   "string true accepted",
			claims: map[string]any{"email": "alice@redhat.com", "email_verified": "true"},
			want:   Identity{Subject: "s1", Email: "alice@redhat.com", EmailVerified: true},
		},
		{
			name:    "unverified email rejected",
			claims:  map[string]any{"email": "alice@redhat.com", "email_verified": false},
			wantErr: true,
		},
		{
			name:    "missing verification rejected",
			claims:  map[string]any{"email": "alice@redhat.com"},
			wantErr: true,
		},
		{
			name:    "missing email rejected",
			claims:  map[string]any{"email_verified": true},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := identityFromClaims(tc.claims, "s1")
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("error = %v, want ErrInvalidToken", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("identity = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidIssuer(t *testing.T) {
	for _, ok := range []string{"https://accounts.google.com", "accounts.google.com"} {
		if !validIssuer(ok) {
			t.Fatalf("validIssuer(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "https://evil.example", "accounts.example.com"} {
		if validIssuer(bad) {
			t.Fatalf("validIssuer(%q) = true", bad)
		}
	}
}
