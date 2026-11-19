// Offline reproduction only: in-memory stores and httptest, no network or auth.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-logr/logr"
	"github.com/openshift-online/gecko/orlop/pkg/apiserver/conversion"
	"github.com/openshift-online/gecko/orlop/pkg/apiserver/handlers"
	pkgschema "github.com/openshift-online/gecko/orlop/pkg/apiserver/schema"
	"github.com/openshift-online/gecko/orlop/pkg/apiserver/storage/memory"
	privatev1 "github.com/openshift-online/gecko/platform-api/api/private/v1"
	publicv1 "github.com/openshift-online/gecko/platform-api/api/public/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func main() {
	ctx := context.Background()
	private, public := runtime.NewScheme(), runtime.NewScheme()
	if err := privatev1.AddToScheme(private); err != nil {
		panic(err)
	}
	if err := publicv1.AddToScheme(public); err != nil {
		panic(err)
	}
	gvk := privatev1.GroupVersion.WithKind("Cluster")
	store := memory.NewMemoryStore("clusters", private, gvk)
	c := &privatev1.Cluster{TypeMeta: metav1.TypeMeta{APIVersion: privatev1.GroupVersion.String(), Kind: "Cluster"}, ObjectMeta: metav1.ObjectMeta{Name: "wif3-first", Namespace: "apahim-dev-test-1", UID: "6b152684-797c-4f17-a258-43538f8013c8", Generation: 2}}
	if err := store.Create(ctx, c); err != nil {
		panic(err)
	}
	converter := conversion.NewConverter(public, private, "")
	var propsV1 apiextv1.JSONSchemaProps
	if err := yaml.Unmarshal([]byte(publicv1.ClusterSchemaYAML), &propsV1); err != nil {
		panic(err)
	}
	var props apiext.JSONSchemaProps
	if err := apiextv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&propsV1, &props, nil); err != nil {
		panic(err)
	}
	structural, err := structuralschema.NewStructural(&props)
	if err != nil {
		panic(err)
	}
	processor, err := pkgschema.NewProcessor(structural, &props)
	if err != nil {
		panic(err)
	}
	h := handlers.NewConvertingResourceHandler(store, processor, converter, gvk, "clusters", public, private, nil, logr.Discard())
	router := chi.NewRouter()
	router.Patch("/apis/gcp.managed.openshift.io/v1/namespaces/{namespace}/clusters/{name}", h.Patch)
	// Identical operation structure to the approved live request; no protected
	// state, credentials or live endpoints are needed for this reproduction.
	patch, err := json.Marshal([]map[string]any{
		{"op": "test", "path": "/metadata/uid", "value": "00000000-0000-0000-0000-000000000000"},
		{"op": "replace", "path": "/spec/platform/gcp/iam/templateApplications", "value": []map[string]any{{"name": "5.0.0", "uid": "fb623828-d923-4540-82f1-df5c01503823", "digest": "sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71"}}},
	})
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/apis/gcp.managed.openshift.io/v1/namespaces/apahim-dev-test-1/clusters/wif3-first", bytes.NewReader(patch))
	req.Header.Set("Content-Type", "application/json-patch+json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	result := map[string]any{"offlineHTTPStatus": w.Code, "responseSHA256": fmt.Sprintf("sha256:%x", sha256.Sum256(w.Body.Bytes())), "offlineMessage": body["message"], "usesLiveNetwork": false}
	_ = json.NewEncoder(os.Stdout).Encode(result)
}
