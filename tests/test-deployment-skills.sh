#!/bin/bash
# Test suite for deployment skills
# Validates YAML structure, cross-platform support, and skill completeness

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/transport.sh"
detect_transport
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKILLS_DIR="$PROJECT_ROOT/.skills"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASS++)) || true
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAIL++)) || true
}

check_info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

echo "========================================"
echo "Deployment Skills Test Suite"
echo "========================================"
echo ""

# ========================================
# Test 1: Directory Structure
# ========================================
echo "--- Test 1: Directory Structure ---"

if [[ -d "$SKILLS_DIR" ]]; then
    check_pass ".skills/ directory exists"
else
    check_fail ".skills/ directory missing"
fi

# Check skill subdirectories
for skill in deploy status cloudflare provision; do
    if [[ -d "$SKILLS_DIR/$skill" ]]; then
        check_pass ".skills/$skill/ directory exists"
    else
        check_fail ".skills/$skill/ directory missing"
    fi
done

echo ""

# ========================================
# Test 2: YAML Files Exist and Parse
# ========================================
echo "--- Test 2: YAML Files ---"

for skill in deploy status cloudflare provision; do
    YAML_FILE="$SKILLS_DIR/$skill.yaml"
    
    if [[ -f "$YAML_FILE" ]]; then
        check_pass "$skill.yaml exists"
        
        # Validate YAML syntax
        if python3 -c "import yaml; yaml.safe_load(open('$YAML_FILE'))" 2>/dev/null; then
            check_pass "$skill.yaml is valid YAML"
        else
            check_fail "$skill.yaml has invalid YAML syntax"
        fi
    else
        check_fail "$skill.yaml missing"
    fi
done

echo ""

# ========================================
# Test 3: SKILL.md Files Exist
# ========================================
echo "--- Test 3: SKILL.md Files ---"

for skill in deploy status cloudflare provision; do
    SKILL_MD="$SKILLS_DIR/$skill/SKILL.md"
    
    if [[ -f "$SKILL_MD" ]]; then
        check_pass "$skill/SKILL.md exists"
        
        # Check for YAML frontmatter
        if head -1 "$SKILL_MD" | grep -q "^---"; then
            check_pass "$skill/SKILL.md has YAML frontmatter"
        else
            check_fail "$skill/SKILL.md missing YAML frontmatter"
        fi
    else
        check_fail "$skill/SKILL.md missing"
    fi
done

echo ""

# ========================================
# Test 4: Required YAML Fields
# ========================================
echo "--- Test 4: Required YAML Fields ---"

for skill in deploy status cloudflare provision; do
    YAML_FILE="$SKILLS_DIR/$skill.yaml"
    
    if [[ -f "$YAML_FILE" ]]; then
        # Check required fields
        for field in name version description parameters steps platforms; do
            if grep -q "^$field:" "$YAML_FILE"; then
                check_pass "$skill.yaml has '$field' field"
            else
                check_fail "$skill.yaml missing '$field' field"
            fi
        done
    fi
done

echo ""

# ========================================
# Test 5: Cross-Platform Support
# ========================================
echo "--- Test 5: Cross-Platform Support ---"

for skill in deploy status cloudflare provision; do
    YAML_FILE="$SKILLS_DIR/$skill.yaml"
    
    if [[ -f "$YAML_FILE" ]]; then
        # Check for platforms section
        if grep -q "platforms:" "$YAML_FILE"; then
            check_pass "$skill.yaml has platforms section"
            
            # Check for required platforms
            for platform in linux macos windows; do
                if grep -q "^\s*- $platform" "$YAML_FILE"; then
                    check_pass "$skill.yaml supports $platform"
                else
                    check_fail "$skill.yaml missing $platform support"
                fi
            done
        else
            check_fail "$skill.yaml missing platforms section"
        fi
    fi
done

echo ""

# ========================================
# Test 6: Automation Flags
# ========================================
echo "--- Test 6: Automation Flags ---"

for skill in deploy status cloudflare provision; do
    YAML_FILE="$SKILLS_DIR/$skill.yaml"
    
    if [[ -f "$YAML_FILE" ]]; then
        # Check for automation flags
        AUTO_COUNT=$(grep -c 'automation: "auto"' "$YAML_FILE" 2>/dev/null || echo "0")
        CONFIRM_COUNT=$(grep -c 'automation: "confirm"' "$YAML_FILE" 2>/dev/null || echo "0")
        GUIDE_COUNT=$(grep -c 'automation: "guide"' "$YAML_FILE" 2>/dev/null || echo "0")
        
        TOTAL=$((AUTO_COUNT + CONFIRM_COUNT + GUIDE_COUNT))
        
        if [[ $TOTAL -gt 0 ]]; then
            check_pass "$skill.yaml has $TOTAL automation flags (auto=$AUTO_COUNT, confirm=$CONFIRM_COUNT, guide=$GUIDE_COUNT)"
        else
            check_fail "$skill.yaml has no automation flags"
        fi
    fi
done

echo ""

# ========================================
# Test 7: Documentation Files
# ========================================
echo "--- Test 7: Documentation Files ---"

for doc_file in TEMPLATE.yaml PLATFORM.md README.md; do
    if [[ -f "$SKILLS_DIR/$doc_file" ]]; then
        check_pass ".skills/$doc_file exists"
    else
        check_fail ".skills/$doc_file missing"
    fi
done

echo ""

# ========================================
# Test 8: References to deploy/ Scripts
# ========================================
echo "--- Test 8: References to deploy/ Scripts ---"

for skill in deploy status cloudflare provision; do
    YAML_FILE="$SKILLS_DIR/$skill.yaml"
    
    if [[ -f "$YAML_FILE" ]]; then
        if grep -q "deploy/" "$YAML_FILE"; then
            check_pass "$skill.yaml references deploy/ scripts"
        else
            check_info "$skill.yaml does not reference deploy/ scripts (may be intentional)"
        fi
    fi
done

echo ""

# ========================================
# Test 9: SKILL.md Content Quality
# ========================================
echo "--- Test 9: SKILL.md Content Quality ---"

for skill in deploy status cloudflare provision; do
    SKILL_MD="$SKILLS_DIR/$skill/SKILL.md"
    
    if [[ -f "$SKILL_MD" ]]; then
        # Check for essential sections
        for section in "Quick Reference" "Usage" "Platform Support"; do
            if grep -q "## .*$section" "$SKILL_MD"; then
                check_pass "$skill/SKILL.md has '$section' section"
            else
                check_info "$skill/SKILL.md missing '$section' section (optional)"
            fi
        done
    fi
done

echo ""

# ========================================
# Test 10: Integration with Project Docs
# ========================================
echo "--- Test 10: Integration with Project Docs ---"

# Check doc/armorclaw.md
if grep -q "Deployment Skills" "$PROJECT_ROOT/doc/armorclaw.md" 2>/dev/null; then
    check_pass "doc/armorclaw.md references deployment skills"
else
    check_fail "doc/armorclaw.md missing deployment skills reference"
fi

# Check README.md
if grep -q "Deployment Skills" "$PROJECT_ROOT/README.md" 2>/dev/null; then
    check_pass "README.md references deployment skills"
else
    check_fail "README.md missing deployment skills reference"
fi

echo ""

# ========================================
# Summary
# ========================================
echo "========================================"
echo "Summary"
echo "========================================"
echo -e "${GREEN}Passed:${NC} $PASS"
echo -e "${RED}Failed:${NC} $FAIL"
echo ""

if [[ $FAIL -gt 0 ]]; then
    echo -e "${RED}Status: FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}Status: PASSED${NC}"
    exit 0
fi
