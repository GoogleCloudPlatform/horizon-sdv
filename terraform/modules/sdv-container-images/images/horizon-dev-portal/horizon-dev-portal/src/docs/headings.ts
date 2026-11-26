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
import GithubSlugger from 'github-slugger';

export type DocHeading = {
  depth: 1 | 2 | 3 | 4 | 5 | 6;
  text: string;
  id: string;
};

function decodeEntities(value: string): string {
  return value
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, ' ');
}

/** Visible heading text, with Markdown emphasis and inline HTML removed. */
export function plainHeadingText(raw: string): string {
  return decodeEntities(
    raw
      .replace(/<[^>]+>/g, '')
      .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
      .replace(/(\*\*|__)(.*?)\1/g, '$2')
      .replace(/(\*|_)(.*?)\1/g, '$2')
      .replace(/`([^`]+)`/g, '$1')
      .replace(/\s+/g, ' ')
      .trim()
  );
}

/**
 * Headings in source order. Explicit HTML ids are kept. Other headings are
 * slugged the same way rehype-slug does, so the contents list matches the page.
 */
export function extractHeadings(markdown: string): DocHeading[] {
  const slugger = new GithubSlugger();
  const headings: DocHeading[] = [];
  let inFence = false;
  // An HTML heading can span lines (<h2 id="x">\n  Title\n</h2>); collect it until it closes.
  let pendingHtml: string | null = null;
  const htmlHeading = /<h([1-6])\b([^>]*)>([\s\S]*?)<\/h\1>/gi;

  const pushHtmlHeadings = (chunk: string): boolean => {
    htmlHeading.lastIndex = 0;
    let htmlMatch: RegExpExecArray | null;
    let sawHtml = false;
    while ((htmlMatch = htmlHeading.exec(chunk)) !== null) {
      sawHtml = true;
      const depth = Number(htmlMatch[1]) as DocHeading['depth'];
      const attrs = htmlMatch[2] ?? '';
      const text = plainHeadingText(htmlMatch[3] ?? '');
      if (!text) {
        continue;
      }
      const explicit = /\bid\s*=\s*["']([^"']+)["']/i.exec(attrs);
      const id = explicit?.[1] ? explicit[1] : slugger.slug(text);
      headings.push({ depth, text, id });
    }
    return sawHtml;
  };
  const opensHtmlHeading = (chunk: string): boolean =>
    /<h[1-6]\b/i.test(chunk.replace(/`[^`]*`/g, '').replace(htmlHeading, ''));

  for (const line of markdown.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (trimmed.startsWith('```') || trimmed.startsWith('~~~')) {
      pendingHtml = null;
      inFence = !inFence;
      continue;
    }
    if (inFence) {
      continue;
    }

    if (pendingHtml !== null) {
      // A blank line ends a Markdown HTML block, so an unclosed tag cannot swallow the rest of the page.
      if (trimmed === '') {
        pendingHtml = null;
        continue;
      }
      pendingHtml += `\n${line}`;
      if (/<\/h[1-6]\s*>/i.test(line)) {
        pushHtmlHeadings(pendingHtml);
        pendingHtml = null;
      }
      continue;
    }

    const sawHtml = pushHtmlHeadings(line);
    if (opensHtmlHeading(line)) {
      pendingHtml = line.replace(htmlHeading, '');
      continue;
    }
    if (sawHtml) {
      continue;
    }

    const atx = /^(#{1,6})\s+(.+?)\s*#*\s*$/.exec(line);
    if (!atx?.[1] || !atx[2]) {
      continue;
    }
    const depth = atx[1].length as DocHeading['depth'];
    const text = plainHeadingText(atx[2]);
    if (!text) {
      continue;
    }
    headings.push({ depth, text, id: slugger.slug(text) });
  }

  return headings;
}

export function documentTitle(headings: DocHeading[], fallback: string): string {
  return headings.find((heading) => heading.depth === 1)?.text ?? fallback;
}
