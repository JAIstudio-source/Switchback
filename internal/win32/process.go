package win32

import (
	"strings"
	"syscall"
	"unsafe"
)

type ProcessEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type WindowInfo struct {
	HWND        uintptr `json:"hwnd"`
	Title       string  `json:"title"`
	ProcessName string  `json:"process_name"`
	PID         uint32  `json:"pid"`
	IsAgent     bool    `json:"is_agent"`
	AgentType   string  `json:"agent_type,omitempty"`
}

// GetProcessNameMap returns a map of PID -> process executable name.
func GetProcessNameMap() map[uint32]string {
	names := make(map[uint32]string)
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snapshot == ^uintptr(0) || snapshot == 0 {
		return names
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var entry ProcessEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		names[entry.ProcessID] = syscall.UTF16ToString(entry.ExeFile[:])
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return names
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
			return 1
		}
		_, pid := GetWindowThreadAndPID(h)
		if ancestors[pid] {
			title := GetWindowTitle(h)
			if strings.TrimSpace(title) != "" {
				found = h
				return 0
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
				return 0
			}
		}
		return 1
	})

	procEnumWindows.Call(cb, 0)
	return found
}

// GetOpenWindows returns all active, visible top-level application windows.
func GetOpenWindows() []WindowInfo {
	procNames := GetProcessNameMap()
	var windowsList []WindowInfo

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		h := HWND(hwnd)
		if !IsWindowVisible(h) {
			return 1
		}

		title := strings.TrimSpace(GetWindowTitle(h))
		if title == "" {
			return 1
		}

		// Skip Windows internal helper windows
		if title == "Program Manager" || title == "Windows Input Experience" || title == "Settings" {
			return 1
		}

		_, pid := GetWindowThreadAndPID(h)
		pname := procNames[pid]

		// Check if it's an AI agent
		isAgent := false
		agentType := ""
		titleLower := strings.ToLower(title)
		pnameLower := strings.ToLower(pname)

		if strings.Contains(titleLower, "antigravity") || strings.Contains(pnameLower, "antigravity") {
			isAgent = true
			agentType = "antigravity"
		} else if strings.Contains(titleLower, "claude") || strings.Contains(pnameLower, "claude") {
			isAgent = true
			agentType = "claude-code"
		} else if strings.Contains(titleLower, "codex") || strings.Contains(pnameLower, "codex") {
			isAgent = true
			agentType = "codex"
		} else if strings.Contains(titleLower, "visual studio code") || strings.Contains(pnameLower, "code.exe") || strings.Contains(titleLower, "cursor") {
			isAgent = true
			agentType = "ide-agent"
		}

		windowsList = append(windowsList, WindowInfo{
			HWND:        uintptr(h),
			Title:       title,
			ProcessName: pname,
			PID:         pid,
			IsAgent:     isAgent,
			AgentType:   agentType,
		})

		return 1
	})

	procEnumWindows.Call(cb, 0)
	return windowsList
}
