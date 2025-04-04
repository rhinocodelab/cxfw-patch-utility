package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Manifest struct {
	Version    string      `json:"version"`
	Operations []Operation `json:"operations"`
}

type Operation struct {
	Operation string                       `json:"operation"`
	Path      string                       `json:"path,omitempty"`
	Source    string                       `json:"source,omitempty"`
	Checksum  string                       `json:"checksum,omitempty"`
	Size      int64                        `json:"size,omitempty"`
	Command   string                       `json:"command,omitempty"`
	Script    string                       `json:"script_content,omitempty"`
	Entries   map[string]map[string]string `json:"entries,omitempty"`
}

// Structure for integrity database entries
type IntegrityEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// Structure for folder-specific JSON content (e.g., .apps.json, .basic.json)
type FolderEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

const logFile = "/newroot/var/log/cxfw_patch.log"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: cxfw_patch_rollback <manifest.json>")
		os.Exit(1)
	}

	manifestPath := os.Args[1]
	logToFile("========== CloudX Firmware Patch Rollback Execution Started ==========")
	logToFile("Loading manifest: " + manifestPath)

	manifest, err := loadManifest(manifestPath)
	if err != nil {
		logToFile("ERROR: Failed to load manifest - " + err.Error())
		os.Exit(1)
	}

	for _, op := range manifest.Operations {
		var err error
		switch op.Operation {
		case "add":
			err = addFile(op)
		case "remove":
			err = removeFile(op)
		case "command":
			err = executeCommand(op)
		case "script":
			err = executeScript(op)
		default:
			logToFile("ERROR: Unknown operation - " + op.Operation)
		}
		if err != nil {
			logToFile("ERROR: Failed to execute operation - " + op.Operation)
			logToFile("Execution stopped due to error.")
			os.Exit(1)
		}
	}
	logToFile("========== CloudX Firmware Patch Rollback Execution Completed ==========")
}

