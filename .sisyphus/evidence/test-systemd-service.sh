#!/usr/bin/env bash

echo "Testing systemd service file generation..."
echo ""

USER="testuser"
tunnel_name="armorclaw-test"
config_file="/home/testuser/.cloudflared/config.yml"

service_content="[Unit]
Description=Cloudflare Tunnel for ArmorClaw
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=/usr/local/bin/cloudflared tunnel run --config=$config_file $tunnel_name
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target"

echo "Generated service content:"
echo "---"
echo "$service_content"
echo "---"

# Verify key components
if echo "$service_content" | grep -q "Description=Cloudflare Tunnel for ArmorClaw"; then
    echo "✓ Description present"
else
    echo "✗ Description missing"
fi

if echo "$service_content" | grep -q "User=$USER"; then
    echo "✓ User configured"
else
    echo "✗ User missing"
fi

if echo "$service_content" | grep -q "ExecStart=/usr/local/bin/cloudflared"; then
    echo "✓ ExecStart path correct"
else
    echo "✗ ExecStart incorrect"
fi

if echo "$service_content" | grep -q "Restart=on-failure"; then
    echo "✓ Restart policy configured"
else
    echo "✗ Restart policy missing"
fi

if echo "$service_content" | grep -q "WantedBy=multi-user.target"; then
    echo "✓ Install target correct"
else
    echo "✗ Install target incorrect"
fi
