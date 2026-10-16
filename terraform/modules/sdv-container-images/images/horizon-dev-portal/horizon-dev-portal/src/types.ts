// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
export interface AppConfig {
  baseUrl?: string;
  /**
   * URL path prefix for this SPA (leading slash, no trailing slash), e.g. `/developer-portal`.
   * Empty or omitted when served at site root. Must match Gateway path prefix and Keycloak redirect URIs.
   */
  publicPath?: string;
  keycloakUrl?: string;
  keycloakClientId?: string;
  /** Keycloak OAuth client id used for client-credentials calls to Horizon API via this app's proxy (not the SPA login client). */
  horizonApiOAuthClientId?: string;
  /** Optional default from config.js when localStorage has no `theme` key */
  theme?: 'light' | 'dark';
}

/** Module child application link from Module Manager catalog (GET /modules). */
export interface ModuleApplication {
  id: string;
  title?: string;
  /** Site-relative path (/) or absolute URL. */
  url: string;
}

export interface ModuleResponse {
  id: string;
  name: string;
  enabled: boolean;
  /** Repo-relative path under the catalog module path; used for Helm packaging (portal/overview.html), not a runtime Git path. */
  overviewPath?: string;
  /** In-cluster Service name; with overviewServiceNamespace, Module Manager serves GET /modules/{name}/overview via HTTP inside the cluster (port 80, path /). */
  overviewService?: string;
  overviewServiceNamespace?: string;
  hardDependencies?: string[];
  softDependencies?: string[];
  hardDependents?: string[];
  softDependents?: string[];
  applicationName?: string;
  applicationNamespace?: string;
  /** Catalog-defined links (e.g. public Gateway paths for child apps). */
  applications?: ModuleApplication[];
  /** Effective Git ref (branch, tag, commit) for the module Argo CD Application. */
  targetRevision?: string;
  /** Module Manager cluster default ref (--target-revision). */
  clusterTargetRevision?: string;
  /** True when the module has an explicit Git ref pin; false when following the platform default. */
  pinned?: boolean;
}

/** GET/PUT /settings/workflows-visibility (Module Manager). */
export interface WorkflowsVisibilityDTO {
  /** Omitted after GET means no filter. Empty array means hide all. */
  allowedSubmittedFrom?: string[] | null;
}

export interface StatusResponse {
  syncStatus?: string;
  healthStatus?: string;
  operationPhase?: string;
  desiredRevision?: string;
  syncRevision?: string;
  /** RFC3339; set when the parent or any module-managed Application has metadata.deletionTimestamp */
  applicationDeletionTimestamp?: string;
  /** Module Manager: when module is disabled, parent+child Argo CD Applications still present */
  remainingManagedApplications?: number;
  /** Module Manager: when module is enabled, labeled parent+child Application count (if list succeeded) */
  managedApplicationCount?: number;
  /** 2 for workloads-android / workloads-common (parent + prefixed child) */
  expectedManagedApplicationCount?: number;
  /** False when the labeled child Application is not on the cluster */
  managedChildApplicationPresent?: boolean;
  /** True during prefixed-module disable (parent skip-reconcile) */
  parentSkipReconcile?: boolean;
}

export interface CatalogEntry {
  module: string;
  templateName: string;
  namespace: string;
  parameters: { name: string; default?: string; description?: string }[];
}

export interface CatalogResponse {
  entries: CatalogEntry[];
}

/** GET /api/cli/v1/manifest */
export interface CliArtifact {
  os: string;
  arch: string;
  filename: string;
  sha256: string;
  sizeBytes: number;
  downloadPath: string;
}

export interface CliManifest {
  name: string;
  version: string;
  buildDate: string;
  artifacts: CliArtifact[];
}

export interface WorkflowSummary {
  name: string;
  namespace: string;
  phase: string;
  startedAt?: string;
  finishedAt?: string;
  workflowTemplate?: string;
  /** Horizon portal user (annotation) or Argo creator label when present */
  startedBy?: string;
  /** Label horizon-sdv.io/submitted-from: api | developer-portal | horizon-cli */
  submittedFrom?: string;
  message?: string;
  archivedLogs?: {
    combined?: { gcsUri?: string };
    steps?: {
      nodeId?: string;
      displayName?: string;
      templateName?: string;
      gcsUri?: string;
      /** Matches Horizon API StepLogLink.artifactName (e.g. main-logs) for downloadArtifact. */
      artifactName?: string;
    }[];
  };
}

