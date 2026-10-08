/*
 #############################################################################
 # Copyright (c) 2024-2026 Accenture, All Rights Reserved.
 #
 # Licensed under the Apache License, Version 2.0 (the "License");
 # you may not use this file except in compliance with the License.
 # You may obtain a copy of the License at
 #
 #         http://www.apache.org/licenses/LICENSE-2.0
 #
 # Unless required by applicable law or agreed to in writing, software
 # distributed under the License is distributed on an "AS IS" BASIS,
 # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 # See the License for the specific language governing permissions and
 # limitations under the License.
 #
 # -----------------------------------------------------
 # http://www.accenture.com
 # -----------------------------------------------------
 # Description:
 #
 # Automates the setup and configuration of an MTK Connect agent and
 # testbench/device creation.
 #
 # 1. It loads environment variables from a .env file using dotenv.
 # 2. It sets up axios with a base URL and authentication credentials
 #    from the environment.
 # 3. It calls:
 #    - configureAgent which:
 #      - Checks if an agent with a specific registration already exists.
 #      - If not, creates a new agent and sets its permissions.
 #    - configureDevices:
 #      - Creates or updates devices associated with the agent.
 #      - Configures device interfaces (e.g., ADB, button, exec, file system,
 #        MJPEG, terminal, touch, and tunnel).
 #
 #############################################################################
 */

'use strict';

require('dotenv').config();

const fs = require('fs');
const BPromise = require('bluebird');
const axios = require('axios');
const _ = require("lodash");

/**
 * Sets up the Axios configuration with a base URL and authentication
 * credentials from environment variables.
 */
const { MTK_CONNECT_DOMAIN, MTK_CONNECT_USERNAME, MTK_CONNECT_PASSWORD, MTK_CONNECT_REGISTRATION, MTK_CONNECT_TESTBENCH, MTK_CONNECT_TESTBENCH_USER, MTK_CONNECT_DEVICES, MTK_CONNECT_HOST_LIST, MTK_CONNECT_LAUNCH_APPLICATION_NAME, MTK_CONNECT_HOST_ONLY, MTK_CONNECT_DEVICE_PREFIX, MTK_CONNECT_DEVICE_NAME_LIST, MTK_CONNECT_HOST_PORT_LIST, MTK_CONNECT_TERMINAL_USER, MTK_CONNECT_TUNNEL_LIST, MTK_CONNECT_TUNNEL_DEVICE_NAME} = process.env;
const registration = MTK_CONNECT_REGISTRATION || fs.readFileSync('/usr/src/config/registration.name', 'utf-8');

/** Tunnel caller port (ADB); env MTK_CONNECT_TUNNEL_PORT from Jenkins/shell, default 8555 */
const mtkTunnelPort = (() => {
  const raw = process.env.MTK_CONNECT_TUNNEL_PORT;
  const n = parseInt(raw != null && raw !== '' ? raw : '8555', 10);
  return Number.isFinite(n) && n > 0 && n <= 65535 ? n : 8555;
})();

// MTK_CONNECT_TUNNEL_DEVICE_NAME: when adb devices are in play (HOST_ONLY
// false) and there are raw TCP tunnels, carry the tunnels on a dedicated
// host-only device with this name at index 1 instead of riding along on the
// first adb device; the adb devices then occupy indexes 2..N+1.
const tunnelDeviceName = (MTK_CONNECT_HOST_ONLY == 'false' && MTK_CONNECT_TUNNEL_LIST && MTK_CONNECT_TUNNEL_DEVICE_NAME) || '';
const deviceCount = parseInt(MTK_CONNECT_DEVICES, 10) + (tunnelDeviceName ? 1 : 0);

axios.defaults.baseURL = `https://${MTK_CONNECT_DOMAIN}/mtk-connect`;
axios.defaults.auth = {
  username: MTK_CONNECT_USERNAME,
  password: MTK_CONNECT_PASSWORD
};

/**
 * The agent object that is created or retrieved by the code.
 */
let agent;

/**
 * Configures the agent by creating a new one or retrieving an existing one
 * with the registration name.
 */
