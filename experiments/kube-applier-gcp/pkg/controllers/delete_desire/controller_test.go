package delete_desire

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
	clocktesting "k8s.io/utils/clock/testing"

	"github.com/openshift/kube-applier-gcp/internal/api/kubeapplier"
	"github.com/openshift/kube-applier-gcp/internal/controllerutils"
	"github.com/openshift/kube-applier-gcp/internal/database/listertesting"
	"github.com/openshift/kube-applier-gcp/pkg/controllers/conditions"
	"github.com/openshift/kube-applier-gcp/pkg/controllers/desirestatuswriter"
	"github.com/openshift/kube-applier-gcp/pkg/controllers/keys"
)

var configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

func fakeDynamic(t *testing.T, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) *fake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	return fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
}

func configMapTarget(name string) kubeapplier.ResourceReference {
	return kubeapplier.ResourceReference{
		Group: "", Version: "v1", Resource: "configmaps", Namespace: "default", Name: name,
	}
}

func newDeleteDesire(t *testing.T, name string, target kubeapplier.ResourceReference) *kubeapplier.DeleteDesire {
	t.Helper()
	d := &kubeapplier.DeleteDesire{}
	d.SetDocumentID("cluster1--" + name)
	d.SetUpdateTime(time.Unix(1, 0))
	d.Spec = kubeapplier.DeleteDesireSpec{
		ManagementCluster: "mc-1",
		ClusterID:         "cluster1",
		TargetItem:        target,
	}
	return d
}

func mustKey(t *testing.T, d *kubeapplier.DeleteDesire) keys.DeleteDesireKey {
	t.Helper()
	key, err := keys.DeleteDesireKeyFromDesire(d)
	if err != nil {
		t.Fatalf("derive key: %v", err)
	}
	return key
}

func newCadenceController(t *testing.T, cfg Config) *DeleteDesireController {
	t.Helper()
	cfg = cfg.withDefaults()
	checker := controllerutils.NewTimeBasedCooldownChecker(cfg.CooldownPeriod)
	checker.SetClock(cfg.Clock)
	return &DeleteDesireController{
		name: "DeleteDesireController",
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[keys.DeleteDesireKey](),
			workqueue.TypedRateLimitingQueueConfig[keys.DeleteDesireKey]{Name: "test"},
		),
		cfg:      cfg,
		cooldown: checker,
	}
}

func existingConfigMap(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{
			"name": name, "namespace": "default",
			"uid": "test-uid-123",
		},
	}}
}

func terminatingConfigMap(name string) *unstructured.Unstructured {
	now := metav1.Now()
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{
			"name": name, "namespace": "default",
			"uid":               "test-uid-123",
			"deletionTimestamp":  now.Format(time.RFC3339),
			"deletionGracePeriodSeconds": int64(0),
		},
	}}
}

// --- evaluate() State Machine Tests ---

func TestEvaluate_TargetAbsent(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)

	if evalErr != nil {
		t.Fatalf("evaluate returned error: %v", evalErr)
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionTrue, "")
}

func TestEvaluate_Target404Race(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"}, existingConfigMap("cm1"))

	callCount := 0
	dyn.PrependReactor("delete", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "cm1")
	})
	dyn.PrependReactor("get", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		callCount++
		if callCount == 1 {
			return true, existingConfigMap("cm1"), nil
		}
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "cm1")
	})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)
	if evalErr != nil {
		t.Fatalf("evaluate returned unexpected error: %v", evalErr)
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionTrue, "")
}

func TestEvaluate_AlreadyTerminating(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})
	dyn.PrependReactor("get", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, terminatingConfigMap("cm1"), nil
	})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)
	if evalErr != nil {
		t.Fatalf("evaluate returned unexpected error: %v", evalErr)
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionFalse, kubeapplier.ConditionReasonWaitingForDeletion)
}

func TestEvaluate_DeleteSucceeds_Terminating(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})

	getCount := 0
	dyn.PrependReactor("get", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		getCount++
		if getCount == 1 {
			return true, existingConfigMap("cm1"), nil
		}
		return true, terminatingConfigMap("cm1"), nil
	})
	dyn.PrependReactor("delete", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)
	if evalErr != nil {
		t.Fatalf("evaluate returned unexpected error: %v", evalErr)
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionFalse, kubeapplier.ConditionReasonWaitingForDeletion)
	assertConditionMessage(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, "test-uid-123")
}

