#!/bin/bash
# Script: patch_processor.sh
# Purpose: Process a patch file by verifying, uncorrupting, extracting, and recording its status.
# Inputs: Expects a patch file and metadata.json in /sda1/data/restore/patch/
# Outputs: Updates /newroot/data/sysconf.db and metadata.json; logs to /tmp/cxfw.log
# Idempotency: Skips processing if patch checksum exists in MD5SUM table
# Dependencies: jq, sqlite3, sha256sum, tar, damage
# Author: Prashant Pokhriyal
# Date: April 2025

# Define log file and database
LOG_FILE="/tmp/cxfw.log"
DB_FILE="/newroot/data/sysconf.db"
METADATA_FILE="/sda1/data/restore/patch/metadata.json"

# Logging function
log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] $1" >> "$LOG_FILE"
}

# Initialize log file
init_logging() {
    if [ ! -f "$LOG_FILE" ]; then
        touch "$LOG_FILE"
        chmod 644 "$LOG_FILE"
        log "Log file created"
    fi
    log "Script execution started"
}

# Read metadata and return value for given key
get_metadata_value() {
    local key="$1"
    
    if [ ! -f "$METADATA_FILE" ]; then
        log "ERROR: metadata.json not found"
        exit 1
    fi
    
    jq -r ".$key" "$METADATA_FILE" 2>> "$LOG_FILE"
    if [ $? -ne 0 ]; then
        log "ERROR: Failed to parse metadata.json"
        exit 1
    fi
}

# Update metadata.json with status
update_metadata_status() {
    local status="$1"
    local temp_file="/tmp/metadata_temp.json"
    
    log "Updating metadata.json with status: $status"
    jq ".status = \"$status\"" "$METADATA_FILE" > "$temp_file" 2>> "$LOG_FILE"
    if [ $? -eq 0 ]; then
        mv "$temp_file" "$METADATA_FILE" 2>> "$LOG_FILE"
        if [ $? -eq 0 ]; then
            log "Successfully updated metadata.json with status: $status"
            return 0
        else
            log "ERROR: Failed to move updated metadata.json"
            return 1
        fi
    else
        log "ERROR: Failed to update metadata.json with jq"
        return 1
    fi
}

# Check if patch is already applied by checking Md5Sum in database
check_patch_applied() {
    local checksum=$(get_metadata_value "checksum")
    
    if [ ! -f "$DB_FILE" ]; then
        log "ERROR: Database file $DB_FILE not found"
        update_metadata_status "fail"
        exit 1
    fi
    
    log "Checking if patch with checksum $checksum is already applied"
    local count=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM MD5SUM WHERE Md5Sum = '$checksum'" 2>> "$LOG_FILE")
    
    if [ $? -ne 0 ]; then
        log "ERROR: Failed to query database"
        update_metadata_status "fail"
        exit 1
    fi
    
    if [ "$count" -gt 0 ]; then
        log "Patch with checksum $checksum already applied"
        return 1
    else
        log "Patch not found in database, proceeding with application"
        return 0
    fi
}

# Verify checksum of patch file
verify_checksum() {
    local patch_file="$1"
    local expected_checksum=$(get_metadata_value "checksum")
    
    log "Verifying checksum of $patch_file"
    local actual_checksum=$(sha256sum "$patch_file" | cut -d' ' -f1)
    
    if [ "$actual_checksum" = "$expected_checksum" ]; then
        log "Checksum verification successful"
        return 0
    else
        log "ERROR: Checksum verification failed. Expected: $expected_checksum, Got: $actual_checksum"
        update_metadata_status "fail"
        return 1
    fi
}

# Copy patch file to /tmp
copy_patch() {
    local file="$1"
    local filename=$(basename "$file")
    
    log "Copying $filename to /tmp"
    cp "$file" "/tmp/$filename" 2>> "$LOG_FILE"
    if [ $? -eq 0 ]; then
        log "Successfully copied $filename to /tmp"
        return 0
    else
        log "ERROR: Failed to copy $filename to /tmp"
        update_metadata_status "fail"
        return 1
    fi
}

# Uncorrupt patch file
uncorrupt_patch() {
    local filename="$1"
    
    log "Attempting to uncorrupt $filename"
    damage uncorrupt "/tmp/$filename" 1 2>> "$LOG_FILE"
    if [ $? -eq 0 ]; then
        log "Successfully uncorrupted $filename"
        return 0
    else
        log "ERROR: Failed to uncorrupt $filename"
        update_metadata_status "fail"
        return 1
    fi
}

# Extract patch file
extract_patch() {
    local filename="$1"
    local temp_dir="/tmp/extract_$(date +%s)"
    
    mkdir -p "$temp_dir"
    log "Created temporary directory $temp_dir"
    
    log "Extracting $filename to $temp_dir"
    tar -xjf "/tmp/$filename" -C "$temp_dir" 2>> "$LOG_FILE"
    if [ $? -eq 0 ]; then
        log "Successfully extracted $filename to $temp_dir"
        return 0
    else
        log "ERROR: Failed to extract $filename"
        update_metadata_status "fail"
        return 1
    fi
}

# Insert patch info into MD5SUM table
insert_patch_info() {
    local filename=$(get_metadata_value "patch_name")
    local checksum=$(get_metadata_value "checksum")
    local status=1  # Assuming 1 means successfully applied
    local datestring=$(date '+%Y-%m-%d %H:%M:%S')
    
    log "Inserting patch information into MD5SUM table"
    sqlite3 "$DB_FILE" "INSERT OR REPLACE INTO MD5SUM (Filename, Md5Sum, Status, DateString) VALUES ('$filename', '$checksum', $status, '$datestring')" 2>> "$LOG_FILE"
    
    if [ $? -eq 0 ]; then
        log "Successfully inserted patch info: $filename, $checksum"
        return 0
    else
        log "ERROR: Failed to insert patch info into database"
        update_metadata_status "fail"
        return 1
    fi
}

# Main processing function
process_patch() {
    local patch_dir="/sda1/data/restore/patch"
    local patch_name=$(get_metadata_value "patch_name")
    local patch_file="$patch_dir/$patch_name"
    
    log "Processing patch: $patch_name"
    
    if [ ! -f "$patch_file" ]; then
        log "ERROR: Patch file $patch_name not found in $patch_dir"
        update_metadata_status "fail"
        exit 1
    fi
    
    # Check if patch is already applied
    check_patch_applied || {
        log "Skipping patch application as it was already applied"
        update_metadata_status "success"  # Already applied is considered success
        exit 0
    }
    
    verify_checksum "$patch_file" || exit 1
    copy_patch "$patch_file" || exit 1
    uncorrupt_patch "$patch_name" || exit 1
    extract_patch "$patch_name" || exit 1
    insert_patch_info || exit 1
    update_metadata_status "success"
}

# Main execution
main() {
    # Check dependencies
    if ! command -v jq >/dev/null 2>&1; then
        log "ERROR: jq is required but not installed"
        exit 1
    fi
    if ! command -v sqlite3 >/dev/null 2>&1; then
        log "ERROR: sqlite3 is required but not installed"
        exit 1
    fi
    
    init_logging
    process_patch
    log "Script execution completed successfully"
    exit 0
}

# Start script
main