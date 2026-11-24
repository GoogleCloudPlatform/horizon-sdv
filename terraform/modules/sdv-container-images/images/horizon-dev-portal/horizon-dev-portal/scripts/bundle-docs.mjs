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
// Copies Horizon docs into the portal's public/ folder so the Documentation
// page serves the guides from the version being built.
//
// Host builds (npm run dev / npm run build) read the git checkout.
// Image builds do not contain the repo. Terraform stages the same tree at
// doc-src/ before docker build, and this script reads that instead.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const portalRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const stagedRoot = path.join(portalRoot, 'doc-src');
const publicDocs = path.join(portalRoot, 'public', 'bundled-docs');
const publicImages = path.join(portalRoot, 'public', 'docs-images');

function findRepoRoot(start) {
  let dir = start;
  for (let i = 0; i < 10; i += 1) {
    if (
      fs.existsSync(path.join(dir, 'docs', 'deployment_guide.md')) &&
      fs.existsSync(path.join(dir, 'release-notes.md'))
    ) {
      return dir;
    }
    const parent = path.dirname(dir);
    if (parent === dir) {
      break;
    }
    dir = parent;
  }
  return null;
}

function walkMarkdown(dir, repoPrefix, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === '.git') {
      continue;
    }
    const absolute = path.join(dir, entry.name);
    const repoPath = `${repoPrefix}/${entry.name}`;
    if (entry.isDirectory()) {
      walkMarkdown(absolute, repoPath, out);
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      out.push({ repoPath, absolute });
    }
  }
}

function resetDir(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
}

function copyRepoFile(absolute, repoPath) {
  const dest = path.join(publicDocs, ...repoPath.split('/'));
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.copyFileSync(absolute, dest);
}

const repoRoot = findRepoRoot(portalRoot);
let docsRoot;
let releaseNotes;
let imagesRoot;

if (repoRoot) {
  docsRoot = path.join(repoRoot, 'docs');
  releaseNotes = path.join(repoRoot, 'release-notes.md');
  imagesRoot = path.join(docsRoot, 'images');
  console.log(`[bundle-docs] source repo ${repoRoot}`);
} else if (fs.existsSync(path.join(stagedRoot, 'docs', 'deployment_guide.md'))) {
  docsRoot = path.join(stagedRoot, 'docs');
  releaseNotes = path.join(stagedRoot, 'release-notes.md');
  imagesRoot = path.join(stagedRoot, 'images');
  console.log(`[bundle-docs] staged source ${stagedRoot}`);
} else {
  console.error(
    '[bundle-docs] could not find docs/deployment_guide.md and release-notes.md above this portal, or a staged doc-src/ tree.'
  );
  process.exit(1);
}

const files = [];
walkMarkdown(docsRoot, 'docs', files);
if (!fs.existsSync(releaseNotes)) {
  console.error(`[bundle-docs] missing ${releaseNotes}`);
  process.exit(1);
}
files.push({ repoPath: 'release-notes.md', absolute: releaseNotes });
files.sort((a, b) => a.repoPath.localeCompare(b.repoPath));

resetDir(publicDocs);
for (const file of files) {
  copyRepoFile(file.absolute, file.repoPath);
}

const manifest = { files: files.map((file) => file.repoPath) };
fs.writeFileSync(path.join(publicDocs, 'manifest.json'), `${JSON.stringify(manifest, null, 2)}\n`);

resetDir(publicImages);
if (fs.existsSync(imagesRoot)) {
  fs.cpSync(imagesRoot, publicImages, { recursive: true });
} else {
  console.warn(`[bundle-docs] no images directory at ${imagesRoot}`);
}

console.log(`[bundle-docs] ${manifest.files.length} markdown files`);
