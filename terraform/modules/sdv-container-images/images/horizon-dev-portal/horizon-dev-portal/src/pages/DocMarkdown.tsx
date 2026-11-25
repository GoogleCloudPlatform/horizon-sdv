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
import { memo, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Box } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Link } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import type { Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeRaw from 'rehype-raw';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import rehypeSlug from 'rehype-slug';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import css from 'highlight.js/lib/languages/css';
import diff from 'highlight.js/lib/languages/diff';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import go from 'highlight.js/lib/languages/go';
import groovy from 'highlight.js/lib/languages/groovy';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import markdown from 'highlight.js/lib/languages/markdown';
import plaintext from 'highlight.js/lib/languages/plaintext';
import python from 'highlight.js/lib/languages/python';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';
import 'highlight.js/styles/github-dark.min.css';
import { loadDocsImageUrl } from '../docs/bundleClient';
import { documentationHref } from '../docs/catalog';
import { resolveDocHref } from '../docs/docLinks';
import { renderMermaidSvg } from '../docs/mermaidRender';

hljs.registerLanguage('bash', bash);
hljs.registerLanguage('css', css);
hljs.registerLanguage('diff', diff);
hljs.registerLanguage('dockerfile', dockerfile);
hljs.registerLanguage('go', go);
hljs.registerLanguage('groovy', groovy);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('json', json);
hljs.registerLanguage('markdown', markdown);
hljs.registerLanguage('plaintext', plaintext);
hljs.registerLanguage('python', python);
hljs.registerLanguage('typescript', typescript);
hljs.registerLanguage('xml', xml);
hljs.registerLanguage('yaml', yaml);

const LANGUAGE_ALIAS: Record<string, string> = {
  sh: 'bash',
  shell: 'bash',
  zsh: 'bash',
  console: 'bash',
  yml: 'yaml',
  md: 'markdown',
  docker: 'dockerfile',
  js: 'javascript',
  ts: 'typescript',
  py: 'python',
  html: 'xml',
  svg: 'xml',
};

// Bundled docs are repo content, not user input. Keep the HTML the guides
// already use (tables, images, heading ids) and drop active content.
const sanitizeSchema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), 'u'],
  attributes: {
    ...defaultSchema.attributes,
    a: [...(defaultSchema.attributes?.a ?? []), 'name', 'id', 'className'],
    code: [...(defaultSchema.attributes?.code ?? []), ['className', /^language-./]],
    span: [...(defaultSchema.attributes?.span ?? []), 'className', 'style'],
    div: [...(defaultSchema.attributes?.div ?? []), 'className', 'style'],
    img: [...(defaultSchema.attributes?.img ?? []), 'width', 'height', 'style'],
    td: [...(defaultSchema.attributes?.td ?? []), 'width', 'valign', 'style', 'colSpan', 'rowSpan'],
    th: [...(defaultSchema.attributes?.th ?? []), 'width', 'valign', 'style', 'colSpan', 'rowSpan'],
    h1: [...(defaultSchema.attributes?.h1 ?? []), 'id'],
    h2: [...(defaultSchema.attributes?.h2 ?? []), 'id'],
    h3: [...(defaultSchema.attributes?.h3 ?? []), 'id'],
    h4: [...(defaultSchema.attributes?.h4 ?? []), 'id'],
    h5: [...(defaultSchema.attributes?.h5 ?? []), 'id'],
    h6: [...(defaultSchema.attributes?.h6 ?? []), 'id'],
  },
};

function nodeText(children: ReactNode): string {
  if (typeof children === 'string' || typeof children === 'number') {
    return String(children);
  }
  if (Array.isArray(children)) {
    return children.map((child) => nodeText(child)).join('');
  }
  return '';
}

function highlightedHtml(source: string, language: string | undefined): string {
  const requested = language?.toLowerCase() ?? '';
  const name = LANGUAGE_ALIAS[requested] ?? requested;
  if (name && hljs.getLanguage(name)) {
    return hljs.highlight(source, { language: name }).value;
  }
  return hljs.highlight(source, { language: 'plaintext' }).value;
}

