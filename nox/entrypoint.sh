#!/bin/sh

# wait in a loop until the audit.log file is created by the endpoint container
while [ ! -f /var/log/audit/audit.log ]; do
    echo "Waiting for /var/log/audit/audit.log to be created..."
    sleep 2
done

echo "audit.log found. Starting nox engine."

# execute the main nox application
exec /nox