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
const GITHUB_BLOB =
  /^https?:\/\/github\.com\/GoogleCloudPlatform\/horizon-sdv\/blob\/[^/]+\/([^?#]+)/i;

/** Image types bundle-docs.mjs copies from anywhere under docs/ (same list as the proxy's docsImageTypes). */
const IMAGE_FILE = /\.(png|jpe?g|gif|svg|webp)$/i;

/** Files without an extension that would otherwise be taken for directories when linking to GitHub. */
const EXTENSIONLESS_FILE =
  /^(Makefile|GNUmakefile|Dockerfile|Containerfile|Jenkinsfile|Vagrantfile|Gemfile|Rakefile|Procfile|Brewfile|LICENSE|LICENCE|NOTICE|COPYING|AUTHORS|CONTRIBUTORS|CHANGELOG|README|OWNERS|CODEOWNERS|VERSION)$/i;

export type ResolvedHref =
  | { kind: 'doc'; path: string; hash: string }
  | { kind: 'hash'; hash: string }
  | { kind: 'asset'; imagePath: string }
  | { kind: 'external'; url: string };

export function normalizeRepoPath(input: string): string {
  const parts: string[] = [];
  for (const part of input.split('/')) {
    if (part === '' || part === '.') {
      continue;
    }
    if (part === '..') {
      parts.pop();
      continue;
    }
    parts.push(part);
  }
  return parts.join('/');
}

function decodeSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** GitHub ref matching the installed release (tags are HORIZON_RELEASE_<x.y.z>), or main for dev builds. */
export function horizonGitRef(horizonVersion: string): string {
  const version = horizonVersion.trim().replace(/^v/i, '');
  return /^\d+\.\d+(\.\d+)?$/.test(version) ? `HORIZON_RELEASE_${version}` : 'main';
}

/**
 * Map a Markdown link onto a bundled document, a bundled image, an in-page
 * anchor, or an external URL. Relative links to files we did not bundle open
 * the same path on the public GitHub repository at `gitRef`.
 */
export function resolveDocHref(
  fromRepoPath: string,
  href: string,
  bundled: ReadonlySet<string>,
  gitRef = 'main'
): ResolvedHref {
  const trimmed = href.trim();
  if (trimmed.startsWith('#')) {
    return { kind: 'hash', hash: trimmed };
  }

  const hashAt = trimmed.indexOf('#');
  const hash = hashAt >= 0 ? trimmed.slice(hashAt) : '';
  const beforeHash = hashAt >= 0 ? trimmed.slice(0, hashAt) : trimmed;
  const beforeQuery = beforeHash.split('?')[0] ?? '';

  if (beforeQuery === '') {
    return { kind: 'hash', hash: hash || '#' };
  }

  const github = GITHUB_BLOB.exec(trimmed);
  if (github?.[1]) {
    const repoPath = normalizeRepoPath(decodeSegment(github[1]));
    if (bundled.has(repoPath)) {
      return { kind: 'doc', path: repoPath, hash };
    }
    return { kind: 'external', url: trimmed };
  }

  if (/^[a-z][a-z0-9+.-]*:/i.test(beforeQuery) || beforeQuery.startsWith('//')) {
    return { kind: 'external', url: trimmed };
  }

  if (beforeQuery.startsWith('/docs/') && IMAGE_FILE.test(beforeQuery)) {
    return {
      kind: 'asset',
      imagePath: normalizeRepoPath(decodeSegment(beforeQuery.slice(1))),
    };
  }
  if (beforeQuery.startsWith('/')) {
    return { kind: 'external', url: trimmed };
  }

  const baseDir = fromRepoPath.includes('/')
    ? fromRepoPath.slice(0, fromRepoPath.lastIndexOf('/'))
    : '';
  const decoded = decodeSegment(beforeQuery);
  const joined = normalizeRepoPath(baseDir ? `${baseDir}/${decoded}` : decoded);

  if (bundled.has(joined)) {
    return { kind: 'doc', path: joined, hash };
  }

  const asReadme = `${joined.replace(/\/$/, '')}/README.md`;
  if (bundled.has(asReadme)) {
    return { kind: 'doc', path: asReadme, hash };
  }

  if (joined.startsWith('docs/') && IMAGE_FILE.test(joined)) {
    return { kind: 'asset', imagePath: joined };
  }

  const basename = joined.split('/').pop() ?? '';
  const directory =
    joined.endsWith('/') || (!basename.includes('.') && !EXTENSIONLESS_FILE.test(basename));
  const gitPath = joined
    .replace(/\/$/, '')
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
  const gitKind = directory ? 'tree' : 'blob';
  return {
    kind: 'external',
    url: `https://github.com/GoogleCloudPlatform/horizon-sdv/${gitKind}/${encodeURIComponent(gitRef)}/${gitPath}${hash}`,
  };
}
