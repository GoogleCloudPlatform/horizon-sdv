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

/** Left-pane category chip (colour: theme.palette.docs.chip). Order of PORTAL_GUIDES is the importance order. */
export const DOC_CATEGORIES = {
  deploy: { label: 'DEPLOY' },
  portal: { label: 'PORTAL' },
  release: { label: 'RELEASE' },
  operate: { label: 'OPERATE' },
  workloads: { label: 'WORKLOADS' },
  architecture: { label: 'ARCHITECTURE' },
  contribute: { label: 'CONTRIBUTE' },
  ai: { label: 'AI' },
  external: { label: 'EXTERNAL' },
} as const;

export type DocCategoryId = keyof typeof DOC_CATEGORIES;

type GuideBase = {
  title: string;
  summary: string;
  category: DocCategoryId;
};

/** Bundled Markdown guide rendered in the right-hand reader (the default entry type). */
export type MarkdownGuide = GuideBase & {
  kind?: 'markdown';
  /** Repo-relative path, also the document id in the URL. */
  path: string;
};

/**
 * Documentation that lives in another application. It opens in a new tab and never loads the reader.
 * `url` is relative to the deployment's domain (like the Landing page app cards), never an absolute host.
 */
export type ExternalGuide = GuideBase & {
  kind: 'external';
  url: string;
};

export type PortalGuide = MarkdownGuide | ExternalGuide;

export function isExternalGuide(guide: PortalGuide): guide is ExternalGuide {
  return guide.kind === 'external';
}

/** Stable React key / identity for a catalog entry. */
export function guideKey(guide: PortalGuide): string {
  return isExternalGuide(guide) ? guide.url : guide.path;
}

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
    summary: 'New features, improvements and fixes in this release.',
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
    summary: 'Setting up and running Android, Cloud Workstations, and OpenBSW workloads.',
    category: 'workloads',
  },
  {
    kind: 'external',
    url: '/mtk-connect/docs/',
    title: 'MTK Connect Documentation',
    summary: 'Remote access docs for physical and virtual testbenches. Opens in a new tab.',
    category: 'external',
  },
  {
    path: 'docs/workloads/guides/pipeline_guide.md',
    title: 'Pipeline Guide',
    summary: 'How workflow pipelines are defined and executed.',
    category: 'workloads',
  },
  {
    path: 'docs/architecture/04_module-architecture.md',
    title: 'Module Architecture',
    summary: 'How modules work and how to create a new one.',
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
    summary: 'Branching, reviews and submitting changes.',
    category: 'contribute',
  },
  {
    path: 'docs/guides/mcp_setup.md',
    title: 'MCP Setup Guide',
    summary: 'Configure MCP Gateway Registry servers and clients.',
    category: 'ai',
  },
];

export const DEFAULT_GUIDE_PATH = 'docs/deployment_guide.md';

const UPGRADE_GUIDES = 'docs/guides/README.md';
const WORKLOAD_GUIDES = 'docs/workloads/guides/README.md';
const PIPELINE_GUIDE = 'docs/workloads/guides/pipeline_guide.md';
const RELEASE_NOTES = 'release-notes.md';
const MCP_SETUP = 'docs/guides/mcp_setup.md';
const MODULE_ARCHITECTURE = 'docs/architecture/04_module-architecture.md';
const PORTAL_USER_GUIDE = 'docs/architecture/02_horizon-portal.md';
const HORIZON_CLI = 'docs/architecture/03_horizon-cli.md';
const ARCHITECTURE_REFERENCES = new Set([
  'docs/architecture/01_overview.md',
  'docs/architecture/06_interfaces.md',
]);

export function guideForPath(repoPath: string): MarkdownGuide | undefined {
  return PORTAL_GUIDES.find(
    (guide): guide is MarkdownGuide => !isExternalGuide(guide) && guide.path === repoPath
  );
}

/**
 * Left-pane row to highlight for a bundled file.
 * Exact guides highlight themselves. Linked files (upgrade steps, workload
 * pages, extended release notes) highlight the guide they belong to.
 */
export function sectionForPath(repoPath: string): MarkdownGuide | undefined {
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
  if (repoPath === HORIZON_CLI) {
    return guideForPath(PORTAL_USER_GUIDE);
  }
  if (ARCHITECTURE_REFERENCES.has(repoPath)) {
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
  const location = isExternalGuide(guide) ? guide.url : guide.path;
  const haystack = `${guide.title} ${guide.summary} ${category} ${location}`.toLowerCase();
  return haystack.includes(needle);
}
