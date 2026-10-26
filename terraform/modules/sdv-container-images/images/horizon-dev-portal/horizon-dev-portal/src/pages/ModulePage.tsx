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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import CloudOutlinedIcon from '@mui/icons-material/CloudOutlined';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import HelpOutlineIcon from '@mui/icons-material/HelpOutline';
import DownloadOutlinedIcon from '@mui/icons-material/DownloadOutlined';
import Inventory2OutlinedIcon from '@mui/icons-material/Inventory2Outlined';
import OpenInNewOutlinedIcon from '@mui/icons-material/OpenInNewOutlined';
import StopOutlinedIcon from '@mui/icons-material/StopOutlined';
import TerminalOutlinedIcon from '@mui/icons-material/TerminalOutlined';
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  Stack,
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
  Paper,
} from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useParams, useSearchParams } from 'react-router-dom';
import { apiHorizon, apiMm } from '../utils/api';
import { authService } from '../utils/auth';
import { config } from '../utils/config';
import { formatMiBToDecimalGBAdornment } from '../utils/formatMiBSize';
import {
  DIALOG_LAYOUT_COOKIE_WORKFLOW_DETAIL,
  DIALOG_LAYOUT_COOKIE_WORKFLOW_LOGS,
  useResizableDialogSize,
} from '../utils/dialogLayoutCookie';
import { useMaxWorkflowLogLines } from '../utils/logBufferSettings';
import { injectOverviewPortalTheme } from '../utils/overviewPortalTheme';
import { workflowPassesSubmittedFromFilter } from '../utils/workflowVisibilityDiscovery';
import type {
  CatalogEntry,
  CatalogResponse,
  ModuleApplication,
  ModuleResponse,
  WorkflowDetail,
  WorkflowListResponse,
  WorkflowSummary,
  OutputArtifact,
  WorkflowsVisibilityDTO,
} from '../types';

type TabKey = 'overview' | 'applications' | 'templates' | 'running' | 'history';

const TAB_KEYS: TabKey[] = ['overview', 'applications', 'templates', 'running', 'history'];

function parseTabKey(raw: string | null): TabKey {
  if (!raw || !TAB_KEYS.includes(raw as TabKey)) {
    return 'overview';
  }
  return raw as TabKey;
}

function resolveApplicationHref(url: string): string {
  if (url.startsWith('http://') || url.startsWith('https://')) {
    return url;
  }
  try {
    return new URL(url, window.location.origin).href;
  } catch {
    return url;
  }
}

function useCatalogEntries(module: string | undefined) {
  const [entries, setEntries] = useState<CatalogEntry[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const load = useCallback(async () => {
    if (!module) {
      return;
    }
    try {
      const r = await apiHorizon('/v1/catalog');
      if (!r.ok) {
        throw new Error(`catalog ${r.status}`);
      }
      const j = (await r.json()) as CatalogResponse;
      setEntries((j.entries || []).filter((e) => e.module === module));
      setErr(null);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'catalog');
    }
  }, [module]);
  useEffect(() => {
    void load();
    const id = window.setInterval(load, 15000);
    return () => window.clearInterval(id);
  }, [load]);
  return { entries, err, reload: load };
}

function templateNames(entries: CatalogEntry[]): Set<string> {
  return new Set(entries.map((e) => e.templateName));
}

