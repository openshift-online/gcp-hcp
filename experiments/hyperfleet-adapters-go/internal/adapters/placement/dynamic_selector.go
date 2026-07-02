package placement

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
)

// DynamicSelector discovers eligible management clusters and DNS zones at
// selection time by cross-checking Secret Manager secrets against Maestro
// consumers, mirroring the logic of the YAML placement adapter's Job script.
//
// MC discovery:
//  1. List SM secrets with label maestro-consumer-name:* → candidate MC names
//  2. List Maestro consumers → registered MC names
//  3. Eligible = intersection of the two sets
//
// DNS zone discovery:
//  1. Find SM secret matching name:argocd-cluster with label infra-type:region
//  2. Access latest version → parse meta_hc_dns_domains (comma-separated)
//
// Selection is round-robin across eligible MCs and domains.
type DynamicSelector struct {
	smClient   *secretmanager.Client
	project    string
	maestroURL string
	httpClient *http.Client

	mcCounter  atomic.Uint64
	domCounter atomic.Uint64
}

// NewDynamicSelector creates a DynamicSelector.
// project is the GCP project ID for Secret Manager lookups.
// maestroURL is the Maestro HTTP base URL (e.g. "http://maestro.hyperfleet.svc.cluster.local:8000").
func NewDynamicSelector(smClient *secretmanager.Client, project, maestroURL string) *DynamicSelector {
	return &DynamicSelector{
		smClient:   smClient,
		project:    project,
		maestroURL: maestroURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Select discovers eligible MCs and DNS zones dynamically, then picks one of
// each using a round-robin counter. The candidates parameter is ignored.
func (s *DynamicSelector) Select(ctx context.Context, _ []Candidate) (mcName, baseDomain string, err error) {
	eligible, err := s.eligibleMCs(ctx)
	if err != nil {
		return "", "", fmt.Errorf("discover eligible MCs: %w", err)
	}
	if len(eligible) == 0 {
		return "", "", fmt.Errorf("no eligible management clusters found (check Secret Manager labels and Maestro consumers)")
	}

	mc := eligible[s.mcCounter.Add(1)%uint64(len(eligible))]

	domains, err := s.hcDNSDomains(ctx)
	if err != nil {
		return "", "", fmt.Errorf("discover DNS domains: %w", err)
	}
	if len(domains) == 0 {
		return "", "", fmt.Errorf("no HC DNS domains found in Secret Manager for project %s", s.project)
	}

	domain := domains[s.domCounter.Add(1)%uint64(len(domains))]

	return mc, domain, nil
}

// eligibleMCs returns MC names present in both Secret Manager and Maestro.
func (s *DynamicSelector) eligibleMCs(ctx context.Context) ([]string, error) {
	smMCs, err := s.smMCNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("secret manager MC lookup: %w", err)
	}

	maestroMCs, err := s.maestroConsumerNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("maestro consumer lookup: %w", err)
	}

	maestroSet := make(map[string]bool, len(maestroMCs))
	for _, m := range maestroMCs {
		maestroSet[m] = true
	}

	var eligible []string
	for _, mc := range smMCs {
		if maestroSet[mc] {
			eligible = append(eligible, mc)
		}
	}
	return eligible, nil
}

// smMCNames lists secrets in the project with label maestro-consumer-name:* and
// returns the label values (MC names).
func (s *DynamicSelector) smMCNames(ctx context.Context) ([]string, error) {
	req := &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", s.project),
		Filter: "labels.maestro-consumer-name:*",
	}

	var names []string
	it := s.smClient.ListSecrets(ctx, req)
	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list secrets: %w", err)
		}
		if mc := secret.Labels["maestro-consumer-name"]; mc != "" {
			names = append(names, mc)
		}
	}
	return names, nil
}

// maestroConsumerNames queries the Maestro HTTP API for registered consumers.
func (s *DynamicSelector) maestroConsumerNames(ctx context.Context) ([]string, error) {
	url := strings.TrimRight(s.maestroURL, "/") + "/api/maestro/v1/consumers?size=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET consumers: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read consumers response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET consumers returned %d: %s", resp.StatusCode, body)
	}

	var page struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("unmarshal consumers: %w", err)
	}

	names := make([]string, 0, len(page.Items))
	for _, c := range page.Items {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	return names, nil
}

// hcDNSDomains reads the argocd-cluster region secret from Secret Manager and
// returns the comma-separated domains from its meta_hc_dns_domains field.
func (s *DynamicSelector) hcDNSDomains(ctx context.Context) ([]string, error) {
	req := &secretmanagerpb.ListSecretsRequest{
		Parent: fmt.Sprintf("projects/%s", s.project),
		Filter: `labels.infra-type:region name:argocd-cluster`,
	}

	var secretName string
	it := s.smClient.ListSecrets(ctx, req)
	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list argocd-cluster secrets: %w", err)
		}
		secretName = secret.Name
		break
	}

	if secretName == "" {
		return nil, fmt.Errorf("no secret matching name:argocd-cluster with labels.infra-type=region found in project %s", s.project)
	}

	result, err := s.smClient.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName + "/versions/latest",
	})
	if err != nil {
		return nil, fmt.Errorf("access secret %s: %w", secretName, err)
	}

	var data struct {
		MetaHCDNSDomains string `json:"meta_hc_dns_domains"`
	}
	if err := json.Unmarshal(result.Payload.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal secret payload: %w", err)
	}

	var domains []string
	for _, d := range strings.Split(data.MetaHCDNSDomains, ",") {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	return domains, nil
}
