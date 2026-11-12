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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { CatalogEntry, OutputArtifact, WorkflowSummary } from '../types';
import { apiHorizon } from '../utils/api';
import {
  appendWorkflowLogLine,
  argoBrowserBaseUrl,
  argoWorkflowUiUrl,
  buildWorkflowLogUrl,
  configForChoice,
  dropdownChoice,
  dropdownLabel,
  dropdownOptions,
  downloadArtifact,
  formatDurationBetween,
  formatLocaleDateTime,
  formatStartedByLabel,
  formatSubmittedFromLabel,
  hasArchivedLogLinks,
  horizonArtifactInlineText,
  isCustomChoice,
  isLogArtifactName,
  isTerminalWorkflowPhase,
  isYamlWorkflowParam,
  nodeArchivedLogLinks,
  parseIsoMs,
  parseTabKey,
  pickLogArtifactForNode,
  predefinedTemplate,
  replaceYamlScalar,
  resolveActiveTab,
  resolveApplicationHref,
  runningPhaseChipColor,
  sortWorkflowsForHistory,
  sortWorkflowsForRunning,
  stageLabelFromDisplayLine,
  templateNames,
  trimBySheddingBusiestStage,
  visibleParamDescription,
  workflowParamHelperText,
} from './modulePageLogic';

vi.mock('../utils/api', () => ({
  apiHorizon: vi.fn(),
}));

const apiHorizonMock = vi.mocked(apiHorizon);

function wf(partial: Partial<WorkflowSummary> & { name: string }): WorkflowSummary {
  return { namespace: 'ns', phase: 'Succeeded', ...partial } as WorkflowSummary;
}

function art(partial: Partial<OutputArtifact> & { name: string }): OutputArtifact {
  return partial as OutputArtifact;
}

const START = '2026-01-01T00:00:00Z';

function at(seconds: number): string {
  return new Date(Date.parse(START) + seconds * 1000).toISOString();
}

describe('tabs', () => {
  it('parses known tab keys and defaults everything else to overview', () => {
    expect(parseTabKey(null)).toBe('overview');
    expect(parseTabKey('history')).toBe('history');
    expect(parseTabKey('Templates')).toBe('overview');
    expect(parseTabKey('nope')).toBe('overview');
  });

  it('keeps applications only once module metadata has loaded and an application exists', () => {
    const state = { moduleMetaLoaded: false, hasApplications: true, hasTemplates: false };
    expect(resolveActiveTab('applications', state)).toBe('overview');
    expect(
      resolveActiveTab('applications', { ...state, moduleMetaLoaded: true, hasApplications: false })
    ).toBe('overview');
    expect(
      resolveActiveTab('applications', { ...state, moduleMetaLoaded: true, hasApplications: true })
    ).toBe('applications');
  });

  it('keeps workflow tabs only when the module has templates', () => {
    const none = { moduleMetaLoaded: true, hasApplications: false, hasTemplates: false };
    const some = { ...none, hasTemplates: true };
    for (const tab of ['templates', 'running', 'history'] as const) {
      expect(resolveActiveTab(tab, none)).toBe('overview');
      expect(resolveActiveTab(tab, some)).toBe(tab);
    }
    expect(resolveActiveTab('overview', none)).toBe('overview');
  });
});

