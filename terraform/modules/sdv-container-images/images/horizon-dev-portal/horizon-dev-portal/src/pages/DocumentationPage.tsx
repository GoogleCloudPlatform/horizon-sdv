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
import { useEffect, useState } from 'react';
import {
  Alert,
  Box,
  CircularProgress,
  List,
  ListItemButton,
  Paper,
  TextField,
  Typography,
} from '@mui/material';
import { alpha, useTheme } from '@mui/material/styles';
import { Link, Navigate, useParams } from 'react-router-dom';
import OpenInNewOutlinedIcon from '@mui/icons-material/OpenInNewOutlined';
import { config } from '../utils/config';
import {
  DEFAULT_GUIDE_PATH,
  DOC_CATEGORIES,
  PORTAL_GUIDES,
  documentationHref,
  guideKey,
  guideForPath,
  guideMatchesQuery,
  isExternalGuide,
  sectionForPath,
  type PortalGuide,
} from '../docs/catalog';
import { loadBundledDoc, loadBundledManifest } from '../docs/bundleClient';
import { horizonGitRef, normalizeRepoPath } from '../docs/docLinks';
import { documentTitle, extractHeadings, type DocHeading } from '../docs/headings';
import { DocMarkdown } from './DocMarkdown';

type LoadState =
  | { status: 'missing'; docPath: string }
  | { status: 'error'; docPath: string; message: string }
  | {
      status: 'ready';
      docPath: string;
      markdown: string;
      bundled: string[];
      headings: DocHeading[];
    };

const NO_HEADINGS: DocHeading[] = [];

function decodeDocPath(splat: string | undefined): string {
  if (!splat) {
    return '';
  }
  try {
    return splat
      .split('/')
      .map((segment) => decodeURIComponent(segment))
      .join('/');
  } catch {
    return splat;
  }
}

const SCREEN_READER_ONLY = {
  position: 'absolute',
  width: '1px',
  height: '1px',
  p: 0,
  m: '-1px',
  overflow: 'hidden',
  clip: 'rect(0, 0, 0, 0)',
  whiteSpace: 'nowrap',
  border: 0,
} as const;

function GuideRow({ guide, active }: { guide: PortalGuide; active: boolean }) {
  const theme = useTheme();
  const dark = theme.palette.mode === 'dark';
  const chip = DOC_CATEGORIES[guide.category];
  const external = isExternalGuide(guide);
  const color = external ? theme.palette.docs.chipExternal : theme.palette.docs.chip;

  const rowSx = {
    display: 'block',
    borderRadius: 0,
    borderLeft: '3px solid',
    borderLeftColor: active ? 'primary.main' : 'transparent',
    borderBottom: '1px solid',
    borderBottomColor: active ? 'transparent' : 'divider',
    bgcolor: active ? alpha(theme.palette.primary.main, dark ? 0.18 : 0.08) : 'transparent',
    py: 1.25,
    px: 1.5,
    '&.Mui-selected, &.Mui-selected:hover': {
      bgcolor: alpha(theme.palette.primary.main, dark ? 0.24 : 0.1),
    },
  };

  const content = (
    <>
      <Box
        sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 1 }}
      >
        <Typography
          sx={{
            fontWeight: active ? 700 : 600,
            fontSize: 14,
            color: active ? 'primary.main' : 'text.primary',
            lineHeight: 1.3,
          }}
        >
          {guide.title}
          {external ? (
            <Box component="span" sx={SCREEN_READER_ONLY}>
              {' (opens in a new tab)'}
            </Box>
          ) : null}
        </Typography>
        {external ? (
          <OpenInNewOutlinedIcon
            aria-hidden="true"
            sx={{ fontSize: 16, mt: '1px', flexShrink: 0, color: 'text.secondary' }}
          />
        ) : null}
      </Box>
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{
          display: '-webkit-box',
          WebkitLineClamp: 2,
          WebkitBoxOrient: 'vertical',
          overflow: 'hidden',
          mt: 0.5,
          fontSize: 12,
          lineHeight: 1.4,
        }}
      >
        {guide.summary}
      </Typography>
      <Box
        component="span"
        sx={{
          display: 'inline-block',
          mt: 0.75,
          px: 0.75,
          py: 0.125,
          borderRadius: 0.5,
          bgcolor: alpha(color, dark ? 0.2 : 0.1),
          color,
          fontSize: 10,
          fontWeight: 700,
          letterSpacing: 0.4,
          lineHeight: 1.6,
        }}
      >
        {chip.label}
      </Box>
    </>
  );

  // External entries are plain links to another application: they never select the reader.
  if (external) {
    return (
      <ListItemButton
        component="a"
        href={guide.url}
        target="_blank"
        rel="noopener noreferrer"
        sx={rowSx}
      >
        {content}
      </ListItemButton>
    );
  }
  return (
    <ListItemButton
      component={Link}
      to={documentationHref(guide.path)}
      selected={active}
      data-active={active ? 'true' : 'false'}
      sx={rowSx}
    >
      {content}
    </ListItemButton>
  );
}

