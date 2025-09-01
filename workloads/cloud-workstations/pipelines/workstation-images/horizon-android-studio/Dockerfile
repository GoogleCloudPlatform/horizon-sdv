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

#######################################################
# NoVNC Builder Container
#######################################################
FROM us-west1-docker.pkg.dev/cloud-workstations-images/predefined/base as novnc-builder

ARG NOVNC_BRANCH=v1.5.0
ARG WEBSOCKIFY_BRANCH=v0.12.0

WORKDIR /out

RUN git clone --quiet --depth 1 --branch $NOVNC_BRANCH https://github.com/novnc/noVNC.git && \
  cd noVNC/utils && \
  git clone  --quiet --depth 1 --branch $WEBSOCKIFY_BRANCH https://github.com/novnc/websockify.git

#######################################################
# End NoVNC Builder Container
#######################################################

# Main container
FROM us-west1-docker.pkg.dev/cloud-workstations-images/predefined/base

ARG ANDROID_STUDIO_VERSION=2024.3.2.6
ARG COMMAND_LINE_TOOLS_VERSION=11076708

ENV ANDROID_STUDIO_VERSION=$ANDROID_STUDIO_VERSION

# Install Android Studio
RUN mkdir -p /opt/google && \
  wget https://redirector.gvt1.com/edgedl/android/studio/ide-zips/$ANDROID_STUDIO_VERSION/android-studio-$ANDROID_STUDIO_VERSION-linux.tar.gz -O android-studio.tar.gz && \
  tar -xzvf android-studio.tar.gz -C /opt/google/ && \
  rm android-studio.tar.gz && \
  wget https://dl.google.com/android/repository/commandlinetools-linux-${COMMAND_LINE_TOOLS_VERSION}_latest.zip -O commandlinetools.zip && \
  unzip commandlinetools.zip -d /opt/google/ && \
  rm commandlinetools.zip && \
  # Create a folder that will house the "Default" Android packages that are
  # intended to be bundled with this workstations image. On startup we will
  # create an overlay mount at $ANDROID_HOME ($HOME/Android/Sdk) to surface
  # both the preinstalled SDKs and any SDKs that user has subsequently installed
  # on their machine.
  mkdir -p /opt/google/Android/Sdk && \
  chmod o+wr /opt/google/Android/Sdk

# Install default packages
RUN cd /opt/google/cmdline-tools/bin && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "build-tools;35.0.0" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "build-tools;35.0.0-rc1" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "emulator" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "platforms;android-34" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "platforms;android-35" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "platform-tools" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "sources;android-35" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "system-images;android-34;google_apis;x86_64" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --install "system-images;android-34;google_apis_playstore;x86_64" && \
  yes | ./sdkmanager --sdk_root=/opt/google/Android/Sdk --update && \
  # Grant users permissions to the sdk_root.
  chown -R 1000:users /opt/google/Android/Sdk

# Set default vm options
RUN echo "-Ddisable.android.first.run=true" >> /opt/google/android-studio/bin/studio64.vmoptions && \
  echo "-Ddisable.config.import=true" >> /opt/google/android-studio/bin/studio64.vmoptions && \
  echo "-Didea.trust.all.projects=true" >> /opt/google/android-studio/bin/studio64.vmoptions && \
  echo "-Duse.gemini.enterprise=true" >> /opt/google/android-studio/bin/studio64.vmoptions

# Enable Gemini in Android Studio
RUN echo "studiobot.chat.use.compose.for.ui=false"  >> /opt/google/android-studio/bin/idea.properties

# Install systemd, GNOME, and VNC. Alternatively, you can source the main
# container build from the image  built using ../../gnome/noVnc/Dockerfile and
# omit much of the following.

