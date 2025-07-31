#!/bin/bash
# Lima VM management script for shmocker
LIMA_VM_NAME="shmocker-buildkit"

case "$1" in
    start)
        echo "Starting Lima VM..."
        limactl start "$LIMA_VM_NAME"
        ;;
    stop)
        echo "Stopping Lima VM..."
        limactl stop "$LIMA_VM_NAME"
        ;;
    restart)
        echo "Restarting Lima VM..."
        limactl stop "$LIMA_VM_NAME"
        limactl start "$LIMA_VM_NAME"
        ;;
    status)
        limactl list | grep "$LIMA_VM_NAME" || echo "VM not found"
        ;;
    shell)
        limactl shell "$LIMA_VM_NAME"
        ;;
    logs)
        limactl shell "$LIMA_VM_NAME" sudo journalctl -u buildkit -f
        ;;
    buildctl)
        shift
        limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "$@"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|shell|logs|buildctl}"
        echo ""
        echo "Commands:"
        echo "  start    - Start the Lima VM"
        echo "  stop     - Stop the Lima VM"
        echo "  restart  - Restart the Lima VM"
        echo "  status   - Show VM status"
        echo "  shell    - Open shell in the VM"
        echo "  logs     - Show BuildKit daemon logs"
        echo "  buildctl - Run buildctl command in VM"
        exit 1
        ;;
esac