describe('links and catalog', () => {
  beforeEach(() => {
    vi.stubGlobal('window', { location: { origin: 'https://portal.example' } });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('leaves absolute URLs alone and resolves relative ones against the origin', () => {
    expect(resolveApplicationHref('https://grafana.example/d/x')).toBe(
      'https://grafana.example/d/x'
    );
    expect(resolveApplicationHref('/module/x')).toBe('https://portal.example/module/x');
    expect(resolveApplicationHref('//[')).toBe('//[');
  });

  it('collects unique template names', () => {
    const entries = [
      { templateName: 'a' },
      { templateName: 'a' },
      { templateName: 'b' },
    ] as CatalogEntry[];
    expect(templateNames(entries)).toEqual(new Set(['a', 'b']));
  });
});

describe('timestamps and durations', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('parses ISO strings and rejects blanks and garbage', () => {
    expect(parseIsoMs(undefined)).toBeNaN();
    expect(parseIsoMs('   ')).toBeNaN();
    expect(parseIsoMs('nope')).toBeNaN();
    expect(parseIsoMs(START)).toBe(Date.parse(START));
  });

  it('formats local date times and passes unparseable values through', () => {
    expect(formatLocaleDateTime(undefined)).toBe('—');
    expect(formatLocaleDateTime('   ')).toBe('—');
    expect(formatLocaleDateTime('not-a-date')).toBe('not-a-date');
    expect(formatLocaleDateTime(START)).toBe(new Date(START).toLocaleString());
  });

  it('formats durations in seconds, minutes, and hours', () => {
    expect(formatDurationBetween(START, at(45))).toBe('45s');
    expect(formatDurationBetween(START, at(90))).toBe('1m 30s');
    expect(formatDurationBetween(START, at(120))).toBe('2m');
    expect(formatDurationBetween(START, at(3600))).toBe('1h');
    expect(formatDurationBetween(START, at(3900))).toBe('1h 5m');
  });

  it('shows a dash when the start is missing or the end is unknown', () => {
    expect(formatDurationBetween(undefined, at(10))).toBe('—');
    expect(formatDurationBetween(START, undefined)).toBe('—');
  });

  it('measures a running workflow up to now when liveEnd is set', () => {
    vi.useFakeTimers();
    vi.setSystemTime(Date.parse(START) + 125_000);
    expect(formatDurationBetween(START, undefined, true)).toBe('2m 5s');
  });

  it('never reports a negative duration', () => {
    expect(formatDurationBetween(at(60), START)).toBe('0s');
  });
});

describe('workflow sorting and labels', () => {
  it('sorts history by finish time, then start time, then name', () => {
    const items = [
      wf({ name: 'old', startedAt: at(0), finishedAt: at(100) }),
      wf({ name: 'new', startedAt: at(200), finishedAt: at(300) }),
    ];
    expect(sortWorkflowsForHistory(items).map((w) => w.name)).toEqual(['new', 'old']);

    const sameFinish = [
      wf({ name: 'p', startedAt: at(0), finishedAt: at(100) }),
      wf({ name: 'q', startedAt: at(50), finishedAt: at(100) }),
    ];
    expect(sortWorkflowsForHistory(sameFinish).map((w) => w.name)).toEqual(['q', 'p']);

    const undated = [wf({ name: 'b' }), wf({ name: 'a' })];
    expect(sortWorkflowsForHistory(undated).map((w) => w.name)).toEqual(['a', 'b']);
    expect(undated.map((w) => w.name)).toEqual(['b', 'a']);
  });

  it('sorts running workflows by start time, newest first', () => {
    const items = [wf({ name: 'z', startedAt: at(0) }), wf({ name: 'm', startedAt: at(100) })];
    expect(sortWorkflowsForRunning(items).map((w) => w.name)).toEqual(['m', 'z']);
  });

  it('maps submitted-from labels to friendly names', () => {
    expect(formatSubmittedFromLabel(undefined)).toBe('—');
    expect(formatSubmittedFromLabel('developer-portal')).toBe('Developer portal');
    expect(formatSubmittedFromLabel('HORIZON-CLI')).toBe('Horizon CLI');
    expect(formatSubmittedFromLabel('api')).toBe('REST API');
    expect(formatSubmittedFromLabel('custom')).toBe('custom');
  });

  it('shows the starter or a dash', () => {
    expect(formatStartedByLabel('  alice  ')).toBe('alice');
    expect(formatStartedByLabel('')).toBe('—');
    expect(formatStartedByLabel(undefined)).toBe('—');
  });

  it('colours phase chips by outcome', () => {
    expect(runningPhaseChipColor('Running')).toBe('info');
    expect(runningPhaseChipColor(' pending ')).toBe('warning');
    expect(runningPhaseChipColor('Succeeded')).toBe('success');
    expect(runningPhaseChipColor('Failed')).toBe('error');
    expect(runningPhaseChipColor('Error')).toBe('error');
    expect(runningPhaseChipColor('Aborted')).toBe('error');
    expect(runningPhaseChipColor('Unknown')).toBe('primary');
    expect(runningPhaseChipColor(undefined)).toBe('primary');
  });

  it('recognises terminal phases', () => {
    expect(isTerminalWorkflowPhase('Succeeded')).toBe(true);
    expect(isTerminalWorkflowPhase(' failed ')).toBe(true);
    expect(isTerminalWorkflowPhase('Error')).toBe(true);
    expect(isTerminalWorkflowPhase('Aborted')).toBe(true);
    expect(isTerminalWorkflowPhase('Running')).toBe(false);
    expect(isTerminalWorkflowPhase('')).toBe(false);
    expect(isTerminalWorkflowPhase(undefined)).toBe(false);
  });
});

describe('workflow parameters', () => {
  it('trims enum options and drops blanks', () => {
    expect(dropdownOptions({ enum: [' a ', '', 'b '] })).toEqual(['a', 'b']);
    expect(dropdownOptions({})).toEqual([]);
  });

  it('splits Name (branch) choices', () => {
    expect(dropdownChoice('AndroidCustom (main)')).toEqual({
      name: 'AndroidCustom',
      detail: 'main',
    });
    expect(dropdownChoice('plain')).toBeNull();
    expect(dropdownChoice('1bad (x)')).toBeNull();
    expect(dropdownChoice('Bad Name (x)')).toBeNull();
  });

  it('labels choices by their name and detects the custom choice', () => {
    expect(dropdownLabel('AndroidCustom (main)')).toBe('AndroidCustom');
    expect(dropdownLabel('plain value')).toBe('plain value');
    expect(isCustomChoice('AndroidCustom (main)')).toBe(true);
    expect(isCustomChoice('AndroidCustom')).toBe(true);
    expect(isCustomChoice('Other (x)')).toBe(false);
  });

  it('replaces a YAML scalar on its own line, including list items', () => {
    expect(replaceYamlScalar('name: old\nbranch: "dev"\n', 'branch', 'main')).toBe(
      'name: old\nbranch: main\n'
    );
    expect(replaceYamlScalar('  - name: old', 'name', 'new')).toBe('  - name: new');
  });

  it('separates the visible help text from the predefined template', () => {
    expect(visibleParamDescription('Pick one. # predefined-template\nrepo: x')).toBe('Pick one.');
    expect(visibleParamDescription(undefined)).toBeUndefined();
    expect(visibleParamDescription('# predefined-template only')).toBeUndefined();

    expect(predefinedTemplate('Help # predefined-template\n url: x \n')).toBe('url: x');
    expect(predefinedTemplate('Help without marker')).toBe('');
    expect(predefinedTemplate(undefined)).toBe('');
  });

  it('fills name and branch into the predefined template for a choice', () => {
    const template = 'name: old\nbranch: old\nurl: u';
    expect(configForChoice(template, 'AndroidCustom (main)')).toBe(
      'name: AndroidCustom\nbranch: main\nurl: u'
    );
    expect(configForChoice(template, 'plain')).toBe(template);
  });

  it('shows short help text only', () => {
    expect(workflowParamHelperText({ description: 'Short help # predefined-template\nx' })).toBe(
      'Short help'
    );
    expect(workflowParamHelperText({ description: 'x'.repeat(72) })).toBe('x'.repeat(72));
    expect(workflowParamHelperText({ description: 'x'.repeat(73) })).toBeUndefined();
    expect(workflowParamHelperText({})).toBeUndefined();
  });

  it('treats a multi-line value as YAML', () => {
    expect(isYamlWorkflowParam('a: 1\nb: 2')).toBe(true);
    expect(isYamlWorkflowParam('plain')).toBe(false);
  });
});

describe('log artifacts', () => {
  it('recognises log-type artifact names', () => {
    expect(isLogArtifactName('main')).toBe(true);
    expect(isLogArtifactName(' MAIN-LOGS ')).toBe(true);
    expect(isLogArtifactName('build-log')).toBe(true);
    expect(isLogArtifactName('mainlogs')).toBe(true);
    expect(isLogArtifactName('output')).toBe(false);
  });

  it('prefers the single log artifact for a node', () => {
    const logs = [
      art({ name: 'x', nodeId: 'n2' }),
      art({ name: 'output', nodeId: 'n1' }),
      art({ name: 'main', nodeId: 'n1' }),
    ];
    expect(pickLogArtifactForNode(logs, { id: 'n2' })).toEqual(logs[0]);
    expect(pickLogArtifactForNode(logs, { id: 'n1' })).toEqual(logs[2]);
  });

  it('falls back to template and display name when the node id does not match', () => {
    const byBoth = [
      art({ name: 'x', templateName: 't', displayName: 'd' }),
      art({ name: 'y', templateName: 't2', displayName: 'd' }),
    ];
    expect(
      pickLogArtifactForNode(byBoth, { id: 'other', templateName: 't', displayName: 'd' })
    ).toEqual(byBoth[0]);

    const byTemplate = [
      art({ name: 'x', templateName: 't' }),
      art({ name: 'y', templateName: 'other' }),
    ];
    expect(pickLogArtifactForNode(byTemplate, { id: 'z', templateName: 't' })).toEqual(
      byTemplate[0]
    );

    const byDisplay = [
      art({ name: 'x', displayName: 'build' }),
      art({ name: 'y', displayName: 'test' }),
    ];
    expect(pickLogArtifactForNode(byDisplay, { id: 'q', displayName: 'build' })).toEqual(
      byDisplay[0]
    );
  });

  it('returns nothing when the match is ambiguous or absent', () => {
    const ambiguous = [
      art({ name: 'a', templateName: 't' }),
      art({ name: 'b', templateName: 't' }),
    ];
    expect(pickLogArtifactForNode(ambiguous, { id: 'q', templateName: 't' })).toBeUndefined();
    expect(pickLogArtifactForNode([], { id: 'zz' })).toBeUndefined();
  });

  it('resolves archived GCS links and the download artifact for a node', () => {
    const archived = {
      steps: [{ nodeId: 'n1', gcsUri: 'gs://bucket/n1.log', artifactName: 'main-logs' }],
    } as WorkflowSummary['archivedLogs'];

    expect(nodeArchivedLogLinks({ id: 'n1' }, archived, [])).toEqual({
      gcsUri: 'gs://bucket/n1.log',
      download: undefined,
      artifactName: 'main-logs',
    });

    const logs = [art({ name: 'main', nodeId: 'n2', gcsUri: 'gs://bucket/n2-main' })];
    expect(nodeArchivedLogLinks({ id: 'n2' }, undefined, logs)).toEqual({
      gcsUri: 'gs://bucket/n2-main',
      download: logs[0],
      artifactName: undefined,
    });

    const byTemplate = {
      steps: [{ templateName: 't', displayName: 'd', gcsUri: 'gs://x', artifactName: 'a' }],
    } as WorkflowSummary['archivedLogs'];
    expect(
      nodeArchivedLogLinks({ id: 'zz', templateName: 't', displayName: 'd' }, byTemplate, []).gcsUri
    ).toBe('gs://x');
  });

  it('reports whether any archived step has a GCS link', () => {
    expect(hasArchivedLogLinks(undefined)).toBe(false);
    expect(hasArchivedLogLinks({ steps: [] })).toBe(false);
    expect(hasArchivedLogLinks({ steps: [{ nodeId: 'n' }] })).toBe(false);
    expect(hasArchivedLogLinks({ steps: [{ gcsUri: 'gs://x' }] })).toBe(true);
  });
});

describe('Argo and artifact URLs', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    apiHorizonMock.mockReset();
  });

  it('strips a trailing /workflows from the configured base URL', () => {
    vi.stubGlobal('window', {
      location: { origin: 'https://portal.example' },
      APP_CONFIG: { baseUrl: 'https://h.example/workflows/' },
    });
    expect(argoBrowserBaseUrl()).toBe('https://h.example');
    expect(argoWorkflowUiUrl('wf one', 'team-a')).toBe(
      'https://h.example/workflows/workflows/team-a/wf%20one'
    );
  });

  it('uses the page origin when no base URL is configured', () => {
    vi.stubGlobal('window', { location: { origin: 'https://portal.example' }, APP_CONFIG: {} });
    expect(argoBrowserBaseUrl()).toBe('https://portal.example');
  });

  it('downloads an artifact through the Horizon API and opens the signed URL', async () => {
    const openMock = vi.fn();
    const alertMock = vi.fn();
    vi.stubGlobal('window', {
      location: { origin: 'https://portal.example' },
      APP_CONFIG: {},
      open: openMock,
    });
    vi.stubGlobal('alert', alertMock);

    apiHorizonMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ url: 'https://signed.example/a' }), { status: 200 })
    );
    await downloadArtifact('wf 1', art({ name: 'main-logs', nodeId: 'n1', templateName: 't' }));

    expect(apiHorizonMock.mock.calls[0][0]).toBe(
      '/v1/workflows/wf%201/downloadArtifact/main-logs?nodeId=n1&templateName=t'
    );
    expect(openMock).toHaveBeenCalledWith(
      'https://signed.example/a',
      '_blank',
      'noopener,noreferrer'
    );
    expect(alertMock).not.toHaveBeenCalled();
  });

  it('alerts when the artifact name is ambiguous or the download fails', async () => {
    const alertMock = vi.fn();
    vi.stubGlobal('window', {
      location: { origin: 'https://portal.example' },
      APP_CONFIG: {},
      open: vi.fn(),
    });
    vi.stubGlobal('alert', alertMock);

    apiHorizonMock.mockResolvedValueOnce(new Response(null, { status: 409 }));
    await downloadArtifact('wf', art({ name: 'main' }));
    expect(alertMock).toHaveBeenLastCalledWith(expect.stringContaining('Ambiguous artifact name'));

    apiHorizonMock.mockResolvedValueOnce(new Response('boom', { status: 500 }));
    await downloadArtifact('wf', art({ name: 'main' }));
    expect(alertMock).toHaveBeenLastCalledWith('boom');
  });

  it('loads inline artifact text for the log viewer', async () => {
    apiHorizonMock.mockResolvedValueOnce(new Response('hello log', { status: 200 }));
    const controller = new AbortController();

    await expect(
      horizonArtifactInlineText(
        'wf',
        { artifactName: 'main-logs', nodeId: 'n1', templateName: 't' },
        controller.signal
      )
    ).resolves.toBe('hello log');

    const [path, init] = apiHorizonMock.mock.calls[0];
    expect(path).toContain('inline=1');
    expect(path).toContain('nodeId=n1');
    expect(init).toEqual({ signal: controller.signal });
  });

  it('throws for ambiguous or failed inline artifact loads', async () => {
    apiHorizonMock.mockResolvedValueOnce(new Response(null, { status: 409 }));
    await expect(
      horizonArtifactInlineText('wf', { artifactName: 'main', nodeId: '' })
    ).rejects.toThrow('Ambiguous artifact name');

    apiHorizonMock.mockResolvedValueOnce(new Response('', { status: 500 }));
    await expect(
      horizonArtifactInlineText('wf', { artifactName: 'main', nodeId: '' })
    ).rejects.toThrow('artifact 500');

    apiHorizonMock.mockResolvedValueOnce(new Response('missing', { status: 404 }));
    await expect(
      horizonArtifactInlineText('wf', { artifactName: 'main', nodeId: '' })
    ).rejects.toThrow('missing');
  });
});

