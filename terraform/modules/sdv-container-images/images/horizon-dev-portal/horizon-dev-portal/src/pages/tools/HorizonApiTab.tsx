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
import { Box, Card, CardContent } from '@mui/material';
import { getAppOriginBase } from '../../utils/publicPath';

/** Same-origin Swagger UI served by the portal proxy (`{publicPath}/api/horizon/swagger`). */
function swaggerSrc(): string {
  return `${getAppOriginBase()}/api/horizon/swagger`;
}

function HorizonApiTab() {
  return (
    <Card
      variant="outlined"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: 'calc(100vh - 235px)',
        overflow: 'hidden',
      }}
    >
      <CardContent
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          minHeight: 'calc(100vh - 235px)',
          p: 0,
          '&:last-child': { pb: 0 },
        }}
      >
        <Box
          component="iframe"
          title="Horizon API"
          src={swaggerSrc()}
          sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
          sx={{
            width: '100%',
            height: '100%',
            flex: 1,
            border: 0,
            bgcolor: 'background.default',
            alignSelf: 'stretch',
          }}
        />
      </CardContent>
    </Card>
  );
}

export default HorizonApiTab;
