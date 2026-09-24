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
import type { ReactNode } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Link as MuiLink,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import { alpha, useTheme } from '@mui/material/styles';
import AccountTreeIcon from '@mui/icons-material/AccountTree';
import AdminPanelSettingsIcon from '@mui/icons-material/AdminPanelSettings';
import CloudQueueIcon from '@mui/icons-material/CloudQueue';
import RocketLaunchIcon from '@mui/icons-material/RocketLaunch';
import ViewModuleIcon from '@mui/icons-material/ViewModule';
import { Link } from 'react-router-dom';
import { HORIZON_LOGO_SRC } from '../constants';
import { authService } from '../utils/auth';
import BuildIcon from '@mui/icons-material/Build';

const HORIZON_SDV_REPO = 'https://github.com/GoogleCloudPlatform/horizon-sdv';
const DEPLOYMENT_GUIDE_URL =
  'https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/deployment_guide.md';

type ConceptCard = {
  icon: ReactNode;
  title: string;
  body: string;
};

const CONCEPT_CARDS: ConceptCard[] = [
  {
    icon: <CloudQueueIcon sx={{ fontSize: 40, color: 'primary.main' }} aria-hidden />,
    title: 'Horizon & SDV',
    body: 'Horizon is a cloud-hosted toolchain for building, testing, and releasing complex embedded software in the automotive SDV space. The goal is simple: the platform should not be your differentiator—your product should.',
  },
  {
    icon: <ViewModuleIcon sx={{ fontSize: 40, color: 'primary.main' }} aria-hidden />,
    title: 'This developer portal',
    body: 'You are signed in to a small SPA that talks to Module Manager and the Horizon API. When a module is enabled and healthy, it reaches READY and appears in the sidebar for day-to-day use.',
  },
  {
    icon: <AccountTreeIcon sx={{ fontSize: 40, color: 'primary.main' }} aria-hidden />,
    title: 'Modules & workflows',
    body: 'Under Administration → Modules, turn workloads on or off. Administration → Settings holds global options such as workflow visibility by submit source. Each READY module exposes overview, workflow templates, running workflows, and history.\n\nDo not reload or close the browser while a module enable, disable, or Git ref apply is running. A reload can cancel the in-flight request while the cluster may still be changing.',
  },
  {
    icon: <RocketLaunchIcon sx={{ fontSize: 40, color: 'primary.main' }} aria-hidden />,
    title: 'Platform delivery',
    body: 'Horizon environments are typically stood up with Terraform and kept in sync with Argo CD. That pattern keeps clusters, services, and GitOps changes traceable and repeatable.',
  },
];

export function WelcomePage() {
  const theme = useTheme();
  const username = authService.getUsername();
  const isDark = theme.palette.mode === 'dark';

  const heroGradient = isDark
    ? `linear-gradient(135deg, ${alpha(theme.palette.primary.dark, 0.42)} 0%, ${alpha(
        theme.palette.background.paper,
        0.92
      )} 52%, ${theme.palette.background.paper} 100%)`
    : `linear-gradient(135deg, ${alpha(theme.palette.primary.light, 0.28)} 0%, ${alpha(
        theme.palette.primary.main,
        0.1
      )} 48%, ${theme.palette.background.paper} 100%)`;

  return (
    <Stack spacing={2}>
      <Paper
        elevation={0}
        sx={{
          overflow: 'hidden',
          border: 1,
          borderColor: 'divider',
          background: heroGradient,
        }}
      >
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={3}
          alignItems={{ xs: 'flex-start', md: 'center' }}
          sx={{ p: { xs: 3, sm: 4 } }}
        >
          <Box
            component="img"
            src={HORIZON_LOGO_SRC}
            alt="Horizon"
            sx={{ height: { xs: 56, sm: 72 }, width: 'auto', flexShrink: 0 }}
          />
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: 1 }}>
              Software-defined vehicle toolchain
            </Typography>
            <Typography
              variant="h4"
              component="h1"
              fontWeight={700}
              gutterBottom
              sx={{ fontSize: { xs: 20, md: 30 } }}
            >
              Welcome{username ? `, ${username}` : ''}
            </Typography>
            <Typography variant="body1" color="text.secondary" sx={{ width: 'auto' }}>
              Explore Horizon modules, kick off workflows, and tune how work appears in this
              cluster—all from one place after sign-in.
            </Typography>
          </Box>
        </Stack>
      </Paper>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} flexWrap="wrap" useFlexGap>
        <Button
          component={Link}
          to="/admin/modules"
          variant="contained"
          size="large"
          startIcon={<AdminPanelSettingsIcon />}
        >
          Open Administration → Modules
        </Button>
        <Button
          component={Link}
          to="/admin/settings"
          variant="outlined"
          size="large"
          startIcon={<AdminPanelSettingsIcon />}
        >
          Administration → Settings
        </Button>
        <Button
          component={Link}
          to="/tools"
          variant="outlined"
          size="large"
          startIcon={<BuildIcon />}
        >
          Tools
        </Button>
      </Stack>

      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: {
            xs: '1fr',
            sm: 'repeat(2, 1fr)',
          },
        }}
      >
        {CONCEPT_CARDS.map((card) => (
          <Card key={card.title} sx={{ height: '100%' }}>
            <CardContent>
              <Stack spacing={1.5}>
                {card.icon}
                <Typography variant="h6" component="h2">
                  {card.title}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-line' }}>
                  {card.body}
                </Typography>
              </Stack>
            </CardContent>
          </Card>
        ))}
      </Box>

      <Paper variant="outlined" sx={{ p: 1.5 }}>
        <Stack spacing={1}>
          <Typography variant="subtitle2" color="text.secondary">
            Learn more
          </Typography>
          <Typography variant="body2" color="text.secondary">
            New to the program? The public repo has the full story, contribution guide, and
            deployment walkthrough.
          </Typography>
          <Stack direction="row" flexWrap="wrap" gap={2} sx={{ pt: 0.5 }}>
            <MuiLink href={HORIZON_SDV_REPO} target="_blank" rel="noopener noreferrer">
              Horizon SDV on GitHub
            </MuiLink>
            <MuiLink href={DEPLOYMENT_GUIDE_URL} target="_blank" rel="noopener noreferrer">
              Deployment guide
            </MuiLink>
          </Stack>
        </Stack>
      </Paper>
    </Stack>
  );
}
