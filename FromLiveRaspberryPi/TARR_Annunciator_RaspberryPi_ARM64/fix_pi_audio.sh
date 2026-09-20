#!/bin/bash
# fix-pi-audio.sh — safe switch between vc4-kms-v3d and vc4-fkms-v3d
# Provides a menu for Fix vs Restore with confirmation and reboot prompt.

CONFIG_FILE="/boot/firmware/config.txt"

do_fix() {
    BACKUP_FILE="/boot/firmware/config.txt.backup.$(date +%Y%m%d%H%M%S)"
    echo "Backing up $CONFIG_FILE to $BACKUP_FILE"
    sudo cp "$CONFIG_FILE" "$BACKUP_FILE" || { echo "Backup failed"; exit 1; }

    if grep -qE '^[^#]*dtoverlay=vc4-kms-v3d' "$CONFIG_FILE"; then
        echo "Found vc4-kms-v3d — commenting out and adding vc4-fkms-v3d..."

        # Comment out the vc4-kms-v3d line
        sudo sed -i 's/^\([^#]*dtoverlay=vc4-kms-v3d.*\)$/#\1/' "$CONFIG_FILE"

        # Only add vc4-fkms-v3d if not already present
        if ! grep -q 'dtoverlay=vc4-fkms-v3d' "$CONFIG_FILE"; then
            sudo sed -i '/^#.*dtoverlay=vc4-kms-v3d.*/a dtoverlay=vc4-fkms-v3d' "$CONFIG_FILE"
        fi

        echo "Fix complete: vc4-kms-v3d disabled, vc4-fkms-v3d enabled."
    else
        echo "vc4-kms-v3d not found or already commented out."
        if ! grep -q 'dtoverlay=vc4-fkms-v3d' "$CONFIG_FILE"; then
            echo "Adding vc4-fkms-v3d at end of file..."
            echo "dtoverlay=vc4-fkms-v3d" | sudo tee -a "$CONFIG_FILE" > /dev/null
        else
            echo "vc4-fkms-v3d is already present."
        fi
    fi

    ask_reboot
}

do_restore() {
    echo "Available backups in /boot/firmware:"
    ls -1 /boot/firmware/config.txt.backup.* 2>/dev/null || { echo "No backups found!"; exit 1; }

    echo
    read -rp "Enter the full path of the backup you want to restore: " BACKUP_FILE

    if [ ! -f "$BACKUP_FILE" ]; then
        echo "Backup file $BACKUP_FILE not found!"
        exit 1
    fi

    echo "You chose to restore: $BACKUP_FILE"
    read -rp "Are you sure you want to overwrite $CONFIG_FILE with this backup? (yes/no): " CONFIRM
    if [[ "$CONFIRM" != "yes" ]]; then
        echo "Restore cancelled."
        exit 0
    fi

    echo "Restoring $BACKUP_FILE to $CONFIG_FILE..."
    sudo cp "$BACKUP_FILE" "$CONFIG_FILE" || { echo "Restore failed"; exit 1; }
    echo "Restore complete."
    ask_reboot
}

ask_reboot() {
    echo
    read -rp "Do you want to reboot now to apply changes? (yes/no): " REBOOT
    if [[ "$REBOOT" == "yes" ]]; then
        echo "Rebooting..."
        sudo reboot
    else
        echo "Please reboot later to apply changes."
    fi
}

echo "=============================="
echo " Raspberry Pi Audio Fix Script"
echo "=============================="
echo "1) Fix Pi Audio (switch vc4-kms-v3d → vc4-fkms-v3d)"
echo "2) Restore Config from Backup"
echo "q) Quit"
echo

read -rp "Choose an option: " CHOICE

case "$CHOICE" in
    1)
        echo "You chose: Fix Pi Audio"
        read -rp "Proceed with fix? (yes/no): " CONFIRM
        [[ "$CONFIRM" == "yes" ]] && do_fix || echo "Cancelled."
        ;;
    2)
        echo "You chose: Restore Config"
        do_restore
        ;;
    q|Q)
        echo "Exiting without changes."
        ;;
    *)
        echo "Invalid choice. Exiting."
        ;;
esac
