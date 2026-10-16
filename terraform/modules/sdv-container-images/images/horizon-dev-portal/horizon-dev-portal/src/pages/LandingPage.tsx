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
  CardHeader,
  CardContent,
  CardActions,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import OpenInNewOutlinedIcon from '@mui/icons-material/OpenInNewOutlined';
import {
  ACCENTURE_LOGO_SRC,
  ARGOCD_LOGO_SRC,
  GERRIT_LOGO_SRC,
  GRAFANA_LOGO_SRC,
  HEADLAMP_DARK_LOGO_SRC,
  JENKINS_LOGO_SRC,
  KEYCLOAK_LOGO_SRC,
  MCP_GATEWAY_REGISTRY_LOGO_SRC,
} from '../constants.ts';
import type { ReactElement } from 'react';

type ApplicationCard = {
  icon: ReactElement;
  title: string;
  description: string;
  action: string;
  href: string;
};

export function LandingPage() {
  const tab = 'applications';

  const mcpGatewayHref = location.protocol + '//mcp.' + location.hostname;

  const APPLICATION_CARDS: ApplicationCard[] = [
    {
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
      icon: (
        <Avatar
          alt="MCP Gateway Registry logo"
          variant="square"
          src={MCP_GATEWAY_REGISTRY_LOGO_SRC}
          sx={{
            width: 54,
            height: 54,
            bgcolor: 'transparent',
            // White-on-transparent PNG: invert so strokes are dark in light mode.
            filter: (theme) => (theme.palette.mode === 'light' ? 'invert(1)' : 'none'),
          }}
        />
      ),
      title: 'MCP Gateway Registry',
      description: 'Centralized registry and gateway for Model Context Protocol services',
      action: 'Open',
      href: mcpGatewayHref,
    },
  ];

  const ADMIN_APPLICATION_CARDS: ApplicationCard[] = [
    {
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
      icon: (
        <Avatar
          alt="Argo CD logo"
          variant="square"
          src={ARGOCD_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Argo CD',
      description: 'Declarative, GitOps continuous delivery tool for Kubernetes',
      action: 'Open',
      href: '/argocd',
    },
    {
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
    {
      // as icon is the same as Argo CD, we use the same logo
      icon: (
        <Avatar
          alt="Argo Workflows logo"
          variant="square"
          src={ARGOCD_LOGO_SRC}
          sx={{ width: 54, height: 54 }}
        />
      ),
      title: 'Argo Workflows',
      description: 'Open source container-native workflow engine',
      action: 'Open',
      href: '/workflows/workflows/argo-workflows',
    },
];

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Landing page
      </Typography>
      <Tabs value={tab} sx={{ mb: 2 }}>
        <Tab label="Applications" value="applications" />
      </Tabs>
      {tab === 'applications' && (
        <>
          <Stack spacing={2} sx={{ mt: 4 }}>
            <Typography variant="h6" color="text.primary" sx={{ letterSpacing: 1 }}>
              Developer applications
            </Typography>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', sm: 'repeat(2,1fr)', md: 'repeat(3,1fr)' },
                gap: 2,
              }}
            >
              {APPLICATION_CARDS.map((e) => (
                <Card
                  key={e.title}
                  variant="outlined"
                  sx={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    justifyContent: 'space-around',
                  }}
                >
                  <CardHeader avatar={e.icon} title={e.title} disableTypography={true}></CardHeader>
                  <CardContent>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{
                        whiteSpace: 'pre-line',
                      }}
                    >
                      {e.description}
                    </Typography>
                  </CardContent>
                  <CardActions>
                    <Button
                      sx={{ mb: 1 }}
                      component="a"
                      href={e.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      variant="contained"
                      endIcon={<OpenInNewOutlinedIcon />}
                      fullWidth
                    >
                      {e.action}
                    </Button>
                  </CardActions>
                </Card>
              ))}
            </Box>
          </Stack>
          <Stack spacing={2} sx={{ mt: 4 }}>
            <Typography variant="h6" color="text.primary" sx={{ letterSpacing: 1 }}>
              Admin applications
            </Typography>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', sm: 'repeat(2,1fr)', md: 'repeat(3,1fr)' },
                gap: 2,
              }}
            >
              {ADMIN_APPLICATION_CARDS.map((e) => (
                <Card
                  key={e.title}
                  variant="outlined"
                  sx={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    justifyContent: 'space-around',
                  }}
                >
                  <CardHeader avatar={e.icon} title={e.title} disableTypography={true}></CardHeader>
                  <CardContent>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{ whiteSpace: 'pre-line' }}
                    >
                      {e.description}
                    </Typography>
                  </CardContent>
                  <CardActions>
                    <Button
                      sx={{ mb: 1 }}
                      component="a"
                      href={e.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      variant="contained"
                      endIcon={<OpenInNewOutlinedIcon />}
                      fullWidth
                    >
                      {e.action}
                    </Button>
                  </CardActions>
                </Card>
              ))}
            </Box>
          </Stack>
        </>
      )}
    </Box>
  );
}
