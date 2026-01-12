/*
 #############################################################################
 # Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
 # This code downloads the agent installer from MTK Connect.
 #
 # 1. It loads environment variables from a .env file using dotenv.
 # 2. It sets up axios with a base URL and authentication credentials
 #    from the environment.
 # 3. The downloadInstaller retrieves a list of artifacts.
 # 4. It searches for the MTK Connect agent artifact.
 #    - If found, it retrieves the artifact.
 #    - If not found, throw an error.
 #
 #############################################################################
 */

'use strict';

require('dotenv').config();

const fs = require('fs');
const axios = require('axios');

/**
 * Sets up the Axios configuration with a base URL and authentication
 * credentials from environment variables.
 */
const { MTK_CONNECT_DOMAIN, MTK_CONNECT_USERNAME, MTK_CONNECT_PASSWORD } = process.env;

/**
 * Retrieve architecture for MTK Connect installer.
 */
const arch = { x64: 'amd64', arm64: 'arm64' }[process.arch] || process.arch;

axios.defaults.baseURL = `https://${MTK_CONNECT_DOMAIN}/mtk-connect`;
axios.defaults.auth = {
  username: MTK_CONNECT_USERNAME,
  password: MTK_CONNECT_PASSWORD
};

/**
 * Downloads an installer from a specified API endpoint and saves it to a file named
 * mtk-connect-agent.node.zip.
 */
async function downloadInstaller() {
  let artifactResponse = await axios.get('/api/v1/installers');
  const installer = artifactResponse.data.data.find((artifact) => {
    return artifact.path.startsWith('mtk-connect-agent') && artifact.path.endsWith(`.linux.${arch}.tgz`);
  });
  if (installer) {
    console.log(`Installer path ${installer.path}`)
    const downloadResponse = await axios.get(`/api/v1/installers/${installer.path}`, {responseType: 'stream'});
    downloadResponse.data.pipe(fs.createWriteStream('mtk-connect-agent.node.tgz'));
  } else {
    throw new Error('installer not found');
  }
}

async function main()  {
  try {
    await downloadInstaller();
  } catch (err) {
    throw err;
  }
}

main()
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
