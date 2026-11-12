// Copyright (c) 2026 Accenture, All Rights Reserved.
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
import type { CatalogEntry, OutputArtifact, WorkflowSummary } from '../types';
import { apiHorizon } from '../utils/api';
import { config } from '../utils/config';

export type TabKey = 'overview' | 'applications' | 'templates' | 'running' | 'history';

const TAB_KEYS: TabKey[] = ['overview', 'applications', 'templates', 'running', 'history'];

const PREDEFINED_TEMPLATE_MARKER = '# predefined-template';

export function parseTabKey(raw: string | null): TabKey {
  if (!raw || !TAB_KEYS.includes(raw as TabKey)) {
    return 'overview';
  }
  return raw as TabKey;
}

/** Must match a rendered Tab: MUI Tabs breaks when the value has no matching Tab. */
export function resolveActiveTab(
  tabFromUrl: TabKey,
  state: { moduleMetaLoaded: boolean; hasApplications: boolean; hasTemplates: boolean }
): TabKey {
  if (tabFromUrl === 'overview') {
    return 'overview';
  }
  if (tabFromUrl === 'applications') {
    if (!state.moduleMetaLoaded) {
      return 'overview';
    }
    if (state.hasApplications) {
      return 'applications';
    }
    return 'overview';
  }
  if (
    state.hasTemplates &&
    (tabFromUrl === 'templates' || tabFromUrl === 'running' || tabFromUrl === 'history')
  ) {
    return tabFromUrl;
  }
  return 'overview';
}

export function resolveApplicationHref(url: string): string {
  if (url.startsWith('http://') || url.startsWith('https://')) {
    return url;
  }
  try {
    return new URL(url, window.location.origin).href;
  } catch {
    return url;
  }
}

export function templateNames(entries: CatalogEntry[]): Set<string> {
  return new Set(entries.map((e) => e.templateName));
}

export function parseIsoMs(iso: string | undefined): number {
  if (!iso?.trim()) {
    return NaN;
  }
  const t = Date.parse(iso);
  return Number.isFinite(t) ? t : NaN;
}

export function formatLocaleDateTime(iso: string | undefined): string {
  if (!iso?.trim()) {
    return '—';
  }
  const d = new Date(iso);
  return Number.isFinite(d.getTime()) ? d.toLocaleString() : iso;
}

/** Human-readable duration; when `liveEnd` is true and `finished` is missing, uses `Date.now()` for running workflows. */
export function formatDurationBetween(
  started?: string,
  finished?: string,
  liveEnd = false
): string {
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

export function sortWorkflowsForHistory(items: WorkflowSummary[]): WorkflowSummary[] {
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
export function formatSubmittedFromLabel(v: string | undefined): string {
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
export function formatStartedByLabel(v: string | undefined): string {
  const raw = (v ?? '').trim();
  return raw || '—';
}

export function sortWorkflowsForRunning(items: WorkflowSummary[]): WorkflowSummary[] {
  return [...items].sort((a, b) => {
    const sb = parseIsoMs(b.startedAt);
    const sa = parseIsoMs(a.startedAt);
    if (Number.isFinite(sb) && Number.isFinite(sa) && sb !== sa) {
      return sb - sa;
    }
    return a.name.localeCompare(b.name);
  });
}

export function runningPhaseChipColor(
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

/** Choices for a workflow parameter that declares an enum. */
export function dropdownOptions(p: { enum?: string[] }): string[] {
  return (p.enum ?? []).map((value) => value.trim()).filter((value) => value !== '');
}

/** `Name (branch)` is one dropdown value. `Name` is both the menu label and the stored row. */
export function dropdownChoice(choice: string): { name: string; detail: string } | null {
  const match = choice.match(/^([A-Za-z][A-Za-z0-9_-]*) \(([^)]+)\)$/);
  if (!match) {
    return null;
  }
  return { name: match[1], detail: match[2] };
}

export function dropdownLabel(choice: string): string {
  return dropdownChoice(choice)?.name ?? choice;
}

export function isCustomChoice(choice: string): boolean {
  return dropdownLabel(choice) === 'AndroidCustom';
}

export function replaceYamlScalar(text: string, key: string, next: string): string {
  return text.replace(
    new RegExp(`^(\\s*(?:-\\s*)?${key}:\\s*)(?:"[^"]*"|'[^']*'|\\S+)`, 'm'),
    `$1${next}`
  );
}

/** Returns only the help sentence before # predefined-template in workflow parameter description.*/
export function visibleParamDescription(description?: string): string | undefined {
  const text = description?.split(PREDEFINED_TEMPLATE_MARKER)[0]?.trim();
  return text || undefined;
}

/** Predefined repo block rendered into the workflow parameter from uploader values. */
export function predefinedTemplate(description?: string): string {
  const at = description?.indexOf(PREDEFINED_TEMPLATE_MARKER) ?? -1;
  if (at < 0) {
    return '';
  }
  return description!.slice(at + PREDEFINED_TEMPLATE_MARKER.length).trim();
}

/** Fill `name` and `branch` in the predefined template. url, file, and fetch-ref-spec stay as rendered. */
export function configForChoice(template: string, choice: string): string {
  const parsed = dropdownChoice(choice);
  if (!parsed) {
    return template;
  }
  return replaceYamlScalar(
    replaceYamlScalar(template, 'name', parsed.name),
    'branch',
    parsed.detail
  );
}

export function workflowParamHelperText(p: { description?: string }): string | undefined {
  const desc = visibleParamDescription(p.description);
  return desc && desc.length <= 72 ? desc : undefined;
}

/** A parameter value that already contains newlines is a YAML document. */
export function isYamlWorkflowParam(value: string): boolean {
  return value.includes('\n');
}

/** Mirrors Horizon API / workflow archive heuristics for log-type outputs. */
export function isLogArtifactName(name: string): boolean {
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
export function pickLogArtifactForNode(
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
export function nodeArchivedLogLinks(
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
export function argoBrowserBaseUrl(): string {
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
export function argoWorkflowUiUrl(workflowName: string, namespace: string): string {
  const base = argoBrowserBaseUrl();
  return `${base}/workflows/workflows/${encodeURIComponent(namespace)}/${encodeURIComponent(workflowName)}`;
}

export async function downloadArtifact(wf: string, a: OutputArtifact) {
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
export function isTerminalWorkflowPhase(phase: string | undefined): boolean {
  if (!phase) {
    return false;
  }
  const p = phase.trim().toLowerCase();
  return ['succeeded', 'failed', 'error', 'aborted'].includes(p);
}

export function hasArchivedLogLinks(archived?: WorkflowSummary['archivedLogs']): boolean {
  if (!archived) {
    return false;
  }
  return (archived.steps || []).some((s) => Boolean(s.gcsUri));
}

/** Parsed `[stage]` prefix from our formatted log lines; empty if unlabeled. */
export function stageLabelFromDisplayLine(line: string): string {
  const m = line.match(/^\[([^\]]+)\]\s+/);
  return m ? m[1].trim() : '';
}

/**
 * When over the cap, repeatedly remove the chronologically oldest line from the
 * stage that has the most lines (tie: most total characters). That way a noisy
 * "build" step sheds its own lines first instead of wiping out short earlier stages.
 */
export function trimBySheddingBusiestStage(lines: string[], max: number): string[] {
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

export function appendWorkflowLogLine(prev: string[], line: string, maxLines: number): string[] {
  return trimBySheddingBusiestStage([...prev, line], maxLines);
}

export function buildWorkflowLogUrl(
  workflowName: string,
  follow: boolean,
  podName?: string
): string {
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
export async function horizonArtifactInlineText(
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
