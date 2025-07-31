#!/bin/bash
# Wrapper script for buildctl that connects to Lima VM
LIMA_VM_NAME="shmocker-buildkit"

if ! limactl list | grep -q "^$LIMA_VM_NAME.*Running"; then
    echo "Error: Lima VM '$LIMA_VM_NAME' is not running"
    echo "Start it with: limactl start $LIMA_VM_NAME"
    exit 1
fi

exec limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "$@"