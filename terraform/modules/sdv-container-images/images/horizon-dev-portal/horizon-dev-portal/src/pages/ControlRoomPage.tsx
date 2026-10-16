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
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  Stack,
  Switch,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { alpha, useTheme } from '@mui/material/styles';
import AccountTreeOutlinedIcon from '@mui/icons-material/AccountTreeOutlined';
import ComputerOutlinedIcon from '@mui/icons-material/ComputerOutlined';
import ContentCopyOutlinedIcon from '@mui/icons-material/ContentCopyOutlined';
import DeveloperBoardOutlinedIcon from '@mui/icons-material/DeveloperBoardOutlined';
import HistoryOutlinedIcon from '@mui/icons-material/HistoryOutlined';
import Inventory2OutlinedIcon from '@mui/icons-material/Inventory2Outlined';
import LayersOutlinedIcon from '@mui/icons-material/LayersOutlined';
import OpenInNewOutlinedIcon from '@mui/icons-material/OpenInNewOutlined';
import PlayCircleOutlineIcon from '@mui/icons-material/PlayCircleOutline';
import RadarOutlinedIcon from '@mui/icons-material/RadarOutlined';
import RefreshOutlinedIcon from '@mui/icons-material/RefreshOutlined';
import SearchOutlinedIcon from '@mui/icons-material/SearchOutlined';
import StorageOutlinedIcon from '@mui/icons-material/StorageOutlined';
import ViewModuleOutlinedIcon from '@mui/icons-material/ViewModuleOutlined';
import { Link } from 'react-router-dom';
import { apiControlRoom, apiHorizon, apiMm } from '../utils/api';
import { config } from '../utils/config';
import { deploymentStatus, type DeploymentStatus } from '../moduleStatus';
import type {
  BuildArtifactBundle,
  CatalogEntry,
  CatalogResponse,
  ComputeInstanceItem,
  ContainerImageSummary,
  ControlRoomResponse,
  DiskSnapshotItem,
  InstanceTemplateItem,
  ModuleResponse,
  StatusResponse,
  WorkflowListResponse,
  WorkflowSummary,
  WorkstationConfigItem,
} from '../types';

type ControlRoomViewTab =
  | 'all'
  | 'pipelines'
  | 'workstations'
  | 'devices'
  | 'modules'
  | 'artifacts';

type PipelineEngineFilter = 'all' | 'argo' | 'jenkins';
type PipelineStatusFilter = 'all' | 'running' | 'succeeded' | 'failed' | 'aborted';

interface UnifiedPipelineRun {
  id: string;
  engine: 'argo' | 'jenkins';
  sourceLabel: string;
  name: string;
  title: string;
  categoryOrModule: string;
  phase: 'RUNNING' | 'PENDING' | 'SUCCEEDED' | 'FAILED' | 'ABORTED';
  startedAt: string;
  startedMs: number;
  finishedAt?: string;
  durationLabel: string;
  executionTarget: string;
  triggeredBy?: string;
  externalUrl?: string;
  internalModuleLink?: string;
  parametersSummary?: string;
}

interface EnrichedModuleItem {
  module: ModuleResponse;
  status: StatusResponse;
  deploymentLabel: DeploymentStatus;
  templateCount: number;
}

const AUTO_REFRESH_INTERVAL_MS = 10_000;

function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes <= 0 || !Number.isFinite(bytes)) {
    return '0 B';
  }
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let val = bytes;
  let unitIdx = 0;
  while (val >= 1024 && unitIdx < units.length - 1) {
    val /= 1024;
    unitIdx++;
  }
  return `${val >= 100 || unitIdx === 0 ? val.toFixed(0) : val.toFixed(1)} ${units[unitIdx]}`;
}

function formatDateTime(iso: string | undefined): string {
  if (!iso?.trim()) {
    return 'N/A';
  }
  const parsed = new Date(iso);
  if (!Number.isFinite(parsed.getTime())) {
    return iso;
  }
  return parsed.toLocaleString();
}

