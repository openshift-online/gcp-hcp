package storage

import (
	"testing"
)

func TestEncodeContinueToken(t *testing.T) {
	tests := []struct {
		name    string
		token   *ContinueToken
		wantErr bool
	}{
		{
			name:  "nil token returns empty string",
			token: nil,
		},
		{
			name: "encodes name only",
			token: &ContinueToken{
				Name: "obj-a",
			},
		},
		{
			name: "encodes namespace and name",
			token: &ContinueToken{
				Namespace: "default",
				Name:      "obj-b",
			},
		},
		{
			name: "encodes all fields",
			token: &ContinueToken{
				Namespace:       "kube-system",
				Name:            "coredns",
				ResourceVersion: "42",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeContinueToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EncodeContinueToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.token == nil {
				if encoded != "" {
					t.Fatalf("expected empty string for nil token, got %q", encoded)
				}
				return
			}
			if encoded == "" {
				t.Fatal("expected non-empty encoded token")
			}
		})
	}
}

func TestDecodeContinueToken(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ContinueToken
		wantErr bool
	}{
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:    "invalid base64",
			input:   "!!!invalid!!!",
			wantErr: true,
		},
		{
			name:    "valid base64 but invalid JSON",
			input:   "bm90LWpzb24",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeContinueToken(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodeContinueToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.want == nil && got != nil {
				t.Fatalf("expected nil, got %+v", got)
			}
		})
	}
}

func TestEncodeDecode_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		token ContinueToken
	}{
		{
			name:  "name only",
			token: ContinueToken{Name: "my-object"},
		},
		{
			name:  "namespace and name",
			token: ContinueToken{Namespace: "ns-1", Name: "obj-1"},
		},
		{
			name:  "all fields",
			token: ContinueToken{Namespace: "prod", Name: "deployment-x", ResourceVersion: "100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeContinueToken(&tt.token)
			if err != nil {
				t.Fatalf("EncodeContinueToken() error: %v", err)
			}

			decoded, err := DecodeContinueToken(encoded)
			if err != nil {
				t.Fatalf("DecodeContinueToken() error: %v", err)
			}

			if decoded.Namespace != tt.token.Namespace {
				t.Errorf("Namespace mismatch: got %q, want %q", decoded.Namespace, tt.token.Namespace)
			}
			if decoded.Name != tt.token.Name {
				t.Errorf("Name mismatch: got %q, want %q", decoded.Name, tt.token.Name)
			}
			if decoded.ResourceVersion != tt.token.ResourceVersion {
				t.Errorf("ResourceVersion mismatch: got %q, want %q", decoded.ResourceVersion, tt.token.ResourceVersion)
			}
		})
	}
}

func TestShouldIncludeObject(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		objName   string
		token     *ContinueToken
		want      bool
	}{
		{
			name:      "nil token includes everything",
			namespace: "default",
			objName:   "anything",
			token:     nil,
			want:      true,
		},
		{
			name:      "object after continue point by name",
			namespace: "default",
			objName:   "obj-b",
			token:     &ContinueToken{Namespace: "default", Name: "obj-a"},
			want:      true,
		},
		{
			name:      "object at continue point excluded",
			namespace: "default",
			objName:   "obj-a",
			token:     &ContinueToken{Namespace: "default", Name: "obj-a"},
			want:      false,
		},
		{
			name:      "object before continue point excluded",
			namespace: "default",
			objName:   "aaa",
			token:     &ContinueToken{Namespace: "default", Name: "obj-a"},
			want:      false,
		},
		{
			name:      "object in later namespace included",
			namespace: "z-namespace",
			objName:   "aaa",
			token:     &ContinueToken{Namespace: "default", Name: "zzz"},
			want:      true,
		},
		{
			name:      "object in earlier namespace excluded",
			namespace: "aaa-namespace",
			objName:   "zzz",
			token:     &ContinueToken{Namespace: "default", Name: "obj-a"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIncludeObject(tt.namespace, tt.objName, tt.token)
			if got != tt.want {
				t.Errorf("ShouldIncludeObject(%q, %q, %+v) = %v, want %v",
					tt.namespace, tt.objName, tt.token, got, tt.want)
			}
		})
	}
}