export interface WorkflowListResponse {
  items: WorkflowSummary[];
  continue?: string;
}

export interface OutputArtifact {
  nodeId?: string;
  name: string;
  /** Basename of the GCS object when it differs from the workflow artifact name (e.g. smoke-result.tgz). */
  fileName?: string;
  displayName?: string;
  module?: string;
  templateName?: string;
  workflowTemplate?: string;
  gcsUri?: string;
}

export interface DependentWorkflowTemplate {
  template: string;
  module?: string;
}

export interface WorkflowDetail {
  name: string;
  namespace: string;
  module?: string;
  phase: string;
  workflowTemplate?: string;
  startedAt?: string;
  finishedAt?: string;
  startedBy?: string;
  submittedFrom?: string;
  message?: string;
  uid?: string;
  dependentWorkflowTemplates?: DependentWorkflowTemplate[];
  nodes?: {
    id: string;
    displayName?: string;
    module?: string;
    templateName?: string;
    workflowTemplate?: string;
    type?: string;
    phase?: string;
    podName?: string;
    startedAt?: string;
  }[];
  outputArtifacts?: OutputArtifact[];
  archivedLogs?: WorkflowSummary['archivedLogs'];
}

export interface WorkstationClusterItem {
  name: string;
  region: string;
  controlPlaneIp?: string;
  subnetwork?: string;
  createTime?: string;
}

export interface WorkstationConfigItem {
  name: string;
  region: string;
  cluster: string;
  machineType: string;
  gpuType?: string;
  gpuCount?: number;
  bootDiskSizeGb?: number;
  persistentDiskSizeGb?: number;
  persistentDiskType?: string;
  sourceSnapshot?: string;
  containerImage: string;
  idleTimeout: string;
  runningTimeout: string;
  replicaZones?: string[];
  degraded?: boolean;
  createTime?: string;
}

export interface WorkstationInstanceItem {
  name: string;
  displayName?: string;
  region: string;
  cluster: string;
  config: string;
  state: string;
  host?: string;
  launchUrl?: string;
  runtimeHostVm?: string;
  runtimeZone?: string;
  machineType?: string;
  gpuType?: string;
  containerImage?: string;
  sourceSnapshot?: string;
  persistentDiskSizeGb?: number;
  createTime?: string;
  startTime?: string;
  updateTime?: string;
}

export interface ComputeInstanceItem {
  name: string;
  role: string;
  status: string;
  zone: string;
  region: string;
  machineType: string;
  architecture: string;
  diskSizeGb?: number;
  internalIp?: string;
  gpuType?: string;
  gpuCount?: number;
  creationTimestamp?: string;
  labels?: Record<string, string>;
}

export interface DiskSnapshotItem {
  name: string;
  status: string;
  diskSizeGb: number;
  storageBytes: number;
  creationTimestamp: string;
  storageLocations?: string[];
  labels?: Record<string, string>;
  isLatest: boolean;
}

export interface InstanceTemplateItem {
  name: string;
  category: string;
  machineType: string;
  diskSizeGb?: number;
  creationTimestamp?: string;
}

export interface ContainerImageSummary {
  packageName: string;
  fullUri: string;
  tags: string[];
  digest: string;
  sizeBytes: number;
  uploadTime?: string;
  updateTime?: string;
}

export interface BuildArtifactBundle {
  id: string;
  bucket: string;
  prefix: string;
  category: string;
  pipelineName: string;
  buildNumber: string;
  targetArch?: string;
  fileCount: number;
  totalSizeBytes: number;
  updatedAt: string;
  keyFiles: string[];
  hasGeminiAssist: boolean;
  gcsUri: string;
  consoleUrl: string;
}

