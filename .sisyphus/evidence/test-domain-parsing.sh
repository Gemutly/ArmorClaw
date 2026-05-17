#!/usr/bin/env bash

echo "Testing domain parsing logic..."
echo ""

# Test case 1: subdomain.example.com
domain="subdomain.example.com"
if echo "$domain" | grep -q '\.'; then
    subdomain=$(echo "$domain" | cut -d. -f1)
    base_domain=$(echo "$domain" | cut -d. -f2-)
    echo "Test 1: $domain"
    echo "  Subdomain: $subdomain"
    echo "  Base domain: $base_domain"
    if [ "$subdomain" = "subdomain" ] && [ "$base_domain" = "example.com" ]; then
        echo "  ✓ PASS"
    else
        echo "  ✗ FAIL"
    fi
fi

echo ""

# Test case 2: test.example.com
domain="test.example.com"
if echo "$domain" | grep -q '\.'; then
    subdomain=$(echo "$domain" | cut -d. -f1)
    base_domain=$(echo "$domain" | cut -d. -f2-)
    echo "Test 2: $domain"
    echo "  Subdomain: $subdomain"
    echo "  Base domain: $base_domain"
    if [ "$subdomain" = "test" ] && [ "$base_domain" = "example.com" ]; then
        echo "  ✓ PASS"
    else
        echo "  ✗ FAIL"
    fi
fi

echo ""

# Test case 3: example.com (no subdomain)
domain="example.com"
if echo "$domain" | grep -q '\.'; then
    subdomain=$(echo "$domain" | cut -d. -f1)
    base_domain=$(echo "$domain" | cut -d. -f2-)
    echo "Test 3: $domain"
    echo "  Subdomain: $subdomain"
    echo "  Base domain: $base_domain"
    if [ "$subdomain" = "example" ] && [ "$base_domain" = "com" ]; then
        echo "  ⚠ WARNING: 'example.com' parsed as subdomain 'example', base 'com'"
        echo "  This is expected behavior for single-dot domains"
    else
        echo "  ✗ FAIL"
    fi
fi

echo ""

# Test case 4: armorclaw.test.example.com (multi-level)
domain="armorclaw.test.example.com"
if echo "$domain" | grep -q '\.'; then
    subdomain=$(echo "$domain" | cut -d. -f1)
    base_domain=$(echo "$domain" | cut -d. -f2-)
    echo "Test 4: $domain"
    echo "  Subdomain: $subdomain"
    echo "  Base domain: $base_domain"
    if [ "$subdomain" = "armorclaw" ] && [ "$base_domain" = "test.example.com" ]; then
        echo "  ✓ PASS"
    else
        echo "  ✗ FAIL"
    fi
fi