func TestEvaluate_DeleteSucceeds_ImmediateGone(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})

	getCount := 0
	dyn.PrependReactor("get", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		getCount++
		if getCount == 1 {
			return true, existingConfigMap("cm1"), nil
		}
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "cm1")
	})
	dyn.PrependReactor("delete", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)
	if evalErr != nil {
		t.Fatalf("evaluate returned unexpected error: %v", evalErr)
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionTrue, "")
}

func TestEvaluate_DeleteReturns500(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})
	dyn.PrependReactor("get", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, existingConfigMap("cm1"), nil
	})
	dyn.PrependReactor("delete", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, &apierrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusInternalServerError,
			Reason: metav1.StatusReasonInternalError, Message: "internal",
		}}
	})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)

	if evalErr == nil {
		t.Fatal("expected error from evaluate")
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionFalse, kubeapplier.ConditionReasonKubeAPIError)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeDegraded, metav1.ConditionTrue, "")
}

func TestEvaluate_GetReturns500(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})
	dyn.PrependReactor("get", "configmaps", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, &apierrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusInternalServerError,
			Reason: metav1.StatusReasonInternalError, Message: "internal",
		}}
	})

	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c := &DeleteDesireController{dyn: dyn}
	mutate, evalErr := c.evaluate(ctx, d)

	if evalErr == nil {
		t.Fatal("expected error from evaluate")
	}
	mutate(d)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionFalse, kubeapplier.ConditionReasonKubeAPIError)
	assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeDegraded, metav1.ConditionTrue, "")
}

func TestEvaluate_PreCheck_MissingFields(t *testing.T) {
	ctx := context.Background()
	c := &DeleteDesireController{}
	tests := []struct {
		name   string
		target kubeapplier.ResourceReference
	}{
		{"missing version", kubeapplier.ResourceReference{Resource: "configmaps", Name: "cm1"}},
		{"missing resource", kubeapplier.ResourceReference{Version: "v1", Name: "cm1"}},
		{"missing name", kubeapplier.ResourceReference{Version: "v1", Resource: "configmaps"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeleteDesire(t, "cm1", tt.target)
			mutate, evalErr := c.evaluate(ctx, d)
			if evalErr == nil {
				t.Fatal("expected error")
			}
			var preCheck *conditions.PreCheckError
			if !errors.As(evalErr, &preCheck) {
				t.Errorf("expected PreCheckError, got %T: %v", evalErr, evalErr)
			}
			mutate(d)
			assertCondition(t, d.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionFalse, kubeapplier.ConditionReasonPreCheckFailed)
		})
	}
}

// --- SyncOnce Tests ---

func TestSyncOnce_DesireNotFound(t *testing.T) {
	ctx := context.Background()
	crud := listertesting.NewFakeCRUD[kubeapplier.DeleteDesire, *kubeapplier.DeleteDesire]()
	c := &DeleteDesireController{specFetcher: &deleteDesireSpecFetcher{reader: crud}}

	key := keys.DeleteDesireKey{ClusterID: "c1", Name: "cluster1--cm1"}
	if err := c.SyncOnce(ctx, key); err != nil {
		t.Fatalf("SyncOnce should return nil for not-found, got: %v", err)
	}
}

