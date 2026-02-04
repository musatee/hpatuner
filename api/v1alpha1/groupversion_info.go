/*
Copyright 2026.
Project: hpatuner
Author: Md. Abul Kalam Musa
Licensed under the Apache License, Version 2.0 (the "License");
*/

// Package v1alpha1 contains API Schema definitions for the mycrds v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=mycrds.akmusa.com
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "mycrds.akmusa.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
