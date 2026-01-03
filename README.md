# MALUSB — USB File Exfiltration Tool

> ⚠️ **EDUCATIONAL PURPOSE ONLY** — For authorized security testing in controlled environments. 

## Overview

MALUSB is a proof-of-concept USB data exfiltration tool written in Go.  It silently monitors for USB drive insertions and automatically copies target files (`.pdf`, `.docx`) to a local directory.

**Use Case:** Red team engagements, malware analysis training, and understanding endpoint security threats.

---

## Features

| Feature | Description |
|---------|-------------|
| **Stealth Mode** | Runs without visible console window |
| **Auto-Detection** | Monitors for USB insertion every 5 seconds |
| **Selective Exfil** | Only copies `.pdf` and `.docx` files |
| **Size Limit** | Skips files larger than 100 MB |
| **Structure Preservation** | Maintains original folder hierarchy |
| **Multi-Drive Support** | Monitors multiple USBs simultaneously |
| **Timeout Protection** | 2-minute timeout per file prevents hanging |

---

## Requirements

- **OS:** Windows 10/11
- **Go:** Version 1.19 or higher
- **Permissions:** Administrator recommended (for certain directories)

---

## Installation

### 1. Install Go

Download from: https://go.dev/dl/

Verify installation:
```bash
go version
```

### 2. Clone/Download the Project

```bash
git clone <repository-url>
cd malusb
```

### 3. Build the Executable

**Visible Console (for testing/debugging):**
```bash
go build -o malusb. exe malusb. go
```

**Stealth Mode (no console window):**
```bash
go build -ldflags="-w -s -H windowsgui" -o malusb.exe malusb.go
```

| Flag | Purpose |
|------|---------|
| `-w` | Omit DWARF debug info |
| `-s` | Omit symbol table |
| `-H windowsgui` | No console window |

---

## Usage

### Running the Tool

```bash
# Run directly
.\malusb. exe

# Or double-click malusb.exe
```

### What Happens

1. Tool starts monitoring for USB drives
2. When USB is inserted, monitoring begins automatically
3. All `.pdf` and `.docx` files are copied to `C:\USB_FETCHED\`
4. Files are organized by drive letter (e.g., `C:\USB_FETCHED\E\`)

### Stopping the Tool

1. Open **Task Manager** (`Ctrl + Shift + Esc`)
2. Go to **Details** tab
3. Find `malusb.exe`
4. Click **End Task**

---

## Configuration

Edit these constants in `malusb.go` to customize behavior:

```go
const (
    ScanInterval     = 5 * time.Second    // USB detection frequency
    DumpDir          = "C:\\USB_FETCHED\\" // Output directory
    FileScanInterval = 10 * time.Second   // File scan frequency
    CopyTimeout      = 120 * time.Second  // Max time per file copy
    MaxFileSizeMB    = 100                // Skip files larger than this
)

// Target file extensions
var targetExtensions = map[string]bool{
    ".pdf":   true,
    ".docx":  true,
    // Add more:  ". xlsx":  true, ".pptx": true,
}
```

---

## Project Structure

```
malusb/
├── malusb. go       # Main source code
├── README.md       # This file
└── C:\USB_FETCHED\ # Created at runtime (exfiltrated files)
    ├── E\          # Files from drive E:
    │   ├── Documents/
    │   │   └── report.pdf
    │   └── contract. docx
    └── F\          # Files from drive F: 
        └── notes.pdf
```

---

## Technical Details

### Windows API Used

| Function | Library | Purpose |
|----------|---------|---------|
| `GetDriveTypeW` | kernel32.dll | Identify removable drives |

### Go Concurrency Model

- **Goroutines:** Lightweight threads for concurrent USB monitoring
- **Channels:** Communication between goroutines (timeout handling)
- **Select:** Multiplexing channel operations

### Detection Vectors

| Indicator | Detection Method |
|-----------|------------------|
| `C:\USB_FETCHED\` directory | File system monitoring |
| High disk I/O on USB insert | EDR behavioral analysis |
| `kernel32.dll` API calls | API hooking |
| No-window process | Process attribute inspection |

---

## Defense & Mitigation

Understanding this tool helps defenders implement:

1. **USB Device Control** — Whitelist approved devices
2. **DLP Solutions** — Monitor file copy operations
3. **Endpoint Detection** — Alert on suspicious process behavior
4. **File Integrity Monitoring** — Detect unauthorized file access

---

## Legal Disclaimer

This tool is provided for **educational and authorized testing purposes only**. 

- ✅ Use in your own lab environment
- ✅ Use with explicit written permission
- ✅ Use for security research and learning
- ❌ Do NOT use on systems you don't own
- ❌ Do NOT use for unauthorized data theft
- ❌ Do NOT deploy in production environments

**The authors are not responsible for misuse of this tool.**

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes with clear comments
4. Submit a pull request

---

## License

MIT License — See LICENSE file for details. 

---

## Author

Built for educational purposes in cybersecurity training environments.
