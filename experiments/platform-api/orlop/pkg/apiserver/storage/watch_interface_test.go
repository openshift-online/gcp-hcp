package storage

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDefaultWatchFilter_Matches(t *testing.T) {
	filter := &DefaultWatchFilter{}

	newObj := func(namespace string, objLabels map[string]string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "test/v1",
				"kind":       "Test",
				"metadata": map[string]interface{}{
					"name":      "test-obj",
					"namespace": namespace,
				},
			},
		}
		if objLabels != nil {
			obj.SetLabels(objLabels)
		}
		return obj
	}

	tests := []struct {
		name  string
		event ResourceEvent
		opts  client.ListOptions
		want  bool
	}{
		{
			name:  "bookmark events always match",
			event: ResourceEvent{Type: EventBookmark},
			opts:  client.ListOptions{Namespace: "other"},
			want:  true,
		},
		{
			name:  "nil object does not match",
			event: ResourceEvent{Type: EventAdded, Object: nil},
			opts:  client.ListOptions{},
			want:  false,
		},
		{
			name:  "matches with no filters",
			event: ResourceEvent{Type: EventAdded, Object: newObj("default", nil)},
			opts:  client.ListOptions{},
			want:  true,
		},
		{
			name:  "matches same namespace",
			event: ResourceEvent{Type: EventModified, Object: newObj("default", nil)},
			opts:  client.ListOptions{Namespace: "default"},
			want:  true,
		},
		{
			name:  "rejects different namespace",
			event: ResourceEvent{Type: EventAdded, Object: newObj("default", nil)},
			opts:  client.ListOptions{Namespace: "other"},
			want:  false,
		},
		{
			name: "matches label selector",
			event: ResourceEvent{
				Type:   EventAdded,
				Object: newObj("default", map[string]string{"app": "web"}),
			},
			opts: client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labels.Set{"app": "web"}),
			},
			want: true,
		},
		{
			name: "rejects non-matching label selector",
			event: ResourceEvent{
				Type:   EventAdded,
				Object: newObj("default", map[string]string{"app": "api"}),
			},
			opts: client.ListOptions{
				LabelSelector: labels.SelectorFromSet(labels.Set{"app": "web"}),
			},
			want: false,
		},
		{
			name: "matches namespace and label selector together",
			event: ResourceEvent{
				Type:   EventAdded,
				Object: newObj("prod", map[string]string{"tier": "frontend"}),
			},
			opts: client.ListOptions{
				Namespace:     "prod",
				LabelSelector: labels.SelectorFromSet(labels.Set{"tier": "frontend"}),
			},
			want: true,
		},
		{
			name: "rejects when namespace matches but labels dont",
			event: ResourceEvent{
				Type:   EventAdded,
				Object: newObj("prod", map[string]string{"tier": "backend"}),
			},
			opts: client.ListOptions{
				Namespace:     "prod",
				LabelSelector: labels.SelectorFromSet(labels.Set{"tier": "frontend"}),
			},
			want: false,
		},
		{
			name:  "deleted event matches",
			event: ResourceEvent{Type: EventDeleted, Object: newObj("default", nil)},
			opts:  client.ListOptions{Namespace: "default"},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.Matches(tt.event, tt.opts)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
