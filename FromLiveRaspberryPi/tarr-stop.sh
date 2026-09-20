#!/bin/bash
echo "Stopping TARR Annunciator..."
if screen -list | grep -q "tarr-annunciator"; then
    screen -S "tarr-annunciator" -X quit
    echo "TARR Annunciator stopped"
else
    echo "TARR Annunciator is not running"
fi