export function ModulePage() {
  const { name: moduleName = '' } = useParams<{ name: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const tabFromUrl = parseTabKey(searchParams.get('tab'));
  const setTab = (k: TabKey) => {
    const n = new URLSearchParams(searchParams);
    n.set('tab', k);
    setSearchParams(n);
  };

  const { entries, err: catErr, reload: reloadCatalog } = useCatalogEntries(moduleName);
  const names = useMemo(() => templateNames(entries), [entries]);
  const hasTemplates = entries.length > 0;

  const [moduleApplications, setModuleApplications] = useState<ModuleApplication[]>([]);
  const [moduleMetaLoaded, setModuleMetaLoaded] = useState(false);
  const hasApplications = moduleApplications.length > 0;

  // Must match a rendered Tab — MUI Tabs breaks when value has no matching Tab.
  const activeTab: TabKey = useMemo(() => {
    if (tabFromUrl === 'overview') {
      return 'overview';
    }
    if (tabFromUrl === 'applications') {
      if (!moduleMetaLoaded) {
        return 'overview';
      }
      if (hasApplications) {
        return 'applications';
      }
      return 'overview';
    }
    if (
      hasTemplates &&
      (tabFromUrl === 'templates' || tabFromUrl === 'running' || tabFromUrl === 'history')
    ) {
      return tabFromUrl;
    }
    return 'overview';
  }, [tabFromUrl, hasApplications, hasTemplates, moduleMetaLoaded]);

  useEffect(() => {
    if (!moduleMetaLoaded) {
      return;
    }
    if (activeTab !== tabFromUrl) {
      setTab(activeTab);
    }
  }, [moduleMetaLoaded, activeTab, tabFromUrl, setTab]);

  const [runList, setRunList] = useState<WorkflowSummary[]>([]);
  const [histList, setHistList] = useState<WorkflowSummary[]>([]);
  /** Workflow names currently undergoing DELETE (blocking); History Result shows "Deletion in progress". */
  const [deletingWorkflows, setDeletingWorkflows] = useState<Record<string, boolean>>({});
  const [listErr, setListErr] = useState<string | null>(null);
  /** True when ModuleCatalog lists an in-cluster overview Service (Module Manager proxies GET /modules/{name}/overview). */
  const [overviewInCluster, setOverviewInCluster] = useState(false);
  /** Raw HTML from Module Manager (theme applied in useMemo from MUI mode). */
  const [overviewHtmlRaw, setOverviewHtmlRaw] = useState<string | null>(null);
  const [overviewErr, setOverviewErr] = useState<string | null>(null);
  const [overviewLoading, setOverviewLoading] = useState(false);
  const theme = useTheme();
  const overviewSrcDoc = useMemo(() => {
    if (!overviewHtmlRaw) {
      return null;
    }
    return injectOverviewPortalTheme(overviewHtmlRaw, theme.palette.mode);
  }, [overviewHtmlRaw, theme.palette.mode]);
  /** null = no filter (show all sources). */
  const [submittedFromAllowlist, setSubmittedFromAllowlist] = useState<string[] | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const r = await apiMm('/settings/workflows-visibility');
        if (!r.ok || cancelled) {
          return;
        }
        const j = (await r.json()) as WorkflowsVisibilityDTO;
        if (j.allowedSubmittedFrom === undefined) {
          setSubmittedFromAllowlist(null);
        } else {
          setSubmittedFromAllowlist(j.allowedSubmittedFrom ?? []);
        }
      } catch {
        if (!cancelled) {
          setSubmittedFromAllowlist(null);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!moduleName) {
      setOverviewInCluster(false);
      setOverviewHtmlRaw(null);
      setModuleApplications([]);
      setModuleMetaLoaded(true);
      return;
    }
    let cancelled = false;

    const loadModuleMeta = (initial: boolean) => {
      if (initial) {
        setModuleMetaLoaded(false);
        setOverviewInCluster(false);
        setOverviewErr(null);
      }
      void (async () => {
        try {
          const r = await apiMm(`/modules/${encodeURIComponent(moduleName)}`);
          if (!r.ok || cancelled) {
            return;
          }
          const m = (await r.json()) as ModuleResponse;
          if (!cancelled) {
            const svc = (m.overviewService ?? '').trim();
            const ns = (m.overviewServiceNamespace ?? '').trim();
            setOverviewInCluster(!!(svc && ns));
            setModuleApplications(m.applications ?? []);
          }
        } catch {
          if (!cancelled && initial) {
            setOverviewInCluster(false);
            setModuleApplications([]);
          }
        } finally {
          if (!cancelled && initial) {
            setModuleMetaLoaded(true);
          }
        }
      })();
    };

    loadModuleMeta(true);
    const pollId = window.setInterval(() => loadModuleMeta(false), 15000);
    return () => {
      cancelled = true;
      window.clearInterval(pollId);
    };
  }, [moduleName]);

  useEffect(() => {
    if (!moduleName || !overviewInCluster) {
      setOverviewHtmlRaw(null);
      setOverviewErr(null);
      setOverviewLoading(false);
      return;
    }
    let cancelled = false;
    setOverviewLoading(true);
    setOverviewErr(null);
    setOverviewHtmlRaw(null);
    (async () => {
      try {
        const r = await apiMm(`/modules/${encodeURIComponent(moduleName)}/overview`);
        if (cancelled) {
          return;
        }
        if (!r.ok) {
          const t = await r.text().catch(() => '');
          setOverviewErr(t.trim() || `HTTP ${r.status}`);
          return;
        }
        const text = await r.text();
        if (!cancelled) {
          setOverviewHtmlRaw(text);
        }
      } catch (e: unknown) {
        if (!cancelled) {
          setOverviewErr(e instanceof Error ? e.message : 'overview fetch failed');
        }
      } finally {
        if (!cancelled) {
          setOverviewLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [moduleName, overviewInCluster]);

  const loadRunning = useCallback(async () => {
    try {
      const r = await apiHorizon('/v1/workflows/running?limit=200');
      if (!r.ok) {
        throw new Error(`running ${r.status}`);
      }
      const j = (await r.json()) as WorkflowListResponse;
      const items = (j.items || [])
        .filter((w) => (w.workflowTemplate ? names.has(w.workflowTemplate) : false))
        .filter((w) => workflowPassesSubmittedFromFilter(w, submittedFromAllowlist));
      setRunList(items);
      setListErr(null);
    } catch (e: unknown) {
      setListErr(e instanceof Error ? e.message : 'running');
    }
  }, [names, submittedFromAllowlist]);

  const loadHistory = useCallback(async () => {
    try {
      const r = await apiHorizon('/v1/workflows/history?limit=200');
      if (!r.ok) {
        throw new Error(`history ${r.status}`);
      }
      const j = (await r.json()) as WorkflowListResponse;
      const items = (j.items || [])
        .filter((w) => (w.workflowTemplate ? names.has(w.workflowTemplate) : false))
        .filter((w) => workflowPassesSubmittedFromFilter(w, submittedFromAllowlist));
      setHistList(items);
      setListErr(null);
    } catch (e: unknown) {
      setListErr(e instanceof Error ? e.message : 'history');
    }
  }, [names, submittedFromAllowlist]);

  useEffect(() => {
    if (activeTab === 'running') {
      if (names.size === 0) {
        setRunList([]);
        return;
      }
      void loadRunning();
      // Short interval so phase (Running → Pending → Running, etc.) feels live while workflows execute.
      const id = window.setInterval(loadRunning, 3000);
      return () => window.clearInterval(id);
    }
    if (activeTab === 'history') {
      if (names.size === 0) {
        setHistList([]);
        return;
      }
      void loadHistory();
      const id = window.setInterval(loadHistory, 4000);
      return () => window.clearInterval(id);
    }
  }, [activeTab, names, loadRunning, loadHistory]);

  // After long idle / background tab, timers are throttled and the CI token may have rotated;
  // refresh when the user comes back so we clear stale 401s without a full page reload.
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState !== 'visible') {
        return;
      }
      void reloadCatalog();
      if (names.size === 0) {
        return;
      }
      if (activeTab === 'running') {
        void loadRunning();
      } else if (activeTab === 'history') {
        void loadHistory();
      }
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, [reloadCatalog, loadRunning, loadHistory, activeTab, names.size]);

  const [selTpl, setSelTpl] = useState<CatalogEntry | null>(null);
  const [detailName, setDetailName] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Module: {moduleName}
      </Typography>
      {/* MUI Tabs must not wrap <Tab> in Fragment — cloneElement only hits direct children. */}
      <Tabs value={activeTab} onChange={(_, v) => setTab(v as TabKey)} sx={{ mb: 2 }}>
        <Tab label="Overview" value="overview" />
        {hasApplications ? <Tab label="Applications" value="applications" /> : null}
        {hasTemplates ? <Tab label="Workflow Templates" value="templates" /> : null}
        {hasTemplates ? <Tab label="Running Workflows" value="running" /> : null}
        {hasTemplates ? <Tab label="History" value="history" /> : null}
      </Tabs>

      {catErr && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {catErr}
        </Alert>
      )}
      {listErr && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {listErr}
        </Alert>
      )}

      {activeTab === 'overview' && (
        <Card
          variant="outlined"
          sx={{
            display: 'flex',
            flexDirection: 'column',
            minHeight: 'calc(100vh - 235px)',
            overflow: 'hidden',
          }}
        >
          <CardContent
            sx={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              minHeight: 'calc(100vh - 235px)',
              p: overviewSrcDoc ? 0 : 2,
              '&:last-child': { pb: overviewSrcDoc ? 0 : 2 },
            }}
          >
            {!overviewSrcDoc && (
              <Typography variant="subtitle1" gutterBottom>
                About this module
              </Typography>
            )}
            {!moduleMetaLoaded && moduleName ? (
              <Box display="flex" justifyContent="center" py={4}>
                <CircularProgress size={32} />
              </Box>
            ) : !overviewInCluster ? (
              <Typography color="text.secondary">
                No overview is configured: add <code>overviewService</code> and{' '}
                <code>overviewServiceNamespace</code> for this module in the ModuleCatalog so Module
                Manager can fetch HTML from the in-cluster overview workload (see module Helm chart{' '}
                <code>portal/overview.html</code>).
              </Typography>
            ) : overviewLoading ? (
              <Box display="flex" justifyContent="center" py={4}>
                <CircularProgress size={32} />
              </Box>
            ) : overviewErr ? (
              <Alert severity="warning">{overviewErr}</Alert>
            ) : overviewSrcDoc ? (
              <Box
                component="iframe"
                title="Module overview"
                srcDoc={overviewSrcDoc}
                sandbox="allow-scripts"
                sx={{
                  width: '100%',
                  height: '100%',
                  flex: 1,
                  border: 0,
                  bgcolor: 'background.default',
                  alignSelf: 'stretch',
                }}
              />
            ) : null}
          </CardContent>
        </Card>
      )}
      {activeTab === 'applications' && hasApplications && (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', sm: 'repeat(auto-fill, minmax(260px, 1fr))' },
            gap: 2,
          }}
        >
          {moduleApplications.map((app) => {
            const href = resolveApplicationHref(app.url);
            const label = (app.title ?? app.id).trim() || app.id;
            return (
              <Card key={app.id} variant="outlined">
                <CardContent>
                  <Typography variant="subtitle1" gutterBottom>
                    {label}
                  </Typography>
                  <Button
                    component="a"
                    href={href}
                    target="_blank"
                    rel="noopener noreferrer"
                    variant="contained"
                    endIcon={<OpenInNewOutlinedIcon />}
                    fullWidth
                  >
                    Open
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </Box>
      )}
      {activeTab === 'templates' && hasTemplates && (
        <TemplatesTab entries={entries} onOpenSubmit={(e) => setSelTpl(e)} />
      )}
      {activeTab === 'running' && hasTemplates && (
        <RunningTab
          items={runList}
          onOpen={(n) => {
            setDetailName(n);
            setDetailOpen(true);
          }}
        />
      )}
      {activeTab === 'history' && hasTemplates && (
        <HistoryTab
          items={histList}
          deletingNames={deletingWorkflows}
          onOpen={(n) => {
            setDetailName(n);
            setDetailOpen(true);
          }}
        />
      )}

      {selTpl && (
        <SubmitDialog
          entry={selTpl}
          moduleName={moduleName}
          onClose={() => setSelTpl(null)}
          onSubmitted={() => {
            setSelTpl(null);
            setTab('running');
            void loadRunning();
          }}
        />
      )}

      {detailOpen && detailName && (
        <WorkflowDetailDialog
          workflowName={detailName}
          listDeletionPending={!!deletingWorkflows[detailName]}
          open={detailOpen}
          onClose={() => setDetailOpen(false)}
          onMutated={() => {
            void loadRunning();
            void loadHistory();
          }}
          onDeleteStarted={(n) => setDeletingWorkflows((p) => ({ ...p, [n]: true }))}
          onDeleteFinished={(n) =>
            setDeletingWorkflows((p) => {
              const q = { ...p };
              delete q[n];
              return q;
            })
          }
        />
      )}
    </Box>
  );
}

