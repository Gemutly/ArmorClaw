#!/usr/bin/env bash

# Test config file generation logic
domain="test.example.com"
tunnel_id="abc123-def456-ghi789"
cloudflared_dir="/tmp/test-cloudflared"
config_file="$cloudflared_dir/config.yml"

mkdir -p "$cloudflared_dir"

cat > "$config_file" <<EOF
tunnel: $tunnel_id
credentials-file: $cloudflared_dir/${tunnel_id}.json

ingress:
  - hostname: $domain
    service: http://localhost:8443
  - hostname: matrix.$domain
    service: http://localhost:6167
  - service: http_status:404
EOF

echo "Config file generated at: $config_file"
echo "---"
cat "$config_file"
echo "---"

# Verify content
if grep -q "hostname: $domain" "$config_file"; then
    echo "✓ Main domain ingress rule found"
else
    echo "✗ Main domain ingress rule missing"
fi

if grep -q "hostname: matrix.$domain" "$config_file"; then
    echo "✓ Matrix subdomain ingress rule found"
else
    echo "✗ Matrix subdomain ingress rule missing"
fi

if grep -q "tunnel: $tunnel_id" "$config_file"; then
    echo "✓ Tunnel ID found"
else
    echo "✗ Tunnel ID missing"
fi

# Cleanup
rm -rf "$cloudflared_dir"
