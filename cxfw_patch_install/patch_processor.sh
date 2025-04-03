#!/bin/bash
# Script: patch_processor.sh
# Purpose: Process a patch file by verifying, uncorrupting, extracting, and recording its status.
# Inputs: Expects a patch file and metadata.json in /sda1/data/cxfw/patch/
# Outputs: Updates /newroot/data/sysconf.db and metadata.json; logs to /tmp/cxfw.log
# Idempotency: Skips processing if patch checksum exists in MD5SUM table
# Dependencies: jq, sqlite3, sha256sum, tar, damage
# Date: April 2025

# Define log file and database
LOG_FILE="/tmp/cxfw.log"
DB_FILE="/newroot/data/sysconf.db"
METADATA_FILE="/sda1/data/cxfw/patch/metadata.json"
PATCH_FILE="/sda1/data/cxfw/patch/patch.cx"
EXTRACT_DIR="/tmp/patch"

# Logging function with levels
debug() { echo "[DEBUG] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"; }
log() { echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"; }
error() { echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"; }

# Generic error handler
handle_error() {
    error "$1"
    update_metadata_status "fail"
    exit 1
}

# Initialize log file
init_logging() {
    touch "$LOG_FILE" && chmod 644 "$LOG_FILE"
    log "Script execution started"
}

# Mount /sda1 before processing
mount_sda1() {
    log "Mounting /sda1 with label 'sukshm'"
    /bin/mount -o nodev -L sukshm /sda1 2>> "$LOG_FILE" || handle_error "Failed to mount /sda1"
    log "/sda1 mounted successfully"
}

# Read metadata.json
get_metadata_value() {
    local key="$1"
    [ ! -f "$METADATA_FILE" ] && handle_error "metadata.json not found"
    jq -r ".$key" "$METADATA_FILE" 2>> "$LOG_FILE" || handle_error "Failed to parse metadata.json"
}

# Update metadata status
update_metadata_status() {
    local status="$1"
    jq ".status = \"$status\"" "$METADATA_FILE" > "/tmp/metadata_temp.json" 2>> "$LOG_FILE" || handle_error "Failed to update metadata.json"
    mv "/tmp/metadata_temp.json" "$METADATA_FILE" 2>> "$LOG_FILE" || handle_error "Failed to move updated metadata.json"
    log "Updated metadata.json status to $status"
}

# Check if patch is already applied
check_patch_applied() {
    local checksum=$(get_metadata_value "checksum")
    [ ! -f "$DB_FILE" ] && handle_error "Database file $DB_FILE not found"
    log "Checking if patch with checksum $checksum is already applied"
    local count=$(sqlite3 "$DB_FILE" "SELECT EXISTS(SELECT 1 FROM MD5SUM WHERE Md5Sum = '$checksum')" 2>> "$LOG_FILE")
    [ "$count" -eq 1 ] && log "Patch already applied" && update_metadata_status "success" && exit 0
}

# Verify checksum
verify_checksum() {
    local expected_checksum=$(get_metadata_value "checksum")
    local actual_checksum=$(sha256sum "$PATCH_FILE" | cut -d' ' -f1)
    [ "$actual_checksum" != "$expected_checksum" ] && handle_error "Checksum mismatch. Expected: $expected_checksum, Got: $actual_checksum"
    log "Checksum verification successful"
}

# Copy patch file
copy_patch() {
    cp "$PATCH_FILE" "/tmp/$(basename "$PATCH_FILE")" 2>> "$LOG_FILE" || handle_error "Failed to copy patch file to /tmp"
    log "Patch file copied successfully"
}

# Uncorrupt patch file
uncorrupt_patch() {
    damage uncorrupt "/tmp/$(basename "$PATCH_FILE")" 1 2>> "$LOG_FILE" || handle_error "Failed to uncorrupt patch.cx"
    log "Successfully uncorrupted patch.cx"
}

# Extract patch file
extract_patch() {
    rm -rf "$EXTRACT_DIR"
    mkdir -p "$EXTRACT_DIR"
    tar -xjf "/tmp/$(basename "$PATCH_FILE")" -C "$EXTRACT_DIR" 2>> "$LOG_FILE" || handle_error "Failed to extract patch.cx"
    log "Successfully extracted patch.cx to $EXTRACT_DIR"
}

# Create .defaultvalues reversal JSON
create_defaultvalues_reversal() {
    /cxfw/generate_defaultvalues_comparison --input "$EXTRACT_DIR/patch_manifest.json" 2>> "$LOG_FILE" || handle_error "Failed to create defaultvalues reversal JSON"
    log "Successfully created defaultvalues_reversal.json"
}
# Install patch_manifest.json
install_patch_manifest() {
    log "Installing patch using patch_manifest.json"
    /cxfw/cxfw_patch_executor "$EXTRACT_DIR/patch_manifest.json" 2>> "$LOG_FILE" || handle_error "Failed to install patch_manifest.json"
    log "Successfully installed patch_manifest.json"
}

# Copy patch_rollback.json and defaultvalues_comparison.json to rollback
copy_patch_rollback() {
    cp "$EXTRACT_DIR/patch_rollback_manifest.json" "/sda1/data/cxfw/rollback/patch_rollback_manifest.json" 2>> "$LOG_FILE" || handle_error "Failed to copy patch_rollback_manifest.json to rollback"
    if [ -f "/tmp/defaultvalues_comparison.json" ]; then
        cp "/tmp/defaultvalues_comparison.json" "/sda1/data/cxfw/rollback/defaultvalues_comparison.json"
    fi
    log "Successfully copied patch_rollback.json to rollback"
}

# Insert patch info into database
insert_patch_info() {
    local checksum=$(get_metadata_value "checksum")
    local status=1
    local datestring=$(date '+%Y-%m-%d %H:%M:%S')
    sqlite3 "$DB_FILE" "INSERT INTO MD5SUM (Filename, Md5Sum, Status, DateString) VALUES ('patch.cx', '$checksum', $status, '$datestring') ON CONFLICT(Md5Sum) DO NOTHING;" 2>> "$LOG_FILE" || handle_error "Failed to insert patch info into database"
    log "Patch info inserted into database"
}

# Main processing function
process_patch() {
    [ ! -f "$PATCH_FILE" ] && handle_error "Patch file patch.cx not found"
    [ ! -f "$METADATA_FILE" ] && handle_error "Metadata file metadata.json not found"
    log "Processing patch: patch.cx"
    check_patch_applied
    verify_checksum
    copy_patch
    uncorrupt_patch
    extract_patch
    create_defaultvalues_reversal
    install_patch_manifest
    copy_patch_rollback
    insert_patch_info
    update_metadata_status "success"
}

# Main execution
main() {
    init_logging
    mount_sda1
    process_patch
    log "Script execution completed successfully"
    exit 0
}

# Start script
main
