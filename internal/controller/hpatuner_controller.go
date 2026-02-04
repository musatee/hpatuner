/*
Copyright 2026.
Project: hpatuner
Author: Md. Abul Kalam Musa
Licensed under the Apache License, Version 2.0 (the "License");
*/

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	mycrdsv1alpha1 "github.com/musatee/hpatuner/api/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// HpaTunerReconciler reconciles a HpaTuner object
type HpaTunerReconciler struct {
	client        client.Client
	Scheme        *runtime.Scheme
	cache         cache.Cache
	eventRecorder record.EventRecorder
}

// +kubebuilder:rbac:groups=mycrds.akmusa.com,resources=hpatuners,                                                          verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=HorizontalPodAutoscaler,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mycrds.akmusa.com,resources=hpatuners/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mycrds.akmusa.com,resources=hpatuners/finalizers,verbs=update

func (r *HpaTunerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Reconciliation started", "resource", req.Name, "namespace", req.Namespace)

	var hpaTunerData mycrdsv1alpha1.HpaTuner
	err := r.client.Get(ctx, req.NamespacedName, &hpaTunerData)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Resource not found, likely deleted", "resource", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch HpaTuner resource", "resource", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, err
	}

	// sending event on reconciliation start (after fetching resource so we have the actual object)
	r.eventRecorder.Eventf(&hpaTunerData, corev1.EventTypeNormal, "Reconciling", "Starting reconciliation for resource %s", req.Name)

	logger.Info("HpaTuner resource fetched successfully",
		"resource", req.Name,
		"targetHpa", hpaTunerData.Spec.HpaName,
		"metricEndpoint", hpaTunerData.Spec.MetricEndpoint)

	hpaNamespacedName := types.NamespacedName{
		Namespace: hpaTunerData.Spec.HpaNamespace,
		Name:      hpaTunerData.Spec.HpaName,
	}

	logger.Info("Fetching target HPA", "hpa", hpaTunerData.Spec.HpaName, "hpaNamespace", hpaTunerData.Spec.HpaNamespace)
	hpa, err := r.getHpa(ctx, hpaNamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Target HPA not found", "hpa", hpaNamespacedName.Name, "hpaNamespace", hpaNamespacedName.Namespace)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to fetch HPA", "hpa", hpaNamespacedName.Name, "hpaNamespace", hpaNamespacedName.Namespace)
		return ctrl.Result{}, err
	}

	logger.Info("HPA fetched successfully", "hpa", hpa.Name, "currentMaxReplicas", hpa.Spec.MaxReplicas)

	logger.Info("Fetching metrics from endpoint", "endpoint", hpaTunerData.Spec.MetricEndpoint)
	error_rate, err := getErrorRate(hpaTunerData.Spec.MetricEndpoint)
	if err != nil {
		logger.Error(err, "Failed to fetch metrics", "endpoint", hpaTunerData.Spec.MetricEndpoint)
		return ctrl.Result{}, err
	}

	logger.Info("Metrics fetched successfully", "errorRate", *error_rate, "threshold", hpaTunerData.Spec.MetricThreshold)

	// Check if threshold is breached
	if *error_rate > float64(hpaTunerData.Spec.MetricThreshold) {
		hpa_current_min_replica := *hpa.Spec.MinReplicas
		hpa_max_ceiling_replica := hpaTunerData.Spec.HpaMaxCeilingReplicas

		hpa_desired_max_replica := hpa_max_ceiling_replica
		hpa_desired_min_replica := min(hpa_current_min_replica+2, hpa_desired_max_replica)

		logger.Info("Threshold breached, updating HPA replicas",
			"currentMin", hpa_current_min_replica,
			"currentMax", hpa.Spec.MaxReplicas,
			"desiredMin", hpa_desired_min_replica,
			"desiredMax", hpa_desired_max_replica)
		// Creating event on updating target HPA
		r.eventRecorder.Eventf(&hpaTunerData, corev1.EventTypeNormal, "ThresholdBreached", "Updating target HPA %s/%s (min: %d->%d, max: %d->%d)",
			hpa.Namespace, hpa.Name, hpa_current_min_replica, hpa_desired_min_replica, hpa.Spec.MaxReplicas, hpa_desired_max_replica)

		if err := r.updateHpaReplicas(ctx, hpa, hpa_desired_min_replica, hpa_desired_max_replica); err != nil {
			if errors.IsConflict(err) {
				logger.Info("Conflict updating HPA, retrying immediately", "error", err)
				return ctrl.Result{Requeue: true}, nil
			}
			if errors.IsNotFound(err) {
				logger.Info("HPA not found during update, likely deleted")
				return ctrl.Result{}, nil
			}
			logger.Error(err, "Failed to patch HPA")
			// Creating Event on Update Failure
			r.eventRecorder.Eventf(&hpaTunerData, corev1.EventTypeWarning, "UpdateFailed", "Failed to update HPA %s/%s: %v", hpa.Namespace, hpa.Name, err)
			return ctrl.Result{}, err
		}
		logger.Info("HPA updated successfully")
		// Creating event on successful update
		r.eventRecorder.Eventf(&hpaTunerData, corev1.EventTypeNormal, "HPAUpdated", "Successfully updated HPA %s/%s", hpa.Namespace, hpa.Name)

		// Update status after successful HPA update
		if err := r.updateStatus(ctx, &hpaTunerData, *error_rate, hpa_desired_min_replica, hpa_desired_max_replica); err != nil {
			logger.Error(err, "Failed to update status after HPA update")
		}
	} else {
		// Threshold not breached, but still update status with current observation
		logger.Info("Threshold not breached, no HPA changes needed")
		if err := r.updateStatus(ctx, &hpaTunerData, *error_rate, *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas); err != nil {
			logger.Error(err, "Failed to update status")
		}
	}
	logger.Info("Reconciliation completed successfully")
	// Creating event on reconciliation completion
	r.eventRecorder.Eventf(&hpaTunerData, corev1.EventTypeNormal, "ReconciliationComplete", "Successfully reconciled resource %s", req.Name)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HpaTunerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mycrdsv1alpha1.HpaTuner{}).
		Named("hpatuner").
		Complete(r)
}
func NewHpaTunerReconciler(client client.Client, Scheme *runtime.Scheme, cache cache.Cache, eventRecorder record.EventRecorder) *HpaTunerReconciler {
	return &HpaTunerReconciler{
		client:        client,
		Scheme:        Scheme,
		cache:         cache,
		eventRecorder: eventRecorder,
	}
}
func getErrorRate(metric_endpoint string) (*float64, error) {
	type responseData struct {
		Error_rate float64 `json:"error_rate"`
		Message    string  `json:"message"`
	}
	var respData responseData

	req, err := http.NewRequest("GET", metric_endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&respData); err != nil {
		return nil, err
	}
	return &respData.Error_rate, nil
}
func (r *HpaTunerReconciler) getHpa(ctx context.Context, namespacedName types.NamespacedName) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	var hpa autoscalingv2.HorizontalPodAutoscaler
	err := r.client.Get(ctx, namespacedName, &hpa)
	if err != nil {
		return nil, err
	}
	return &hpa, nil
}
func (r *HpaTunerReconciler) updateHpaReplicas(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler, desired_min_replica, desired_max_replica int32) error {
	hpaPatch := &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2",
			Kind:       "HorizontalPodAutoscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      hpa.Name,
			Namespace: hpa.Namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: hpa.Spec.ScaleTargetRef,
			MinReplicas:    &desired_min_replica,
			MaxReplicas:    desired_max_replica,
		},
	}
	// Apply patch using Server-Side Apply
	return r.client.Patch(ctx, hpaPatch, client.Apply, client.FieldOwner("hpatuner-controller"), client.ForceOwnership)
}

func (r *HpaTunerReconciler) updateStatus(ctx context.Context, hpaTuner *mycrdsv1alpha1.HpaTuner, metricValue float64, observedMin, observedMax int32) error {
	// Update status fields
	hpaTuner.Status.LastMetricValue = strconv.FormatFloat(metricValue, 'f', 2, 64)
	hpaTuner.Status.LastObservedMin = observedMin
	hpaTuner.Status.LastObservedMax = observedMax
	hpaTuner.Status.LastUpdateTime = metav1.Now()

	return r.client.Status().Update(ctx, hpaTuner)
}
