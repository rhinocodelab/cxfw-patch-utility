# Firmware Patch Creator

Firmware Patch Creator is a CLI tool designed to generate JSON manifest files for firmware update patches. The manifest file includes operations such as adding, removing files, executing bash commands, embedding scripts, and modifying specific configuration values in `/sda1/data/.defaultvalues`.

## Features
- Add files from a specified directory.
- Remove files from the system.
- Execute bash commands as part of the update process.
- Embed scripts directly into the manifest.
- Modify entries in the `.defaultvalues` file with sectioned or standalone key-value pairs.
- Prevent duplicate entries in the manifest file.
- Append new operations to an existing manifest instead of overwriting.

## Installation
Ensure you have Python 3 installed, then clone the repository and navigate to the directory:
```sh
$ git clone https://github.com/your-repo/firmware-patch-creator.git
$ cd firmware-patch-creator
```
Give the script execution permissions if necessary:
```sh
$ chmod +x firmware_patch_creator.py
```

## Usage
### 1. Add files
To add files, specify the target paths and provide the local directory containing the actual files:
```sh
$ ./firmware_patch_creator.py --add /sda1/data/apps/file1.bin /sda1/data/core/file2.bin
```
The script will prompt for the local directory path containing these files. The source path in the manifest will always be `/tmp/filename`.

### 2. Remove files
To remove files, specify their paths:
```sh
$ ./firmware_patch_creator.py --remove /sda1/data/apps/oldfile.bin /sda1/data/core/legacy.bin
```

### 3. Execute bash commands
To add bash commands to the manifest:
```sh
$ ./firmware_patch_creator.py --command "systemctl restart service" "rm -rf /tmp/tempfiles"
```

### 4. Embed scripts
To embed script files directly into the manifest:
```sh
$ ./firmware_patch_creator.py --script my_script.sh another_script.sh
```
If no script files are provided, the tool will prompt the user to enter a script name and content interactively.

### 5. Modify `.defaultvalues` file
To modify configuration values, use:
```sh
$ ./firmware_patch_creator.py --modify-defaults "[Imprivata]:CheckDualDisplay=0" "CheckCertificate=0"
```
- Entries formatted as `[Section]:key=value` belong to a specific section.
- Standalone entries are directly added to the file.

### 6. Specifying a custom manifest name
```sh
$ ./firmware_patch_creator.py --add /sda1/data/apps/newfile.bin --manifest my_patch.json
```

## Sample JSON Output
```json
{
  "version": "1.0",
  "operations": [
    {
      "operation": "add",
      "path": "/sda1/data/apps/file1.bin",
      "source": "/tmp/file1.bin",
      "checksum": "abc123...",
      "size": 2048
    },
    {
      "operation": "remove",
      "path": "/sda1/data/core/legacy.bin"
    },
    {
      "operation": "command",
      "command": "systemctl restart service"
    },
    {
      "operation": "script",
      "script_name": "my_script.sh",
      "script_content": "#!/bin/bash\necho 'Hello World'"
    },
    {
      "operation": "modify_defaults",
      "entries": {
        "Imprivata": {"CheckDualDisplay": "0"},
        "CheckCertificate": "0"
      }
    }
  ]
}
```

## Notes
- The tool ensures no duplicate entries are added.
- The `--add` option requires specifying the local directory.
- If no script files are provided with `--script`, the tool prompts for script input.
- Manifest files are updated incrementally rather than being overwritten.

## License
This project is licensed under the MIT License.

## Contributing
Pull requests are welcome. For major changes, open an issue first to discuss the proposed updates.

## Author
[Your Name]