function TemplatesTab({
  entries,
  onOpenSubmit,
}: {
  entries: CatalogEntry[];
  onOpenSubmit: (e: CatalogEntry) => void;
}) {
  if (entries.length === 0) {
    return (
      <Typography color="text.secondary">
        No exposed workflow templates for this module (see Horizon catalog).
      </Typography>
    );
  }
  const sortedEntries = [...entries].sort((a, b) => a.templateName.localeCompare(b.templateName));

  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: 'repeat(2,1fr)', md: 'repeat(3,1fr)' },
        gap: 2,
      }}
    >
      {sortedEntries.map((e) => (
        <Card key={`${e.module}/${e.templateName}`} variant="outlined">
          <CardContent>
            <Typography variant="h6">{e.templateName}</Typography>
            <Typography variant="caption" color="text.secondary" display="block">
              {e.namespace}
            </Typography>
            <Button sx={{ mt: 2 }} variant="contained" onClick={() => onOpenSubmit(e)}>
              Configure &amp; submit
            </Button>
          </CardContent>
        </Card>
      ))}
    </Box>
  );
}

function parseIsoMs(iso: string | undefined): number {
  if (!iso?.trim()) {
    return NaN;
  }
  const t = Date.parse(iso);
  return Number.isFinite(t) ? t : NaN;
}

function formatLocaleDateTime(iso: string | undefined): string {
  if (!iso?.trim()) {
    return '—';
  }
  const d = new Date(iso);
  return Number.isFinite(d.getTime()) ? d.toLocaleString() : iso;
}

/** Human-readable duration; when `liveEnd` is true and `finished` is missing, uses `Date.now()` for running workflows. */
function formatDurationBetween(started?: string, finished?: string, liveEnd = false): string {
  const a = parseIsoMs(started);
  if (!Number.isFinite(a)) {
    return '—';
  }
  const b = parseIsoMs(finished);
  const end = Number.isFinite(b) ? b : liveEnd ? Date.now() : NaN;
  if (!Number.isFinite(end)) {
    return '—';
  }
  let sec = Math.max(0, Math.floor((end - a) / 1000));
  if (sec < 60) {
    return `${sec}s`;
  }
  const m = Math.floor(sec / 60);
  sec %= 60;
  if (m < 60) {
    return sec > 0 ? `${m}m ${sec}s` : `${m}m`;
  }
  const h = Math.floor(m / 60);
  const mm = m % 60;
  return mm > 0 ? `${h}h ${mm}m` : `${h}h`;
}

function sortWorkflowsForHistory(items: WorkflowSummary[]): WorkflowSummary[] {
  return [...items].sort((a, b) => {
    const fb = parseIsoMs(b.finishedAt);
    const fa = parseIsoMs(a.finishedAt);
    if (Number.isFinite(fb) && Number.isFinite(fa) && fb !== fa) {
      return fb - fa;
    }
    const sb = parseIsoMs(b.startedAt);
    const sa = parseIsoMs(a.startedAt);
    if (Number.isFinite(sb) && Number.isFinite(sa) && sb !== sa) {
      return sb - sa;
    }
    return a.name.localeCompare(b.name);
  });
}

/** Maps API label horizon-sdv.io/submitted-from to a short UI label. */
function formatSubmittedFromLabel(v: string | undefined): string {
  const raw = (v ?? '').trim();
  if (!raw) {
    return '—';
  }
  const s = raw.toLowerCase();
  if (s === 'developer-portal') {
    return 'Developer portal';
  }
  if (s === 'horizon-cli') {
    return 'Horizon CLI';
  }
  if (s === 'api') {
    return 'REST API';
  }
  return raw;
}

/** Keycloak username (Portal) or Argo creator label; empty when unknown. */
function formatStartedByLabel(v: string | undefined): string {
  const raw = (v ?? '').trim();
  return raw || '—';
}

function sortWorkflowsForRunning(items: WorkflowSummary[]): WorkflowSummary[] {
  return [...items].sort((a, b) => {
    const sb = parseIsoMs(b.startedAt);
    const sa = parseIsoMs(a.startedAt);
    if (Number.isFinite(sb) && Number.isFinite(sa) && sb !== sa) {
      return sb - sa;
    }
    return a.name.localeCompare(b.name);
  });
}

function runningPhaseChipColor(
  phase: string | undefined
): 'default' | 'primary' | 'secondary' | 'error' | 'warning' | 'info' | 'success' {
  const p = (phase ?? '').trim().toLowerCase();
  if (p === 'running') {
    return 'info';
  }
  if (p === 'pending') {
    return 'warning';
  }
  if (p === 'succeeded') {
    return 'success';
  }
  if (p === 'failed' || p === 'error' || p === 'aborted') {
    return 'error';
  }
  return 'primary';
}