func logToFile(message string) {
	logEntry := time.Now().Format("2006-01-02 15:04:05") + " | " + message + "\n"
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer file.Close()
		file.WriteString(logEntry)
	}
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func computeChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func addFile(op Operation) error {
	if op.Source == "" || op.Path == "" {
		logToFile("ERROR: Invalid add operation, missing source or path")
		return fmt.Errorf("invalid add operation, missing source or path")
	}
	// The destination path is provided in op.Path (e.g., "/sda1/data/basic/app2.bin")
	destFile := op.Path
	sourceFile := op.Source // e.g., "/sda1/data/restore/backup/_sda1_data_basic_app2.bin"

	// Step 1: Create destination directory if it doesn't exist
	destDir := filepath.Dir(destFile)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		logToFile("ERROR: Failed to create directory - " + destDir)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Step 2: Copy file from source to destination
	logToFile("INFO: Copying file from " + sourceFile + " to " + destFile)
	err := copyFile(sourceFile, destFile)
	if err != nil {
		logToFile("ERROR: Failed to copy file - " + err.Error())
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Step 3: Compute and verify checksums
	sourceChecksum, err := computeChecksum(sourceFile)
	if err != nil {
		logToFile("ERROR: Failed to compute source checksum - " + err.Error())
		return fmt.Errorf("failed to compute source checksum: %w", err)
	}

	destChecksum, err := computeChecksum(destFile)
	if err != nil {
		logToFile("ERROR: Failed to compute destination checksum - " + err.Error())
		return fmt.Errorf("failed to compute destination checksum: %w", err)
	}

	if sourceChecksum != destChecksum {
		logToFile("ERROR: Checksum mismatch for copied file " + destFile)
		return fmt.Errorf("checksum mismatch for %s: source %s, got %s", destFile, sourceChecksum, destChecksum)
	}
	logToFile("INFO: File checksum verified successfully - " + destFile)

	// Step 4: Update integrity database with the verified hash
	dbHash, err := updateIntegrityDatabase(destFile, destChecksum)
	if err != nil {
		logToFile("ERROR: Failed to update integrity database - " + err.Error())
		return fmt.Errorf("failed to update integrity database: %w", err)
	}

	// Step 5: Update folder-specific JSON file
	err = updateFolderFile(destDir, dbHash)
	if err != nil {
		logToFile("ERROR: Failed to update folder file - " + err.Error())
		return fmt.Errorf("failed to update folder file: %w", err)
	}

	// Step 6: Remove source file after successful verification and DB update
	err = os.Remove(sourceFile)
	if err != nil {
		logToFile("WARNING: Failed to remove source file - " + err.Error())
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	logToFile("SUCCESS: File added and verified successfully - " + destFile)
	return nil
}

// Helper function to copy file contents
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Ensure file permissions are preserved
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func removeFile(op Operation) error {
	if op.Path == "" {
		logToFile("ERROR: Invalid remove operation, missing path")
		return fmt.Errorf("invalid remove operation, missing path")
	}

	// Step 1: Calculate and store the hash of the file to be removed
	var fileHash string
	if _, err := os.Stat(op.Path); err == nil {
		hash, err := computeChecksum(op.Path)
		if err != nil {
			logToFile("ERROR: Failed to compute file checksum - " + err.Error())
			return fmt.Errorf("failed to compute file checksum: %w", err)
		}
		fileHash = hash
		logToFile("INFO: Computed hash for file to be removed: " + op.Path + " - " + fileHash)
	} else if os.IsNotExist(err) {
		logToFile("WARNING: File does not exist, proceeding with database cleanup - " + op.Path)
	} else {
		logToFile("ERROR: Failed to check file existence - " + err.Error())
		return fmt.Errorf("failed to check file existence: %w", err)
	}

	// Step 2: Remove the file from the path
	if _, err := os.Stat(op.Path); err == nil {
		logToFile("INFO: Removing file " + op.Path)
		if err := os.Remove(op.Path); err != nil {
			logToFile("ERROR: Failed to remove file - " + err.Error())
			return fmt.Errorf("failed to remove file: %w", err)
		}
		logToFile("SUCCESS: File removed from path - " + op.Path)
	}

	// Step 3: Remove the hash from integrity database and update folder-specific JSON
	dbHash, err := removeFromIntegrityDatabase(op.Path)
	if err != nil {
		logToFile("ERROR: Failed to update integrity database - " + err.Error())
		return fmt.Errorf("failed to update integrity database: %w", err)
	}

	// Update folder-specific JSON file if database was modified
	if dbHash != "" {
		dir := filepath.Dir(op.Path)
		err = updateFolderFile(dir, dbHash)
		if err != nil {
			logToFile("ERROR: Failed to update folder file - " + err.Error())
			return fmt.Errorf("failed to update folder file: %w", err)
		}
	}

	logToFile("SUCCESS: File removal operation completed - " + op.Path)
	return nil
}

func removeFromIntegrityDatabase(filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	dbPath := filepath.Join(dir, ".db.json")

	key, err := extractKeyFromImage()
	if err != nil {
		return "", fmt.Errorf("failed to extract key: %w", err)
	}

	var entries []IntegrityEntry
	if _, err := os.Stat(dbPath); err == nil {
		encryptedData, err := os.ReadFile(dbPath)
		if err != nil {
			return "", fmt.Errorf("failed to read encrypted db file: %w", err)
		}

		decryptedData, err := decryptFile(key, encryptedData)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt db file: %w", err)
		}

		err = json.Unmarshal(decryptedData, &entries)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal db data: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check db file existence: %w", err)
	}

	// Remove the entry for the file
	updatedEntries := []IntegrityEntry{}
	found := false
	for _, entry := range entries {
		if entry.Path != filePath {
			updatedEntries = append(updatedEntries, entry)
		} else {
			found = true
		}
	}

	if !found && len(entries) > 0 {
		logToFile("WARNING: File hash not found in integrity database - " + filePath)
	}

	// Marshal updated data
	updatedJSON, err := json.MarshalIndent(updatedEntries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal updated db: %w", err)
	}

	// Encrypt and write back
	encryptedData, err := encryptFile(key, updatedJSON)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt updated db: %w", err)
	}

	err = os.WriteFile(dbPath, encryptedData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write encrypted db: %w", err)
	}

	// Calculate hash of encrypted .db.json
	dbHash, err := computeChecksum(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to compute db hash: %w", err)
	}

	logToFile("INFO: Integrity database updated - removed entry for " + filePath)
	return dbHash, nil
}

func executeCommand(op Operation) error {
	if op.Command == "" {
		logToFile("ERROR: Invalid command operation, missing command")
		return fmt.Errorf("invalid command operation, missing command")
	}

	logToFile("INFO: Executing command: " + op.Command)
	cmd := exec.Command("sh", "-c", op.Command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logToFile("ERROR: Command execution failed - " + err.Error())
		return fmt.Errorf("command execution failed: %w", err)
	}

	logToFile("SUCCESS: Command executed successfully")
	return nil
}

