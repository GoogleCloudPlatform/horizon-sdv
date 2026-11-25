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
const ACTIVE_ELEMENTS = ['script', 'iframe', 'embed', 'object', 'audio', 'video'];
// Embedded raster pictures (data:image/png;base64,...) are normal in exported diagrams and stay.
// Everything else with a script-capable scheme goes: javascript:, vbscript:, other data: types.
const SAFE_DATA_URL = /^\s*data:image\/(png|jpe?g|gif|webp);/i;
function isScriptUrl(value: string): boolean {
  return /^\s*(javascript|vbscript|data):/i.test(value) && !SAFE_DATA_URL.test(value);
}
const URL_ATTRIBUTES = ['href', 'xlink:href', 'src', 'action', 'formaction'];

export function sanitizeSvg(source: string): string {
  const doc = new DOMParser().parseFromString(source, 'image/svg+xml');
  const root = doc.documentElement;
  if (!root || root.localName !== 'svg' || doc.querySelector('parsererror')) {
    throw new Error('invalid svg');
  }

  for (const tag of ACTIVE_ELEMENTS) {
    for (const element of Array.from(doc.getElementsByTagNameNS('*', tag))) {
      element.remove();
    }
  }

  for (const element of Array.from(doc.getElementsByTagNameNS('*', '*'))) {
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

  return new XMLSerializer().serializeToString(doc);
}
