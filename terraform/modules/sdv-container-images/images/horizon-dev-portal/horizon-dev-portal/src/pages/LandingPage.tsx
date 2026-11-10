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
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  CardActions,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import OpenInNewOutlinedIcon from '@mui/icons-material/OpenInNewOutlined';
import {
  ACCENTURE_LOGO_SRC,
  ARGO_CD_LOGO_SRC,
  ARGO_WORKFLOWS_LOGO_SRC,
  GERRIT_LOGO_SRC,
  GRAFANA_LOGO_SRC,
  HEADLAMP_DARK_LOGO_SRC,
  JENKINS_LOGO_SRC,
  KEYCLOAK_LOGO_SRC,
  MCP_GATEWAY_REGISTRY_LOGO_SRC,
} from '../constants.ts';
import type { ReactElement } from 'react';
import { usePortalAccess } from '../rbac/AccessProvider';
import { HoverTooltip, RoleBadge } from '../rbac/GatedControl';
import { applicationAllowed, applicationAudience, gatedTooltip } from '../rbac/portalAccess';

type ApplicationCard = {
  id: string;
  audience: 'developer' | 'admin';
  icon: ReactElement;
  title: string;
  description: string;
  action: string;
  href: string;
};

function ApplicationCardView({ card }: { card: ApplicationCard }) {
  const access = usePortalAccess();
  const audience = applicationAudience(access, card.id, card.audience);
  const allowed = applicationAllowed(access, card.id, audience);
  const tooltip = !access
    ? 'Checking your access…'
    : allowed
      ? ''
      : gatedTooltip(access, 'platform.adminApplications');

  return (
    <Card
      variant="outlined"
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-around',
      }}
    >
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns:
            audience === 'admin' ? 'auto minmax(0, 1fr) auto' : 'auto minmax(0, 1fr)',
          alignItems: 'center',
          columnGap: 1.5,
          px: 2,
          pt: 2,
        }}
      >
        <Box sx={{ display: 'flex' }}>{card.icon}</Box>
        <Typography
          component="div"
          variant="subtitle1"
          sx={{
            minWidth: 0,
            fontWeight: 500,
            lineHeight: 1.3,
            overflowWrap: 'anywhere',
          }}
        >
          {card.title}
        </Typography>
        {audience === 'admin' ? <RoleBadge label="Admin" /> : null}
      </Box>
      <CardContent>
        <Typography
          variant="caption"
          color="text.secondary"
          display="block"
          sx={{ whiteSpace: 'pre-line' }}
        >
          {card.description}
        </Typography>
      </CardContent>
      <CardActions>
        <HoverTooltip title={tooltip} fullWidth>
          <Button
            sx={{ mb: 1 }}
            component={allowed ? 'a' : 'button'}
            href={allowed ? card.href : undefined}
            target={allowed ? '_blank' : undefined}
            rel={allowed ? 'noopener noreferrer' : undefined}
            variant="contained"
            disabled={!allowed}
            startIcon={!allowed ? <LockOutlinedIcon /> : undefined}
            endIcon={allowed ? <OpenInNewOutlinedIcon /> : undefined}
            fullWidth
          >
            {card.action}
          </Button>
        </HoverTooltip>
      </CardActions>
    </Card>
  );
}

function ApplicationGrid({ cards }: { cards: ApplicationCard[] }) {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: {
          xs: '1fr',
          md: 'repeat(2, minmax(0, 1fr))',
          lg: 'repeat(3, minmax(0, 1fr))',
        },
        gap: 2,
      }}
    >
      {cards.map((card) => (
        <ApplicationCardView key={card.id} card={card} />
      ))}
    </Box>
  );
}