async function configureAgent() {
  let user;
  let group;
  let rsp = await axios.get('/api/v1/agents', {params: {q: JSON.stringify({registration: registration})}})
  if (rsp.status === 200 && rsp.data.data.length === 1) {
    console.log(`agent with registration ${registration} already exists`);
    agent = rsp.data.data[0];
  } else {
    console.log(`creating agent using registration ${registration}`);
    rsp = await axios.post('/api/v1/agents', {
      name: MTK_CONNECT_TESTBENCH,
      registration: registration
    })
    agent = rsp.data.data;
    if (MTK_CONNECT_TESTBENCH_USER === 'everyone') {
      console.log(`Using group permissions.`);
      rsp = await axios.get('/api/v1/groups', {params: {fields:'id,name', q: JSON.stringify({name: 'everyone'})}});
      group = rsp.data.data[0];
      await axios.put(`/api/v1/agents/${agent.id}/permissions/group`, {group: {id: group.id}, permission: 'book'});
    } else {
      console.log(`Using user permissions ${MTK_CONNECT_TESTBENCH_USER}.`);
      rsp = await axios.get('/api/v1/users', {params: {fields:'id,username', q: JSON.stringify({username: MTK_CONNECT_TESTBENCH_USER})}});
      if (rsp.data.data.length === 0) {
        console.log(`creating account for ${MTK_CONNECT_TESTBENCH_USER}`);
        rsp = await axios.post('/api/v1/users', {username: MTK_CONNECT_TESTBENCH_USER, authentication: 'saml'});
        user = rsp.data.data;
      } else {
        user = rsp.data.data[0];
      }
      await axios.put(`/api/v1/agents/${agent.id}/permissions/user`, {user: {id: user.id}, permission: 'book'});
    }
  }
  console.log(`Created agent using registration ${registration}`);
}

/**
 * Configures the devices by creating them if they don't already exist.
 */
async function configureDevices() {
  await BPromise.mapSeries(_.times(deviceCount), configureDevice)
}

/**
 * Configures a single device by retrieving it or creating a new one.
 * @param {number} i - The index of the device to configure.
 */
