#!/bin/bash
set -e

echo "Installing librato daemon..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

# Create system user
echo "Creating system user 'librato'..."
useradd -r -s /bin/false librato 2>/dev/null || echo "User 'librato' already exists"

# Create directories
echo "Creating directories..."
mkdir -p /var/run/librato
mkdir -p /etc/librato

# Set permissions
echo "Setting permissions..."
chown librato:librato /var/run/librato
chmod 755 /var/run/librato

# Install binary
echo "Installing binary..."
if [ ! -f "librato" ]; then
    echo "Error: librato binary not found in current directory"
    echo "Please build the binary first with: make build"
    exit 1
fi

cp librato /usr/local/bin/
chmod 755 /usr/local/bin/librato

# Install config
echo "Installing configuration..."
if [ ! -f "/etc/librato/config.json" ]; then
    if [ -f "config.daemon.json" ]; then
        cp config.daemon.json /etc/librato/config.json
        echo "Copied config.daemon.json to /etc/librato/config.json"
    else
        echo "Warning: config.daemon.json not found, skipping config installation"
    fi
else
    echo "Config already exists at /etc/librato/config.json, skipping"
fi

chown librato:librato /etc/librato/config.json 2>/dev/null || true

# Install systemd unit
echo "Installing systemd service..."
cp systemd/librato.service /etc/systemd/system/
systemctl daemon-reload

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit /etc/librato/config.json to configure your watch and library directories"
echo "2. Create the watch and quarantine directories configured in config.json"
echo "3. Ensure the librato user has appropriate permissions to those directories"
echo "4. Enable the service: sudo systemctl enable librato"
echo "5. Start the service: sudo systemctl start librato"
echo "6. Check status: sudo systemctl status librato"
echo "7. View logs: sudo journalctl -u librato -f"
echo ""
