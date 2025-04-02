#!/usr/bin/env python3
import os
import sys
import json
import hashlib
import argparse
from typing import List, Dict

class FirmwarePatchCreator:
    """CLI tool for creating firmware update patch manifests."""
    
    def __init__(self):
        self.valid_paths = [
            "/sda1/data/apps/",
            "/sda1/data/basic/",
            "/sda1/data/core/",
            "/sda1/boot/"
        ]
        self.default_values_path = "/sda1/data/.defaultvalues"
    
    def calculate_sha256(self, file_path: str) -> str:
        """Calculate SHA256 hash of a file."""
        if not os.path.exists(file_path):
            return "0" * 64  # Placeholder hash if file doesn't exist
        
        sha256_hash = hashlib.sha256()
        try:
            with open(file_path, "rb") as f:
                for byte_block in iter(lambda: f.read(4096), b""):
                    sha256_hash.update(byte_block)
            return sha256_hash.hexdigest()
        except Exception:
            return "0" * 64  # Return placeholder hash on error
    
    def is_valid_path(self, path: str) -> bool:
        """Check if the path is within the allowed firmware paths."""
        path = os.path.abspath(path)
        return any(path.startswith(valid_path) for valid_path in self.valid_paths)
    
    

    def create_patch_manifest(self, add_files=None, add_dir=None, remove_files=None, commands=None, scripts=None, modify_defaults=None, manifest_name="patch_manifest.json"):
        """
        Create a JSON manifest for firmware updates and a corresponding restore manifest.
        """
        add_files = add_files or []
        remove_files = remove_files or []
        commands = commands or []
        scripts = scripts or []
        modify_defaults = modify_defaults or {}

        if os.path.exists(manifest_name):
            try:
                with open(manifest_name, "r") as f:
                    manifest = json.load(f)
            except Exception:
                manifest = {"version": "1.0", "operations": []}
        else:
            manifest = {"version": "1.0", "operations": []}

        operations = []
        restore_operations = []  # To store the reversal operations

         # Define backup directory
        backup_dir = "/sda1/data/restore/backup/"

        # Remove operations first
        for file_path in remove_files:
            if self.is_valid_path(file_path):
                operations.append({"operation": "remove", "path": file_path})
                # Generate backup filename for restore
                backup_filename = backup_dir + file_path.replace("/", "_")
                # In restore, we assume the file should be restored (but without original content)
                restore_operations.append({"operation": "add", "path": file_path, "source": backup_filename})

        # Add operations second
        if add_files and add_dir:
            add_dir = os.path.abspath(add_dir)
            for file_path in add_files:
                full_source_path = os.path.join(add_dir, os.path.basename(file_path))

                if not os.path.exists(full_source_path):
                    print(f"Warning: {full_source_path} not found, skipping.")
                    continue

                file_size = os.path.getsize(full_source_path)
                checksum = self.calculate_sha256(full_source_path)
                target_dir = os.path.dirname(file_path)

                operations.append({
                    "operation": "add",
                    "path": target_dir,
                    "source": "/tmp/" + os.path.basename(file_path),
                    "checksum": checksum,
                    "size": file_size
                })

                # In restore, we need to remove the file that was added
                restore_operations.append({"operation": "remove", "path": os.path.join(target_dir, os.path.basename(file_path))})

        # Add remaining operations (commands, scripts, modify_defaults)
        for command in commands:
            operations.append({"operation": "command", "command": command})

        for script in scripts:
            operations.append({
                "operation": "script",
                "script_name": script["script_name"],
                "script_content": script["script_content"]
            })

        if modify_defaults:
            operations.append({"operation": "modify_defaults", "entries": modify_defaults})

        # Save patch_manifest.json
        manifest["operations"] = operations
        try:
            with open(manifest_name, "w") as f:
                json.dump(manifest, f, indent=2)
            print(f"Firmware patch manifest updated: {manifest_name}")
        except Exception as e:
            print(f"Error saving manifest: {e}")
            sys.exit(1)

        # Save patch_restore_manifest.json
        restore_manifest_name = "patch_restore_manifest.json"
        restore_manifest = {"version": "1.0", "operations": restore_operations}

        try:
            with open(restore_manifest_name, "w") as f:
                json.dump(restore_manifest, f, indent=2)
            print(f"Firmware restore manifest created: {restore_manifest_name}")
        except Exception as e:
            print(f"Error saving restore manifest: {e}")
            sys.exit(1)

    def parse_modify_defaults(self, modify_defaults: List[str]) -> Dict[str, Dict[str, str]]:
        """Parses modify-defaults arguments into a structured dictionary."""
        parsed_defaults = {}
        for entry in modify_defaults:
            if ":" in entry:
                section, key_value = entry.split(":", 1)
            else:
                section = "global"  # Default section
                key_value = entry
        
            if "=" in key_value:
                key, value = key_value.split("=", 1)
                if section not in parsed_defaults:
                    parsed_defaults[section] = {}
                parsed_defaults[section][key] = value
            else:
                print(f"Warning: Skipping invalid modify-defaults entry '{entry}'")
    
        return parsed_defaults

def main():
    parser = argparse.ArgumentParser(description="Firmware Update Patch Manifest Creator")
    parser.add_argument("--add", nargs="+", help="Files to add (target paths within valid locations)")
    parser.add_argument("--remove", nargs="+", help="Files to remove")
    parser.add_argument("--command", nargs="+", help="Bash commands to execute")
    parser.add_argument("--script", nargs="*", help="Path to script files to embed in the JSON manifest")
    parser.add_argument("--modify-defaults", nargs="*", help="Modify .defaultvalues file (formatted as [Section]:key=value or key=value)")
    parser.add_argument("--manifest", default="patch_manifest.json", help="Name of the manifest file")
    
    args = parser.parse_args()
    scripts = []
    
    if args.script is not None and len(args.script) == 0:
        script_name = input("Enter script name (e.g., my_script.sh): ").strip()
        print("Enter script content below. Press Ctrl+D (Linux/macOS) or Ctrl+Z (Windows) on a new line to finish:")
        try:
            script_content = sys.stdin.read()
            if script_content.strip():
                scripts.append({"script_name": script_name, "script_content": script_content})
        except EOFError:
            print("No script content provided.")
            sys.exit(1)
    
    if args.script:
        for script_path in args.script:
            if os.path.exists(script_path):
                with open(script_path, "r") as script_file:
                    scripts.append({"script_name": os.path.basename(script_path), "script_content": script_file.read()})
            else:
                print(f"Warning: Script {script_path} not found, skipping.")

    add_dir = None
    if args.add:
        add_dir = input("Enter the local directory containing files to be added: ").strip()
        if not os.path.isdir(add_dir):
            print("Error: Provided add directory does not exist.")
            sys.exit(1)
    
    modify_defaults = {}
    if args.modify_defaults:
        creator = FirmwarePatchCreator()
        modify_defaults = creator.parse_modify_defaults(args.modify_defaults)

    creator = FirmwarePatchCreator()
    creator.create_patch_manifest(
        add_files=args.add,
        add_dir=add_dir,
        remove_files=args.remove,
        commands=args.command,
        scripts=scripts,
        modify_defaults=modify_defaults,
        manifest_name=args.manifest
    )

if __name__ == "__main__":
    main()
