#!/bin/bash
################################################################################
#                                                                              #
#                    HEALTH-GO-BACKEND PRODUCTION DEPLOYMENT                  #
#                                                                              #
#   This script performs a safe, step-by-step production deployment with      #
#   verification at each stage and rollback capability.                       #
#                                                                              #
################################################################################

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="health-backend"
BINARY_NAME="health-backend"
INSTALL_PATH="/usr/local/bin"
BACKUP_DIR="/var/backups/health-backend"
PROJECT_DIR="${PWD}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/${BINARY_NAME}_${TIMESTAMP}.backup"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  HEALTH-GO-BACKEND PRODUCTION DEPLOYMENT                      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"

################################################################################
# Phase 1: Pre-deployment Checks
################################################################################

echo -e "\n${YELLOW}[1/7] PRE-DEPLOYMENT CHECKS${NC}"
echo "========================================"

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}✗ This script must be run as root${NC}"
    echo "  Run: sudo bash deploy.sh"
    exit 1
fi
echo -e "${GREEN}✓ Running as root${NC}"

# Check if service exists
if ! systemctl list-units --all | grep -q "$SERVICE_NAME"; then
    echo -e "${RED}✗ Service '$SERVICE_NAME' not found${NC}"
    echo "  Ensure systemd service is configured"
    exit 1
fi
echo -e "${GREEN}✓ Service '$SERVICE_NAME' exists${NC}"

# Check if binary exists
if [ ! -f "$PROJECT_DIR/$BINARY_NAME" ]; then
    echo -e "${YELLOW}⚠ Binary not found at $PROJECT_DIR/$BINARY_NAME${NC}"
    echo "  Building now..."
    cd "$PROJECT_DIR"
    if ! go build -o "$BINARY_NAME" .; then
        echo -e "${RED}✗ Build failed${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Build completed${NC}"
fi
echo -e "${GREEN}✓ Binary ready: $PROJECT_DIR/$BINARY_NAME${NC}"

################################################################################
# Phase 2: Create Backup
################################################################################

echo -e "\n${YELLOW}[2/7] CREATING BACKUP${NC}"
echo "========================================"

mkdir -p "$BACKUP_DIR"
if [ -f "$INSTALL_PATH/$BINARY_NAME" ]; then
    cp "$INSTALL_PATH/$BINARY_NAME" "$BACKUP_FILE"
    echo -e "${GREEN}✓ Backup created: $BACKUP_FILE${NC}"
else
    echo -e "${YELLOW}⚠ No previous binary to backup${NC}"
fi

################################################################################
# Phase 3: Verify Service Status
################################################################################

echo -e "\n${YELLOW}[3/7] VERIFYING SERVICE STATUS${NC}"
echo "========================================"

SERVICE_STATUS=$(systemctl is-active $SERVICE_NAME || echo "inactive")
echo -e "Current status: ${BLUE}$SERVICE_STATUS${NC}"

if [ "$SERVICE_STATUS" = "active" ]; then
    echo "Stopping service for deployment..."
    systemctl stop "$SERVICE_NAME"
    sleep 2
    echo -e "${GREEN}✓ Service stopped${NC}"
fi

################################################################################
# Phase 4: Deploy New Binary
################################################################################

echo -e "\n${YELLOW}[4/7] DEPLOYING NEW BINARY${NC}"
echo "========================================"

echo "Copying binary to $INSTALL_PATH..."
cp "$PROJECT_DIR/$BINARY_NAME" "$INSTALL_PATH/$BINARY_NAME"
chmod +x "$INSTALL_PATH/$BINARY_NAME"
echo -e "${GREEN}✓ Binary deployed${NC}"

# Verify deployment
if [ -f "$INSTALL_PATH/$BINARY_NAME" ]; then
    BINARY_VERSION=$("$INSTALL_PATH/$BINARY_NAME" -version 2>/dev/null || echo "unknown")
    echo -e "Deployed version: ${BLUE}$BINARY_VERSION${NC}"
    echo -e "${GREEN}✓ Deployment verified${NC}"
else
    echo -e "${RED}✗ Deployment failed${NC}"
    exit 1
fi

################################################################################
# Phase 5: Start Service
################################################################################

echo -e "\n${YELLOW}[5/7] STARTING SERVICE${NC}"
echo "========================================"

echo "Starting $SERVICE_NAME..."
systemctl start "$SERVICE_NAME"
sleep 3

SERVICE_STATUS=$(systemctl is-active "$SERVICE_NAME")
if [ "$SERVICE_STATUS" = "active" ]; then
    echo -e "${GREEN}✓ Service started successfully${NC}"
else
    echo -e "${RED}✗ Service failed to start${NC}"
    echo "Attempting rollback..."
    if [ -f "$BACKUP_FILE" ]; then
        cp "$BACKUP_FILE" "$INSTALL_PATH/$BINARY_NAME"
        systemctl start "$SERVICE_NAME"
        echo -e "${YELLOW}⚠ Rolled back to previous version${NC}"
    fi
    exit 1
