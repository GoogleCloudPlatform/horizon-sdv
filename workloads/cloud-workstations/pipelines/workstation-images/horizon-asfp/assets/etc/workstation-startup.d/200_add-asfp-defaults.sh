#!/bin/bash
# Sets defaults for ASfP
# 1. Sets RAM allocation to 70% of available ram up to 30gb
# 2. Sets intellisense.filesize to 10000

echo "Setting Android Studio for Platform Options"

function getSeventyPercentOfMemory() {
  local memory=$(free -m 2>&1 | sed -nr 's/.*Mem:\s*([0-9]+).*/\1/p')
  # Get ~70% of memory. Bash doesn't do floating point, so we fake it by
  # multiplying 70 and dividing by 100, we then truncate to GB by dividing then
  # multiplying
  local seventy_percent=$(($memory * 70 / 100 / 100 * 100))
  echo "$seventy_percent"
}

# Set memory to 70% of available memory for developer workstations,
# Or up to 64 GB for administrator workstations to sync and index a large ASOP project.
vm_memory=$(getSeventyPercentOfMemory)
if [[ $vm_memory -lt 64000 ]]; then
  sed -i "s/-Xmx20000m/-Xmx${vm_memory}m/" /opt/android-studio-for-platform-canary/bin/studio64.vmoptions
else
  sed -i "s/-Xmx20000m/-Xmx64000m/" /opt/android-studio-for-platform-canary/bin/studio64.vmoptions
fi

sed -i 's/-Didea.max.intellisense.filesize=999999/-Didea.max.intellisense.filesize=10000/' /opt/android-studio-for-platform-canary/bin/studio64.vmoptions
