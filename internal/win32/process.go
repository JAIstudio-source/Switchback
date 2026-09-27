package win32

import (
	"strings"
	"syscall"
	"unsafe"
)

type ProcessEntry32W struct {
	Size              uint32
	Usage             uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	Threads           uint32
	ParentProcessID   uint32
	PriClassBase      int32
	Flags             uint32
	ExeFile           [260]uint16
}

// GetProcessAncestors returns a map of ancestor PID -> true, up to maxDepth.
func GetProcessAncestors(startPID uint32, maxDepth int) map[uint32]bool {
	ancestors := make(map[uint32]bool)
	if startPID == 0 {
		startPID = GetCurrentPID()
	}

	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == ^uintptr(0) || snapshot == 0 {
		return ancestors
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	// Build child -> parent mapping
	parentMap := make(map[uint32]uint32)
	var entry ProcessEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		parentMap[entry.ProcessID] = entry.ParentProcessID
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}

	curr := startPID
	for depth := 0; depth < maxDepth; depth++ {
		parent, exists := parentMap[curr]
		if !exists || parent == 0 || parent == curr {
			break
		}
		ancestors[parent] = true
		curr = parent
	}

	return ancestors
}

// GetCurrentPID returns the current process ID.
func GetCurrentPID() uint32 {
	pid, _, _ := procGetCurrentProcessId.Call()
	return uint32(pid)
}

// FindAncestorWindow attempts to find the primary visible top-level window
// belonging to any ancestor of the current process (e.g. Terminal, IDE).
func FindAncestorWindow() HWND {
	ancestors := GetProcessAncestors(0, 10)
	var found HWND

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		h := HWND(hwnd)
		if !IsWindowVisible(h) {
			return 1 // continue enumeration
		}
		_, pid := GetWindowThreadAndPID(h)
		if ancestors[pid] {
			title := GetWindowTitle(h)
			// Filter out empty or tooltip/hidden helper windows
			if strings.TrimSpace(title) != "" {
				found = h
				return 0 // stop enumeration
			}
		}
		return 1
	})

	procEnumWindows.Call(cb, 0)
	return found
}

// FindWindowByTitlePattern searches visible top-level windows for one matching any pattern.
func FindWindowByTitlePattern(patterns []string) HWND {
	if len(patterns) == 0 {
		return 0
	}
	var found HWND

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		h := HWND(hwnd)
		if !IsWindowVisible(h) {
			return 1
		}
		title := strings.ToLower(GetWindowTitle(h))
		if title == "" {
			return 1
		}
		for _, pat := range patterns {
			if strings.Contains(title, strings.ToLower(pat)) {
				found = h
				return 0 // found
			}
		}
		return 1
	})

	procEnumWindows.Call(cb, 0)
	return found
}