async function configureDevice(i) {
  const index = i + 1;
  // With a dedicated tunnel device at index 1, the adb host/port/name lists
  // are offset by one: adb device N lives at agent index N+1.
  const isTunnelDevice = tunnelDeviceName && index === 1;
  const adbIndex = tunnelDeviceName ? index - 1 : index;
  const q = {
    'agent.registration': registration,
    index: index
  }

  console.log(`device ${index} ... `);

  // Device display name: tunnel device name for the dedicated tunnel device,
  // else per-index MTK_CONNECT_DEVICE_NAME_LIST entry when provided, else
  // "<prefix> <index>".
  const deviceNames = (MTK_CONNECT_DEVICE_NAME_LIST || '').split(',');
  const deviceName = isTunnelDevice
    ? tunnelDeviceName
    : deviceNames[adbIndex - 1] || `${MTK_CONNECT_DEVICE_PREFIX} ${index}`;

  const rsp = await axios.get('/api/v1/devices', {params: {q: JSON.stringify(q)}})
  if (rsp.status === 200 && rsp.data.data.length === 1) {
    console.log(`device ${index} already exists`);
  } else {
    console.log(`creating device ${index}`);
    await axios.post(`/api/v1/agents/${agent.id}/devices`, {
      name: deviceName
    });
  }

  const adbPorts = MTK_CONNECT_HOST_PORT_LIST.split(",");
  const adbHosts = MTK_CONNECT_HOST_LIST.split(",");

  console.log(adbPorts);
  console.log(adbHosts);
  if (!isTunnelDevice) {
    console.log(+adbPorts[adbIndex - 1]);
    console.log(adbHosts[adbIndex - 1]);
  }
  console.log(`tunnel caller.port=${mtkTunnelPort} (MTK_CONNECT_TUNNEL_PORT)`);

  // HOST terminal login user: MTK_CONNECT_TERMINAL_USER when set (host images
  // whose interactive account is neither builder nor jenkins), else the
  // historical builder -> jenkins fallback chain.
  const terminalArgs = MTK_CONNECT_TERMINAL_USER
    ? ['-c', `cd /home/${MTK_CONNECT_TERMINAL_USER}; su ${MTK_CONNECT_TERMINAL_USER}; bash --login`, '']
    : ['-c', '[ -d /home/builder ] && { cd /home/builder; su builder; bash --login; } || { cd /home/jenkins; su jenkins; bash --login; }', ''];

  // adb-mode HOST terminal: keep the historical plain login shell unless a
  // terminal user is explicitly requested.
  const adbTerminalArgs = MTK_CONNECT_TERMINAL_USER ? terminalArgs : ['-c', 'cd ~/; bash --login', ''];

  // MTK_CONNECT_TUNNEL_LIST: comma-separated name:port entries, each exposed
  // as a raw TCP tunnel to the device host (MTK_CONNECT_HOST_LIST) for the
  // MTK Connect Tunnel client; caller.port = port gives one-click tunnels on
  // the same local port.
  const tcpTunnelTypes = (tunnelHost) =>
    MTK_CONNECT_TUNNEL_LIST.split(',').map((entry) => {
      const [name, port] = entry.split(':');
      return {
        'name': name,
        'driver': 'tcp',
        'host': tunnelHost,
        'port': +port,
        'caller': { 'port': +port }
      };
    });

  if (MTK_CONNECT_HOST_ONLY == 'false') {
    if (isTunnelDevice) {
      // Dedicated tunnel-carrier device: HOST fs/terminal plus all raw TCP
      // tunnels; the adb devices at index 2..N+1 keep only their adb tunnel.
      const data = {
        interface: {
          'fs': {
            'types': [
              {
                'name': 'HOST',
                'driver': 'native',
                'root': '/root'
              }
            ]
          },
          'terminal': {
            'types': [
              {
                'name': 'HOST',
                'driver': 'spawn',
                'command': 'bash',
                'args': terminalArgs
              }
            ]
          },
          'tunnel': {
            'types': tcpTunnelTypes(adbHosts[0])
          }
        }
      }
      await axios.patch(`/api/v1/agents/${agent.id}/devices/${index}`, data);
      return;
    }
    const data = {
      interface: {
        'adb': {
          'mode': 'tcp',
          'port': +adbPorts[adbIndex - 1],
          'host': adbHosts[adbIndex - 1]
        },
        'button': {
          'driver': 'adb',
          'skin': 'Default Android'
        },
        'exec': {
          'types': [
            {
              'name': 'device',
              'driver': 'adb',
              'cwd': '/'
            }
          ]
        },
        'fs': {
          'types': [
            {
              'name': 'adb',
              'driver': 'adb',
              'root': '/'
            },
            {
              'name': 'HOST',
              'driver': 'native',
              'root': '/root'
            }
          ]
        },
        'log': {
          'types': [
            {
              'name': 'logcat',
              'driver': 'logcat'
            }
          ]
        },
        'mjpeg': {
          'types': [
            {
              'name': 'screen',
              'driver': 'minicap',
              'scale': 1.0
            }
          ]
        },
        'terminal': {
          'types': [
            {
              'name': 'adb',
              'driver': 'adb',
              'icon': 'adb'
            },
            {
              'name': 'HOST',
              'driver': 'spawn',
              'command': 'bash',
              'args': adbTerminalArgs
            }
          ]
        },
        'touch': {
          'driver': 'adb',
          'native': true
        },
        'tunnel': {
          'types': [
            {
              'name': 'adb',
              'driver': 'adb',
              'caller': { 'port': mtkTunnelPort }
            }
          ]
        }
      }
    }
    // Legacy ride-along (no MTK_CONNECT_TUNNEL_DEVICE_NAME): raw TCP tunnels
    // attach to adb device 1 only — every device would otherwise register the
    // same caller ports, and one tunnel set serves the testbench.
    if (MTK_CONNECT_TUNNEL_LIST && !tunnelDeviceName && index === 1) {
      data.interface.tunnel.types.push(...tcpTunnelTypes(adbHosts[adbIndex - 1]));
    }
    await axios.patch(`/api/v1/agents/${agent.id}/devices/${index}`, data);
  } else {
    const data = {
      interface: {
        'fs': {
          'types': [
            {
              'name': 'HOST',
              'driver': 'native',
              'root': '/root'
            }
          ]
        },
        'terminal': {
          'types': [
            {
              'name': 'HOST',
              'driver': 'spawn',
              'command': 'bash',
              'args': terminalArgs
            }
          ]
        }
      }
    }

    if (MTK_CONNECT_TUNNEL_LIST) {
      data.interface.tunnel = {
        'types': tcpTunnelTypes(adbHosts[adbIndex - 1])
      };
    }

    if (MTK_CONNECT_LAUNCH_APPLICATION_NAME) {
      console.log(`Launch application: ${MTK_CONNECT_LAUNCH_APPLICATION_NAME}`);
      data.interface.terminal.types[0].args = ['-c', 'cd /home/builder; su builder -c "eval ${MTK_CONNECT_LAUNCH_APPLICATION_NAME}"', ''];
    }
    await axios.patch(`/api/v1/agents/${agent.id}/devices/${index}`, data);
  }
}

async function main()  {
  try {
    console.log(`configureAgent`);
    await configureAgent();
  } catch (err) {
    if (MTK_CONNECT_TESTBENCH_USER != 'everyone') {
      console.log(`ERROR: Please check you have logged into MTK Connect at least once to avoid access issues.`);
    }
    throw err;
  }

  console.log(`configureDevices`);
  await configureDevices();
}

main()
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
