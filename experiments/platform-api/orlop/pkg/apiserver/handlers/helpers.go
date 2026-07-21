package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/thetechnick/orlop/pkg/apiserver/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// writeError writes an error response with a Status object.
func writeError(w http.ResponseWriter, code int, message string) {
	status := metav1.Status{
		TypeMeta: metav1.TypeMeta{
			APIVersion: constants.APIVersionV1,
			Kind:       constants.KindStatus,
		},
		Status:  metav1.StatusFailure,
		Message: message,
		Code:    int32(code),
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(status)
}

// specChanged checks if the spec field has changed between two objects.
func specChanged(old, new runtime.Object) bool {
	oldJSON, _ := json.Marshal(old)
	newJSON, _ := json.Marshal(new)

	var oldMap, newMap map[string]interface{}
	json.Unmarshal(oldJSON, &oldMap)
	json.Unmarshal(newJSON, &newMap)

	oldSpec, _ := json.Marshal(oldMap["spec"])
	newSpec, _ := json.Marshal(newMap["spec"])

	return string(oldSpec) != string(newSpec)
}