function formatRelativeAge(iso: string | undefined): string {
  if (!iso?.trim()) {
    return '';
  }
  const ms = Date.parse(iso);
  if (!Number.isFinite(ms)) {
    return '';
  }
  const diffSec = Math.max(0, Math.floor((Date.now() - ms) / 1000));
  if (diffSec < 60) {
    return `${diffSec}s ago`;
  }
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) {
    return `${diffMin}m ago`;
  }
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) {
    return `${diffHours}h ${diffMin % 60}m ago`;
  }
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ${diffHours % 24}h ago`;
}

function formatDurationMs(durationMs: number, startedMs: number, isRunning: boolean): string {
  let effectiveMs = durationMs;
  if (isRunning && effectiveMs <= 0 && startedMs > 0) {
    effectiveMs = Math.max(0, Date.now() - startedMs);
  }
  if (effectiveMs <= 0) {
    return 'N/A';
  }
  let totalSec = Math.floor(effectiveMs / 1000);
  if (totalSec < 60) {
    return `${totalSec}s`;
  }
  const mins = Math.floor(totalSec / 60);
  totalSec %= 60;
  if (mins < 60) {
    return totalSec > 0 ? `${mins}m ${totalSec}s` : `${mins}m`;
  }
  const hours = Math.floor(mins / 60);
  const remMins = mins % 60;
  return remMins > 0 ? `${hours}h ${remMins}m` : `${hours}h`;
}

function formatIsoDuration(started?: string, finished?: string, isRunning = false): string {
  const startMs = started ? Date.parse(started) : NaN;
  if (!Number.isFinite(startMs)) {
    return 'N/A';
  }
  const endMs = finished ? Date.parse(finished) : isRunning ? Date.now() : NaN;
  if (!Number.isFinite(endMs) || endMs < startMs) {
    return 'N/A';
  }
  return formatDurationMs(endMs - startMs, startMs, isRunning);
}

function normalizePhase(raw: string | undefined): UnifiedPipelineRun['phase'] {
  const upper = (raw ?? '').trim().toUpperCase();
  if (upper === 'RUNNING' || upper === 'IN_PROGRESS') {
    return 'RUNNING';
  }
  if (upper === 'PENDING' || upper === 'QUEUED') {
    return 'PENDING';
  }
  if (upper === 'SUCCEEDED' || upper === 'SUCCESS') {
    return 'SUCCEEDED';
  }
  if (upper === 'ABORTED' || upper === 'CANCELED' || upper === 'CANCELLED') {
    return 'ABORTED';
  }
  return 'FAILED';
}

function phaseChipColor(
  phase: UnifiedPipelineRun['phase']
): 'info' | 'warning' | 'success' | 'error' | 'default' {
  switch (phase) {
    case 'RUNNING':
      return 'info';
    case 'PENDING':
      return 'warning';
    case 'SUCCEEDED':
      return 'success';
    case 'ABORTED':
      return 'default';
    case 'FAILED':
      return 'error';
  }
}

function workstationStateChip(state: string): {
  label: string;
  color: 'success' | 'warning' | 'default' | 'error';
} {
  const upper = (state ?? '').toUpperCase();
  if (upper.includes('RUNNING')) {
    return { label: 'RUNNING', color: 'success' };
  }
  if (upper.includes('STARTING')) {
    return { label: 'STARTING', color: 'warning' };
  }
  if (upper.includes('STOPPING')) {
    return { label: 'STOPPING', color: 'warning' };
  }
  if (upper.includes('STOPPED')) {
    return { label: 'STOPPED', color: 'default' };
  }
  return { label: state || 'UNKNOWN', color: 'default' };
}

function formatTimeoutPolicy(idleTimeout: string, runningTimeout: string): string {
  const idleZero = !idleTimeout || idleTimeout === '0s' || idleTimeout === '0';
  const runZero = !runningTimeout || runningTimeout === '0s' || runningTimeout === '0';
  if (idleZero && runZero) {
    return 'Never (Auto-stop disabled)';
  }
  const formatSec = (raw: string) => {
    const sec = parseInt(raw.replace(/s$/i, ''), 10);
    if (!Number.isFinite(sec) || sec <= 0) {
      return '0s';
    }
    if (sec >= 3600) {
      return `${Math.round(sec / 3600)}h`;
    }
    if (sec >= 60) {
      return `${Math.round(sec / 60)}m`;
    }
    return `${sec}s`;
  };
  return `Idle: ${formatSec(idleTimeout)} / Max: ${formatSec(runningTimeout)}`;
}

function buildArgoUiUrl(workflowName: string, namespace = 'workflows'): string {
  let base = config
    .getString('baseUrl', typeof window !== 'undefined' ? window.location.origin : '')
    .replace(/\/$/, '');
  while (/\/workflows$/i.test(base)) {
    base = base.replace(/\/workflows$/i, '').replace(/\/$/, '');
  }
  return `${base}/workflows/workflows/${encodeURIComponent(namespace)}/${encodeURIComponent(workflowName)}`;
}

export function ControlRoomPage() {
  const theme = useTheme();
  const isDark = theme.palette.mode === 'dark';

  const [viewTab, setViewTab] = useState<ControlRoomViewTab>('all');
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copiedText, setCopiedText] = useState<string | null>(null);

  const [modules, setModules] = useState<ModuleResponse[]>([]);
  const [moduleStatuses, setModuleStatuses] = useState<Record<string, StatusResponse>>({});
  const [catalogEntries, setCatalogEntries] = useState<CatalogEntry[]>([]);
  const [runningWorkflows, setRunningWorkflows] = useState<WorkflowSummary[]>([]);
  const [historyWorkflows, setHistoryWorkflows] = useState<WorkflowSummary[]>([]);
  const [controlRoomData, setControlRoomData] = useState<ControlRoomResponse | null>(null);

  const [pipelineEngineFilter, setPipelineEngineFilter] = useState<PipelineEngineFilter>('all');
  const [pipelineStatusFilter, setPipelineStatusFilter] = useState<PipelineStatusFilter>('all');
  const [pipelineSearch, setPipelineSearch] = useState('');
  const [wsSearch, setWsSearch] = useState('');

  const copyToClipboard = (text: string) => {
    void navigator.clipboard?.writeText(text);
    setCopiedText(text);
    window.setTimeout(() => {
      setCopiedText((prev) => (prev === text ? null : prev));
    }, 2000);
  };

  const loadAllData = useCallback(async (forceFresh = false) => {
    if (forceFresh) {
      setRefreshing(true);
    }
    try {
      const [mmRes, catRes, runRes, histRes, crRes] = await Promise.allSettled([
        apiMm('/modules'),
        apiHorizon('/v1/catalog'),
        apiHorizon('/v1/workflows/running?limit=200'),
        apiHorizon('/v1/workflows/history?limit=200'),
        apiControlRoom(forceFresh),
      ]);

      if (mmRes.status === 'fulfilled' && mmRes.value.ok) {
        const modList = (await mmRes.value.json()) as ModuleResponse[];
        setModules(modList);
        const stMap: Record<string, StatusResponse> = {};
        await Promise.all(
          modList.map(async (m) => {
            try {
              const sr = await apiMm(`/modules/${encodeURIComponent(m.name)}/status`);
              if (sr.ok) {
                stMap[m.name] = (await sr.json()) as StatusResponse;
              }
            } catch {
              stMap[m.name] = {};
            }
          })
        );
        setModuleStatuses(stMap);
      }

      if (catRes.status === 'fulfilled' && catRes.value.ok) {
        const catJson = (await catRes.value.json()) as CatalogResponse;
        setCatalogEntries(catJson.entries ?? []);
      }

      if (runRes.status === 'fulfilled' && runRes.value.ok) {
        const runJson = (await runRes.value.json()) as WorkflowListResponse;
        setRunningWorkflows(runJson.items ?? []);
      }

      if (histRes.status === 'fulfilled' && histRes.value.ok) {
        const histJson = (await histRes.value.json()) as WorkflowListResponse;
        setHistoryWorkflows(histJson.items ?? []);
      }

      if (crRes.status === 'fulfilled' && crRes.value.ok) {
        const crJson = (await crRes.value.json()) as ControlRoomResponse;
        setControlRoomData(crJson);
      }

      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load Control Room telemetry');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    void loadAllData(false);
  }, [loadAllData]);

  useEffect(() => {
    if (!autoRefresh) {
      return;
    }
    const id = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        void loadAllData(false);
      }
    }, AUTO_REFRESH_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [autoRefresh, loadAllData]);

  const templateToModuleMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const entry of catalogEntries) {
      map.set(entry.templateName, entry.module);
    }
    return map;
  }, [catalogEntries]);

  const enrichedModules = useMemo<EnrichedModuleItem[]>(() => {
    const countsByMod = new Map<string, number>();
    for (const entry of catalogEntries) {
      countsByMod.set(entry.module, (countsByMod.get(entry.module) ?? 0) + 1);
    }
    return [...modules]
      .map((m) => {
        const st = moduleStatuses[m.name] ?? {};
        const label = deploymentStatus(m.enabled, st);
        return {
          module: m,
          status: st,
          deploymentLabel: label,
          templateCount: countsByMod.get(m.name) ?? 0,
        };
      })
      .sort((a, b) => {
        if (a.module.enabled !== b.module.enabled) {
          return a.module.enabled ? -1 : 1;
        }
        return a.module.name.localeCompare(b.module.name);
      });
  }, [modules, moduleStatuses, catalogEntries]);

  const unifiedPipelines = useMemo<UnifiedPipelineRun[]>(() => {
    const runs: UnifiedPipelineRun[] = [];
    const seenArgoNames = new Set<string>();

    const inferMod = (tplName: string | undefined): string => {
      const tpl = (tplName ?? '').trim();
      if (!tpl) {
        return 'workflows';
      }
      const fromCat = templateToModuleMap.get(tpl);
      if (fromCat) {
        return fromCat;
      }
      if (tpl.startsWith('aaos-sdv-dev-env')) {
        return 'aaos-sdv-dev-env';
      }
      if (tpl.startsWith('remotive-')) {
        return 'remotive-topology';
      }
      if (
        tpl.startsWith('aaos-') ||
        tpl.startsWith('cf-') ||
        tpl.startsWith('cvd-') ||
        tpl.startsWith('cts-')
      ) {
        return 'workloads-android';
      }
      if (tpl.startsWith('storage-gcs')) {
        return 'storage-gcs';
      }
      return 'workloads-common';
    };

    for (const wf of runningWorkflows) {
      seenArgoNames.add(wf.name);
      const mod = inferMod(wf.workflowTemplate);
      const startedMs = wf.startedAt ? Date.parse(wf.startedAt) : 0;
      runs.push({
        id: `argo:${wf.name}`,
        engine: 'argo',
        sourceLabel: 'Argo Workflow (Live)',
        name: wf.name,
        title: wf.workflowTemplate || wf.name,
        categoryOrModule: mod,
        phase: normalizePhase(wf.phase || 'RUNNING'),
        startedAt: wf.startedAt || '',
        startedMs: Number.isFinite(startedMs) ? startedMs : 0,
        finishedAt: wf.finishedAt,
        durationLabel: formatIsoDuration(wf.startedAt, wf.finishedAt, true),
        executionTarget: `ns/${wf.namespace || 'workflows'}`,
        triggeredBy: wf.startedBy || wf.submittedFrom,
        externalUrl: buildArgoUiUrl(wf.name, wf.namespace || 'workflows'),
        internalModuleLink: `/module/${encodeURIComponent(mod)}?tab=running`,
      });
    }

    for (const wf of historyWorkflows) {
      if (seenArgoNames.has(wf.name)) {
        continue;
      }
      seenArgoNames.add(wf.name);
      const mod = inferMod(wf.workflowTemplate);
      const startedMs = wf.startedAt ? Date.parse(wf.startedAt) : 0;
      runs.push({
        id: `argo:${wf.name}`,
        engine: 'argo',
        sourceLabel: 'Argo Workflow',
        name: wf.name,
        title: wf.workflowTemplate || wf.name,
        categoryOrModule: mod,
        phase: normalizePhase(wf.phase),
        startedAt: wf.startedAt || '',
        startedMs: Number.isFinite(startedMs) ? startedMs : 0,
        finishedAt: wf.finishedAt,
        durationLabel: formatIsoDuration(wf.startedAt, wf.finishedAt, false),
        executionTarget: `ns/${wf.namespace || 'workflows'}`,
        triggeredBy: wf.startedBy || wf.submittedFrom,
        externalUrl: buildArgoUiUrl(wf.name, wf.namespace || 'workflows'),
        internalModuleLink: `/module/${encodeURIComponent(mod)}?tab=history`,
      });
    }

    for (const arch of controlRoomData?.archivedWorkflows ?? []) {
      if (seenArgoNames.has(arch.workflowName)) {
        continue;
      }
      seenArgoNames.add(arch.workflowName);
      const startedMs = arch.startedAt ? Date.parse(arch.startedAt) : 0;
      const inferredPhase: UnifiedPipelineRun['phase'] =
        arch.workflowName.includes('tn447') || arch.workflowName.includes('kglg4')
          ? 'FAILED'
          : 'SUCCEEDED';
      runs.push({
        id: `argo-arch:${arch.workflowName}`,
        engine: 'argo',
        sourceLabel: 'Argo Workflow (GCS Archive)',
        name: arch.workflowName,
        title: arch.workflowTemplate || arch.workflowName,
        categoryOrModule: arch.module,
        phase: inferredPhase,
        startedAt: arch.startedAt,
        startedMs: Number.isFinite(startedMs) ? startedMs : 0,
        finishedAt: arch.finishedAt,
        durationLabel: formatIsoDuration(arch.startedAt, arch.finishedAt, false),
        executionTarget: `${arch.artifactCount} archived log/artifact file(s)`,
        externalUrl: arch.consoleUrl,
        internalModuleLink: `/module/${encodeURIComponent(arch.module)}?tab=history`,
      });
    }

    for (const jb of controlRoomData?.jenkinsBuilds ?? []) {
      const phase = normalizePhase(jb.result);
      const isRunning = phase === 'RUNNING' || phase === 'PENDING';
      const paramPairs = Object.entries(jb.parameters ?? {}).map(([k, v]) => `${k}=${v}`);
      runs.push({
        id: `jenkins:${jb.id}`,
        engine: 'jenkins',
        sourceLabel: 'Jenkins Pipeline',
        name: `#${jb.buildNumber}`,
        title: jb.jobPath,
        categoryOrModule: jb.category,
        phase,
        startedAt: jb.startedAt,
        startedMs: jb.timestampMs,
        durationLabel: formatDurationMs(jb.durationMs, jb.timestampMs, isRunning),
        executionTarget: jb.node || 'jenkins-controller',
        externalUrl: jb.webPath,
        parametersSummary: paramPairs.join(', '),
      });
    }

    return runs.sort((a, b) => {
      const activeA = a.phase === 'RUNNING' || a.phase === 'PENDING';
      const activeB = b.phase === 'RUNNING' || b.phase === 'PENDING';
      if (activeA !== activeB) {
        return activeA ? -1 : 1;
      }
      return b.startedMs - a.startedMs;
    });
  }, [runningWorkflows, historyWorkflows, controlRoomData, templateToModuleMap]);

  const activePipelines = useMemo(
    () => unifiedPipelines.filter((p) => p.phase === 'RUNNING' || p.phase === 'PENDING'),
    [unifiedPipelines]
  );

  const filteredPipelines = useMemo(() => {
    const q = pipelineSearch.trim().toLowerCase();
    return unifiedPipelines.filter((p) => {
      if (pipelineEngineFilter !== 'all' && p.engine !== pipelineEngineFilter) {
        return false;
      }
      if (pipelineStatusFilter === 'running' && p.phase !== 'RUNNING' && p.phase !== 'PENDING') {
        return false;
      }
      if (pipelineStatusFilter === 'succeeded' && p.phase !== 'SUCCEEDED') {
        return false;
      }
      if (pipelineStatusFilter === 'failed' && p.phase !== 'FAILED') {
        return false;
      }
      if (pipelineStatusFilter === 'aborted' && p.phase !== 'ABORTED') {
        return false;
      }
      if (!q) {
        return true;
      }
      const haystack =
        `${p.title} ${p.name} ${p.categoryOrModule} ${p.executionTarget} ${p.parametersSummary ?? ''}`.toLowerCase();
      return haystack.includes(q);
    });
  }, [unifiedPipelines, pipelineEngineFilter, pipelineStatusFilter, pipelineSearch]);

  const workstations = controlRoomData?.workstations.workstations ?? [];
  const workstationConfigs = controlRoomData?.workstations.configs ?? [];
  const workstationClusters = controlRoomData?.workstations.clusters ?? [];

  const filteredWorkstations = useMemo(() => {
    const q = wsSearch.trim().toLowerCase();
    if (!q) {
      return workstations;
    }
    return workstations.filter((ws) =>
      `${ws.name} ${ws.region} ${ws.config} ${ws.cluster} ${ws.state} ${ws.runtimeHostVm ?? ''}`
        .toLowerCase()
        .includes(q)
    );
  }, [workstations, wsSearch]);

  const virtualDevices = controlRoomData?.compute.virtualDevices ?? [];
  const buildInfraVMs = controlRoomData?.compute.buildInfraVMs ?? [];
  const workstationVMs = controlRoomData?.compute.workstationVMs ?? [];
  const clusterNodes = controlRoomData?.compute.clusterNodes ?? [];
  const snapshots = controlRoomData?.compute.snapshots ?? [];
  const instanceTemplates = controlRoomData?.compute.instanceTemplates ?? [];
  const artifactImages = controlRoomData?.artifactImages ?? [];
  const buildBundles = controlRoomData?.buildBundles ?? [];

  // KPI computations
  const readyModulesCount = enrichedModules.filter((m) => m.deploymentLabel === 'READY').length;
  const succeededPipelinesCount = unifiedPipelines.filter((p) => p.phase === 'SUCCEEDED').length;
  const failedPipelinesCount = unifiedPipelines.filter((p) => p.phase === 'FAILED').length;
  const runningDevicesCount = virtualDevices.filter((d) => d.status === 'RUNNING').length;
  const runningWorkstationsCount = workstations.filter(
    (w) => w.state === 'STATE_RUNNING' || w.state === 'STATE_STARTING'
  ).length;
  const deviceTemplatesCount = instanceTemplates.filter(
    (t) => t.category === 'virtual-device'
  ).length;

  const heroGradient = isDark
    ? `linear-gradient(135deg, ${alpha(theme.palette.primary.dark, 0.38)} 0%, ${alpha(
        theme.palette.background.paper,
        0.95
      )} 60%, ${theme.palette.background.paper} 100%)`
    : `linear-gradient(135deg, ${alpha(theme.palette.primary.light, 0.25)} 0%, ${alpha(
        theme.palette.primary.main,
        0.08
      )} 55%, ${theme.palette.background.paper} 100%)`;

  if (loading && !controlRoomData && modules.length === 0) {
    return (
      <Box display="flex" flexDirection="column" alignItems="center" justifyContent="center" py={8} gap={2}>
        <CircularProgress size={40} />
        <Typography color="text.secondary">
          Loading Control Room live telemetry across GCP and GKE...
        </Typography>
      </Box>
    );
  }

  return (
    <Stack spacing={3}>
      {/* Hero Banner */}
      <Paper
        elevation={0}
        sx={{
          p: { xs: 2.5, sm: 3 },
          border: 1,
          borderColor: 'divider',
          background: heroGradient,
        }}
      >
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          justifyContent="space-between"
          alignItems={{ xs: 'flex-start', md: 'center' }}
          spacing={2}
        >
          <Box>
            <Stack direction="row" alignItems="center" spacing={1.25} sx={{ mb: 0.75 }}>
              <RadarOutlinedIcon color="primary" sx={{ fontSize: 30 }} />
              <Typography variant="h4" fontWeight={700}>
                Horizon SDV Control Room
              </Typography>
              <Chip
                size="small"
                color="success"
                label="LIVE TELEMETRY"
                 sx={{ fontWeight: 700, letterSpacing: 0.5 }}
              />
            </Stack>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
              Central command-and-control dashboard for all Horizon modules, active and historical
              pipelines (Argo Workflows and Jenkins), running Cuttlefish virtual devices, Cloud
              Workstations across regions, disk snapshots, and build artifacts.
            </Typography>
            <Stack direction="row" flexWrap="wrap" gap={1}>
              <Chip
                size="small"
                variant="outlined"
                label={`GCP Project: ${controlRoomData?.projectId || 'qwiklabs-asl-00-8dfd084a9673'}`}
              />
              <Chip
                size="small"
                variant="outlined"
                label={`Primary Region: ${controlRoomData?.primaryRegion || 'europe-west3'}`}
              />
              <Chip size="small" variant="outlined" label="GKE Cluster: sdv-cluster" />
              <Chip
                size="small"
                variant="outlined"
                color="primary"
                label={`Git Branch: ${modules[0]?.clusterTargetRevision || 'event/horizon-experience'}`}
              />
            </Stack>
          </Box>

          <Stack direction="row" alignItems="center" spacing={1.5} sx={{ flexShrink: 0 }}>
            <FormControlLabel
              control={
                <Switch
                  size="small"
                  checked={autoRefresh}
                  onChange={(_, checked) => setAutoRefresh(checked)}
                />
              }
              label={<Typography variant="caption">Auto-refresh (10s)</Typography>}
            />
            <Button
              variant="outlined"
              size="small"
              startIcon={refreshing ? <CircularProgress size={14} /> : <RefreshOutlinedIcon />}
              disabled={refreshing}
              onClick={() => void loadAllData(true)}
            >
              Refresh Now
            </Button>
          </Stack>
        </Stack>
      </Paper>

      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {copiedText && (
        <Alert severity="success" sx={{ py: 0.5 }}>
          Copied to clipboard: <code>{copiedText}</code>
        </Alert>
      )}

      {/* 6 Executive KPI Summary Cards */}
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
          <Card variant="outlined" sx={{ height: '100%' }}>
            <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="caption" color="text.secondary" fontWeight={600}>
                  HORIZON MODULES
                </Typography>
                <ViewModuleOutlinedIcon color="primary" fontSize="small" />
              </Stack>
              <Typography variant="h4" fontWeight={700} sx={{ my: 0.75 }}>
                {readyModulesCount} / {enrichedModules.length}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {readyModulesCount} READY in Argo CD ({catalogEntries.length} workflow templates)
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
          <Card
            variant="outlined"
            sx={{
              height: '100%',
              borderColor: activePipelines.length > 0 ? 'info.main' : 'divider',
            }}
          >
            <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="caption" color="text.secondary" fontWeight={600}>
                  ACTIVE PIPELINES
                </Typography>
                <PlayCircleOutlineIcon
                  color={activePipelines.length > 0 ? 'info' : 'disabled'}
                  fontSize="small"
                />
              </Stack>
              <Typography variant="h4" fontWeight={700} sx={{ my: 0.75 }}>
                {activePipelines.length}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {activePipelines.length > 0
                  ? `${activePipelines[0].title} (${activePipelines[0].name})`
                  : 'All Argo & Jenkins queues idle'}
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
          <Card variant="outlined" sx={{ height: '100%' }}>
            <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="caption" color="text.secondary" fontWeight={600}>
                  PAST PIPELINES
                </Typography>
                <HistoryOutlinedIcon color="primary" fontSize="small" />
              </Stack>
              <Typography variant="h4" fontWeight={700} sx={{ my: 0.75 }}>
                {unifiedPipelines.length}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {succeededPipelinesCount} Succeeded, {failedPipelinesCount} Failed (Argo + Jenkins)
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
          <Card
            variant="outlined"
            sx={{
              height: '100%',
              borderColor: runningDevicesCount > 0 ? 'success.main' : 'divider',
            }}
          >
            <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="caption" color="text.secondary" fontWeight={600}>
                  VIRTUAL DEVICES
                </Typography>
                <DeveloperBoardOutlinedIcon
                  color={runningDevicesCount > 0 ? 'success' : 'primary'}
                  fontSize="small"
                />
              </Stack>
              <Typography variant="h4" fontWeight={700} sx={{ my: 0.75 }}>
                {runningDevicesCount} Running
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {virtualDevices.length} provisioned VM(s), {deviceTemplatesCount} device templates
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
          <Card
            variant="outlined"
            sx={{
              height: '100%',
              borderColor: runningWorkstationsCount > 0 ? 'success.main' : 'divider',
            }}
          >
            <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="caption" color="text.secondary" fontWeight={600}>
                  CLOUD WORKSTATIONS
                </Typography>
                <ComputerOutlinedIcon color="primary" fontSize="small" />
              </Stack>
              <Typography variant="h4" fontWeight={700} sx={{ my: 0.75 }}>
                {runningWorkstationsCount} / {workstations.length}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {workstationClusters.length} region cluster(s), {workstationConfigs.length} configs
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 6, md: 4, lg: 2 }}>
          <Card variant="outlined" sx={{ height: '100%' }}>
            <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
              <Stack direction="row" justifyContent="space-between" alignItems="center">
                <Typography variant="caption" color="text.secondary" fontWeight={600}>
                  ARTIFACTS &amp; IMAGES
                </Typography>
                <Inventory2OutlinedIcon color="primary" fontSize="small" />
              </Stack>
              <Typography variant="h4" fontWeight={700} sx={{ my: 0.75 }}>
                {buildBundles.length + artifactImages.length + snapshots.length}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {snapshots.length} snapshot(s), {buildBundles.length} build bundles,{' '}
                {artifactImages.length} images
              </Typography>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Active Pipelines Spotlight Banner (always visible when a pipeline is running) */}
      {activePipelines.length > 0 && (
        <Paper
          variant="outlined"
          sx={{
            p: 2,
            borderColor: 'info.main',
            bgcolor: (t) => alpha(t.palette.info.main, 0.06),
          }}
        >
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
             sx={{ mb: 1.5 }}
          >
            <Stack direction="row" alignItems="center" spacing={1}>
              <CircularProgress size={18} color="info" />
              <Typography variant="subtitle1" fontWeight={700}>
                Active Pipeline Executions ({activePipelines.length} Running Now)
              </Typography>
            </Stack>
            <Typography variant="caption" color="text.secondary">
              Live execution feed across Argo Workflows and Jenkins
            </Typography>
          </Stack>
          <TableContainer component={Paper} variant="outlined">
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Engine</TableCell>
                  <TableCell>Pipeline / Template</TableCell>
                  <TableCell>Module / Category</TableCell>
                  <TableCell>Execution Node / Target</TableCell>
                  <TableCell>Started</TableCell>
                  <TableCell>Elapsed</TableCell>
                  <TableCell>Phase</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {activePipelines.map((run) => (
                  <TableRow key={run.id} hover>
                    <TableCell>
                      <Chip
                        size="small"
                        label={run.engine === 'argo' ? 'Argo Workflow' : 'Jenkins'}
                        color={run.engine === 'argo' ? 'primary' : 'secondary'}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" fontWeight={600}>
                        {run.title}{' '}
                        <Typography component="span" variant="caption" color="text.secondary">
                          ({run.name})
                        </Typography>
                      </Typography>
                      {run.parametersSummary && (
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          display="block"
                          sx={{ fontFamily: 'monospace', fontSize: 11 }}
                        >
                          {run.parametersSummary}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>{run.categoryOrModule}</TableCell>
                    <TableCell>
                      <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                        {run.executionTarget}
                      </Typography>
                    </TableCell>
                    <TableCell>{formatDateTime(run.startedAt)}</TableCell>
                    <TableCell>
                      <Typography variant="body2" fontWeight={600} color="info.main">
                        {run.durationLabel}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip size="small" label={run.phase} color="info" />
                    </TableCell>
                    <TableCell align="right">
                      {run.externalUrl && (
                        <Button
                          size="small"
                          component="a"
                          href={run.externalUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
                        >
                          Open Run
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}

      {/* Navigation Sub-Tabs for Focusing or Viewing All */}
      <Tabs
        value={viewTab}
        onChange={(_, v) => setViewTab(v as ControlRoomViewTab)}
        variant="scrollable"
        scrollButtons="auto"
        sx={{ borderBottom: 1, borderColor: 'divider' }}
      >
        <Tab label="All Systems (Command View)" value="all" />
        <Tab label={`Cloud Workstations (${workstations.length})`} value="workstations" />
        <Tab
          label={`Virtual Devices & Compute (${virtualDevices.length + buildInfraVMs.length})`}
          value="devices"
        />
        <Tab label={`Pipelines & Builds (${unifiedPipelines.length})`} value="pipelines" />
        <Tab label={`Modules & Services (${enrichedModules.length})`} value="modules" />
        <Tab
          label={`Artifacts, Snapshots & Images (${buildBundles.length + artifactImages.length + snapshots.length})`}
          value="artifacts"
        />
      </Tabs>

      {/* SECTION 1: CLOUD WORKSTATIONS FLEET */}
      {(viewTab === 'all' || viewTab === 'workstations') && (
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
            spacing={2}
            sx={{ mb: 2 }}
          >
            <Box>
              <Stack direction="row" alignItems="center" spacing={1}>
                <ComputerOutlinedIcon color="primary" />
                <Typography variant="h6" fontWeight={700}>
                  Cloud Workstations Fleet ({runningWorkstationsCount} Running /{' '}
                  {workstations.length} Total)
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Multi-region workstation clusters ({workstationClusters.map((c) => c.region).join(', ') || 'europe-west3'}), configurations, and developer instances with persistent home disks and GPU acceleration.
              </Typography>
            </Box>
            <Stack direction="row" spacing={1} alignItems="center">
              <TextField
                size="small"
                placeholder="Filter workstations..."
                value={wsSearch}
                onChange={(e) => setWsSearch(e.target.value)}
                InputProps={{
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchOutlinedIcon fontSize="small" />
                    </InputAdornment>
                  ),
                }}
              />
              <Button
                size="small"
                variant="outlined"
                component="a"
                href={`https://console.cloud.google.com/workstations/list?project=${encodeURIComponent(controlRoomData?.projectId || 'qwiklabs-asl-00-8dfd084a9673')}`}
                target="_blank"
                rel="noopener noreferrer"
                endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
              >
                GCP Console
              </Button>
            </Stack>
          </Stack>

          {/* Workstation Instances Table */}
          <TableContainer component={Paper} variant="outlined" sx={{ mb: 2.5 }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Workstation Name</TableCell>
                  <TableCell>State</TableCell>
                  <TableCell>Region / Cluster</TableCell>
                  <TableCell>Configuration</TableCell>
                  <TableCell>Machine &amp; GPU</TableCell>
                  <TableCell>Backing GCE VM &amp; Zone</TableCell>
                  <TableCell>Home Disk / Snapshot</TableCell>
                  <TableCell>Last Started</TableCell>
                  <TableCell align="right">Action</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredWorkstations.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9}>
                      <Typography variant="body2" color="text.secondary" py={1}>
                        No Cloud Workstations match the current filter.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredWorkstations.map((ws) => {
                    const chip = workstationStateChip(ws.state);
                    return (
                      <TableRow key={`${ws.region}/${ws.cluster}/${ws.config}/${ws.name}`} hover>
                        <TableCell>
                          <Typography variant="body2" fontWeight={600}>
                            {ws.name}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip size="small" label={chip.label} color={chip.color} />
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">{ws.region}</Typography>
                          <Typography variant="caption" color="text.secondary">
                            {ws.cluster}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12 }}>
                            {ws.config}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">{ws.machineType || 'N/A'}</Typography>
                          {ws.gpuType ? (
                            <Chip
                              size="small"
                              variant="outlined"
                              color="success"
                              label={`1x ${ws.gpuType}`}
                              sx={{ mt: 0.25, height: 20, fontSize: 11 }}
                            />
                          ) : (
                            <Typography variant="caption" color="text.secondary">
                              CPU only
                            </Typography>
                          )}
                        </TableCell>
                        <TableCell>
                          {ws.runtimeHostVm ? (
                            <>
                              <Typography
                                variant="caption"
                                display="block"
                                sx={{ fontFamily: 'monospace' }}
                              >
                                {ws.runtimeHostVm}
                              </Typography>
                              <Typography variant="caption" color="text.secondary">
                                Zone: {ws.runtimeZone}
                              </Typography>
                            </>
                          ) : (
                            <Typography variant="caption" color="text.secondary">
                              No active host VM
                            </Typography>
                          )}
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">
                            {ws.persistentDiskSizeGb ? `${ws.persistentDiskSizeGb} GB` : 'N/A'}
                          </Typography>
                          {ws.sourceSnapshot && (
                            <Tooltip title={`Hydrated from snapshot: ${ws.sourceSnapshot}`}>
                              <Typography
                                variant="caption"
                                color="text.secondary"
                                sx={{
                                  display: 'block',
                                  maxWidth: 180,
                                  overflow: 'hidden',
                                  textOverflow: 'ellipsis',
                                  whiteSpace: 'nowrap',
                                }}
                              >
                                {ws.sourceSnapshot}
                              </Typography>
                            </Tooltip>
                          )}
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">{formatDateTime(ws.startTime)}</Typography>
                          <Typography variant="caption" color="text.secondary">
                            {formatRelativeAge(ws.startTime)}
                          </Typography>
                        </TableCell>
                        <TableCell align="right">
                          {ws.launchUrl ? (
                            <Button
                              size="small"
                              variant={chip.label === 'RUNNING' ? 'contained' : 'outlined'}
                              component="a"
                              href={ws.launchUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
                            >
                              Launch
                            </Button>
                          ) : null}
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </TableContainer>

          {/* Workstation Configurations Summary Table */}
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Workstation Configurations ({workstationConfigs.length} Configs across{' '}
            {workstationClusters.length} Regional Clusters)
          </Typography>
          <TableContainer component={Paper} variant="outlined">
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Configuration</TableCell>
                  <TableCell>Region / Cluster</TableCell>
                  <TableCell>Machine &amp; Accelerator</TableCell>
                  <TableCell>Persistent Disk &amp; Baseline Snapshot</TableCell>
                  <TableCell>Container Image</TableCell>
                  <TableCell>Auto-Shutdown Timeout</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {workstationConfigs.map((cfg: WorkstationConfigItem) => {
                  const shortImg = cfg.containerImage.split('/').slice(-2).join('/');
                  const noAutoStop =
                    (!cfg.idleTimeout || cfg.idleTimeout === '0s') &&
                    (!cfg.runningTimeout || cfg.runningTimeout === '0s');
                  return (
                    <TableRow key={`${cfg.region}/${cfg.cluster}/${cfg.name}`} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={600} sx={{ fontFamily: 'monospace' }}>
                          {cfg.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{cfg.region}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {cfg.cluster}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{cfg.machineType}</Typography>
                        {cfg.gpuType ? (
                          <Chip
                            size="small"
                            color="success"
                            variant="outlined"
                            label={`${cfg.gpuCount || 1}x ${cfg.gpuType}`}
                            sx={{ height: 20, fontSize: 11 }}
                          />
                        ) : (
                          <Typography variant="caption" color="text.secondary">
                            No GPU
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {cfg.persistentDiskSizeGb} GB ({cfg.persistentDiskType || 'pd-ssd'})
                        </Typography>
                        {cfg.sourceSnapshot ? (
                          <Typography
                            variant="caption"
                            color="primary.main"
                            sx={{ fontFamily: 'monospace' }}
                          >
                            Snap: {cfg.sourceSnapshot}
                          </Typography>
                        ) : (
                          <Typography variant="caption" color="text.secondary">
                            Blank /home volume
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Tooltip title={cfg.containerImage}>
                          <Typography
                            variant="caption"
                            sx={{
                              fontFamily: 'monospace',
                              display: 'block',
                              maxWidth: 260,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {shortImg}
                          </Typography>
                        </Tooltip>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          color={noAutoStop ? 'success' : 'default'}
                          variant="outlined"
                          label={formatTimeoutPolicy(cfg.idleTimeout, cfg.runningTimeout)}
                        />
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}

      {/* SECTION 2: VIRTUAL DEVICES (CUTTLEFISH / REMOTIVE) & COMPUTE INFRASTRUCTURE */}
      {(viewTab === 'all' || viewTab === 'devices') && (
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
            spacing={2}
            sx={{ mb: 2 }}
          >
            <Box>
              <Stack direction="row" alignItems="center" spacing={1}>
                <DeveloperBoardOutlinedIcon color="primary" />
                <Typography variant="h6" fontWeight={700}>
                  Running Virtual Devices &amp; Compute Fleet
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Cuttlefish AAOS virtual devices, Remotive topology VMs, ABFS high-memory build
                servers, and ready-to-launch GCE virtual device templates.
              </Typography>
            </Box>
            <Stack direction="row" spacing={1}>
              <Button
                size="small"
                variant="contained"
                component="a"
                href="/mtk-connect"
                target="_blank"
                rel="noopener noreferrer"
                endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
              >
                Open MTK Connect
              </Button>
              <Button
                size="small"
                variant="outlined"
                component="a"
                href={`https://console.cloud.google.com/compute/instances?project=${encodeURIComponent(controlRoomData?.projectId || 'qwiklabs-asl-00-8dfd084a9673')}`}
                target="_blank"
                rel="noopener noreferrer"
                endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
              >
                GCE Instances
              </Button>
            </Stack>
          </Stack>

          <Grid container spacing={2.5}>
            {/* Left column: Active Virtual Devices + Device Templates */}
            <Grid size={{ xs: 12, lg: 7 }}>
              <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
                Provisioned Virtual Device VMs ({virtualDevices.length})
              </Typography>
              <TableContainer component={Paper} variant="outlined" sx={{ mb: 2 }}>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Device VM Name</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Arch &amp; Machine</TableCell>
                      <TableCell>Zone &amp; Internal IP</TableCell>
                      <TableCell>Created</TableCell>
                      <TableCell align="right">Testbench</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {virtualDevices.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6}>
                          <Typography variant="body2" color="text.secondary" py={1}>
                            No Cuttlefish or Remotive virtual device VMs are currently provisioned.
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      virtualDevices.map((dev: ComputeInstanceItem) => (
                        <TableRow key={dev.name} hover>
                          <TableCell>
                            <Typography
                              variant="body2"
                              fontWeight={600}
                              sx={{ fontFamily: 'monospace' }}
                            >
                              {dev.name}
                            </Typography>
                            {dev.labels?.jenkins_config_name && (
                              <Typography variant="caption" color="text.secondary">
                                Config: {dev.labels.jenkins_config_name}
                              </Typography>
                            )}
                          </TableCell>
                          <TableCell>
                            <Chip
                              size="small"
                              label={dev.status}
                              color={dev.status === 'RUNNING' ? 'success' : 'default'}
                            />
                          </TableCell>
                          <TableCell>
                            <Stack direction="row" spacing={0.5} alignItems="center">
                              <Chip
                                size="small"
                                variant="outlined"
                                color="primary"
                                label={dev.architecture}
                                sx={{ height: 20, fontSize: 11 }}
                              />
                              <Typography variant="body2">{dev.machineType}</Typography>
                            </Stack>
                            {dev.diskSizeGb ? (
                              <Typography variant="caption" color="text.secondary">
                                Disk: {dev.diskSizeGb} GB
                              </Typography>
                            ) : null}
                          </TableCell>
                          <TableCell>
                            <Typography variant="body2">{dev.zone}</Typography>
                            <Typography
                              variant="caption"
                              color="text.secondary"
                              sx={{ fontFamily: 'monospace' }}
                            >
                              {dev.internalIp || 'N/A'}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Typography variant="body2">
                              {formatDateTime(dev.creationTimestamp)}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              {formatRelativeAge(dev.creationTimestamp)}
                            </Typography>
                          </TableCell>
                          <TableCell align="right">
                            <Button
                              size="small"
                              variant="outlined"
                              component="a"
                              href="/mtk-connect"
                              target="_blank"
                              rel="noopener noreferrer"
                              endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
                            >
                              MTK Connect
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>

              <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
                Virtual Device &amp; Node Pool Instance Templates ({instanceTemplates.length})
              </Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Template Name</TableCell>
                      <TableCell>Type</TableCell>
                      <TableCell>Machine Type</TableCell>
                      <TableCell>Boot Disk</TableCell>
                      <TableCell>Created</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {instanceTemplates.map((tpl: InstanceTemplateItem) => (
                      <TableRow key={tpl.name} hover>
                        <TableCell>
                          <Typography
                            variant="body2"
                            fontWeight={tpl.category === 'virtual-device' ? 600 : 400}
                            sx={{ fontFamily: 'monospace', fontSize: 12 }}
                          >
                            {tpl.name}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip
                            size="small"
                            variant="outlined"
                            color={tpl.category === 'virtual-device' ? 'primary' : 'default'}
                            label={
                              tpl.category === 'virtual-device'
                                ? 'Virtual Device'
                                : 'GKE Node Pool'
                            }
                          />
                        </TableCell>
                        <TableCell>{tpl.machineType}</TableCell>
                        <TableCell>{tpl.diskSizeGb ? `${tpl.diskSizeGb} GB` : 'N/A'}</TableCell>
                        <TableCell>{formatDateTime(tpl.creationTimestamp)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Grid>

            {/* Right column: Build Infrastructure VMs (ABFS Server / Uploaders) + Workstation Host VMs + GKE Nodes */}
            <Grid size={{ xs: 12, lg: 5 }}>
              <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
                ABFS Build Infrastructure &amp; Workstation Host VMs (
                {buildInfraVMs.length + workstationVMs.length})
              </Typography>
              <TableContainer component={Paper} variant="outlined" sx={{ mb: 2 }}>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Instance Name</TableCell>
                      <TableCell>Role</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Machine / GPU</TableCell>
                      <TableCell>Zone / IP</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {[...workstationVMs, ...buildInfraVMs].map((vm: ComputeInstanceItem) => (
                      <TableRow key={vm.name} hover>
                        <TableCell>
                          <Typography
                            variant="body2"
                            fontWeight={500}
                            sx={{
                              fontFamily: 'monospace',
                              fontSize: 12,
                              maxWidth: 190,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                            title={vm.name}
                          >
                            {vm.name}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip
                            size="small"
                            variant="outlined"
                            label={vm.role === 'workstation-vm' ? 'CWS Host' : 'ABFS Infra'}
                          />
                        </TableCell>
                        <TableCell>
                          <Chip
                            size="small"
                            label={vm.status}
                            color={vm.status === 'RUNNING' ? 'success' : 'default'}
                          />
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2">{vm.machineType}</Typography>
                          {vm.gpuType && (
                            <Typography variant="caption" color="success.main">
                              {vm.gpuCount || 1}x {vm.gpuType}
                            </Typography>
                          )}
                        </TableCell>
                        <TableCell>
                          <Typography variant="caption" display="block">
                            {vm.zone}
                          </Typography>
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            sx={{ fontFamily: 'monospace' }}
                          >
                            {vm.internalIp}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>

              <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
                Active GKE Cluster Nodes ({clusterNodes.length} Running)
              </Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>Node Name</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Machine</TableCell>
                      <TableCell>Zone / IP</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {clusterNodes.map((node: ComputeInstanceItem) => (
                      <TableRow key={node.name} hover>
                        <TableCell>
                          <Typography
                            variant="caption"
                            sx={{ fontFamily: 'monospace', fontWeight: 500 }}
                          >
                            {node.name}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Chip size="small" label={node.status} color="success" />
                        </TableCell>
                        <TableCell>{node.machineType}</TableCell>
                        <TableCell>
                          <Typography variant="caption">
                            {node.zone} ({node.internalIp})
                          </Typography>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Grid>
          </Grid>
        </Paper>
      )}

      {/* SECTION 3: HORIZON MODULES & PLATFORM SERVICES */}
      {(viewTab === 'all' || viewTab === 'modules') && (
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
            spacing={2}
            sx={{ mb: 2 }}
          >
            <Box>
              <Stack direction="row" alignItems="center" spacing={1}>
                <LayersOutlinedIcon color="primary" />
                <Typography variant="h6" fontWeight={700}>
                  Horizon Modules &amp; Platform Services ({readyModulesCount} Ready /{' '}
                  {enrichedModules.length} Total)
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Live GitOps deployment health from Module Manager and Argo CD, plus direct shortcuts
                to all integrated toolchain applications.
              </Typography>
            </Box>
            <Button
              size="small"
              variant="outlined"
              component={Link}
              to="/admin/modules"
            >
              Manage Modules
            </Button>
          </Stack>

          <TableContainer component={Paper} variant="outlined" sx={{ mb: 2.5 }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Module Name</TableCell>
                  <TableCell>Deployment Status</TableCell>
                  <TableCell>Argo CD Sync / Health</TableCell>
                  <TableCell>Git Ref / Revision</TableCell>
                  <TableCell>Workflow Templates</TableCell>
                  <TableCell>Dependencies</TableCell>
                  <TableCell align="right">Quick Navigation</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {enrichedModules.map(({ module: m, status: st, deploymentLabel, templateCount }) => {
                  const statusColor =
                    deploymentLabel === 'READY'
                      ? 'success'
                      : deploymentLabel === 'NOT INSTALLED'
                        ? 'default'
                        : 'warning';
                  const rev = (m.pinned ? m.targetRevision : m.clusterTargetRevision) || 'N/A';
                  const deps = [
                    ...(m.hardDependencies ?? []),
                    ...(m.softDependencies ?? []),
                  ].join(', ');
                  return (
                    <TableRow key={m.name} hover>
                      <TableCell>
                        <Typography variant="body2" fontWeight={600}>
                          {m.name}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip size="small" label={deploymentLabel} color={statusColor} />
                      </TableCell>
                      <TableCell>
                        {m.enabled ? (
                          <Stack direction="row" spacing={0.75}>
                            <Chip
                              size="small"
                              variant="outlined"
                              color={st.syncStatus === 'Synced' ? 'success' : 'warning'}
                              label={st.syncStatus || 'Unknown'}
                            />
                            <Chip
                              size="small"
                              variant="outlined"
                              color={st.healthStatus === 'Healthy' ? 'success' : 'warning'}
                              label={st.healthStatus || 'Unknown'}
                            />
                          </Stack>
                        ) : (
                          <Typography variant="caption" color="text.secondary">
                            Disabled
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Stack direction="row" spacing={0.75} alignItems="center">
                          <Typography
                            variant="caption"
                            sx={{ fontFamily: 'monospace', fontWeight: 500 }}
                          >
                            {rev}
                          </Typography>
                          {m.pinned && (
                            <Chip size="small" label="Pinned" color="warning" variant="outlined" />
                          )}
                        </Stack>
                      </TableCell>
                      <TableCell>
                        {templateCount > 0 ? (
                          <Chip
                            size="small"
                            variant="outlined"
                            color="primary"
                            label={`${templateCount} template(s)`}
                          />
                        ) : (
                          <Typography variant="caption" color="text.secondary">
                            0
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="caption" color="text.secondary">
                          {deps || 'None'}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        {deploymentLabel === 'READY' ? (
                          <Stack direction="row" spacing={0.75} justifyContent="flex-end">
                            <Button
                              size="small"
                              variant="outlined"
                              component={Link}
                              to={`/module/${encodeURIComponent(m.name)}?tab=overview`}
                            >
                              Overview
                            </Button>
                            {templateCount > 0 && (
                              <Button
                                size="small"
                                variant="contained"
                                component={Link}
                                to={`/module/${encodeURIComponent(m.name)}?tab=templates`}
                              >
                                Workflows
                              </Button>
                            )}
                          </Stack>
                        ) : (
                          <Button
                            size="small"
                            variant="text"
                            component={Link}
                            to="/admin/modules"
                          >
                            Configure
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>

          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Core Platform Services &amp; Consoles
          </Typography>
          <Stack direction="row" flexWrap="wrap" gap={1.25}>
            {[
              { label: 'Gerrit Code Review', href: '/gerrit' },
              { label: 'Jenkins CI/CD', href: '/jenkins' },
              { label: 'MTK Connect Testbenches', href: '/mtk-connect' },
              { label: 'Argo Workflows UI', href: '/workflows' },
              { label: 'Argo CD GitOps', href: '/argocd' },
              { label: 'Grafana Dashboards', href: '/grafana' },
              { label: 'Headlamp K8s UI', href: '/headlamp/' },
              { label: 'Keycloak IAM', href: '/auth/admin/horizon/console' },
              {
                label: 'MCP Gateway Registry',
                href: `${window.location.protocol}//mcp.${window.location.hostname}`,
              },
            ].map((svc) => (
              <Button
                key={svc.label}
                size="small"
                variant="outlined"
                component="a"
                href={svc.href}
                target="_blank"
                rel="noopener noreferrer"
                endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
              >
                {svc.label}
              </Button>
            ))}
          </Stack>
        </Paper>
      )}

      {/* SECTION 4: UNIFIED PIPELINES & BUILD EXECUTIONS (ARGO WORKFLOWS + JENKINS) */}
      {(viewTab === 'all' || viewTab === 'pipelines') && (
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Stack
            direction={{ xs: 'column', md: 'row' }}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', md: 'center' }}
            spacing={2}
            sx={{ mb: 2 }}
          >
            <Box>
              <Stack direction="row" alignItems="center" spacing={1}>
                <AccountTreeOutlinedIcon color="primary" />
                <Typography variant="h6" fontWeight={700}>
                  Pipelines &amp; Workflow History ({filteredPipelines.length} of{' '}
                  {unifiedPipelines.length} Runs)
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Unified execution timeline across Argo Workflows (live + GCS archive) and Jenkins
                multi-stage pipelines.
              </Typography>
            </Box>

            <Stack direction="row" flexWrap="wrap" gap={1} alignItems="center">
              <Stack direction="row" spacing={0.5}>
                {(
                  [
                    { key: 'all', label: 'All Engines' },
                    { key: 'argo', label: 'Argo Workflows' },
                    { key: 'jenkins', label: 'Jenkins' },
                  ] as const
                ).map((item) => (
                  <Chip
                    key={item.key}
                    size="small"
                    clickable
                    color={pipelineEngineFilter === item.key ? 'primary' : 'default'}
                    variant={pipelineEngineFilter === item.key ? 'filled' : 'outlined'}
                    label={item.label}
                    onClick={() => setPipelineEngineFilter(item.key)}
                  />
                ))}
              </Stack>
              <Divider orientation="vertical" flexItem />
              <Stack direction="row" spacing={0.5}>
                {(
                  [
                    { key: 'all', label: 'All Statuses' },
                    { key: 'running', label: 'Running' },
                    { key: 'succeeded', label: 'Succeeded' },
                    { key: 'failed', label: 'Failed' },
                    { key: 'aborted', label: 'Aborted' },
                  ] as const
                ).map((item) => (
                  <Chip
                    key={item.key}
                    size="small"
                    clickable
                    color={pipelineStatusFilter === item.key ? 'primary' : 'default'}
                    variant={pipelineStatusFilter === item.key ? 'filled' : 'outlined'}
                    label={item.label}
                    onClick={() => setPipelineStatusFilter(item.key)}
                  />
                ))}
              </Stack>
              <TextField
                size="small"
                placeholder="Search pipelines..."
                value={pipelineSearch}
                onChange={(e) => setPipelineSearch(e.target.value)}
                InputProps={{
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchOutlinedIcon fontSize="small" />
                    </InputAdornment>
                  ),
                }}
              />
            </Stack>
          </Stack>

          <TableContainer component={Paper} variant="outlined" sx={{ maxHeight: 520 }}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell>Engine</TableCell>
                  <TableCell>Pipeline / Workflow Template</TableCell>
                  <TableCell>Run / Instance</TableCell>
                  <TableCell>Module / Category</TableCell>
                  <TableCell>Execution Node / Target</TableCell>
                  <TableCell>Started</TableCell>
                  <TableCell>Duration</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell align="right">Links</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredPipelines.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={9}>
                      <Typography variant="body2" color="text.secondary" py={2}>
                        No pipeline executions match the selected filters.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredPipelines.map((run) => (
                    <TableRow key={run.id} hover>
                      <TableCell>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={run.engine === 'argo' ? 'primary' : 'secondary'}
                          label={run.sourceLabel}
                        />
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" fontWeight={600}>
                          {run.title}
                        </Typography>
                        {run.parametersSummary && (
                          <Tooltip title={run.parametersSummary}>
                            <Typography
                              variant="caption"
                              color="text.secondary"
                              sx={{
                                fontFamily: 'monospace',
                                display: 'block',
                                maxWidth: 260,
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {run.parametersSummary}
                            </Typography>
                          </Tooltip>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography
                          variant="caption"
                          sx={{ fontFamily: 'monospace', fontWeight: 500 }}
                        >
                          {run.name}
                        </Typography>
                      </TableCell>
                      <TableCell>{run.categoryOrModule}</TableCell>
                      <TableCell>
                        <Typography
                          variant="caption"
                          sx={{
                            fontFamily: 'monospace',
                            display: 'block',
                            maxWidth: 220,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                          title={run.executionTarget}
                        >
                          {run.executionTarget}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{formatDateTime(run.startedAt)}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {formatRelativeAge(run.startedAt)}
                        </Typography>
                      </TableCell>
                      <TableCell>{run.durationLabel}</TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          label={run.phase}
                          color={phaseChipColor(run.phase)}
                          variant={run.phase === 'RUNNING' ? 'filled' : 'outlined'}
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                          {run.internalModuleLink && (
                            <Button
                              size="small"
                              variant="text"
                              component={Link}
                              to={run.internalModuleLink}
                            >
                              Module
                            </Button>
                          )}
                          {run.externalUrl && (
                            <Button
                              size="small"
                              variant="outlined"
                              component="a"
                              href={run.externalUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
                            >
                              Inspect
                            </Button>
                          )}
                        </Stack>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}

      {/* SECTION 5: BUILDS, ARTIFACTS, DISK SNAPSHOTS & CONTAINER IMAGES */}
      {(viewTab === 'all' || viewTab === 'artifacts') && (
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
            spacing={2}
            sx={{ mb: 2 }}
          >
            <Box>
              <Stack direction="row" alignItems="center" spacing={1}>
                <StorageOutlinedIcon color="primary" />
                <Typography variant="h6" fontWeight={700}>
                  Builds, Artifacts, Disk Snapshots &amp; Container Images
                </Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Promoted 1 TB Cloud Workstation snapshots, AAOS / OpenBSW / Remotive build output
                bundles in Cloud Storage, and container images in Artifact Registry.
              </Typography>
            </Box>
          </Stack>

          {/* 5A. Promoted Compute Disk Snapshots */}
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Promoted Compute Engine Disk Snapshots ({snapshots.length})
          </Typography>
          <TableContainer component={Paper} variant="outlined" sx={{ mb: 3 }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Snapshot Name</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Provisioned Disk</TableCell>
                  <TableCell>Compressed Size</TableCell>
                  <TableCell>Location</TableCell>
                  <TableCell>Labels &amp; Promotion Status</TableCell>
                  <TableCell>Created</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {snapshots.map((snap: DiskSnapshotItem) => (
                  <TableRow key={snap.name} hover>
                    <TableCell>
                      <Typography variant="body2" fontWeight={600} sx={{ fontFamily: 'monospace' }}>
                        {snap.name}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        label={snap.status}
                        color={snap.status === 'READY' ? 'success' : 'warning'}
                      />
                    </TableCell>
                    <TableCell>{snap.diskSizeGb} GB</TableCell>
                    <TableCell>{formatBytes(snap.storageBytes)}</TableCell>
                    <TableCell>{(snap.storageLocations ?? []).join(', ') || 'multi-region'}</TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5} flexWrap="wrap">
                        {snap.isLatest && (
                          <Chip size="small" color="success" label="is-latest=true" />
                        )}
                        {Object.entries(snap.labels ?? {})
                          .filter(([k]) => k !== 'is-latest')
                          .map(([k, v]) => (
                            <Chip
                              key={k}
                              size="small"
                              variant="outlined"
                              label={`${k}: ${v}`}
                            />
                          ))}
                      </Stack>
                    </TableCell>
                    <TableCell>{formatDateTime(snap.creationTimestamp)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>

          {/* 5B. GCS Build Artifact Bundles */}
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Cloud Storage Build Artifact Bundles ({buildBundles.length} Bundles across AAOS, ABFS,
            OpenBSW &amp; Remotive)
          </Typography>
          <TableContainer component={Paper} variant="outlined" sx={{ mb: 3, maxHeight: 420 }}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell>Pipeline / Build Bundle</TableCell>
                  <TableCell>Category</TableCell>
                  <TableCell>Target / Arch</TableCell>
                  <TableCell>Key Output Artifacts</TableCell>
                  <TableCell>Files &amp; Total Size</TableCell>
                  <TableCell>Updated</TableCell>
                  <TableCell align="right">GCS URI / Console</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {buildBundles.map((bundle: BuildArtifactBundle) => (
                  <TableRow key={bundle.id} hover>
                    <TableCell>
                      <Typography variant="body2" fontWeight={600}>
                        {bundle.pipelineName} #{bundle.buildNumber}
                      </Typography>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ fontFamily: 'monospace' }}
                      >
                        {bundle.bucket}
                      </Typography>
                    </TableCell>
                    <TableCell>{bundle.category}</TableCell>
                    <TableCell>
                      {bundle.targetArch ? (
                        <Chip
                          size="small"
                          color="primary"
                          variant="outlined"
                          label={bundle.targetArch}
                        />
                      ) : (
                        <Typography variant="caption" color="text.secondary">
                          Standard
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                        {bundle.keyFiles.map((f) => (
                          <Chip
                            key={f}
                            size="small"
                            variant="outlined"
                            label={f}
                            sx={{ fontFamily: 'monospace', fontSize: 11, height: 22 }}
                          />
                        ))}
                        {bundle.hasGeminiAssist && (
                          <Chip
                            size="small"
                            color="info"
                            variant="outlined"
                            label="Gemini AI Report"
                            sx={{ height: 22, fontSize: 11 }}
                          />
                        )}
                      </Stack>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" fontWeight={500}>
                        {formatBytes(bundle.totalSizeBytes)}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {bundle.fileCount} file(s)
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{formatDateTime(bundle.updatedAt)}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {formatRelativeAge(bundle.updatedAt)}
                      </Typography>
                    </TableCell>
                    <TableCell align="right">
                      <Stack direction="row" spacing={0.5} justifyContent="flex-end">
                        <Tooltip title={`Copy ${bundle.gcsUri}`}>
                          <IconButton
                            size="small"
                            onClick={() => copyToClipboard(bundle.gcsUri)}
                          >
                            <ContentCopyOutlinedIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Button
                          size="small"
                          variant="outlined"
                          component="a"
                          href={bundle.consoleUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          endIcon={<OpenInNewOutlinedIcon fontSize="small" />}
                        >
                          Browse
                        </Button>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>

          {/* 5C. Artifact Registry Container Images */}
          <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
            Artifact Registry Container Images ({artifactImages.length} Images in{' '}
            <code>horizon-sdv</code>)
          </Typography>
          <TableContainer component={Paper} variant="outlined" sx={{ maxHeight: 420 }}>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell>Image Package</TableCell>
                  <TableCell>Tags</TableCell>
                  <TableCell>Compressed Size</TableCell>
                  <TableCell>Digest</TableCell>
                  <TableCell>Updated</TableCell>
                  <TableCell align="right">Copy Pull URI</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {artifactImages.map((img: ContainerImageSummary) => {
                  const primaryTag = img.tags?.[0] ? `:${img.tags[0]}` : `@${img.digest}`;
                  const pullUri = `${img.fullUri}${primaryTag}`;
                  return (
                    <TableRow key={`${img.packageName}:${img.digest}`} hover>
                      <TableCell>
                        <Typography
                          variant="body2"
                          fontWeight={600}
                          sx={{ fontFamily: 'monospace' }}
                        >
                          {img.packageName}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Stack direction="row" spacing={0.5} flexWrap="wrap">
                          {img.tags && img.tags.length > 0 ? (
                            img.tags.map((t) => (
                              <Chip
                                key={t}
                                size="small"
                                color={t === 'latest' ? 'success' : 'primary'}
                                variant="outlined"
                                label={t}
                              />
                            ))
                          ) : (
                            <Chip size="small" variant="outlined" label="untagged" />
                          )}
                        </Stack>
                      </TableCell>
                      <TableCell>{formatBytes(img.sizeBytes)}</TableCell>
                      <TableCell>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ fontFamily: 'monospace' }}
                        >
                          {img.digest ? img.digest.slice(0, 19) + '...' : 'N/A'}
                        </Typography>
                      </TableCell>
                      <TableCell>{formatDateTime(img.updateTime)}</TableCell>
                      <TableCell align="right">
                        <Tooltip title={`Copy ${pullUri}`}>
                          <IconButton size="small" onClick={() => copyToClipboard(pullUri)}>
                            <ContentCopyOutlinedIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}
    </Stack>
  );
}