export interface ArchivedWorkflowItem {
  workflowName: string;
  workflowTemplate: string;
  module: string;
  startedAt: string;
  finishedAt: string;
  artifactCount: number;
  totalSizeBytes: number;
  gcsUri: string;
  consoleUrl: string;
}

export interface JenkinsBuildItem {
  id: string;
  jobPath: string;
  category: string;
  buildNumber: number;
  result: string;
  timestampMs: number;
  startedAt: string;
  durationMs: number;
  node?: string;
  webPath: string;
  parameters?: Record<string, string>;
}

export interface ABFSAccelerationSummary {
  cleanBaselineTotalSec: number;
  cleanBaselineArm64TotalSec: number;
  cleanBaselineCompileSec: number;
  cleanBaselineSourceDiskGb: number;
  cleanBaselineTotalDiskGb: number;
  abfsColdSeedTotalSec: number;
  abfsColdSeedCompileSec: number;
  abfsColdSeedNinjaEvalSec: number;
  abfsColdSeedSourceDiskGb: number;
  abfsColdSeedTotalDiskGb: number;
  abfsBestWarmTotalSec: number;
  abfsAvgWarmTotalSec: number;
  abfsBestCompileSec: number;
  abfsBestNinjaEvalSec: number;
  abfsWarmCacheHitRatio: number;
  abfsWarmTotalActions: number;
  abfsWarmDiskFootprintGb: number;
  endToEndSpeedupPeak: number;
  endToEndSpeedupAvg: number;
  compileSpeedupPeak: number;
  ninjaEvalSpeedupPeak: number;
  timeSavedPerBuildSec: number;
  timeSavedPct: number;
  sourceDiskReductionPct: number;
  totalDiskReductionPct: number;
  abfsServerName: string;
  abfsServerStatus: string;
  abfsServerMachineType: string;
  abfsServerZone: string;
  abfsSpanNodes?: string[];
  abfsInfraJobsTotal: number;
  abfsInfraJobsSucceeded: number;
}

export interface ABFSBenchmarkRun {
  id: string;
  pipeline: string;
  buildNumber: number;
  mode: string;
  modeLabel: string;
  isAbfs: boolean;
  result: string;
  startedAt: string;
  lunchTarget: string;
  revision: string;
  arch: string;
  totalDurationSec: number;
  compileDurationSec?: number;
  ninjaEvalSec?: number;
  initAndSyncEstimateSec?: number;
  totalNinjaActions?: number;
  cacheHits?: number;
  cacheMisses?: number;
  cacheHitRatio?: number;
  hasRemoteCacheStats: boolean;
  buildInfoHash?: string;
  diskInitialUsage?: string;
  diskAfterSourceInit?: string;
  diskAfterSourceInitGb?: number;
  diskFinalUsage?: string;
  diskFinalUsageGb?: number;
  diskCapacity?: string;
  kernelRevision?: string;
  abfsCachedBuild: boolean;
  abfsCacheDir?: string;
  manifestServer?: string;
  speedupVsBaseline?: number;
  compileSpeedupVsBaseline?: number;
  timeSavedSec?: number;
  jenkinsWebPath?: string;
  gcsUri?: string;
  gcsConsoleUrl?: string;
}

export interface ControlRoomABFS {
  summary: ABFSAccelerationSummary;
  runs: ABFSBenchmarkRun[];
}

export interface ControlRoomResponse {
  generatedAt: string;
  projectId: string;
  primaryRegion: string;
  workstations: {
    clusters: WorkstationClusterItem[];
    configs: WorkstationConfigItem[];
    workstations: WorkstationInstanceItem[];
  };
  compute: {
    virtualDevices: ComputeInstanceItem[];
    workstationVMs: ComputeInstanceItem[];
    buildInfraVMs: ComputeInstanceItem[];
    clusterNodes: ComputeInstanceItem[];
    snapshots: DiskSnapshotItem[];
    instanceTemplates: InstanceTemplateItem[];
  };
  artifactImages: ContainerImageSummary[];
  buildBundles: BuildArtifactBundle[];
  archivedWorkflows: ArchivedWorkflowItem[];
  jenkinsBuilds: JenkinsBuildItem[];
  abfs: ControlRoomABFS;
  warnings?: string[];
}