/** Owns the filter text so typing re-renders only the list, not the open guide. */
function GuideList({ activePath }: { activePath: string | undefined }) {
  const [query, setQuery] = useState('');
  const visibleGuides = PORTAL_GUIDES.filter(
    (guide) =>
      (!isExternalGuide(guide) && guide.path === activePath) || guideMatchesQuery(guide, query)
  );

  return (
    <Paper
      variant="outlined"
      sx={{
        position: { md: 'sticky' },
        top: { md: 88 },
        maxHeight: { xs: 360, md: 'calc(100vh - 120px)' },
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      <TextField
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="Filter documents ..."
        aria-label="Filter documents"
        size="small"
        fullWidth
        sx={{ p: 1.5, pb: 1, flexShrink: 0 }}
      />
      <List
        component="nav"
        aria-label="Guides"
        dense
        sx={{ overflow: 'auto', p: 0, borderTop: 1, borderColor: 'divider' }}
      >
        {visibleGuides.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ px: 1, py: 2 }}>
            No documents match.
          </Typography>
        ) : (
          visibleGuides.map((guide) => (
            <GuideRow
              key={guideKey(guide)}
              guide={guide}
              active={!isExternalGuide(guide) && activePath === guide.path}
            />
          ))
        )}
      </List>
    </Paper>
  );
}