func TestSyncOnce_Success_TargetAbsent(t *testing.T) {
	ctx := context.Background()
	dyn := fakeDynamic(t, map[schema.GroupVersionResource]string{configMapGVR: "ConfigMapList"})

	specCRUD := listertesting.NewFakeCRUD[kubeapplier.DeleteDesire, *kubeapplier.DeleteDesire]()
	statusCRUD := listertesting.NewFakeCRUD[kubeapplier.DeleteDesire, *kubeapplier.DeleteDesire]()
	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	created, err := specCRUD.Create(ctx, d)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	statusFetcher := &deleteDesireStatusFetcher{crud: statusCRUD}
	writer := desirestatuswriter.New[kubeapplier.DeleteDesire, keys.DeleteDesireKey, *kubeapplier.DeleteDesire](
		statusFetcher, &deleteDesireReplacer{crud: statusCRUD}, &deleteDesireCreator{crud: statusCRUD},
	)
	c := &DeleteDesireController{
		specFetcher: &deleteDesireSpecFetcher{reader: specCRUD},
		dyn:         dyn,
		writer:      writer,
	}

	key := mustKey(t, created)
	if err := c.SyncOnce(ctx, key); err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	updated, err := statusCRUD.Get(ctx, created.DocumentID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	assertCondition(t, updated.Status.Conditions, kubeapplier.ConditionTypeSuccessful, metav1.ConditionTrue, "")
	if !updated.Status.ObservedDesireUpdateTime.Equal(created.GetUpdateTime()) {
		t.Errorf("ObservedDesireUpdateTime: got %v, want %v", updated.Status.ObservedDesireUpdateTime, created.GetUpdateTime())
	}
}

// --- Cadence Tests ---

func TestHandleAdd_QueuesImmediately(t *testing.T) {
	c := newCadenceController(t, Config{})
	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	c.handleAdd(d)
	if c.queue.Len() != 1 {
		t.Errorf("expected 1 item, got %d", c.queue.Len())
	}
}

func TestHandleUpdate_ChangedQueuesImmediately(t *testing.T) {
	c := newCadenceController(t, Config{})
	d1 := newDeleteDesire(t, "cm1", configMapTarget("cm1"))
	d1.SetUpdateTime(time.Unix(1, 0))
	d2 := d1.DeepCopy()
	d2.SetUpdateTime(time.Unix(2, 0))
	c.handleUpdate(d1, d2)
	if c.queue.Len() != 1 {
		t.Errorf("expected 1 item, got %d", c.queue.Len())
	}
}

func TestHandleUpdate_UnchangedConsultsCooldown(t *testing.T) {
	now := time.Now()
	fakeClock := clocktesting.NewFakePassiveClock(now)
	c := newCadenceController(t, Config{
		CooldownPeriod: 1 * time.Minute,
		Clock:          fakeClock,
	})
	d := newDeleteDesire(t, "cm1", configMapTarget("cm1"))

	c.handleUpdate(d, d)
	if c.queue.Len() != 1 {
		t.Fatalf("expected 1 item after first unchanged update, got %d", c.queue.Len())
	}
	key, _ := c.queue.Get()
	c.queue.Done(key)

	fakeClock.SetTime(now.Add(30 * time.Second))
	c.handleUpdate(d, d)
	if c.queue.Len() != 0 {
		t.Errorf("expected 0 items within cooldown, got %d", c.queue.Len())
	}

	fakeClock.SetTime(now.Add(2 * time.Minute))
	c.handleUpdate(d, d)
	if c.queue.Len() != 1 {
		t.Errorf("expected 1 item after cooldown, got %d", c.queue.Len())
	}
}

func TestDefaultCooldownPeriod_IsOneMinute(t *testing.T) {
	if DefaultCooldownPeriod != 1*time.Minute {
		t.Errorf("expected 1m, got %v", DefaultCooldownPeriod)
	}
}

// --- Helpers ---

func assertCondition(t *testing.T, conds []metav1.Condition, condType string, status metav1.ConditionStatus, reason string) {
	t.Helper()
	for _, c := range conds {
		if c.Type == condType {
			if c.Status != status {
				t.Errorf("condition %s: expected status %s, got %s", condType, status, c.Status)
			}
			if reason != "" && c.Reason != reason {
				t.Errorf("condition %s: expected reason %s, got %s", condType, reason, c.Reason)
			}
			return
		}
	}
	t.Errorf("condition %s not found in %+v", condType, conds)
}

func assertConditionMessage(t *testing.T, conds []metav1.Condition, condType string, substr string) {
	t.Helper()
	for _, c := range conds {
		if c.Type == condType {
			if !strings.Contains(c.Message, substr) {
				t.Errorf("condition %s message %q does not contain %q", condType, c.Message, substr)
			}
			return
		}
	}
	t.Errorf("condition %s not found", condType)
}

// suppress unused import warnings
var _ = types.UID("")
