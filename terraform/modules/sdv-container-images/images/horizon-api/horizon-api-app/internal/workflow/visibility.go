// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package workflow

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation"
)

// LabelSubmittedFrom is set on Workflow metadata when the run is dispatched via Horizon (sensor maps webhook body).
// Values match Argo-style lowercase tokens (compare workflows.argoproj.io/submit-from-ui).
const LabelSubmittedFrom = "horizon-sdv.io/submitted-from"
// LabelSubmittedBy is the Workflow annotation that holds the owner: the token subject of the
// caller who submitted the run through Horizon API (sensor maps it from the webhook body).
const LabelSubmittedBy = "horizon-sdv.io/submitted-by"
// AnnotationSubmittedByName is the Workflow annotation with the submitter's username, for display
// only (sensors map it from the submittedBy webhook field). Never use it for access decisions.
const AnnotationSubmittedByName = "horizon-sdv.io/submitted-by-name"
const LabelModule = "horizon-sdv.io/module"

const (
	SubmittedFromAPI             = "api"
	SubmittedFromDeveloperPortal = "developer-portal"
	SubmittedFromHorizonCLI      = "horizon-cli"
	SubmitParamSubmittedFrom     = "horizonSubmittedFrom"
	// SubmitParamSubmittedBy is the webhook field and workflow parameter with the caller's username;
	// sensors map the field to AnnotationSubmittedByName.
	SubmitParamSubmittedBy = "submittedBy"
	// WebhookFieldSubmittedBy is the webhook field with the owner; sensors map it to LabelSubmittedBy.
	WebhookFieldSubmittedBy = "horizonSubmittedBy"
	// SubmitParamSampleSoftEnabled is set only by Horizon API (runtime injection), not by clients.
	SubmitParamSampleSoftEnabled = "sampleSoftEnabled"
	// SubmittedFromCustomToken is a reserved header value: the real id is read from HeaderSubmittedFromDetail.
	SubmittedFromCustomToken = "custom"
	// HeaderSubmittedFromDetail carries the integration id when X-Horizon-Submitted-From is "custom".
	HeaderSubmittedFromDetail = "X-Horizon-Submitted-From-Detail"
	// HeaderSubmittedBy names the user a run is for. It is honoured only from a caller with the
	// impersonate role; from anyone else it is ignored.
	HeaderSubmittedBy = "X-Horizon-Submitted-By"
)

// IsHorizonInjectedWorkflowParameter is true for workflow parameters that must not appear in the
// Horizon catalog or OpenAPI submit schema; the API injects them when dispatching to Argo Events.
func IsHorizonInjectedWorkflowParameter(name string) bool {
	return name == SubmitParamSampleSoftEnabled
}

var allowedSubmittedFrom = map[string]bool{
	SubmittedFromAPI:             true,
	SubmittedFromDeveloperPortal: true,
	SubmittedFromHorizonCLI:      true,
}

var horizonWorkflowListLabelSelector string

func init() {
	req, err := labels.NewRequirement(LabelSubmittedFrom, selection.Exists, nil)
	if err != nil {
		panic("workflow.HorizonWorkflowListLabelSelector: " + err.Error())
	}
	horizonWorkflowListLabelSelector = labels.NewSelector().Add(*req).String()
}

// HorizonWorkflowListLabelSelector limits workflow List to runs that carry horizon-sdv.io/submitted-from
// (Horizon-dispatched), including built-in and custom integration ids.
func HorizonWorkflowListLabelSelector() string {
	return horizonWorkflowListLabelSelector
}

// SubmittedFromLabelValue returns the workflow label value (may be empty).
func SubmittedFromLabelValue(u *unstructured.Unstructured) string {
	if u == nil {
		return ""
	}
	labels := u.GetLabels()
	if labels == nil {
		return ""
	}
	return strings.TrimSpace(labels[LabelSubmittedFrom])
}

// ModuleLabelValue returns horizon-sdv.io/module label value (may be empty).
func ModuleLabelValue(u *unstructured.Unstructured) string {
	if u == nil {
		return ""
	}
	labels := u.GetLabels()
	if labels == nil {
		return ""
	}
	return strings.TrimSpace(labels[LabelModule])
}

// IsHorizonClientVisible reports whether the workflow may be listed or read via Horizon API / portal / CLI.
func IsHorizonClientVisible(u *unstructured.Unstructured) bool {
	v := SubmittedFromLabelValue(u)
	if allowedSubmittedFrom[v] {
		return true
	}
	return v != "" && len(validation.IsValidLabelValue(v)) == 0
}

// ParseSubmittedFromHeader reads X-Horizon-Submitted-From; missing header means generic REST (api).
// If the header is "custom" (case-insensitive), the integration id is taken from HeaderSubmittedFromDetail.
func ParseSubmittedFromHeader(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Horizon-Submitted-From"))
	if raw == "" {
		return SubmittedFromAPI, nil
	}
	if strings.EqualFold(raw, SubmittedFromCustomToken) {
		detail := strings.TrimSpace(r.Header.Get(HeaderSubmittedFromDetail))
		if detail == "" {
			return "", fmt.Errorf("%s is %q: set non-empty %s to the integration id (Kubernetes label rules)",
				"X-Horizon-Submitted-From", SubmittedFromCustomToken, HeaderSubmittedFromDetail)
		}
		return ParseSubmittedFromValue(detail)
	}
	return ParseSubmittedFromValue(raw)
}

// ParseSubmittedByHeader reads X-Horizon-Submitted-By; a missing header returns an empty string.
// The value is client-supplied, so callers must only use it for a caller allowed to impersonate.
func ParseSubmittedByHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(HeaderSubmittedBy))
}

// ParseSubmittedFromValue validates and normalizes submitted-from values to canonical built-in ids,
// or returns any other value that is a valid Kubernetes label value (custom integration id).
func ParseSubmittedFromValue(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("submitted-from value cannot be empty")
	}
	k := strings.ToLower(raw)
	switch k {
	case SubmittedFromAPI, "rest", "rest-api":
		return SubmittedFromAPI, nil
	case SubmittedFromDeveloperPortal, "developer_portal", "portal":
		return SubmittedFromDeveloperPortal, nil
	case SubmittedFromHorizonCLI, "horizon_cli", "cli":
		return SubmittedFromHorizonCLI, nil
	default:
		if errs := validation.IsValidLabelValue(raw); len(errs) == 0 {
			return raw, nil
		}
		return "", fmt.Errorf(
			"invalid submitted-from value %q: use built-in %q, %q, or %q, header %q with %q, or any valid Kubernetes label value (max 63 chars; see label rules)",
			raw, SubmittedFromAPI, SubmittedFromDeveloperPortal, SubmittedFromHorizonCLI, SubmittedFromCustomToken, HeaderSubmittedFromDetail)
	}
}