function ContentsList({ headings }: { headings: DocHeading[] }) {
  const visible = headings.filter((heading) => heading.depth === 2 || heading.depth === 3);
  const dark = useTheme().palette.mode === 'dark';
  if (visible.length === 0) {
    return null;
  }
  return (
    <Box
      component="nav"
      aria-label="On this page"
      sx={{
        p: 1.5,
        border: 1,
        borderColor: 'docs.contentsBorder',
        borderRadius: 1,
        bgcolor: dark ? 'background.default' : 'grey.50',
        boxShadow: dark
          ? '0 1px 3px rgba(0,0,0,.5), 0 1px 2px rgba(0,0,0,.35)'
          : '0 1px 3px rgba(60,64,67,.2), 0 1px 2px rgba(60,64,67,.12)',
      }}
    >
      <Typography
        variant="caption"
        sx={{
          display: 'block',
          fontSize: 10,
          fontWeight: 700,
          letterSpacing: 0.8,
          color: 'text.secondary',
          mb: 0.75,
        }}
      >
        ON THIS PAGE
      </Typography>
      <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {visible.map((heading) => (
          <Box component="li" key={`${heading.depth}-${heading.id}`} sx={{ my: 0.5 }}>
            <Typography
              component="a"
              href={`#${heading.id}`}
              variant="body2"
              sx={{
                display: 'block',
                pl: heading.depth === 3 ? 1.5 : 0,
                color: 'docs.contentsLink',
                textDecoration: 'none',
                fontSize: 12,
                lineHeight: 1.4,
                '&:hover': { textDecoration: 'underline' },
              }}
            >
              {heading.text}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

export function DocumentationPage() {
  const params = useParams();
  const docPath = normalizeRepoPath(decodeDocPath(params['*']));
  const [load, setLoad] = useState<LoadState | null>(null);
  const horizonVersion = config.getString('horizonVersion', '');
  const gitRef = horizonGitRef(horizonVersion);

  const exact = guideForPath(docPath);
  const section = sectionForPath(docPath);

  useEffect(() => {
    if (!docPath) {
      return;
    }
    const ctrl = new AbortController();
    loadBundledManifest()
      .then(async (bundled): Promise<LoadState> => {
        if (!bundled.includes(docPath)) {
          return { status: 'missing', docPath };
        }
        const markdown = await loadBundledDoc(docPath, ctrl.signal);
        return { status: 'ready', docPath, markdown, bundled, headings: extractHeadings(markdown) };
      })
      .then((next) => {
        if (!ctrl.signal.aborted) {
          setLoad(next);
        }
      })
      .catch((error: unknown) => {
        if (ctrl.signal.aborted) {
          return;
        }
        const message = error instanceof Error ? error.message : 'Could not load this guide.';
        if (message === 'missing') {
          setLoad({ status: 'missing', docPath });
          return;
        }
        setLoad({ status: 'error', docPath, message });
      });
    return () => ctrl.abort();
  }, [docPath]);

  const current = load?.docPath === docPath ? load : null;
  const headings = current?.status === 'ready' ? current.headings : NO_HEADINGS;
  const pageTitle =
    exact?.title ??
    (current?.status === 'ready'
      ? documentTitle(headings, docPath)
      : docPath.split('/').pop() || 'Documentation');
  const versionLabel = horizonVersion
    ? `bundled with Horizon ${horizonVersion}`
    : 'bundled with this deployment';

  useEffect(() => {
    if (current?.status !== 'ready') {
      return;
    }
    const hash = window.location.hash.replace(/^#/, '');
    if (!hash) {
      window.scrollTo({ top: 0 });
      return;
    }
    const target = document.getElementById(decodeURIComponent(hash));
    target?.scrollIntoView();
  }, [current, docPath]);

  useEffect(() => {
    const row = document.querySelector<HTMLElement>('[data-active="true"]');
    row?.scrollIntoView({ block: 'nearest' });
  }, [docPath]);

  return (
    <Box>
      <Typography variant="h4" component="h1" gutterBottom>
        Documentation
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Guides bundled with this deployment — always matching your installed Horizon version.
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '300px minmax(0, 1fr)' },
          gap: 2,
          alignItems: 'start',
        }}
      >
        <GuideList activePath={section?.path} />

        <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 }, minHeight: 480 }}>
          {!docPath ? (
            <Navigate to={documentationHref(DEFAULT_GUIDE_PATH)} replace />
          ) : current?.status === 'missing' ? (
            <Alert severity="warning">
              This document is not in the bundle for this deployment.{' '}
              <Link to={documentationHref(DEFAULT_GUIDE_PATH)}>Open the deployment guide</Link>
            </Alert>
          ) : current?.status === 'error' ? (
            <Alert severity="error">{current.message}</Alert>
          ) : current?.status !== 'ready' ? (
            <Box display="flex" justifyContent="center" py={8}>
              <CircularProgress />
            </Box>
          ) : (
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1fr) 200px' },
                gap: 3,
                alignItems: 'start',
              }}
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography
                  variant="h4"
                  component="h2"
                  sx={{ fontSize: { xs: 26, sm: 32 }, fontWeight: 700 }}
                >
                  {pageTitle}
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{
                    mt: 0.5,
                    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                    fontSize: 12,
                    wordBreak: 'break-word',
                  }}
                >
                  {docPath}
                  {' · '}
                  {versionLabel}
                </Typography>
                {section && !exact ? (
                  <Typography variant="body2" sx={{ mt: 1 }}>
                    Part of <Link to={documentationHref(section.path)}>{section.title}</Link>
                  </Typography>
                ) : null}
                <Box sx={{ display: { xs: 'block', lg: 'none' }, mt: 2 }}>
                  <ContentsList headings={headings} />
                </Box>
                <Box sx={{ mt: 2 }}>
                  <DocMarkdown
                    docPath={docPath}
                    markdown={current.markdown}
                    bundled={current.bundled}
                    gitRef={gitRef}
                  />
                </Box>
              </Box>
              <Box
                sx={{
                  display: { xs: 'none', lg: 'block' },
                  position: 'sticky',
                  top: 88,
                  maxHeight: 'calc(100vh - 120px)',
                  overflow: 'auto',
                }}
              >
                <ContentsList headings={headings} />
              </Box>
            </Box>
          )}
        </Paper>
      </Box>
    </Box>
  );
}