func executeScript(op Operation) error {
	if op.Script == "" {
		logToFile("ERROR: Invalid script operation, missing script content")
		return fmt.Errorf("invalid script operation, missing script content")
	}

	logToFile("INFO: Executing script")
	cmd := exec.Command("sh", "-c", op.Script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logToFile("ERROR: Script execution failed - " + err.Error())
		return fmt.Errorf("script execution failed: %w", err)
	}

	logToFile("SUCCESS: Script executed successfully")
	return nil
}

func updateIntegrityDatabase(filePath, hash string) (string, error) {
	dir := filepath.Dir(filePath)
	dbPath := filepath.Join(dir, ".db.json")

	key, err := extractKeyFromImage()
	if err != nil {
		return "", fmt.Errorf("failed to extract key: %w", err)
	}

	var entries []IntegrityEntry
	if _, err := os.Stat(dbPath); err == nil {
		encryptedData, err := os.ReadFile(dbPath)
		if err != nil {
			return "", fmt.Errorf("failed to read encrypted db file: %w", err)
		}

		decryptedData, err := decryptFile(key, encryptedData)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt db file: %w", err)
		}

		err = json.Unmarshal(decryptedData, &entries)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal db data: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check db file existence: %w", err)
	}

	// Check for existing entry by path and hash
	for i, entry := range entries {
		if entry.Path == filePath {
			if entry.Hash == hash {
				logToFile("INFO: File already exists with matching hash in database - " + filePath)
				// Return current .db.json hash without modification
				dbHash, err := computeChecksum(dbPath)
				if err != nil {
					return "", fmt.Errorf("failed to compute db hash: %w", err)
				}
				return dbHash, nil
			}
			// Update hash if path matches but hash differs
			entries[i].Hash = hash
			logToFile("INFO: Updated existing file hash in database - " + filePath)
			goto writeUpdate
		}
	}

	// Add new entry if no match found
	entries = append(entries, IntegrityEntry{
		Path: filePath,
		Hash: hash,
	})
	logToFile("INFO: Added new file entry to database - " + filePath)

writeUpdate:
	// Marshal updated data
	updatedJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal updated db: %w", err)
	}

	// Encrypt and write back
	encryptedData, err := encryptFile(key, updatedJSON)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt updated db: %w", err)
	}

	err = os.WriteFile(dbPath, encryptedData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write encrypted db: %w", err)
	}

	// Calculate hash of encrypted .db.json
	dbHash, err := computeChecksum(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to compute db hash: %w", err)
	}

	return dbHash, nil
}

func updateFolderFile(dir, dbHash string) error {
	// Extract folder name and construct the specific JSON filename
	folderName := filepath.Base(dir)
	folderFile := filepath.Join(dir, "."+folderName+".json") // e.g., .apps.json, .basic.json
	dbPath := filepath.Join(dir, ".db.json")                 // Path to .db.json

	key, err := extractKeyFromImage()
	if err != nil {
		return fmt.Errorf("failed to extract key: %w", err)
	}

	// Read and decrypt existing folder-specific JSON
	var folderData FolderEntry
	if _, err := os.Stat(folderFile); err == nil {
		encryptedData, err := os.ReadFile(folderFile)
		if err != nil {
			return fmt.Errorf("failed to read encrypted folder file: %w", err)
		}

		decryptedData, err := decryptFile(key, encryptedData)
		if err != nil {
			return fmt.Errorf("failed to decrypt folder file: %w", err)
		}

		err = json.Unmarshal(decryptedData, &folderData)
		if err != nil {
			return fmt.Errorf("failed to unmarshal folder data: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check folder file existence: %w", err)
	} else {
		// If file doesn't exist, initialize with the correct path
		folderData.Path = dbPath
	}

	// Update the hash value (path remains constant)
	folderData.Hash = dbHash

	// Marshal updated data
	updatedJSON, err := json.MarshalIndent(folderData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated folder data: %w", err)
	}

	// Encrypt and write back
	encryptedData, err := encryptFile(key, updatedJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt updated folder data: %w", err)
	}

	err = os.WriteFile(folderFile, encryptedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write encrypted folder file: %w", err)
	}

	logToFile("INFO: Folder database updated with db hash: " + dbHash)
	return nil
}

// Ensure these helper functions are present
func extractKeyFromImage() ([]byte, error) {
	tempKeyFile := "/tmp/extracted_key.txt"
	cmd := exec.Command("steghide", "extract", "-sf", "/sda1/data/.gems.jpeg", "-xf", tempKeyFile, "-p", "Sundyne@123")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("steghide extraction failed: %v", err)
	}
	defer os.Remove(tempKeyFile)
	key, err := os.ReadFile(tempKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted key: %v", err)
	}
	return key, nil
}

func decryptFile(key, encryptedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}
	return plaintext, nil
}

func encryptFile(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}
