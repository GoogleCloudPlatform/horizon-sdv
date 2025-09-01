#!/bin/bash

echo "Setting nested virtualization, cvdnetwork, and render permissions."

if [[ $(getent group kvm) ]]; then
  usermod -aG kvm user
fi

if [[ $(getent group cvdnetwork) ]]; then
  usermod -aG cvdnetwork user
fi

if [[ $(getent group render) ]]; then
  usermod -aG render user
fi
