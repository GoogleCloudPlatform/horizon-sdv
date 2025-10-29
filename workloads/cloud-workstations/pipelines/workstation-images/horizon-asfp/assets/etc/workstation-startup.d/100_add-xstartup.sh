#!/bin/bash

echo "Configuring VNC .xstartup"

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    exec runuser user "${BASH_SOURCE[0]}"
fi

if [[ ! -f /home/user/.vnc/xstartup ]]; then
  mkdir -p /home/user/.vnc/
  cat <<EOT > /home/user/.vnc/xstartup
#!/bin/sh

export XDG_SESSION_TYPE=x11
export XKL_XMODMAP_DISABLE=1
export XDG_CURRENT_DESKTOP=ubuntu:GNOME

# Uncomment the following lines to get the "thick" ubuntu:GNOME desktop.
# export DESKTOP_SESSION=ubuntu-xorg
# export XDG_SESSION_DESKTOP="$DESKTOP_SESSION"
# export XDG_CONFIG_DIRS=/etc/xdg/xdg-ubuntu-xorg:/etc/xdg
# export XDG_DATA_DIRS=/usr/share/"$DESKTOP_SESSION":/usr/share/gnome:/usr/local/share/:/usr/share/:/var/lib/snapd/desktop
# export GNOME_SHELL_SESSION_MODE=ubuntu

unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS

[ -x /etc/vnc/xstartup ] && exec /etc/vnc/xstartup
[ -r $HOME/.Xresources ] && xrdb $HOME/.Xresources
vncconfig -nowin &
dbus-launch --exit-with-session gnome-session
EOT
  chmod 755 /home/user/.vnc/xstartup
fi
