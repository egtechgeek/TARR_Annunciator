#!/bin/bash

# This script will update the TARR Annunciator application to the latest version.
# It must be run as root.

# Exit on any error
set -e

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root!"
    exit 1
fi

echo "Updating TARR Annunciator..."

# Clone the latest version from GitHub
echo "Cloning the latest version of TARR Annunciator..."
git clone https://github.com/egtechgeek/TARR_annunciator/ /tmp/TARR_Annunciator

# Copy necessary files and directories, overwriting the old ones
echo "Copying updated files to the Annunciator directory..."

cp -r /tmp/TARR_Annunciator/templates /opt/TARR_Annunciator/templates
cp -r /tmp/TARR_Annunciator/static /opt/TARR_Annunciator/static
cp /tmp/TARR_Annunciator/app.py /opt/TARR_Annunciator/app.py
cp /tmp/TARR_Annunciator/install.sh /opt/TARR_Annunciator/install.sh
cp /tmp/TARR_Annunciator/update.sh /opt/TARR_Annunciator/update.sh

# Set the executable permissions on the install and update scripts
chmod +x /opt/TARR_Annunciator/install.sh
chmod +x /opt/TARR_Annunciator/update.sh

# Clean up the temporary directory
rm -rf /tmp/TARR_Annunciator

echo "TARR Annunciator has been successfully updated!"
