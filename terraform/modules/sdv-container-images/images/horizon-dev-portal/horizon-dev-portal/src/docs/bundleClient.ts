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
import { apiDocs } from '../utils/api';
import { sanitizeSvg } from './svgSanitize';

type Manifest = {
  files?: unknown;
};

function encodePath(repoPath: string): string {
  return repoPath
    .split('/')
    .filter((segment) => segment !== '')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

function isHtmlResponse(response: Response): boolean {
  const type = response.headers.get('content-type') ?? '';
  return type.includes('text/html');
}

let manifestPromise: Promise<string[]> | null = null;

export function loadBundledManifest(): Promise<string[]> {
  if (!manifestPromise) {
    manifestPromise = apiDocs('/v1/files/manifest.json')
      .then(async (response) => {
        if (!response.ok || isHtmlResponse(response)) {
          throw new Error('Documentation bundle is missing from this portal build.');
        }
        const body = (await response.json()) as Manifest;
        if (!Array.isArray(body.files) || body.files.some((file) => typeof file !== 'string')) {
          throw new Error('Documentation bundle manifest is invalid.');
        }
        return body.files as string[];
      })
      .catch((error: unknown) => {
        manifestPromise = null;
        throw error;
      });
  }
  return manifestPromise;
}

export async function loadBundledDoc(repoPath: string, signal?: AbortSignal): Promise<string> {
  const response = await apiDocs(`/v1/files/${encodePath(repoPath)}`, { signal });
  if (!response.ok || isHtmlResponse(response)) {
    throw new Error('missing');
  }
  return response.text();
}

/**
 * Fetch a bundled image (repo path under docs/) with the user token and return an object URL the caller
 * must revoke. The proxy serves images by their path relative to docs/.
 */
export async function loadDocsImageUrl(imagePath: string, signal?: AbortSignal): Promise<string> {
  const relative = imagePath.startsWith('docs/') ? imagePath.slice('docs/'.length) : imagePath;
  const response = await apiDocs(`/v1/images/${encodePath(relative)}`, { signal });
  if (!response.ok || isHtmlResponse(response)) {
    throw new Error('missing');
  }
  const type = response.headers.get('content-type') ?? '';
  if (type.includes('svg') || relative.toLowerCase().endsWith('.svg')) {
    const clean = sanitizeSvg(await response.text());
    return URL.createObjectURL(new Blob([clean], { type: 'image/svg+xml' }));
  }
  return URL.createObjectURL(await response.blob());
}
