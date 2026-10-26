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
import { useState } from 'react';
import HorizonCliTab from './tools/HorizonCliTab.tsx';

export function ToolsPage() {
  const TOOL_APPLICATIONS = [
    {
      name: 'Horizon CLI',
      value: 'horizonCli',
      jsx: <HorizonCliTab />,
    },
  ];

  const [tab, setTab] = useState(TOOL_APPLICATIONS[0].value);

  const currentTab = TOOL_APPLICATIONS.find((app) => app.value === tab) || TOOL_APPLICATIONS[0];

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Tools
      </Typography>
      <Tabs value={tab} onChange={(_, value) => setTab(value)} sx={{ mb: 2 }}>
        {TOOL_APPLICATIONS.map((app) => (
          <Tab key={app.value} label={app.name} value={app.value} />
        ))}
      </Tabs>
      {currentTab.jsx}
    </Box>
  );
}