# Install and configure systemd
RUN apt-get update && apt-get install -y \
  systemd && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/* &&\
  ln -s /dev/null /etc/systemd/system/apache2.service && \
  ln -s /dev/null /etc/systemd/system/getty@tty1.service && \
  ln -s /dev/null /etc/systemd/system/ldconfig.service && \
  /sbin/ldconfig -Xv && \
  ln -s /dev/null /etc/systemd/system/systemd-modules-load.service && \
  ln -s /dev/null /etc/systemd/system/ssh.socket && \
  ln -s /dev/null /etc/systemd/system/ssh.service && \
  echo "d /run/sshd 0755 root root" > /usr/lib/tmpfiles.d/sshd.conf && \
  echo -e "x /run/docker.socket - - - - -\nx /var/run/docker.socket - - - - -" > /usr/lib/tmpfiles.d/docker.conf

# Install and configure GNOME
RUN apt-get update && apt-get install -y \
    gnome-software \
    gnome-software-common \
    gnome-software-plugin-snap \
    libappstream-glib8 && \
  apt-get install -y \
    libgd3 && \
  apt-get install -y \
    colord \
    gnome-control-center \
    gvfs-backends \
    hplip \
    libgphoto2-6 \
    libsane1 \
    sane-utils \
    ubuntu-desktop-minimal && \
  apt-get remove -y gnome-initial-setup && \
  apt-get remove -y --purge cloud-init && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/* && \
  chmod -x /usr/lib/ubuntu-release-upgrader/check-new-release-gtk

# Install TigerVNC and noVNC
COPY --from=novnc-builder /out/noVNC /opt/noVNC
RUN apt-get update && apt-get install -y \
    dbus-x11 \
    tigervnc-common \
    tigervnc-scraping-server \
    tigervnc-standalone-server \
    tigervnc-xorg-extension \
    python3-numpy  && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# Additional Tooling

# Gnome Terminal.
RUN apt-get update --yes && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y gnome-terminal && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# Install google-chrome
RUN wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb && \
  dpkg -i google-chrome-stable_current_amd64.deb && \
  # apt install -y ./google-chrome*.deb && \
  rm google-chrome*.deb && \
  # Ensure chrome will work in our headless environment without gpu support by
  # specifying environment variables and flags.
  echo "QT_X11_NO_MITSHM=1 _X11_NO_MITSHM=1 _MITSHM=0 $(tail -n -1 /opt/google/chrome/google-chrome) --disable-gpu --disable-dev-shm-usage --use-gl=swiftshader" >> /opt/google/chrome/google-chrome && \
  sed -i 'N;$!P;D' /opt/google/chrome/google-chrome && \
  # Set google-chrome as the default browser.
  xdg-settings set default-web-browser google-chrome.desktop && \
  if [[ ! -f  /usr/bin/x-www-browser ]]; then update-alternatives --install /usr/bin/x-www-browser x-www-browser /usr/bin/google-chrome-stable 200; fi && \
  update-alternatives --set x-www-browser /usr/bin/google-chrome-stable && \
  if [[ ! -f  /usr/bin/gnome-www-browser ]]; then update-alternatives --install /usr/bin/gnome-www-browser gnome-www-browser /usr/bin/google-chrome-stable 200; fi  && \
  update-alternatives --set gnome-www-browser /usr/bin/google-chrome-stable

# Merge assets into the container.
COPY assets/. /

# Run VNC Server and NoVNC as services.
RUN ln -s /etc/systemd/system/tigervnc.service /etc/systemd/system/multi-user.target.wants/ && \
  ln -s /etc/systemd/system/novnc.service /etc/systemd/system/multi-user.target.wants/ && \
  systemctl enable tigervnc && \
  systemctl enable novnc

# Automatically start android studio
RUN ln -s /usr/share/applications/jetbrains-studio.desktop /etc/xdg/autostart/jetbrains-studio.desktop && \
  # Provide execute permission to entrypoint script
  chmod +x /google/scripts/entrypoint.sh

ENTRYPOINT ["/google/scripts/entrypoint.sh"]
