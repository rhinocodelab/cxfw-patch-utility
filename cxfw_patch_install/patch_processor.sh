#!/bin/sh
# Script: patch_processor.sh
# Purpose: Process a patch file by verifying, uncorrupting, extracting, and recording its status.
# Date: April 2025

# Define log file and database
LOG_FILE="/tmp/cxfw.log"
DB_FILE="/newroot/data/sysconf.db"
METADATA_FILE="/newroot/data/metadata.json"
PATCH_FILE="/sda1/data/cxfw/patch/patch.cx"
EXTRACT_DIR="/tmp/patch"

# Logging functions with levels
debug() { echo "[DEBUG] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"; }
log() { echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"; }
error() { echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') $1" >> "$LOG_FILE"; }

# Generic error handler
handle_error() {
    error "$1"
    update_metadata_status "fail" >/dev/null 2>&1 || true
    exit 1
}

# Initialize log file
init_logging() {
    /bin/touch "$LOG_FILE" && chmod 644 "$LOG_FILE" || handle_error "Failed to initialize log file"
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
    key="$1"
    if [ ! -f "$METADATA_FILE" ]; then
        handle_error "metadata.json not found in get_metadata_value"
    fi
    value=$(jq -r ".$key" "$METADATA_FILE" 2>> "$LOG_FILE")
    if [ $? -ne 0 ]; then
        handle_error "Failed to parse metadata.json for key $key"
    fi
    echo "$value"
}

# Update metadata status directly in the original file
update_metadata_status() {
    status="$1"

    # Ensure metadata file exists before updating
    if [ ! -f "$METADATA_FILE" ]; then
        handle_error "Metadata file $METADATA_FILE not found"
    fi

    # Use jq to update the status safely, writing to a temp file first
    tmp_file="/tmp/metadata_temp.json"
    jq --arg status "$status" '.status = $status' "$METADATA_FILE" > "$tmp_file" 2>> "$LOG_FILE" || handle_error "Failed to update metadata.json"

    # Verify the temp file is not empty and contains valid JSON before replacing the original
    if [ -s "$tmp_file" ] && jq empty "$tmp_file" 2>> "$LOG_FILE"; then
        mv "$tmp_file" "$METADATA_FILE" 2>> "$LOG_FILE" || handle_error "Failed to move updated metadata.json"
        log "Updated metadata.json status to '$status'"
    else
        handle_error "metadata.json update resulted in an invalid file"
    fi
}


# Check if patch is already applied
check_patch_applied() {
    checksum=$(get_metadata_value "checksum")
    if [ ! -f "$DB_FILE" ]; then
        log "Database file $DB_FILE not found"
    else
        log "Checking if patch with checksum $checksum is already applied"
        count=$(sqlite3 "$DB_FILE" "SELECT EXISTS(SELECT 1 FROM MD5SUM WHERE Md5Sum = '$checksum')" 2>> "$LOG_FILE" || handle_error "Database query failed")
        [ "$count" -eq 1 ] && log "Patch already applied" && update_metadata_status "success" && exit 0
    fi
}

# Verify checksum
verify_checksum() {
    expected_checksum=$(get_metadata_value "checksum")
    actual_checksum=$(sha256sum "$PATCH_FILE" | cut -d' ' -f1) || handle_error "Failed to compute checksum"
    [ "$actual_checksum" != "$expected_checksum" ] && handle_error "Checksum mismatch. Expected: $expected_checksum, Got: $actual_checksum"
    log "Checksum verification successful"
}


# Extract patch file
extract_patch() {
    tar -xjf "$PATCH_FILE" -C /tmp/ 2>> "$LOG_FILE" || handle_error "Failed to extract patch.cx"
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

    # Check if the patch executor exists and is executable
    if [ ! -x "/cxfw/cxfw_patch_executor" ]; then
        handle_error "Patch executor /cxfw/cxfw_patch_executor not found or not executable"
    fi

    # Run the patch executor
    /cxfw/cxfw_patch_executor "$EXTRACT_DIR/patch_manifest.json" 2>> "$LOG_FILE"
    
    # Capture exit status
    exit_status=$?
    
    if [ $exit_status -ne 0 ]; then
        handle_error "Failed to install patch_manifest.json (exit code: $exit_status)"
    fi

    log "Successfully installed patch_manifest.json"
}


# Copy patch_rollback.json and defaultvalues_comparison.json to rollback
copy_patch_rollback() {
    cp "$EXTRACT_DIR/patch_rollback_manifest.json" "/sda1/data/cxfw/rollback/patch_rollback_manifest.json" 2>> "$LOG_FILE" || handle_error "Failed to copy patch_rollback_manifest.json to rollback"
    if [ -f "/tmp/defaultvalues_comparison.json" ]; then
        cp "/tmp/defaultvalues_comparison.json" "/sda1/data/cxfw/rollback/defaultvalues_comparison.json" 2>> "$LOG_FILE" || handle_error "Failed to copy defaultvalues_comparison.json"
    fi
    log "Successfully copied patch_rollback.json to rollback"
}

# Insert patch info into database, creating table if needed
insert_patch_info() {
    if [ ! -f "$DB_FILE" ]; then
        handle_error "Database file $DB_FILE not found"
    fi

    # Check if MD5SUM table exists
    table_exists=$(sqlite3 "$DB_FILE" "SELECT name FROM sqlite_master WHERE type='table' AND name='MD5SUM';" 2>> "$LOG_FILE")

    # Create the table if it does not exist
    if [ -z "$table_exists" ]; then
        log "MD5SUM table not found. Creating table..."
        sqlite3 "$DB_FILE" <<EOF 2>> "$LOG_FILE" || handle_error "Failed to create MD5SUM table"
        CREATE TABLE MD5SUM (
            Filename TEXT NOT NULL,
            Md5Sum TEXT PRIMARY KEY,
            PatchVersion TEXT,
            PatchName TEXT,
            Description TEXT,
            Status INTEGER,
            DateString TEXT
        );
EOF
        log "MD5SUM table created successfully"
    fi

    # Extract metadata values
    patch_version=$(get_metadata_value "patch_version")
    patch_name=$(get_metadata_value "patch_name")
    checksum=$(get_metadata_value "checksum")
    description=$(get_metadata_value "description")
    status=1  # Assuming 1 means "applied"
    datestring=$(date '+%Y-%m-%d %H:%M:%S')

    # Insert into MD5SUM table (only if checksum is not already present)
    sqlite3 "$DB_FILE" <<EOF 2>> "$LOG_FILE" || handle_error "Failed to insert patch info into database"
    INSERT INTO MD5SUM (Filename, Md5Sum, PatchVersion, PatchName, Description, Status, DateString)
    VALUES ('patch.cx', '$checksum', '$patch_version', '$patch_name', '$description', $status, '$datestring')
    ON CONFLICT(Md5Sum) DO NOTHING;
EOF

    log "Patch info inserted into database successfully"
}



# Main processing function
process_patch() {
    if [ ! -f "$PATCH_FILE" ]; then
        handle_error "Patch file patch.cx not found"
    fi
    if [ ! -f "$METADATA_FILE" ]; then
        handle_error "Metadata file metadata.json not found"
    fi
    log "Processing patch: patch.cx"
    check_patch_applied
    verify_checksum
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