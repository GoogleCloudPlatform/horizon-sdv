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

FROM debian:12.9
ADD configure.sh /root
ADD secret-jenkins.json /root
ADD secret-mtk-connect.json /root
RUN apt update -y && apt install vim curl skopeo jq -y
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
RUN mv /kubectl /usr/local/bin && chmod +x /usr/local/bin/kubectl && chmod +x /root/configure.sh
CMD ["/bin/bash", "-c", "/root/configure.sh"]
