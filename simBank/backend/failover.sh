#!/bin/bash
# This is the Witness for the DB server.
set -eo pipefail

NEW_PRIMARY_IP="$1"
OFFLINE_NODE_IP="$2"
DB_USER="user_name"
DB_NAME="database_name"
PG_DATA="/var/lib/postgresql/17/main"

# Function to dynamically map IP to the correct non-root user
get_ssh_user() {
    local ip="$1"
    if [ "$ip" == "host_address" ]; then
        echo "host_username"
    elif [ "$ip" == "host_address" ]; then
        echo "host_username"
    else
        echo "unknown"
    fi
}

PRIMARY_USER=$(get_ssh_user "$NEW_PRIMARY_IP")
OFFLINE_USER=$(get_ssh_user "$OFFLINE_NODE_IP")

echo "=================================================="
echo "WITNESS TRIGGERED FAILOVER AT $(date)"
echo "Promoting New Primary: $NEW_PRIMARY_IP (SSH User: $PRIMARY_USER)"
echo "Target for Rewind:     $OFFLINE_NODE_IP (SSH User: $OFFLINE_USER)"
echo "=================================================="

#  Promote the surviving node via SSH using sudo
echo "[STEP 1] Promoting $NEW_PRIMARY_IP..."
ssh -o StrictHostKeyChecking=no "${PRIMARY_USER}@${NEW_PRIMARY_IP}" \
    "sudo pg_ctlcluster 17 main promote"

echo "Node $NEW_PRIMARY_IP is now active Master!"

#  Loop in the background until the dead server responds
echo ":) Monitoring $OFFLINE_NODE_IP until back online..."
until ssh -o ConnectTimeout=2 -o StrictHostKeyChecking=no "${OFFLINE_USER}@${OFFLINE_NODE_IP}" "echo ready" > /dev/null 2>&1; do
    sleep 5
done

echo ":) Node $OFFLINE_NODE_IP is back online! Initiating timeline rewind..."

#  Heal the returning server using sudo and turn it into a Standby
REWIND_CMDS=$(cat <<EOF
sudo systemctl stop postgresql@17-main || true
sudo -u postgres pg_rewind \
    --target-pgdata=$PG_DATA \
    --source-server="host=$NEW_PRIMARY_IP port=5432 user=postgres dbname=$DB_NAME" \
    --progress
sudo -u postgres touch $PG_DATA/standby.signal
sudo systemctl start postgresql@17-main
EOF
)

if ssh -o StrictHostKeyChecking=no "${OFFLINE_USER}@${OFFLINE_NODE_IP}" "$REWIND_CMDS"; then
    echo ":) [SUCCESS] Node $OFFLINE_NODE_IP healed with pg_rewind & re-attached as Standby!"
else
    echo ":( [ERROR] Automated pg_rewind failed on $OFFLINE_NODE_IP."
    exit 1
fi
