#!/bin/bash
echo "Connecting to TARR Annunciator session..."
if screen -list | grep -q "tarr-annunciator"; then
    echo "Press Ctrl+A then D to detach from the session"
    screen -r tarr-annunciator
else
    echo "TARR Annunciator session is not running"
    echo "Start it with: ./tarr-start.sh"
fi
