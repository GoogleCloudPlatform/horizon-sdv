// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"context"
	"encoding/json"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type mirroredKCCStatus struct {
	Phase              string
	ObservedGeneration int64
	Conditions         []metav1.Condition
	Raw                []byte
	Host               string
	State              string
	ExternalRef        string
}

// mirrorKCCStatus copies Config Connector status onto a Horizon object.
// When status.observedGeneration is present and behind metadata.generation, the result stays Pending
// and does not report the Horizon generation as observed. A missing observedGeneration keeps the
// previous behavior of reporting the Horizon generation once any status exists.
// readyAs renames a KCC Ready condition. An empty readyAs keeps the type Ready.
func mirrorKCCStatus(kccObj *unstructured.Unstructured, horizonGeneration int64, previous []metav1.Condition, readyAs string) mirroredKCCStatus {
	out := mirroredKCCStatus{Phase: "Pending"}
	statusObj, found, err := unstructured.NestedMap(kccObj.Object, "status")
	if err != nil || !found || statusObj == nil {
		return out
	}
	if ref, ok := statusObj["externalRef"].(string); ok {
		out.ExternalRef = ref
	}
	if observed, ok := kccStatusObservedGeneration(statusObj); ok && observed != kccObj.GetGeneration() {
		return out
	}
	out.ObservedGeneration = horizonGeneration
	if rawConditions, found, _ := unstructured.NestedSlice(statusObj, "conditions"); found {
		var ready bool
		out.Conditions, ready = conditionsFromKCC(rawConditions, previous, readyAs)
		if ready {
			out.Phase = "Ready"
		}
	}
	rawObserved, found, _ := unstructured.NestedMap(statusObj, "observedState")
	if !found || rawObserved == nil {
		return out
	}
	if host, ok := rawObserved["host"].(string); ok {
		out.Host = host
	}
	if state, ok := rawObserved["state"].(string); ok {
		out.State = state
	}
	if data, err := json.Marshal(rawObserved); err == nil {
		out.Raw = data
	}
	return out
}

func kccStatusObservedGeneration(status map[string]interface{}) (int64, bool) {
	value, ok := status["observedGeneration"]
	if !ok || value == nil {
		return 0, false
	}
	switch n := value.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		parsed, err := n.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// kccChildBlocksDelete reports one Config Connector child whose parentRef.name is parentName.
// A missing CRD is not a blocker. Any other list error is returned.
func kccChildBlocksDelete(ctx context.Context, c client.Reader, namespace, parentName string, listGVK schema.GroupVersionKind) (string, error) {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(listGVK)
	if err := c.List(ctx, ul, client.InNamespace(namespace)); err != nil {
		if meta.IsNoMatchError(err) {
			return "", nil
		}
		return "", err
	}
	for i := range ul.Items {
		parent, _, _ := unstructured.NestedString(ul.Items[i].Object, "spec", "parentRef", "name")
		if parent == parentName {
			return ul.Items[i].GetName(), nil
		}
	}
	return "", nil
}

// finishKCCDelete deletes the owned Config Connector object and removes finalizer only after it is gone.
func finishKCCDelete(ctx context.Context, c client.Client, owner client.Object, gvk schema.GroupVersionKind, finalizer string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(gvk)
	kccObj.SetName(owner.GetName())
	kccObj.SetNamespace(owner.GetNamespace())
	if err := c.Delete(ctx, kccObj); err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to delete Config Connector object", "kind", gvk.Kind)
		return ctrl.Result{}, err
	}
	err := c.Get(ctx, client.ObjectKeyFromObject(kccObj), kccObj)
	switch {
	case err == nil:
		logger.Info("waiting for Config Connector object to be deleted", "kind", gvk.Kind, "name", owner.GetName())
		return kccWait(), nil
	case apierrors.IsNotFound(err):
		controllerutil.RemoveFinalizer(owner, finalizer)
		return ctrl.Result{}, c.Update(ctx, owner)
	default:
		return ctrl.Result{}, err
	}
}
