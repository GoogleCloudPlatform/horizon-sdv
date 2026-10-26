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
import { Box, Tab, Tabs, Typography } from '@mui/material';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

const TOOL_TABS = [
  { name: 'Horizon CLI', value: 'horizonCli', path: '/tools/horizon-cli' },
  { name: 'Horizon API', value: 'horizonApi', path: '/tools/horizon-api' },
] as const;

export function ToolsPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const tab =
    TOOL_TABS.find((item) => location.pathname === item.path)?.value ?? TOOL_TABS[0].value;

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Tools
      </Typography>
      <Tabs
        value={tab}
        onChange={(_, value: (typeof TOOL_TABS)[number]['value']) => {
          const next = TOOL_TABS.find((item) => item.value === value);
          if (next) {
            navigate(next.path);
          }
        }}
        sx={{ mb: 2 }}
      >
        {TOOL_TABS.map((item) => (
          <Tab key={item.value} label={item.name} value={item.value} />
        ))}
      </Tabs>
      <Outlet />
    </Box>
  );
}
