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
package catalog

import (
	"testing"

	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
)

func TestExtractParametersOmitsHorizonInjected(t *testing.T) {
	t.Parallel()
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"arguments": map[string]interface{}{
				"parameters": []interface{}{
					map[string]interface{}{"name": "sampleEnv", "value": ""},
					map[string]interface{}{"name": workflow.SubmitParamSampleSoftEnabled, "value": "false"},
				},
			},
		},
	}
	params, err := extractParameters(obj)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 || params[0].Name != "sampleEnv" {
		t.Fatalf("params: %+v", params)
	}
}
