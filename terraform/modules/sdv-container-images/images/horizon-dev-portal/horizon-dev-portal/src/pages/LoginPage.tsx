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
import { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  TextField,
  Typography,
} from '@mui/material';
import LockOutlinedIcon from '@mui/icons-material/LockOutlined';
import { authService } from '../utils/auth';
import { HORIZON_LOGO_SRC } from '../constants';

export function LoginPage() {
  const [user, setUser] = useState('');
  const [password, setPassword] = useState('');
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [authed, setAuthed] = useState<boolean | null>(null);

  useEffect(() => {
    authService
      .init()
      .then(setAuthed)
      .catch(() => setAuthed(false));
  }, []);

  if (authed === true) {
    return <Navigate to="/" replace />;
  }
  if (authed === null) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="100vh">
        <CircularProgress />
      </Box>
    );
  }

  const onPasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr(null);
    setLoading(true);
    try {
      await authService.loginWithPassword(user, password);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'Login failed');
      setLoading(false);
    }
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        backgroundColor: 'background.default',
        p: 2,
      }}
    >
      <Card sx={{ width: '100%', maxWidth: 420, boxShadow: 6 }}>
        <CardContent sx={{ p: { xs: 2.5, sm: 4 } }}>
          <Box
            sx={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              mb: 3,
            }}
          >
            <Box
              component="img"
              src={HORIZON_LOGO_SRC}
              alt="Horizon"
              sx={{ height: 72, width: 'auto', mb: 2 }}
            />
            <Typography
              variant="h5"
              fontWeight="bold"
              gutterBottom
              align="center"
              sx={{ overflowWrap: 'anywhere' }}
            >
              Horizon Developer Portal
            </Typography>
            <Box
              sx={{
                width: 40,
                height: 40,
                borderRadius: '50%',
                backgroundColor: 'primary.main',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                mt: 1,
              }}
            >
              <LockOutlinedIcon sx={{ color: 'white', fontSize: 22 }} />
            </Box>
          </Box>

          <Divider sx={{ mb: 3 }} />

          <Button
            fullWidth
            variant="contained"
            size="large"
            onClick={() => authService.login()}
            sx={{ mb: 2, py: 1.5, whiteSpace: 'normal' }}
          >
            Sign in with SSO (Keycloak)
          </Button>

          <Typography variant="body2" color="text.secondary" align="center" sx={{ mb: 2 }}>
            Or use username and password (if enabled on the client)
          </Typography>

          <Box component="form" onSubmit={onPasswordSubmit}>
            <TextField
              fullWidth
              label="Username"
              margin="normal"
              value={user}
              onChange={(e) => setUser(e.target.value)}
              autoComplete="username"
            />
            <TextField
              fullWidth
              label="Password"
              type="password"
              margin="normal"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
            {err && (
              <Typography color="error" variant="body2" sx={{ mt: 1 }}>
                {err}
              </Typography>
            )}
            <Button
              fullWidth
              type="submit"
              variant="outlined"
              disabled={loading || !user}
              sx={{ mt: 2, py: 1.5 }}
            >
              {loading ? 'Signing in…' : 'Sign in'}
            </Button>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
