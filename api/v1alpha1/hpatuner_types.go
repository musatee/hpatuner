/*
Copyright 2026.
Project: hpatuner
Author: Md. Abul Kalam Musa
Licensed under the Apache License, Version 2.0 (the "License");
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

func init() {
	SchemeBuilder.Register(&HpaTuner{}, &HpaTunerList{})
}

// HpaTunerSpec defines the desired state of HpaTuner
type HpaTunerSpec struct {
	HpaName               string `json:"hpaName"`
	HpaNamespace          string `json:"hpaNamespace"`
	MetricEndpoint        string `json:"metricEndpoint"`
	MetricThreshold       int64  `json:"metricThreshold"`
	HpaMaxCeilingReplicas int32  `json:"hpaMaxReplicas"`
}

// HpaTunerStatus defines the observed state of HpaTuner.
type HpaTunerStatus struct {
	LastObservedMin int32       `json:"lastObservedMin,omitempty"`
	LastObservedMax int32       `json:"lastObservedMax,omitempty"`
	LastMetricValue string      `json:"lastMetricValue,omitempty"`
	LastUpdateTime  metav1.Time `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// HpaTuner is the Schema for the hpatuners API
type HpaTuner struct {
	metav1.TypeMeta `json:",inline"`
	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`
	// spec defines the desired state of HpaTuner
	// +required
	Spec HpaTunerSpec `json:"spec"`
	// status defines the observed state of HpaTuner
	// +optional
	Status HpaTunerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true
// HpaTunerList contains a list of HpaTuner
type HpaTunerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HpaTuner `json:"items"`
}