function MermaidBlock({ source }: { source: string }) {
  const theme = useTheme();
  const dark = theme.palette.mode === 'dark';
  const [result, setResult] = useState<{
    source: string;
    dark: boolean;
    svg: string | null;
    failed: boolean;
  } | null>(null);

  useEffect(() => {
    let cancelled = false;
    const id = `doc-mermaid-${Math.random().toString(36).slice(2, 10)}`;
    renderMermaidSvg(id, source, dark)
      .then((svg) => {
        if (!cancelled) {
          setResult({ source, dark, svg, failed: false });
        }
      })
      .catch(() => {
        if (!cancelled) {
          setResult({ source, dark, svg: null, failed: true });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [source, dark]);

  const current = result && result.source === source && result.dark === dark ? result : null;
  if (current?.failed) {
    return <CodeBlock source={source} language="text" />;
  }
  if (!current?.svg) {
    return (
      <Box
        sx={{
          my: 2,
          px: 2,
          py: 3,
          borderRadius: 1,
          border: 1,
          borderColor: 'divider',
          bgcolor: 'action.hover',
          color: 'text.secondary',
          fontSize: 13,
        }}
      >
        Rendering diagram…
      </Box>
    );
  }
  return (
    <Box
      sx={{
        my: 2,
        p: 2,
        overflowX: 'auto',
        borderRadius: 1,
        border: 1,
        borderColor: 'divider',
        bgcolor: 'docs.diagramBackground',
        '& svg': { maxWidth: '100%', height: 'auto', display: 'block', mx: 'auto' },
      }}
      dangerouslySetInnerHTML={{ __html: current.svg }}
    />
  );
}

function CodeBlock({ source, language }: { source: string; language?: string }) {
  const html = highlightedHtml(source, language);
  return (
    <Box
      component="pre"
      sx={{
        my: 2,
        p: 2,
        borderRadius: 1,
        overflow: 'auto',
        bgcolor: 'docs.codeBackground',
        color: 'docs.codeText',
        fontSize: 13,
        lineHeight: 1.55,
      }}
    >
      <Box
        component="code"
        className="hljs"
        sx={{
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
          whiteSpace: 'pre',
        }}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </Box>
  );
}

function AuthImage({
  imagePath,
  alt,
  width,
  height,
}: {
  imagePath: string;
  alt: string;
  width?: number | string;
  height?: number | string;
}) {
  const [loaded, setLoaded] = useState<{ imagePath: string; url: string | null } | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    let objectUrl: string | null = null;
    loadDocsImageUrl(imagePath, ctrl.signal)
      .then((url) => {
        if (ctrl.signal.aborted) {
          URL.revokeObjectURL(url);
          return;
        }
        objectUrl = url;
        setLoaded({ imagePath, url });
      })
      .catch(() => {
        if (!ctrl.signal.aborted) {
          setLoaded({ imagePath, url: null });
        }
      });
    return () => {
      ctrl.abort();
      if (objectUrl) {
        URL.revokeObjectURL(objectUrl);
      }
    };
  }, [imagePath]);

  const current = loaded?.imagePath === imagePath ? loaded : null;
  if (!current) {
    return <Box component="span" sx={{ display: 'inline-block', minHeight: 24 }} />;
  }
  if (!current.url) {
    return (
      <Box component="span" sx={{ color: 'text.secondary', fontStyle: 'italic' }}>
        {alt || imagePath.split('/').pop()}
      </Box>
    );
  }
  return (
    <img
      src={current.url}
      alt={alt}
      width={width}
      height={height}
      style={{ maxWidth: '100%', height: 'auto' }}
    />
  );
}

/**
 * Open a bundled image in a new tab. The blob is shown through an <img> in a blank
 * page, never navigated to directly, so an SVG cannot run script in the portal origin.
 */
async function openDocsImage(imagePath: string): Promise<void> {
  const win = window.open('', '_blank');
  if (!win) {
    return;
  }
  win.opener = null;
  try {
    const url = await loadDocsImageUrl(imagePath);
    const doc = win.document;
    doc.title = imagePath.split('/').pop() ?? 'Image';
    const img = doc.createElement('img');
    img.alt = doc.title;
    img.style.maxWidth = '100%';
    img.onload = () => URL.revokeObjectURL(url);
    img.onerror = () => URL.revokeObjectURL(url);
    img.src = url;
    doc.body.style.margin = '0';
    doc.body.replaceChildren(img);
  } catch {
    win.close();
  }
}

function markdownComponents(
  docPath: string,
  bundled: ReadonlySet<string>,
  gitRef: string
): Components {
  return {
    a: ({ href, children }) => {
      if (!href) {
        return <span>{children}</span>;
      }
      const resolved = resolveDocHref(docPath, href, bundled, gitRef);
      if (resolved.kind === 'doc') {
        return <Link to={documentationHref(resolved.path, resolved.hash)}>{children}</Link>;
      }
      if (resolved.kind === 'hash') {
        return <a href={resolved.hash}>{children}</a>;
      }
      if (resolved.kind === 'asset') {
        const { imagePath } = resolved;
        return (
          <a
            href={`#${encodeURIComponent(imagePath)}`}
            onClick={(event) => {
              event.preventDefault();
              void openDocsImage(imagePath);
            }}
          >
            {children}
          </a>
        );
      }
      return (
        <a href={resolved.url} target="_blank" rel="noopener noreferrer">
          {children}
        </a>
      );
    },
    img: ({ src, alt, width, height }) => {
      if (!src || typeof src !== 'string') {
        return null;
      }
      const resolved = resolveDocHref(docPath, src, bundled, gitRef);
      if (resolved.kind === 'asset') {
        return (
          <AuthImage imagePath={resolved.imagePath} alt={alt ?? ''} width={width} height={height} />
        );
      }
      return (
        <img
          src={src}
          alt={alt ?? ''}
          width={width}
          height={height}
          style={{ maxWidth: '100%', height: 'auto' }}
        />
      );
    },
    pre: ({ children }) => <>{children}</>,
    code: ({ className, children }) => {
      const text = nodeText(children).replace(/\n$/, '');
      const language = /language-([\w-]+)/.exec(className ?? '')?.[1];
      const block = Boolean(language) || text.includes('\n');
      if (!block) {
        return <code>{text}</code>;
      }
      if (language === 'mermaid') {
        return <MermaidBlock source={text} />;
      }
      return <CodeBlock source={text} language={language} />;
    },
    table: ({ children }) => (
      <Box sx={{ overflowX: 'auto', my: 2 }}>
        <table>{children}</table>
      </Box>
    ),
  };
}

/** Parsing a large guide (release notes are ~400 KB) is expensive, so only re-render when the inputs change. */
export const DocMarkdown = memo(function DocMarkdown({
  docPath,
  markdown,
  bundled,
  gitRef,
}: {
  docPath: string;
  markdown: string;
  bundled: readonly string[];
  gitRef: string;
}) {
  const bundledSet = useMemo(() => new Set(bundled), [bundled]);
  const components = useMemo(
    () => markdownComponents(docPath, bundledSet, gitRef),
    [docPath, bundledSet, gitRef]
  );

  return (
    <Box
      sx={{
        color: 'text.primary',
        fontSize: 15,
        lineHeight: 1.65,
        '& h1, & h2, & h3, & h4': { scrollMarginTop: '88px', fontWeight: 600, lineHeight: 1.3 },
        '& h1': { fontSize: '1.75rem', mt: 0, mb: 2 },
        '& h2': { fontSize: '1.35rem', mt: 3, mb: 1 },
        '& h3': { fontSize: '1.15rem', mt: 2.5, mb: 1 },
        '& h4': { fontSize: '1rem', mt: 2, mb: 1 },
        '& p': { my: 1.25 },
        '& ul, & ol': { pl: 3, my: 1 },
        '& li': { my: 0.5 },
        '& a': { color: 'primary.main' },
        '& hr': { border: 0, borderTop: 1, borderColor: 'divider', my: 3 },
        '& blockquote': {
          m: 0,
          my: 2,
          pl: 2,
          borderLeft: 3,
          borderColor: 'divider',
          color: 'text.secondary',
        },
        '& table': { borderCollapse: 'collapse', width: '100%', minWidth: 480 },
        '& th, & td': {
          border: 1,
          borderColor: 'divider',
          px: 1.5,
          py: 1,
          verticalAlign: 'top',
          textAlign: 'left',
        },
        '& th': { bgcolor: 'action.hover' },
        '& :not(pre) > code': {
          px: 0.5,
          py: 0.25,
          borderRadius: 0.5,
          bgcolor: 'action.hover',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
          fontSize: '0.875em',
          overflowWrap: 'anywhere',
        },
        '& img': { maxWidth: '100%', height: 'auto' },
      }}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, [rehypeSanitize, sanitizeSchema], rehypeSlug]}
        components={components}
      >
        {markdown}
      </ReactMarkdown>
    </Box>
  );
});
