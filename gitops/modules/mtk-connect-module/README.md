<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# mtk-connect module

Helm chart: `gitops/modules/mtk-connect-module`  
Argo CD parent Application: **`mod-mtk-connect`** (enabled from the Developer Portal, catalog name `mtk-connect`).

Packages MTK Connect as a Horizon module so it can be enabled and disabled from Administration → Modules.

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **STATUS** | proposed |
| **OWNER** | Accenture |
| **CONTACT** | module: [horizon-sdv@accenture.com](mailto:horizon-sdv@accenture.com) |
| **CHART VERSION** | 0.1.0 |
| **CONTAINS_COMMERCIAL** | no |

## Catalog

| Setting | Value |
|---------|--------|
| Module name | `mtk-connect` |
| Hard dependencies | none |
| `autoDisableWhenUnused` | `false` (owns MongoDB data; stays enabled until you disable it) |
| Overview service | `mod-mtk-connect-overview` in namespace `<prefix>mtk-connect-overview` |

The catalog entry is in `gitops/apps/module-manager/templates/module-catalog.yaml`.
