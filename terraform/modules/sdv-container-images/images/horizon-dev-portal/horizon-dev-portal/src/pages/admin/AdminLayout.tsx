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

export function AdminLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const tab = location.pathname.includes('/admin/settings') ? 'settings' : 'modules';

  return (
    <Box>
      <Typography variant="h4" gutterBottom sx={{ overflowWrap: 'anywhere' }}>
        Administration
      </Typography>
      <Tabs
        value={tab}
        onChange={(_, v) => {
          navigate(v === 'settings' ? '/admin/settings' : '/admin/modules');
        }}
        variant="scrollable"
        scrollButtons="auto"
        allowScrollButtonsMobile
        sx={{ mb: 2 }}
      >
        <Tab label="Modules" value="modules" />
        <Tab label="Settings" value="settings" />
      </Tabs>
      <Outlet />
    </Box>
  );
}
