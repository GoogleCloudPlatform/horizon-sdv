// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
)

const kccWaitInterval = 10 * time.Second

// kccWait retries a reconcile that is waiting on a parent or a KCC object.
func kccWait() ctrl.Result {
	return ctrl.Result{RequeueAfter: kccWaitInterval}
}

// sortedKeyValueList builds the KCC [{key, value}] form with a stable key order.
func sortedKeyValueList(values map[string]string) []interface{} {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		out = append(out, map[string]interface{}{"key": key, "value": values[key]})
	}
	return out
}

func toInterfaceSlice(values []string) []interface{} {
	if len(values) == 0 {
		return nil
	}
	out := make([]interface{}, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func setKeyValueList(spec map[string]interface{}, field string, values map[string]string) {
	if list := sortedKeyValueList(values); len(list) > 0 {
		spec[field] = list
	}
}

// objectRef copies a KCC object reference. An empty reference is omitted.
func objectRef(external, name, namespace string) map[string]interface{} {
	ref := map[string]interface{}{}
	if external != "" {
		ref["external"] = external
	}
	if name != "" {
		ref["name"] = name
	}
	if namespace != "" {
		ref["namespace"] = namespace
	}
	if len(ref) == 0 {
		return nil
	}
	return ref
}

// conditionsFromKCC projects KCC conditions. lastTransitionTime is taken from KCC
// when present. Otherwise the previous Horizon time is kept while the condition
// is unchanged, so a steady KCC status does not rewrite Horizon status.
func conditionsFromKCC(raw []interface{}, previous []metav1.Condition, readyAs string) (conditions []metav1.Condition, ready bool) {
	for _, item := range raw {
		condMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		cType, _ := condMap["type"].(string)
		cStatus, _ := condMap["status"].(string)
		cReason, _ := condMap["reason"].(string)
		cMessage, _ := condMap["message"].(string)
		if cType == "Ready" && cStatus == "True" {
			ready = true
		}
		if cType == "Ready" && readyAs != "" {
			cType = readyAs
		}
		conditions = append(conditions, metav1.Condition{
			Type:               cType,
			Status:             metav1.ConditionStatus(cStatus),
			Reason:             cReason,
			Message:            cMessage,
			LastTransitionTime: conditionTransitionTime(condMap, previous, cType, cStatus, cReason, cMessage),
		})
	}
	return conditions, ready
}

func conditionTransitionTime(raw map[string]interface{}, previous []metav1.Condition, cType, cStatus, cReason, cMessage string) metav1.Time {
	if value, _, _ := unstructured.NestedString(raw, "lastTransitionTime"); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return metav1.NewTime(parsed)
		}
	}
	for _, existing := range previous {
		if existing.Type == cType && string(existing.Status) == cStatus && existing.Reason == cReason && existing.Message == cMessage && !existing.LastTransitionTime.IsZero() {
			return existing.LastTransitionTime
		}
	}
	return metav1.Now()
}
