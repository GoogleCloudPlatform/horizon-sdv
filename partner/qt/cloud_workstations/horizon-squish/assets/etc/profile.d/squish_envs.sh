#!/bin/bash

# Copyright (C) 2025 The Qt Company Ltd.
# All rights reserved.
#
# This file is part of Squish.
#
# Licensees holding a valid Squish License Agreement may use this
# file in accordance with the Squish License Agreement provided with
# the Software.
#
# This file is provided AS IS with NO WARRANTY OF ANY KIND, INCLUDING THE
# WARRANTY OF DESIGN, MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.
#
# See the LICENSE file in this directory.
#
# Contact qatools.support@qt.io if any conditions of this licensing are
# not clear to you.

export ANDROID_HOME=${HOME}/Android/Sdk
export PATH=${PATH}:/opt/google/cmdline-tools/bin:${ANDROID_HOME}/tools:${ANDROID_HOME}/tools/bin:${ANDROID_HOME}/platform-tools
