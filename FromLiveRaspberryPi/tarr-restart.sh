#!/bin/bash
echo "Restarting TARR Annunciator..."
"/home/pi/tarr-stop.sh"
sleep 2
"/home/pi/tarr-start.sh"