describe('live log buffer', () => {
  it('reads the stage label from a formatted line', () => {
    expect(stageLabelFromDisplayLine('[build] compiling')).toBe('build');
    expect(stageLabelFromDisplayLine('[ build ]  spaced')).toBe('build');
    expect(stageLabelFromDisplayLine('no prefix')).toBe('');
    expect(stageLabelFromDisplayLine('[x]no-space')).toBe('');
  });

  it('keeps every line while under the cap', () => {
    const lines = ['a', 'b'];
    expect(trimBySheddingBusiestStage(lines, 5)).toEqual(['a', 'b']);
  });

  it('sheds the oldest line of the busiest stage first', () => {
    const lines = ['[a] 1', '[b] 2', '[a] 3', '[a] 4'];
    expect(trimBySheddingBusiestStage(lines, 2)).toEqual(['[b] 2', '[a] 4']);
  });

  it('breaks ties by total characters', () => {
    const lines = ['[x] aaaa', '[y] b', '[x] c', '[y] dddd'];
    expect(trimBySheddingBusiestStage(lines, 3)).toEqual(['[y] b', '[x] c', '[y] dddd']);
  });

  it('treats unlabelled lines as their own stage', () => {
    const lines = ['plain1', 'plain2', '[a] z'];
    expect(trimBySheddingBusiestStage(lines, 2)).toEqual(['plain2', '[a] z']);
  });

  it('appends a line and keeps the cap', () => {
    expect(appendWorkflowLogLine(['[a] 1'], '[a] 2', 10)).toEqual(['[a] 1', '[a] 2']);
    expect(appendWorkflowLogLine(['[a] 1'], '[a] 2', 1)).toEqual(['[a] 2']);
  });

  it('builds the log stream URL', () => {
    expect(buildWorkflowLogUrl('wf 1', true)).toBe('/v1/workflows/wf%201/log?follow=true');
    expect(buildWorkflowLogUrl('wf', false, 'pod-a')).toBe(
      '/v1/workflows/wf/log?follow=false&podName=pod-a'
    );
  });
});
