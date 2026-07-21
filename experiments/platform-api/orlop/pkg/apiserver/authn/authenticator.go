package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	rbacv1 "github.com/thetechnick/orlop/apis/private/rbac/v1"
	"github.com/thetechnick/orlop/pkg/apiserver/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ServiceAccountTokenType is the type of Secret that holds a ServiceAccount token.
	ServiceAccountTokenType = "kubernetes.io/service-account-token"

	// TokenKey is the key in Secret.Data that holds the bearer token.
	TokenKey = "token"

	// ServiceAccountNameKey is the annotation key that links a Secret to its ServiceAccount.
	ServiceAccountNameKey = "kubernetes.io/service-account.name"

	// ServiceAccountUIDKey is the annotation key that stores the ServiceAccount UID.
	ServiceAccountUIDKey = "kubernetes.io/service-account.uid"
)

// Authenticator validates bearer tokens against ServiceAccounts.
type Authenticator struct {
	serviceAccountStore storage.ResourceStore
	secretStore         storage.ResourceStore
}

// NewAuthenticator creates a new ServiceAccount-based authenticator.
func NewAuthenticator(serviceAccountStore, secretStore storage.ResourceStore) *Authenticator {
	return &Authenticator{
		serviceAccountStore: serviceAccountStore,
		secretStore:         secretStore,
	}
}

// UserInfo holds authenticated user information.
type UserInfo struct {
	Username string   // Username (format: system:serviceaccount:{namespace}:{name})
	UID      string   // ServiceAccount UID
	Groups   []string // Groups the user belongs to
}

// Authenticate validates a bearer token and returns user information.
func (a *Authenticator) Authenticate(ctx context.Context, token string) (*UserInfo, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// List all secrets to find matching token
	// In production, this should be optimized with indexing
	secretList, err := a.secretStore.List(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	secrets, err := extractSecrets(secretList)
	if err != nil {
		return nil, fmt.Errorf("failed to extract secrets: %w", err)
	}

	// Find secret with matching token
	var matchedSecret *rbacv1.Secret
	for _, secret := range secrets {
		if secret.Type != ServiceAccountTokenType {
			continue
		}

		secretToken, ok := secret.Data[TokenKey]
		if !ok {
			continue
		}

		if string(secretToken) == token {
			matchedSecret = secret
			break
		}
	}

	if matchedSecret == nil {
		return nil, fmt.Errorf("invalid token")
	}

	// Get ServiceAccount name from secret annotations
	annotations := matchedSecret.GetAnnotations()
	saName := annotations[ServiceAccountNameKey]
	if saName == "" {
		return nil, fmt.Errorf("secret missing service account annotation")
	}

	// Verify ServiceAccount exists
	saObj, err := a.serviceAccountStore.Get(ctx, matchedSecret.GetNamespace(), saName)
	if err != nil {
		return nil, fmt.Errorf("service account not found: %w", err)
	}

	sa, err := convertToServiceAccount(saObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert service account: %w", err)
	}

	// Construct username
	username := fmt.Sprintf("system:serviceaccount:%s:%s", sa.GetNamespace(), sa.GetName())

	// Construct groups
	groups := []string{
		"system:serviceaccounts",
		fmt.Sprintf("system:serviceaccounts:%s", sa.GetNamespace()),
	}

	return &UserInfo{
		Username: username,
		UID:      string(sa.GetUID()),
		Groups:   groups,
	}, nil
}

// GenerateToken generates a new random bearer token for a ServiceAccount.
// This should be called when creating a Secret for a ServiceAccount.
func GenerateToken() (string, error) {
	// Generate 32 random bytes
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	return token, nil
}

// CreateServiceAccountToken creates a Secret with a token for the given ServiceAccount.
func CreateServiceAccountToken(sa *rbacv1.ServiceAccount) (*rbacv1.Secret, error) {
	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	secret := &rbacv1.Secret{
		Type: ServiceAccountTokenType,
		Data: map[string][]byte{
			TokenKey: []byte(token),
		},
	}

	secret.SetName(fmt.Sprintf("%s-token", sa.GetName()))
	secret.SetNamespace(sa.GetNamespace())

	annotations := map[string]string{
		ServiceAccountNameKey: sa.GetName(),
		ServiceAccountUIDKey:  string(sa.GetUID()),
	}
	secret.SetAnnotations(annotations)

	return secret, nil
}

// extractSecrets extracts Secret objects from a list.
func extractSecrets(list client.ObjectList) ([]*rbacv1.Secret, error) {
	// Convert via JSON marshaling
	data, err := json.Marshal(list)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list: %w", err)
	}

	var secretList rbacv1.SecretList
	if err := json.Unmarshal(data, &secretList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to SecretList: %w", err)
	}

	result := make([]*rbacv1.Secret, len(secretList.Items))
	for i := range secretList.Items {
		result[i] = &secretList.Items[i]
	}
	return result, nil
}

// convertToServiceAccount converts a client.Object to a ServiceAccount.
func convertToServiceAccount(obj client.Object) (*rbacv1.ServiceAccount, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}

	var sa rbacv1.ServiceAccount
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to ServiceAccount: %w", err)
	}

	return &sa, nil
}

// ExtractBearerToken extracts the bearer token from an Authorization header.
func ExtractBearerToken(authHeader string) string {
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(authHeader, bearerPrefix) {
		return strings.TrimPrefix(authHeader, bearerPrefix)
	}
	return ""
}