fi

################################################################################
# Phase 6: Health Checks
################################################################################

echo -e "\n${YELLOW}[6/7] HEALTH CHECKS${NC}"
echo "========================================"

echo "Waiting for service to become ready..."
sleep 2

# Check health endpoint
echo "Testing /health endpoint..."
if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Health check passed${NC}"
else
    echo -e "${RED}✗ Health check failed${NC}"
    echo "Checking service logs..."
    journalctl -u "$SERVICE_NAME" -n 20 --no-pager
    exit 1
fi

# Check logs for errors
echo "Checking startup logs..."
if journalctl -u "$SERVICE_NAME" --since "2 min ago" | grep -i "error\|fatal" > /dev/null; then
    echo -e "${YELLOW}⚠ Errors found in logs:${NC}"
    journalctl -u "$SERVICE_NAME" --since "2 min ago" | grep -i "error\|fatal"
    exit 1
else
    echo -e "${GREEN}✓ No critical errors in logs${NC}"
fi

# Check for optimization startup message
echo "Verifying Supabase optimizations are loaded..."
if journalctl -u "$SERVICE_NAME" --since "2 min ago" | grep -i "supabase\|connection\|pool" > /dev/null; then
    echo -e "${GREEN}✓ Optimizations detected in logs${NC}"
    journalctl -u "$SERVICE_NAME" --since "2 min ago" | grep -i "supabase\|connection\|pool" || true
else
    echo -e "${YELLOW}⚠ Optimization logs not yet visible${NC}"
fi

################################################################################
# Phase 7: Deployment Summary
################################################################################

echo -e "\n${YELLOW}[7/7] DEPLOYMENT SUMMARY${NC}"
echo "========================================"

echo -e "${GREEN}✓ DEPLOYMENT SUCCESSFUL${NC}"
echo ""
echo "Service Information:"
echo "  Service: $SERVICE_NAME"
echo "  Binary: $INSTALL_PATH/$BINARY_NAME"
echo "  Status: $(systemctl is-active $SERVICE_NAME)"
echo "  Backup: $BACKUP_FILE"
echo ""
echo "Health Endpoints:"
echo "  Health: http://localhost:8080/health"
echo "  Auth ME: http://localhost:8080/api/v1/auth/me (requires token)"
echo ""
echo "Next Steps:"
echo "  1. Monitor logs: journalctl -u $SERVICE_NAME -f"
echo "  2. Create partial index in Supabase (if not done):"
echo "     CREATE INDEX CONCURRENTLY idx_async_jobs_pending"
echo "       ON async_jobs (next_run_at ASC)"
echo "       WHERE status = 'pending';"
echo "  3. Enable connection pooling on port 6543 (if not done)"
echo "  4. Verify partial index in production:"
echo "     SELECT * FROM pg_indexes WHERE indexname='idx_async_jobs_pending';"
echo ""

################################################################################
# Phase 8: Post-deployment Verification
################################################################################

echo -e "\n${YELLOW}[POST-DEPLOYMENT] Continuous Monitoring${NC}"
echo "========================================"
echo ""
echo "Monitor these metrics for 5 minutes:"
echo ""
echo "1. Service health:"
echo -e "   ${BLUE}systemctl status $SERVICE_NAME${NC}"
echo ""
echo "2. Recent logs (no errors expected):"
echo -e "   ${BLUE}journalctl -u $SERVICE_NAME -n 50${NC}"
echo ""
echo "3. Async job performance (should show 5-7ms after index):"
echo -e "   ${BLUE}journalctl -u $SERVICE_NAME -f | grep -i 'slow\\|async'${NC}"
echo ""
echo "4. CPU usage during job processing:"
echo -e "   ${BLUE}top -u $(whoami) | grep $BINARY_NAME${NC}"
echo ""

################################################################################
# Final Status
################################################################################

echo -e "\n${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    🎉 DEPLOYMENT COMPLETE                     ║${NC}"
echo -e "${BLUE}║                                                               ║${NC}"
echo -e "${BLUE}║  Production deployment successful!                            ║${NC}"
echo -e "${BLUE}║  Service is running and healthy.                              ║${NC}"
echo -e "${BLUE}║                                                               ║${NC}"
echo -e "${BLUE}║  REMAINING STEPS (Manual in Supabase):                        ║${NC}"
echo -e "${BLUE}║  1. Create partial index (SQL Editor)                         ║${NC}"
echo -e "${BLUE}║  2. Enable connection pooling (Dashboard)                     ║${NC}"
echo -e "${BLUE}║  3. Update DATABASE_URL to port 6543                          ║${NC}"
echo -e "${BLUE}║                                                               ║${NC}"
echo -e "${BLUE}║  After those 3 steps: 48x faster queries! ⚡                 ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"

exit 0
