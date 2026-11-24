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

/** Left-pane category chip. Order of PORTAL_GUIDES is the importance order. */
export const DOC_CATEGORIES = {
  deploy: { label: 'DEPLOY', color: '#1a73e8' },
  portal: { label: 'PORTAL', color: '#1967d2' },
  release: { label: 'RELEASE', color: '#c2185b' },
  operate: { label: 'OPERATE', color: '#188038' },
  workloads: { label: 'WORKLOADS', color: '#1a73e8' },
  architecture: { label: 'ARCHITECTURE', color: '#8430ce' },
  governance: { label: 'GOVERNANCE', color: '#5f6368' },
  ai: { label: 'AI', color: '#007b83' },
} as const;

export type DocCategoryId = keyof typeof DOC_CATEGORIES;

export type PortalGuide = {
  /** Repo-relative path, also the document id in the URL. */
  path: string;
  title: string;
  summary: string;
  category: DocCategoryId;
};

/**
 * Guides shown in the reader, most important first.
 * The bundle also contains the rest of docs/ and release-notes.md so links
 * from these guides open the matching file. Those files are not separate rows.
 */
export const PORTAL_GUIDES: PortalGuide[] = [
  {
    path: 'docs/deployment_guide.md',
    title: 'Deployment Guide',
    summary: 'End-to-end install and configuration of a Horizon environment.',
    category: 'deploy',
  },
  {
    path: 'docs/architecture/02_horizon-portal.md',
    title: 'Portal User Guide',
    summary: 'Using the Developer Portal: modules, workflows, and tools.',
    category: 'portal',
  },
  {
    path: 'release-notes.md',
    title: 'Release Notes',
    summary: 'What changed in this Horizon release, plus the extended notes that exist.',
    category: 'release',
  },
  {
    path: 'docs/guides/README.md',
    title: 'Upgrade Guides',
    summary: 'Version-to-version upgrade steps for an existing environment.',
    category: 'operate',
  },
  {
    path: 'docs/workloads/guides/README.md',
    title: 'Workload Guides',
    summary: 'Set up and run Android, Cloud Workstations, and OpenBSW workloads.',
    category: 'workloads',
  },
  {
    path: 'docs/workloads/guides/pipeline_guide.md',
    title: 'Pipeline Guide',
    summary: 'How workload Jenkins pipelines are defined, seeded, and updated.',
    category: 'workloads',
  },
  {
    path: 'docs/architecture/04_module-architecture.md',
    title: 'Module Architecture',
    summary: 'How the portal, Module Manager, API, and GitOps fit together.',
    category: 'architecture',
  },
  {
    path: 'docs/architecture/05_creating-modules.md',
    title: 'Creating Modules',
    summary: 'Build, register, and enable a new Horizon module.',
    category: 'architecture',
  },
  {
    path: 'docs/contributing.md',
    title: 'Contributing Guide',
    summary: 'Contributor agreement, reviews, and how to propose a change.',
    category: 'governance',
  },
  {
    path: 'docs/guides/mcp_setup.md',
    title: 'MCP Setup Guide',
    summary: 'Configure MCP Gateway Registry servers and IDE clients.',
    category: 'ai',
  },
];

export const DEFAULT_GUIDE_PATH = PORTAL_GUIDES[0].path;

const UPGRADE_GUIDES = 'docs/guides/README.md';
const WORKLOAD_GUIDES = 'docs/workloads/guides/README.md';
const PIPELINE_GUIDE = 'docs/workloads/guides/pipeline_guide.md';
const RELEASE_NOTES = 'release-notes.md';
const MCP_SETUP = 'docs/guides/mcp_setup.md';
const MODULE_ARCHITECTURE = 'docs/architecture/04_module-architecture.md';

export function guideForPath(repoPath: string): PortalGuide | undefined {
  return PORTAL_GUIDES.find((guide) => guide.path === repoPath);
}

/**
 * Left-pane row to highlight for a bundled file.
 * Exact guides highlight themselves. Linked files (upgrade steps, workload
 * pages, extended release notes) highlight the guide they belong to.
 */
export function sectionForPath(repoPath: string): PortalGuide | undefined {
  const exact = guideForPath(repoPath);
  if (exact) {
    return exact;
  }
  if (repoPath.startsWith('docs/extended-release-notes/')) {
    return guideForPath(RELEASE_NOTES);
  }
  if (repoPath === PIPELINE_GUIDE) {
    return guideForPath(PIPELINE_GUIDE);
  }
  if (repoPath.startsWith('docs/workloads/')) {
    return guideForPath(WORKLOAD_GUIDES);
  }
  if (repoPath.startsWith('docs/guides/') && repoPath !== MCP_SETUP) {
    return guideForPath(UPGRADE_GUIDES);
  }
  if (repoPath.startsWith('docs/architecture/')) {
    return guideForPath(MODULE_ARCHITECTURE);
  }
  return undefined;
}

export function documentationHref(repoPath: string, hash = ''): string {
  const encoded = repoPath
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
  return `/documentation/${encoded}${hash}`;
}

export function guideMatchesQuery(guide: PortalGuide, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (!needle) {
    return true;
  }
  const category = DOC_CATEGORIES[guide.category].label;
  const haystack = `${guide.title} ${guide.summary} ${category} ${guide.path}`.toLowerCase();
  return haystack.includes(needle);
}