export function LandingPage() {
  const tab = 'applications';
  const mcpGatewayHref = location.protocol + '//mcp.' + location.hostname;

  const developerCards: ApplicationCard[] = [
    {
      id: 'gerrit',
      audience: 'developer',
      icon: (
        <Avatar
          alt="Gerrit logo"
          variant="square"
          src={GERRIT_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Gerrit',
      description: 'Code review system for Git-based version control',
      action: 'Open',
      href: '/gerrit',
    },
    {
      id: 'jenkins',
      audience: 'developer',
      icon: (
        <Avatar
          alt="Jenkins logo"
          variant="square"
          src={JENKINS_LOGO_SRC}
          sx={{ width: 48, height: 54 }}
        />
      ),
      title: 'Jenkins',
      description: 'Open source automation server for building, testing, and deploying',
      action: 'Open',
      href: '/jenkins',
    },
    {
      id: 'mtk-connect',
      audience: 'developer',
      icon: (
        <Avatar
          alt="MTK Connect logo"
          variant="square"
          src={ACCENTURE_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'MTK Connect',
      description: 'Remote access for physical and virtual testbenches',
      action: 'Open',
      href: '/mtk-connect',
    },
    {
      id: 'mcp-gateway',
      audience: 'developer',
      icon: (
        <Avatar
          alt="MCP Gateway Registry logo"
          variant="square"
          src={MCP_GATEWAY_REGISTRY_LOGO_SRC}
          sx={{
            width: 54,
            height: 54,
            bgcolor: 'transparent',
            filter: (theme) => (theme.palette.mode === 'light' ? 'invert(1)' : 'none'),
          }}
        />
      ),
      title: 'MCP Gateway Registry',
      description: 'Centralized registry and gateway for Model Context Protocol services',
      action: 'Open',
      href: mcpGatewayHref,
    },
    {
      id: 'argo-workflows',
      audience: 'developer',
      icon: (
        <Avatar
          alt="Argo Workflows logo"
          variant="square"
          src={ARGO_WORKFLOWS_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Argo Workflows',
      description: 'Open source container-native workflow engine',
      action: 'Open',
      href: '/workflows/workflows/argo-workflows',
    },
  ];

  const adminCards: ApplicationCard[] = [
    {
      id: 'keycloak',
      audience: 'admin',
      icon: (
        <Avatar
          alt="Keycloak logo"
          variant="square"
          src={KEYCLOAK_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Keycloak',
      description: 'Open source identity and access management solution',
      action: 'Open',
      href: '/auth/admin/horizon/console',
    },
    {
      id: 'argo-cd',
      audience: 'admin',
      icon: (
        <Avatar
          alt="Argo CD logo"
          variant="square"
          src={ARGO_CD_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Argo CD',
      description: 'Declarative, GitOps continuous delivery tool for Kubernetes',
      action: 'Open',
      href: '/argocd',
    },
    {
      id: 'headlamp',
      audience: 'admin',
      icon: (
        <Avatar
          alt="Headlamp logo"
          variant="square"
          src={HEADLAMP_DARK_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Headlamp',
      description: 'User-friendly Kubernetes UI',
      action: 'Open',
      href: '/headlamp/',
    },
    {
      id: 'grafana',
      audience: 'admin',
      icon: (
        <Avatar
          alt="Grafana logo"
          variant="square"
          src={GRAFANA_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Grafana',
      description:
        'Grafana is a multi-platform open source analytics and interactive visualization web application',
      action: 'Open',
      href: '/grafana',
    },
  ];

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Landing page
      </Typography>
      <Tabs
        value={tab}
        variant="scrollable"
        scrollButtons="auto"
        allowScrollButtonsMobile
        sx={{ mb: 2 }}
      >
        <Tab label="Applications" value="applications" />
      </Tabs>
      <Stack spacing={2} sx={{ mt: 4 }}>
        <Typography variant="h6" color="text.primary" sx={{ letterSpacing: 1 }}>
          Developer applications
        </Typography>
        <ApplicationGrid cards={developerCards} />
      </Stack>
      <Stack spacing={2} sx={{ mt: 4 }}>
        <Typography variant="h6" color="text.primary" sx={{ letterSpacing: 1 }}>
          Admin applications
        </Typography>
        <ApplicationGrid cards={adminCards} />
      </Stack>
    </Box>
  );
}