function RunningTab({
  items,
  onOpen,
}: {
  items: WorkflowSummary[];
  onOpen: (name: string) => void;
}) {
  const rows = useMemo(() => sortWorkflowsForRunning(items), [items]);
  if (rows.length === 0) {
    return <Typography color="text.secondary">No running workflows for this module.</Typography>;
  }
  return (
    <TableContainer component={Paper} variant="outlined">
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Template</TableCell>
            <TableCell>Started</TableCell>
            <TableCell>Duration</TableCell>
            <TableCell>Triggered from</TableCell>
            <TableCell>Submitted by</TableCell>
            <TableCell>Phase</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((w) => (
            <TableRow
              key={w.name}
              hover
              sx={{ cursor: 'pointer' }}
              onClick={() => onOpen(w.name)}
              title={w.name}
            >
              <TableCell>
                <Typography fontWeight={500}>{w.workflowTemplate ?? '—'}</Typography>
              </TableCell>
              <TableCell>{formatLocaleDateTime(w.startedAt)}</TableCell>
              <TableCell>{formatDurationBetween(w.startedAt, w.finishedAt, true)}</TableCell>
              <TableCell>{formatSubmittedFromLabel(w.submittedFrom)}</TableCell>
              <TableCell title={formatStartedByLabel(w.startedBy)}>
                <Typography
                  variant="body2"
                  sx={{
                    fontFamily: 'monospace',
                    fontSize: 12,
                    maxWidth: 220,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {formatStartedByLabel(w.startedBy)}
                </Typography>
              </TableCell>
              <TableCell>
                <Chip
                  size="small"
                  label={w.phase?.trim() || '—'}
                  color={runningPhaseChipColor(w.phase)}
                  variant="outlined"
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function HistoryTab({
  items,
  onOpen,
  deletingNames,
}: {
  items: WorkflowSummary[];
  onOpen: (name: string) => void;
  deletingNames?: Record<string, boolean>;
}) {
  const rows = useMemo(() => sortWorkflowsForHistory(items), [items]);
  if (rows.length === 0) {
    return <Typography color="text.secondary">No history for this module yet.</Typography>;
  }
  return (
    <TableContainer component={Paper} variant="outlined">
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Template</TableCell>
            <TableCell>Started</TableCell>
            <TableCell>Finished</TableCell>
            <TableCell>Duration</TableCell>
            <TableCell>Triggered from</TableCell>
            <TableCell>Submitted by</TableCell>
            <TableCell>Result</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((w) => (
            <TableRow
              key={w.name}
              hover
              sx={{
                cursor: 'pointer',
                ...(deletingNames?.[w.name]
                  ? { bgcolor: 'action.selected', '&:hover': { bgcolor: 'action.selected' } }
                  : {}),
              }}
              onClick={() => onOpen(w.name)}
              title={`Instance: ${w.name}`}
            >
              <TableCell>
                <Typography fontWeight={500}>{w.workflowTemplate ?? '—'}</Typography>
              </TableCell>
              <TableCell>{formatLocaleDateTime(w.startedAt)}</TableCell>
              <TableCell>{formatLocaleDateTime(w.finishedAt)}</TableCell>
              <TableCell>{formatDurationBetween(w.startedAt, w.finishedAt, false)}</TableCell>
              <TableCell>{formatSubmittedFromLabel(w.submittedFrom)}</TableCell>
              <TableCell title={formatStartedByLabel(w.startedBy)}>
                <Typography
                  variant="body2"
                  sx={{
                    fontFamily: 'monospace',
                    fontSize: 12,
                    maxWidth: 220,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {formatStartedByLabel(w.startedBy)}
                </Typography>
              </TableCell>
              <TableCell>
                {deletingNames?.[w.name] ? (
                  <Chip
                    size="small"
                    label="Deletion in progress"
                    color="default"
                    variant="outlined"
                    disabled
                  />
                ) : (
                  <Chip
                    size="small"
                    label={w.phase?.trim() || '—'}
                    color={runningPhaseChipColor(w.phase)}
                    variant="outlined"
                  />
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function workflowParamInputProps(
  p: { name: string; description?: string },
  value: string,
  templateName: string
): { endAdornment?: React.ReactNode } | undefined {
  const adornments: React.ReactNode[] = [];
  if (p.name === 'largeUploadSizeMiB' && templateName === 'sample-smoke-test') {
    const gbLabel = formatMiBToDecimalGBAdornment(value);
    if (gbLabel) {
      adornments.push(
        <Typography
          key="gb"
          component="span"
          variant="body2"
          color="text.secondary"
          sx={{ whiteSpace: 'nowrap', mr: 0.5 }}
        >
          {gbLabel}
        </Typography>
      );
    }
  }
  if (p.description && p.description.length > 72) {
    adornments.push(
      <Tooltip
        key="help"
        title={p.description}
        placement="left"
        slotProps={{
          tooltip: { sx: { maxWidth: 320, whiteSpace: 'normal' } },
        }}
      >
        <IconButton size="small" edge="end" aria-label={`Help for ${p.name}`} tabIndex={-1}>
          <HelpOutlineIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    );
  }
  if (adornments.length === 0) {
    return undefined;
  }
  return {
    endAdornment: <InputAdornment position="end">{adornments}</InputAdornment>,
  };
}

function workflowParamHelperText(p: { description?: string }): string | undefined {
  const desc = p.description?.trim();
  return desc && desc.length <= 72 ? desc : undefined;
}

function SubmitDialog({
  entry,
  moduleName,
  onClose,
  onSubmitted,
}: {
  entry: CatalogEntry;
  moduleName: string;
  onClose: () => void;
  onSubmitted: () => void;
}) {
  const [params, setParams] = useState<Record<string, string>>(() => {
    const o: Record<string, string> = {};
    for (const p of entry.parameters) {
      o[p.name] = p.default ?? '';
    }
    return o;
  });
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const submit = async () => {
    setLoading(true);
    setErr(null);
    const ctrl = new AbortController();
    const tid = window.setTimeout(() => ctrl.abort(), 45000);
    try {
      const path = `/v1/modules/${encodeURIComponent(moduleName)}/workflowTemplates/${encodeURIComponent(entry.templateName)}/submit`;
      const submitParams = { ...params };
      const username = authService.getUsername();
      const hasSubmittedBy = entry.parameters.some((p) => p.name === 'submittedBy');
      if (hasSubmittedBy && username) {
        submitParams.submittedBy = username;
      }
      const submitHeaders: Record<string, string> = {
        'Content-Type': 'application/json',
        'X-Horizon-Submitted-From': 'developer-portal',
      };
      if (username) {
        submitHeaders['X-Horizon-Submitted-By'] = username;
      }
      const r = await apiHorizon(path, {
        method: 'POST',
        headers: submitHeaders,
        body: JSON.stringify({ parameters: submitParams }),
        signal: ctrl.signal,
      });
      if (!r.ok) {
        const t = await r.text();
        throw new Error(t || `submit ${r.status}`);
      }
      onSubmitted();
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === 'AbortError') {
        setErr(
          'Submit timed out waiting for Horizon API (check Argo Events webhook and cluster connectivity).'
        );
      } else {
        setErr(e instanceof Error ? e.message : 'submit failed');
      }
    } finally {
      window.clearTimeout(tid);
      setLoading(false);
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Submit: {entry.templateName}</DialogTitle>
      <DialogContent>
        {entry.parameters
          .filter((p) => p.name !== 'submittedBy')
          .map((p) => (
            <TextField
              key={p.name}
              margin="dense"
              label={p.name}
              helperText={workflowParamHelperText(p)}
              fullWidth
              required={!p.default}
              value={params[p.name] ?? ''}
              onChange={(e) => setParams((prev) => ({ ...prev, [p.name]: e.target.value }))}
              InputProps={workflowParamInputProps(p, params[p.name] ?? '', entry.templateName)}
            />
          ))}
        {err && (
          <Alert severity="error" sx={{ mt: 1 }}>
            {err}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={loading} onClick={() => void submit()}>
          {loading ? 'Submitting…' : 'SUBMIT'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** Mirrors Horizon API / workflow archive heuristics for log-type outputs. */
function isLogArtifactName(name: string): boolean {
  const n = name.toLowerCase().trim();
  /** Default Argo container / archived stdout artifact name (does not contain "log"). */
  if (n === 'main') {
    return true;
  }
  return n.includes('log') || n === 'main-logs' || n === 'mainlogs';
}

/**
 * Pick the log-type output artifact for this DAG row. Prefer exact node id; avoid `find` by templateName alone
 * when multiple steps share a template name across nested workflows.
 */
function pickLogArtifactForNode(
  logOutputs: OutputArtifact[],
  n: { id: string; displayName?: string; templateName?: string }
): OutputArtifact | undefined {
  const sameNode = logOutputs.filter((a) => a.nodeId === n.id);
  if (sameNode.length === 1) {
    return sameNode[0];
  }
  if (sameNode.length > 1) {
    const namedLogs = sameNode.filter((a) => isLogArtifactName(a.name));
    return namedLogs.length === 1 ? namedLogs[0] : sameNode[0];
  }
  const tpl = n.templateName?.trim();
  const disp = n.displayName?.trim();
  if (tpl && disp) {
    const td = logOutputs.filter((a) => a.templateName === tpl && a.displayName === disp);
    if (td.length === 1) {
      return td[0];
    }
  }
  if (tpl) {
    const tOnly = logOutputs.filter((a) => a.templateName === tpl);
    if (tOnly.length === 1) {
      return tOnly[0];
    }
  }
  if (disp) {
    const dOnly = logOutputs.filter((a) => a.displayName === disp);
    if (dOnly.length === 1) {
      return dOnly[0];
    }
  }
  return undefined;
}

/** Resolve archived GCS URI and Horizon download artifact for a workflow node. */
function nodeArchivedLogLinks(
  n: { id: string; displayName?: string; templateName?: string },
  archived: WorkflowSummary['archivedLogs'] | undefined,
  logOutputs: OutputArtifact[]
): { gcsUri?: string; download?: OutputArtifact; artifactName?: string } {
  let gcsUri: string | undefined;
  let artifactName: string | undefined;

  const step = archived?.steps?.find((s) => {
    if (!s.gcsUri) {
      return false;
    }
    if (s.nodeId === n.id) {
      return true;
    }
    const tpl = n.templateName?.trim();
    const disp = n.displayName?.trim();
    if (tpl && disp && s.templateName === tpl && s.displayName === disp) {
      return true;
    }
    if (!s.nodeId && disp && s.displayName === disp) {
      return true;
    }
    return false;
  });
  if (step?.gcsUri) {
    gcsUri = step.gcsUri;
  }
  if (step?.artifactName) {
    artifactName = step.artifactName;
  }

  const logArt = pickLogArtifactForNode(logOutputs, n);

  if (!gcsUri && logArt?.gcsUri) {
    gcsUri = logArt.gcsUri;
  }

  return {
    gcsUri,
    download: logArt,
    artifactName,
  };
}

/** Site origin for opening cluster UIs; strips a trailing `/workflows` if PUBLIC_BASE_URL mistakenly includes the Argo UI mount. */
function argoBrowserBaseUrl(): string {
  let base = config
    .getString('baseUrl', typeof window !== 'undefined' ? window.location.origin : '')
    .replace(/\/$/, '');
  while (/\/workflows$/i.test(base)) {
    base = base.replace(/\/workflows$/i, '').replace(/\/$/, '');
  }
  return base;
}

/**
 * Public Argo Workflows UI path (gateway serves UI under /workflows on the cluster domain).
 * -> Second /workflows => Required for Argo's internal router
 * */
function argoWorkflowUiUrl(workflowName: string, namespace: string): string {
  const base = argoBrowserBaseUrl();
  return `${base}/workflows/workflows/${encodeURIComponent(namespace)}/${encodeURIComponent(workflowName)}`;
}

/** Poll workflow GET so phase, DAG nodes, and archived log links stay current; slower interval when terminal (TTL detection). */
const WORKFLOW_DETAIL_POLL_MS = 4000;

function WorkflowDetailDialog({
  workflowName,
  listDeletionPending,
  open,
  onClose,
  onMutated,
  onDeleteStarted,
  onDeleteFinished,
}: {
  workflowName: string;
  /** Parent still has this workflow in deleting set (blocking DELETE may continue after dialog closes). */
  listDeletionPending: boolean;
  open: boolean;
  onClose: () => void;
  /** Refresh running/history tables after delete or abort. */
  onMutated?: () => void;
  /** Called after the user confirms delete, before the blocking DELETE request. */
  onDeleteStarted?: (name: string) => void;
  /** Always called when the delete attempt finishes (success, HTTP error, or thrown). */
  onDeleteFinished?: (name: string) => void;
}) {
  const [detail, setDetail] = useState<WorkflowDetail | null>(null);
  const [err, setErr] = useState<string | null>(null);
  /** Abort request in flight only — does not block closing the dialog during a long DELETE. */
  const [abortBusy, setAbortBusy] = useState(false);
  /** This dialog instance is awaiting the blocking DELETE response. */
  const [deleteBusy, setDeleteBusy] = useState(false);
  const deletionPendingUi = deleteBusy || listDeletionPending;
  /** null = unknown; true = Workflow CR still in cluster; false = deleted/TTL (Argo UI has nothing to open). */
  const [argoReachable, setArgoReachable] = useState<boolean | null>(null);
  /** `null` = closed; `{}` = whole workflow; otherwise pod logs or archived step log fetch. */
  const [logDialog, setLogDialog] = useState<null | {
    podName?: string;
    stageLabel?: string;
    /** When pods are gone (e.g. podGC), load main archived log via signed GCS URL. */
    archivedArtifact?: { artifactName: string; nodeId: string; templateName?: string };
  }>(null);

  const dlgLayout = useResizableDialogSize({
    storageKey: DIALOG_LAYOUT_COOKIE_WORKFLOW_DETAIL,
    defaultWidth: () =>
      typeof window !== 'undefined' ? Math.min(1200, Math.floor(window.innerWidth * 0.96)) : 1200,
    defaultHeight: () =>
      typeof window !== 'undefined' ? Math.min(720, Math.floor(window.innerHeight * 0.88)) : 720,
    minWidth: 720,
    minHeight: 400,
  });

  useEffect(() => {
    if (!open) {
      return;
    }
    let cancelled = false;
    setDetail(null);
    setErr(null);
    setArgoReachable(null);
    (async () => {
      try {
        const r = await apiHorizon(`/v1/workflows/${encodeURIComponent(workflowName)}`);
        if (!r.ok) {
          if (r.status === 404) {
            setArgoReachable(false);
          }
          throw new Error(`get workflow ${r.status}`);
        }
        if (cancelled) {
          return;
        }
        setArgoReachable(true);
        setDetail((await r.json()) as WorkflowDetail);
        setErr(null);
      } catch (e: unknown) {
        if (!cancelled) {
          setErr(e instanceof Error ? e.message : 'load');
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, workflowName]);

  useEffect(() => {
    if (!open || !detail) {
      return;
    }
    const terminal = isTerminalWorkflowPhase(detail.phase);
    const period = terminal ? 20000 : WORKFLOW_DETAIL_POLL_MS;

    const tick = async () => {
      try {
        const r = await apiHorizon(`/v1/workflows/${encodeURIComponent(workflowName)}`);
        if (!r.ok) {
          if (r.status === 404) {
            setArgoReachable(false);
          }
          return;
        }
        setArgoReachable(true);
        const next = (await r.json()) as WorkflowDetail;
        setDetail(next);
        setErr(null);
      } catch {
        /* keep last successful detail on transient errors */
      }
    };

    const id = window.setInterval(() => {
      void tick();
    }, period);

    return () => {
      clearInterval(id);
    };
  }, [open, workflowName, detail?.phase]);

  const argoUrl =
    detail?.namespace && argoReachable !== false
      ? argoWorkflowUiUrl(workflowName, detail.namespace)
      : undefined;

  const terminal = detail ? isTerminalWorkflowPhase(detail.phase) : false;
  const detailTitle = detail
    ? (() => {
        const root = detail.workflowTemplate?.trim() || 'Workflow';
        const deps = (detail.dependentWorkflowTemplates || [])
          .map((v) => v.template.trim())
          .filter((v) => v.length > 0);
        if (deps.length === 0) {
          return root;
        }
        return `${root} (${deps.join(', ')})`;
      })()
    : 'Workflow';

  const doAbort = async () => {
    if (
      !window.confirm(
        `Abort workflow "${workflowName}"? The run will stop gracefully (shutdown Stop) and may show as Aborted when finished.`
      )
    ) {
      return;
    }
    setAbortBusy(true);
    setErr(null);
    try {
      const r = await apiHorizon(`/v1/workflows/${encodeURIComponent(workflowName)}/abort`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
      if (!r.ok) {
        const t = await r.text();
        throw new Error(t || `abort ${r.status}`);
      }
      onMutated?.();
      const rr = await apiHorizon(`/v1/workflows/${encodeURIComponent(workflowName)}`);
      if (rr.ok) {
        setDetail((await rr.json()) as WorkflowDetail);
      }
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'abort failed');
    } finally {
      setAbortBusy(false);
    }
  };

  const doDelete = async () => {
    if (
      !window.confirm(
        `Permanently delete workflow "${workflowName}" from the cluster? This cannot be undone.`
      )
    ) {
      return;
    }
    onDeleteStarted?.(workflowName);
    setDeleteBusy(true);
    setErr(null);
    try {
      const r = await apiHorizon(`/v1/workflows/${encodeURIComponent(workflowName)}`, {
        method: 'DELETE',
      });
      if (!r.ok) {
        const t = await r.text();
        throw new Error(t || `delete ${r.status}`);
      }
      onMutated?.();
      onClose();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'delete failed');
    } finally {
      setDeleteBusy(false);
      onDeleteFinished?.(workflowName);
    }
  };

  const handleDialogClose = () => {
    if (abortBusy) {
      return;
    }
    onClose();
  };

  return (
    <>
      <Dialog
        open={open}
        onClose={handleDialogClose}
        disableEscapeKeyDown={abortBusy}
        maxWidth={false}
        fullWidth={false}
        scroll="paper"
        PaperProps={{ sx: dlgLayout.paperSx }}
      >
        <DialogTitle>
          <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1}>
            <Stack spacing={0.75} sx={{ flex: 1, minWidth: 0 }}>
              {detail?.namespace ? (
                <Tooltip
                  title={
                    argoReachable === false
                      ? 'This workflow is no longer in the cluster (TTL or delete). It will not appear in Argo Workflows.'
                      : 'Open workflow in Argo Workflows UI'
                  }
                >
                  <span style={{ alignSelf: 'flex-start' }}>
                    <Button
                      size="small"
                      variant="text"
                      startIcon={<OpenInNewOutlinedIcon sx={{ fontSize: 18 }} />}
                      href={argoUrl}
                      component={argoReachable === false ? 'button' : 'a'}
                      target={argoReachable === false ? undefined : '_blank'}
                      rel={argoReachable === false ? undefined : 'noopener noreferrer'}
                      disabled={argoReachable === false}
                      sx={{
                        textTransform: 'none',
                        color: argoReachable === false ? 'text.disabled' : undefined,
                      }}
                    >
                      Open in Argo Workflows
                    </Button>
                  </span>
                </Tooltip>
              ) : null}
              <Typography variant="h6" component="div">
                {detailTitle}
              </Typography>
              {detail ? (
                <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                  {detail.module ? (
                    <Chip
                      size="small"
                      variant="outlined"
                      color="primary"
                      label={`MOD: ${detail.module}`}
                    />
                  ) : null}
                  {detail.workflowTemplate ? (
                    <Chip
                      size="small"
                      variant="outlined"
                      label={`WT: ${detail.workflowTemplate}`}
                    />
                  ) : null}
                  {(detail.dependentWorkflowTemplates || []).map((dep) => (
                    <Stack
                      key={`${dep.module || 'unknown'}:${dep.template}`}
                      direction="row"
                      spacing={0.5}
                      useFlexGap
                    >
                      {dep.module ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color="primary"
                          label={`MOD: ${dep.module}`}
                        />
                      ) : null}
                      <Chip size="small" variant="outlined" label={`WT: ${dep.template}`} />
                    </Stack>
                  ))}
                </Stack>
              ) : null}
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{ fontFamily: 'monospace', wordBreak: 'break-all' }}
              >
                {workflowName}
              </Typography>
              {deletionPendingUi ? (
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                  Deletion in progress (waiting for cluster and artifact cleanup)…
                </Typography>
              ) : null}
            </Stack>
            {detail && (
              <Stack direction="row" spacing={0.5} sx={{ flexShrink: 0 }}>
                {!terminal && (
                  <Tooltip title="Request graceful stop (Horizon API abort)">
                    <span>
                      <Button
                        size="small"
                        color="warning"
                        variant="outlined"
                        startIcon={<StopOutlinedIcon />}
                        disabled={abortBusy || deleteBusy || listDeletionPending}
                        onClick={() => void doAbort()}
                      >
                        Abort
                      </Button>
                    </span>
                  </Tooltip>
                )}
                {terminal && (
                  <Tooltip title="Remove workflow CR from cluster (terminal workflows only)">
                    <span>
                      <Button
                        size="small"
                        color={deletionPendingUi ? 'inherit' : 'error'}
                        variant="outlined"
                        startIcon={
                          deletionPendingUi ? (
                            <CircularProgress color="inherit" size={14} sx={{ ml: 0.25 }} />
                          ) : (
                            <DeleteOutlineIcon />
                          )
                        }
                        disabled={abortBusy || deleteBusy || listDeletionPending}
                        sx={
                          deletionPendingUi
                            ? { color: 'text.disabled', borderColor: 'action.disabled' }
                            : undefined
                        }
                        onClick={() => void doDelete()}
                      >
                        Delete
                      </Button>
                    </span>
                  </Tooltip>
                )}
              </Stack>
            )}
          </Stack>
        </DialogTitle>
        <DialogContent dividers sx={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
          {err && (
            <Alert severity="error" sx={{ mb: 1 }}>
              {err}
            </Alert>
          )}
          {!detail && !err && (
            <Box display="flex" justifyContent="center" p={2}>
              <CircularProgress size={28} />
            </Box>
          )}
          {detail && (
            <WorkflowDetailSections
              wf={workflowName}
              detail={detail}
              onOpenClusterLogs={() => setLogDialog({})}
              onShowNodeLogs={(opts) => setLogDialog(opts)}
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDialogClose} disabled={abortBusy}>
            Close
          </Button>
        </DialogActions>
        {dlgLayout.ResizeHandle}
      </Dialog>
      {logDialog !== null && (
        <LogStreamDialog
          workflowName={workflowName}
          phase={detail?.phase}
          archivedLogs={detail?.archivedLogs}
          podName={logDialog.podName}
          stageLabel={logDialog.stageLabel}
          archivedArtifact={logDialog.archivedArtifact}
          onClose={() => setLogDialog(null)}
        />
      )}
    </>
  );
}

function WorkflowDetailSections({
  wf,
  detail,
  onOpenClusterLogs,
  onShowNodeLogs,
}: {
  wf: string;
  detail: WorkflowDetail;
  onOpenClusterLogs: () => void;
  onShowNodeLogs: (opts: {
    podName?: string;
    stageLabel: string;
    archivedArtifact?: { artifactName: string; nodeId: string; templateName?: string };
  }) => void;
}) {
  const arts = detail.outputArtifacts || [];
  const fileArtifacts = arts.filter((a) => !isLogArtifactName(a.name));
  const logOutputArtifacts = arts.filter((a) => isLogArtifactName(a.name));
  const archived = detail.archivedLogs;
  const terminal = isTerminalWorkflowPhase(detail.phase);
  // Pod rows only for large DAGs; sort by startedAt + id so polling phase updates do not reshuffle rows.
  const dagNodesForLogs = useMemo(() => {
    const all = detail.nodes || [];
    const pods = all.filter((n) => n.type === 'Pod');
    const rows = pods.length > 0 ? pods : all;
    return [...rows].sort((a, b) => {
      const ta = parseIsoMs(a.startedAt);
      const tb = parseIsoMs(b.startedAt);
      if (Number.isFinite(ta) && Number.isFinite(tb) && ta !== tb) {
        return ta - tb;
      }
      if (Number.isFinite(ta) !== Number.isFinite(tb)) {
        return Number.isFinite(ta) ? -1 : 1;
      }
      return (a.id || '').localeCompare(b.id || '');
    });
  }, [detail.nodes]);

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
        <Chip size="small" label={detail.phase} color="primary" variant="outlined" />
        <Typography variant="body2" color="text.secondary" component="span">
          {detail.workflowTemplate ?? '—'}
        </Typography>
      </Stack>

      <Typography variant="body2" color="text.secondary">
        Triggered from: <strong>{formatSubmittedFromLabel(detail.submittedFrom)}</strong>
        {' · '}
        Submitted by:{' '}
        <Box
          component="code"
          title={formatStartedByLabel(detail.startedBy)}
          sx={{ fontSize: 12, wordBreak: 'break-all' }}
        >
          {formatStartedByLabel(detail.startedBy)}
        </Box>
      </Typography>

      <Card variant="outlined" sx={{ bgcolor: 'action.hover' }}>
        <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
          <Typography variant="body2" color="text.secondary">
            Horizon API OAuth client:{' '}
            <Box component="code" sx={{ fontSize: 12 }}>
              {config.getString('horizonApiOAuthClientId', 'horizon-api-ci')}
            </Box>
          </Typography>
        </CardContent>
      </Card>

      <Card variant="outlined">
        <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
            <Inventory2OutlinedIcon fontSize="small" color="action" />
            <Typography variant="subtitle1">Artifacts (files)</Typography>
          </Stack>
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
            Non-log outputs from all steps — GCS location and signed download.
          </Typography>
          {fileArtifacts.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              None
            </Typography>
          ) : (
            <Stack spacing={1}>
              {fileArtifacts.map((a) => (
                <Stack
                  key={`${a.nodeId}-${a.name}-${a.templateName}`}
                  direction="row"
                  alignItems="center"
                  spacing={1}
                  flexWrap="wrap"
                  useFlexGap
                >
                  <Typography variant="body2" fontWeight={500} component="span">
                    {a.fileName ?? a.name}
                  </Typography>
                  {(a.displayName || a.templateName) && (
                    <Chip size="small" variant="outlined" label={a.displayName || a.templateName} />
                  )}
                  {a.module && (
                    <Chip
                      size="small"
                      variant="outlined"
                      label={`MOD: ${a.module}`}
                      color="primary"
                    />
                  )}
                  {a.workflowTemplate && (
                    <Chip
                      size="small"
                      variant="outlined"
                      label={`WT: ${a.workflowTemplate}`}
                      color="primary"
                    />
                  )}
                  {a.gcsUri && (
                    <Tooltip title="Open GCS URI">
                      <Button
                        size="small"
                        startIcon={<CloudOutlinedIcon />}
                        href={a.gcsUri}
                        target="_blank"
                        rel="noreferrer"
                      >
                        GCS
                      </Button>
                    </Tooltip>
                  )}
                  <Button
                    size="small"
                    variant="contained"
                    disableElevation
                    startIcon={<DownloadOutlinedIcon />}
                    onClick={() => void downloadArtifact(wf, a)}
                  >
                    Download
                  </Button>
                </Stack>
              ))}
            </Stack>
          )}
        </CardContent>
      </Card>

      <Card variant="outlined">
        <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
            <TerminalOutlinedIcon fontSize="small" color="action" />
            <Typography variant="subtitle1">Cluster log stream</Typography>
          </Stack>
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
            Live NDJSON from Horizon (all stages; tagged lines). Distinct from GCS log archives in
            the DAG nodes table.
          </Typography>
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Button
              variant="contained"
              startIcon={<TerminalOutlinedIcon />}
              onClick={onOpenClusterLogs}
            >
              {terminal ? 'Open stream' : 'Open live stream'}
            </Button>
            {terminal && <Chip size="small" label="Workflow finished" />}
          </Stack>
        </CardContent>
      </Card>

      <Box>
        <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
          DAG nodes
        </Typography>
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
          Show uses the live cluster log stream while the workflow is running. For finished
          workflows, Show loads archived main logs from GCS in the same dialog when available. GCS /
          Download are the same archive objects (not file/build artifacts). Module and Template list
          per-node fields from the API only — no workflow-level fallback — so blanks mean the
          backend did not set them for that row.
        </Typography>
        <TableContainer component={Paper} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Module</TableCell>
                <TableCell>Template</TableCell>
                <TableCell>Stage</TableCell>
                <TableCell>Phase</TableCell>
                <TableCell>Pod</TableCell>
                <TableCell align="center">Show</TableCell>
                <TableCell align="center">GCS</TableCell>
                <TableCell align="center">Download</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {dagNodesForLogs.map((n) => {
                const { gcsUri, download, artifactName } = nodeArchivedLogLinks(
                  n,
                  archived,
                  logOutputArtifacts
                );
                const stageLabel = n.displayName || n.id;
                const archivedArtifactResolved = download
                  ? {
                      artifactName: download.name,
                      nodeId: download.nodeId ?? n.id,
                      templateName: download.templateName ?? n.templateName,
                    }
                  : artifactName
                    ? { artifactName, nodeId: n.id, templateName: n.templateName }
                    : undefined;
                const archivedArtifactWhenNoPod =
                  !n.podName && archivedArtifactResolved ? archivedArtifactResolved : undefined;
                const canShow = Boolean(n.podName) || Boolean(archivedArtifactResolved);
                const showTitle =
                  terminal && archivedArtifactResolved
                    ? 'Archived main container log from GCS (finished workflow)'
                    : n.podName
                      ? 'Cluster logs for this pod (Horizon NDJSON stream)'
                      : archivedArtifactResolved
                        ? 'Archived main container log from GCS (pod may have been removed by podGC)'
                        : 'No pod yet — logs open when the step schedules a pod, or when archived logs appear in status';
                return (
                  <TableRow key={n.id}>
                    <TableCell>{n.module?.trim() || '—'}</TableCell>
                    <TableCell>{n.workflowTemplate?.trim() || '—'}</TableCell>
                    <TableCell>{stageLabel}</TableCell>
                    <TableCell>{n.phase ?? '—'}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: 12 }}>
                      {n.podName ?? '—'}
                    </TableCell>
                    <TableCell align="center">
                      {canShow ? (
                        <Tooltip title={showTitle}>
                          <Button
                            size="small"
                            variant="outlined"
                            onClick={() =>
                              terminal && archivedArtifactResolved
                                ? onShowNodeLogs({
                                    stageLabel,
                                    archivedArtifact: archivedArtifactResolved,
                                  })
                                : onShowNodeLogs({
                                    podName: n.podName || undefined,
                                    stageLabel,
                                    archivedArtifact: archivedArtifactWhenNoPod,
                                  })
                            }
                          >
                            Show
                          </Button>
                        </Tooltip>
                      ) : (
                        <Tooltip title={showTitle}>
                          <span>
                            <Button size="small" variant="outlined" disabled>
                              Show
                            </Button>
                          </span>
                        </Tooltip>
                      )}
                    </TableCell>
                    <TableCell align="center">
                      {gcsUri ? (
                        <Tooltip title="Open in GCS">
                          <Button
                            size="small"
                            startIcon={<CloudOutlinedIcon />}
                            href={gcsUri}
                            target="_blank"
                            rel="noreferrer"
                          >
                            GCS
                          </Button>
                        </Tooltip>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                    <TableCell align="center">
                      {download ? (
                        <Button
                          size="small"
                          variant="outlined"
                          startIcon={<DownloadOutlinedIcon />}
                          onClick={() => void downloadArtifact(wf, download)}
                        >
                          Download
                        </Button>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
    </Stack>
  );
}

async function downloadArtifact(wf: string, a: OutputArtifact) {
  const q = new URLSearchParams();
  if (a.nodeId) {
    q.set('nodeId', a.nodeId);
  }
  if (a.templateName) {
    q.set('templateName', a.templateName);
  }
  const path = `/v1/workflows/${encodeURIComponent(wf)}/downloadArtifact/${encodeURIComponent(a.name)}?${q.toString()}`;
  const r = await apiHorizon(path);
  if (r.status === 409) {
    alert('Ambiguous artifact name — pick node/template in Horizon API.');
    return;
  }
  if (!r.ok) {
    const t = await r.text();
    alert(t || `download ${r.status}`);
    return;
  }
  const j = (await r.json()) as { url?: string };
  if (j.url) {
    window.open(j.url, '_blank', 'noopener,noreferrer');
  }
}

/** Matches Horizon API terminal phases (finished workflows — History tab). */
function isTerminalWorkflowPhase(phase: string | undefined): boolean {
  if (!phase) {
    return false;
  }
  const p = phase.trim().toLowerCase();
  return ['succeeded', 'failed', 'error', 'aborted'].includes(p);
}

function hasArchivedLogLinks(archived?: WorkflowSummary['archivedLogs']): boolean {
  if (!archived) {
    return false;
  }
  return (archived.steps || []).some((s) => Boolean(s.gcsUri));
}

/** Parsed `[stage]` prefix from our formatted log lines; empty if unlabeled. */
function stageLabelFromDisplayLine(line: string): string {
  const m = line.match(/^\[([^\]]+)\]\s+/);
  return m ? m[1].trim() : '';
}

/**
 * When over the cap, repeatedly remove the chronologically oldest line from the
 * stage that has the most lines (tie: most total characters). That way a noisy
 * "build" step sheds its own lines first instead of wiping out short earlier stages.
 */
function trimBySheddingBusiestStage(lines: string[], max: number): string[] {
  if (lines.length <= max) {
    return lines;
  }
  const out = lines.slice();
  while (out.length > max) {
    const counts = new Map<string, number>();
    for (const ln of out) {
      const s = stageLabelFromDisplayLine(ln) || '_';
      counts.set(s, (counts.get(s) ?? 0) + 1);
    }
    let maxN = -1;
    for (const n of counts.values()) {
      if (n > maxN) {
        maxN = n;
      }
    }
    const tied = [...counts.entries()].filter(([, n]) => n === maxN).map(([s]) => s);
    let victimStage = tied[0] ?? '';
    if (tied.length > 1) {
      let bestChars = -1;
      for (const s of tied) {
        let chars = 0;
        for (const ln of out) {
          if ((stageLabelFromDisplayLine(ln) || '_') === s) {
            chars += ln.length;
          }
        }
        if (chars > bestChars) {
          bestChars = chars;
          victimStage = s;
        }
      }
    }
    const idx = out.findIndex((ln) => (stageLabelFromDisplayLine(ln) || '_') === victimStage);
    if (idx < 0) {
      out.shift();
      continue;
    }
    out.splice(idx, 1);
  }
  return out;
}

function appendWorkflowLogLine(prev: string[], line: string, maxLines: number): string[] {
  return trimBySheddingBusiestStage([...prev, line], maxLines);
}

function buildWorkflowLogUrl(workflowName: string, follow: boolean, podName?: string): string {
  const q = new URLSearchParams();
  q.set('follow', follow ? 'true' : 'false');
  if (podName) {
    q.set('podName', podName);
  }
  return `/v1/workflows/${encodeURIComponent(workflowName)}/log?${q.toString()}`;
}

/**
 * Loads archived artifact text via Horizon (`inline=1`), which proxies GCS server-side — avoids browser CORS on
 * direct `storage.googleapis.com` signed URLs.
 */
async function horizonArtifactInlineText(
  wf: string,
  sel: { artifactName: string; nodeId: string; templateName?: string },
  signal?: AbortSignal
): Promise<string> {
  const q = new URLSearchParams();
  q.set('inline', '1');
  if (sel.nodeId) {
    q.set('nodeId', sel.nodeId);
  }
  if (sel.templateName) {
    q.set('templateName', sel.templateName);
  }
  const path = `/v1/workflows/${encodeURIComponent(wf)}/downloadArtifact/${encodeURIComponent(sel.artifactName)}?${q.toString()}`;
  const r = await apiHorizon(path, { signal });
  if (r.status === 409) {
    throw new Error('Ambiguous artifact name — add templateName or use Download in the table.');
  }
  if (!r.ok) {
    const t = await r.text();
    throw new Error(t || `artifact ${r.status}`);
  }
  return r.text();
}

function LogStreamDialog({
  workflowName,
  phase,
  archivedLogs,
  podName,
  stageLabel,
  archivedArtifact,
  onClose,
}: {
  workflowName: string;
  phase?: string;
  archivedLogs?: WorkflowSummary['archivedLogs'];
  /** When set, stream only this pod’s logs (Horizon `podName` query). */
  podName?: string;
  /** Shown under the title for context (stage / node label). */
  stageLabel?: string;
  /** When pods are gone, fetch archived main log text from GCS via Horizon-signed URL. */
  archivedArtifact?: { artifactName: string; nodeId: string; templateName?: string };
  onClose: () => void;
}) {
  const [lines, setLines] = useState<string[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [emptyHint, setEmptyHint] = useState(false);
  const [fromArchive, setFromArchive] = useState(false);
  const maxLines = useMaxWorkflowLogLines();
  const maxLinesRef = useRef(maxLines);
  maxLinesRef.current = maxLines;

  const logDlgLayout = useResizableDialogSize({
    storageKey: DIALOG_LAYOUT_COOKIE_WORKFLOW_LOGS,
    defaultWidth: () =>
      typeof window !== 'undefined' ? Math.min(1280, Math.floor(window.innerWidth * 0.96)) : 1280,
    defaultHeight: () =>
      typeof window !== 'undefined' ? Math.min(820, Math.floor(window.innerHeight * 0.9)) : 820,
    minWidth: 720,
    minHeight: 400,
  });

  const terminal = isTerminalWorkflowPhase(phase);

  useEffect(() => {
    const ac = new AbortController();
    let cancelled = false;
    setLines([]);
    setErr(null);
    setEmptyHint(false);
    setFromArchive(false);

    if (!podName && archivedArtifact) {
      (async () => {
        try {
          const text = await horizonArtifactInlineText(workflowName, archivedArtifact, ac.signal);
          if (cancelled) {
            return;
          }
          const raw = text.split('\n');
          const cap = maxLinesRef.current;
          setLines(raw.length > cap ? raw.slice(-cap) : raw);
          setFromArchive(true);
          if (!text.trim()) {
            setEmptyHint(true);
          }
        } catch (e: unknown) {
          if ((e as Error).name === 'AbortError') {
            return;
          }
          setErr(e instanceof Error ? e.message : 'archived log load');
        }
      })();
      return () => {
        cancelled = true;
        ac.abort();
      };
    }

    void (async () => {
      let sawLogLine = false;
      let reportedUpstream = false;
      try {
        const url = buildWorkflowLogUrl(workflowName, !terminal, podName);
        const r = await apiHorizon(url, { signal: ac.signal });
        if (!r.ok) {
          throw new Error(`logs ${r.status}`);
        }
        const reader = r.body?.getReader();
        if (!reader) {
          throw new Error('no stream');
        }
        const dec = new TextDecoder();
        let buf = '';
        while (!cancelled) {
          const { done, value } = await reader.read();
          if (done) {
            break;
          }
          buf += dec.decode(value, { stream: true });
          const parts = buf.split('\n');
          buf = parts.pop() ?? '';
          for (const line of parts) {
            const t = line.trim();
            if (!t) {
              continue;
            }
            try {
              const o = JSON.parse(t) as {
                heartbeat?: boolean;
                result?: string;
                reason?: string;
                detail?: string;
                msg?: string;
                line?: string;
                message?: string;
                displayName?: string;
                templateName?: string;
                podName?: string;
                nodeId?: string;
              };
              if (o.heartbeat) {
                continue;
              }
              if (o.result === 'done') {
                if (o.reason === 'upstream_error' && o.detail) {
                  reportedUpstream = true;
                  setErr(o.detail);
                }
                continue;
              }
              const label =
                o.displayName?.trim() ||
                o.templateName?.trim() ||
                o.podName?.trim() ||
                o.nodeId?.trim() ||
                '';
              const body = o.msg ?? o.line ?? o.message ?? t;
              const text = label ? `[${label}] ${body}` : body;
              sawLogLine = true;
              setLines((prev) => appendWorkflowLogLine(prev, text, maxLinesRef.current));
            } catch {
              sawLogLine = true;
              setLines((prev) => appendWorkflowLogLine(prev, t, maxLinesRef.current));
            }
          }
        }
        if (!cancelled && terminal && !sawLogLine && !reportedUpstream) {
          setEmptyHint(true);
        }
      } catch (e: unknown) {
        if ((e as Error).name === 'AbortError') {
          return;
        }
        setErr(e instanceof Error ? e.message : 'log stream');
      }
    })();
    return () => {
      cancelled = true;
      ac.abort();
    };
  }, [
    workflowName,
    phase,
    podName,
    archivedArtifact?.artifactName,
    archivedArtifact?.nodeId,
    archivedArtifact?.templateName,
  ]);

  return (
    <Dialog
      open
      onClose={onClose}
      maxWidth={false}
      fullWidth={false}
      PaperProps={{ sx: logDlgLayout.paperSx }}
    >
      <DialogTitle>
        Logs — {workflowName}
        {podName ? (
          <Typography
            component="div"
            variant="body2"
            color="text.secondary"
            sx={{ mt: 0.5, fontWeight: 400 }}
          >
            {stageLabel ? `${stageLabel} · ` : ''}
            <Box component="span" sx={{ fontFamily: 'monospace', fontSize: 13 }}>
              {podName}
            </Box>
          </Typography>
        ) : archivedArtifact ? (
          <Typography
            component="div"
            variant="body2"
            color="text.secondary"
            sx={{ mt: 0.5, fontWeight: 400 }}
          >
            {stageLabel ? `${stageLabel} · ` : ''}
            archived ({archivedArtifact.artifactName})
          </Typography>
        ) : null}
      </DialogTitle>
      <DialogContent
        sx={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' }}
      >
        {terminal && !fromArchive && (
          <Alert severity="info" sx={{ mb: 1 }}>
            Finished workflows often have no live cluster logs (pods removed). Prefer{' '}
            <strong>Archived log links</strong> (GCS / Download in the DAG nodes table) on the
            workflow details panel when available.
          </Alert>
        )}
        {fromArchive && (
          <Alert severity="info" sx={{ mb: 1 }}>
            Loaded from Argo archived main container log in GCS (step pod may have been removed
            after the step finished).
          </Alert>
        )}
        {err && <Alert severity="error">{err}</Alert>}
        {emptyHint && !err && (
          <Alert severity="warning" sx={{ mb: 1 }}>
            No log lines were returned from the cluster stream.
            {hasArchivedLogLinks(archivedLogs)
              ? ' Open the previous dialog and use GCS or Download in the DAG nodes table.'
              : ' If logging was configured to archive, check the workflow in Argo or GCS.'}
          </Alert>
        )}
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 0.5 }}>
          Buffer up to {maxLines.toLocaleString()} lines (configure under Administration →
          Settings). When full, lines drop from the busiest step first so short stages (e.g. checks)
          are kept longer than a very chatty build.
        </Typography>
        <Paper
          variant="outlined"
          sx={{
            p: 1,
            flex: 1,
            minHeight: 120,
            overflow: 'auto',
            fontFamily: 'monospace',
            fontSize: 12,
            whiteSpace: 'pre-wrap',
          }}
        >
          {lines.length > 0 ? lines.join('\n') : emptyHint || err ? '' : 'Loading…'}
        </Paper>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
      {logDlgLayout.ResizeHandle}
    </Dialog>
  );
}
