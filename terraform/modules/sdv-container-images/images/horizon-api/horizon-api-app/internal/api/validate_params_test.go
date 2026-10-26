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
package api

import (
	"strings"
	"testing"

	"github.com/acn-horizon-sdv/horizon-api/internal/catalog"
	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
)

func TestValidateParamsRejectsInjectedSampleSoftEnabled(t *testing.T) {
	t.Parallel()
	ent := catalog.Entry{
		Parameters: []catalog.Parameter{
			{Name: "sampleEnv", Default: ""},
		},
	}
	err := validateParams(ent, map[string]string{
		"sampleEnv":                           "x",
		workflow.SubmitParamSampleSoftEnabled: "true",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown parameter error, got %v", err)
	}
}
