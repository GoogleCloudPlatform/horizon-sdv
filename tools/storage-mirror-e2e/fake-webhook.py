#!/usr/bin/env python3

# Copyright (c) 2026 Accenture, All Rights Reserved.
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

"""Stands in for the Argo Events webhook in run.sh --apiserver-only: records every workflow-dispatch body.

usage: fake-webhook.py PORT OUT.jsonl
"""

import http.server
import sys

PORT = int(sys.argv[1])
OUT = sys.argv[2]


class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        with open(OUT, "a") as f:
            f.write(body.decode() + "\n")
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")

    def log_message(self, *args):
        pass


http.server.HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
