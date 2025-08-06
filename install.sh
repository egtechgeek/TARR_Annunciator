#!/bin/bash

# Run as root
if [ "$(id -u)" -ne 0 ]; then
    echo "This script must be run as root"
    exit 1
fi

# Update package list and upgrade all packages
echo "Updating package list and upgrading installed packages..."
apt-get update -y && apt-get upgrade -y

# Install Python 3, pip, and other required dependencies
echo "Installing dependencies..."
apt-get install -y python3 python3-pip python3-dev mpg123 libasound2-dev python3-venv git

# Clone the TARR Annunciator repository into /opt/TARR_Annunciator/
echo "Checking For Annunciator Updates..."
git clone https://egtechgeek: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@github.com/egtechgeek/TARR_annunciator/ /opt/TARR_Annunciator

# Create a Python virtual environment
echo "Creating a Python virtual environment in /opt/TARR_Annunciator..."
cd /opt/TARR_Annunciator
python3 -m venv /opt/TARR_Annunciator

# Activate the virtual environment
echo "Activating virtual environment..."
source /opt/TARR_Annunciator/bin/activate

# Upgrade pip
echo "Upgrading pip..."
pip install --upgrade pip

# Install Flask, Pygame, and other necessary packages
echo "Installing Flask, Pygame, and other packages..."
pip install flask pygame pydub playsound requests

# Install Gunicorn in the virtual environment
echo "Installing Gunicorn..."
pip install gunicorn

# Install Nginx
echo "Installing Nginx..."
apt-get install -y nginx

# Create an Nginx configuration file for the Flask app
echo "Creating Nginx configuration..."
tee /etc/nginx/sites-available/annunciator <<EOF
server {
    listen 80;
    server_name _;  # Will work for any domain or IP

    # Serve static files
    location /static/ {
        alias /opt/TARR_Annunciator/static/;
    }

    # Reverse proxy to Gunicorn
    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF

# Link the Nginx configuration to sites-enabled
echo "Linking Nginx configuration..."
ln -s /etc/nginx/sites-available/annunciator /etc/nginx/sites-enabled/

# Start Gunicorn in the virtual environment with 4 workers and bind to port 8000
echo "Starting Gunicorn..."
cd /opt/TARR_Annunciator
/opt/TARR_Annunciator/bin/gunicorn -w 4 -b 127.0.0.1:8000 app:app &

# Start Nginx
echo "Starting Nginx..."
systemctl restart nginx

# Create systemd service files to ensure Gunicorn and Nginx start on boot
echo "Creating systemd service files..."

# Create Gunicorn service
tee /etc/systemd/system/gunicorn.service <<EOF
[Unit]
Description=Gunicorn instance to serve Annunciator
After=network.target

[Service]
User=root
Group=www-data
WorkingDirectory=/opt/TARR_Annunciator
ExecStart=/opt/TARR_Annunciator/bin/gunicorn -w 4 -b 127.0.0.1:8000 app:app

[Install]
WantedBy=multi-user.target
EOF

# Create Nginx service if it's not already running
systemctl enable nginx

# Enable Gunicorn to start on boot
systemctl enable gunicorn

# Start Gunicorn service
systemctl start gunicorn

# Remove the default Nginx site
echo "Removing default Nginx site..."
sudo rm /etc/nginx/sites-enabled/default
sudo rm /etc/nginx/sites-available/default

# Test the Nginx configuration for syntax errors
echo "Testing Nginx configuration..."
sudo nginx -t

# Reload Nginx to apply changes
echo "Reloading Nginx..."
sudo systemctl reload nginx

# Final message
echo "Installation complete. Gunicorn and Nginx are running and will start on boot."
