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

// SVG shown through <img> never runs script, but a blob URL can be opened as a page (for example via
// the browser's "Open image in new tab"), and then scripts run in the portal origin. The proxy's
// sandbox header is lost once the response becomes a blob, so strip active content before that.
// Mermaid SVG is inserted into the page itself, where HTML inside <foreignObject> is live too.
const ACTIVE_ELEMENTS = [
  'script',
  'iframe',
  'frame',
  'frameset',
  'embed',
  'object',
  'applet',
  'audio',
  'video',
  'base',
  'meta',
  'link',
  'form',
  'portal',
];
// Embedded raster pictures (data:image/png;base64,...) are normal in exported diagrams and stay.
// Everything else with a script-capable scheme goes: javascript:, vbscript:, other data: types.
const SAFE_DATA_URL = /^data:image\/(png|jpe?g|gif|webp);/i;
function isScriptUrl(value: string): boolean {
  // URL parsers drop whitespace and control characters, so "java\tscript:" is still javascript:.
  // eslint-disable-next-line no-control-regex
  const compact = value.replace(/[\u0000-\u0020]/g, '');
  return /^(javascript|vbscript|data):/i.test(compact) && !SAFE_DATA_URL.test(compact);
}
const URL_ATTRIBUTES = ['href', 'xlink:href', 'src', 'action', 'formaction'];

function scrub(scope: Element, removeForeignObject: boolean): void {
  const removed = removeForeignObject ? [...ACTIVE_ELEMENTS, 'foreignObject'] : ACTIVE_ELEMENTS;
  for (const tag of removed) {
    for (const element of Array.from(scope.getElementsByTagNameNS('*', tag))) {
      element.remove();
    }
  }

  for (const element of [scope, ...Array.from(scope.getElementsByTagNameNS('*', '*'))]) {
    const local = element.localName;
    // <set>/<animate> can assign a javascript: URL to a link or handler at runtime.
    if (local === 'set' || local === 'animate') {
      const target = (element.getAttribute('attributeName') ?? '').toLowerCase();
      if (target === 'href' || target === 'xlink:href' || target.startsWith('on')) {
        element.remove();
        continue;
      }
    }
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      if (name.startsWith('on')) {
        element.removeAttribute(attribute.name);
      } else if (URL_ATTRIBUTES.includes(name) && isScriptUrl(attribute.value)) {
        element.removeAttribute(attribute.name);
      }
    }
  }
}

/** Clean an SVG image file. Bundled diagrams do not use <foreignObject>, so it is removed. */
export function sanitizeSvg(source: string): string {
  const doc = new DOMParser().parseFromString(source, 'image/svg+xml');
  const root = doc.documentElement;
  if (!root || root.localName !== 'svg' || doc.querySelector('parsererror')) {
    throw new Error('invalid svg');
  }
  scrub(root, true);
  return new XMLSerializer().serializeToString(doc);
}

/**
 * Clean Mermaid output before it is inserted into the page. Mermaid serializes as HTML (not XML)
 * and draws labels with HTML inside <foreignObject>, so that is kept and only its content is cleaned.
 */
export function sanitizeMermaidSvg(source: string): string {
  const body = new DOMParser().parseFromString(source, 'text/html').body;
  const root = body.firstElementChild;
  if (!root || root.localName !== 'svg' || body.children.length !== 1) {
    throw new Error('invalid svg');
  }
  scrub(root, false);
  return root.outerHTML;
}
