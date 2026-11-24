package identity

import (
	"errors"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lowercases domain only", in: "Alice@Example.COM", want: "Alice@example.com"},
		{name: "trims surrounding space", in: "  alice@redhat.com  ", want: "alice@redhat.com"},
		{name: "preserves local part case", in: "Alice.Smith+ci@RedHat.com", want: "Alice.Smith+ci@redhat.com"},
		{name: "nfc normalizes local part", in: "Jos\u0065\u0301@redhat.com", want: "Jos\u00e9@redhat.com"},
		{name: "subdomain kept lowercase", in: "a@MAIL.RedHat.COM", want: "a@mail.redhat.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeEmail(tc.in)
			if err != nil {
				t.Fatalf("NormalizeEmail(%q) error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeEmail(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	invalid := []string{"", "no-at-sign", "@redhat.com", "alice@", "a b@redhat.com", "a@@redhat.com"}
	for _, in := range invalid {
		t.Run("invalid "+in, func(t *testing.T) {
			if _, err := NormalizeEmail(in); !errors.Is(err, ErrInvalidEmail) {
				t.Fatalf("NormalizeEmail(%q) error = %v, want ErrInvalidEmail", in, err)
			}
		})
	}
}

func TestNewPolicyRejectsEmptyAllowlist(t *testing.T) {
	if _, err := NewPolicy(nil); err == nil {
		t.Fatal("NewPolicy(nil) succeeded, want error")
	}
	if _, err := NewPolicy([]string{"  "}); err == nil {
		t.Fatal("NewPolicy with blank domain succeeded, want error")
	}
	if _, err := NewPolicy([]string{"bad domain"}); err == nil {
		t.Fatal("NewPolicy with invalid domain succeeded, want error")
	}
}

func TestPolicyAuthorize(t *testing.T) {
	policy, err := NewPolicy([]string{"redhat.com"})
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}

	tests := []struct {
		name    string
		hd      string
		email   string
		wantErr error
	}{
		{name: "red hat workspace", hd: "redhat.com", email: "alice@redhat.com"},
		{name: "mixed case hd accepted", hd: "RedHat.com", email: "alice@redhat.com"},
		{name: "consumer account", hd: "", email: "alice@gmail.com", wantErr: ErrHostedDomainMissing},
		{name: "foreign workspace", hd: "evil.example", email: "alice@evil.example", wantErr: ErrHostedDomainNotAllowed},
		{name: "email domain disagrees with hd", hd: "redhat.com", email: "alice@gmail.com", wantErr: ErrDomainMismatch},
		{name: "suffix trick is not a match", hd: "redhat.com", email: "alice@notredhat.com", wantErr: ErrDomainMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.Authorize(tc.hd, tc.email)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Authorize(%q, %q) = %v, want nil", tc.hd, tc.email, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Authorize(%q, %q) = %v, want %v", tc.hd, tc.email, err, tc.wantErr)
			}
		})
	}
}
