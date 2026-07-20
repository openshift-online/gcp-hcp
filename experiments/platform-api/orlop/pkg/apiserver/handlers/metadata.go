package handlers

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateMetadata(obj metav1.Object) error {
	var allErrs field.ErrorList
	fldPath := field.NewPath("metadata")

	for k, v := range obj.GetLabels() {
		labelPath := fldPath.Child("labels")
		for _, msg := range validation.IsQualifiedName(k) {
			allErrs = append(allErrs, field.Invalid(labelPath, k, msg))
		}
		for _, msg := range validation.IsValidLabelValue(v) {
			allErrs = append(allErrs, field.Invalid(labelPath.Key(k), v, msg))
		}
	}

	allErrs = append(allErrs, apivalidation.ValidateAnnotations(obj.GetAnnotations(), fldPath.Child("annotations"))...)

	return allErrs.ToAggregate()
}
