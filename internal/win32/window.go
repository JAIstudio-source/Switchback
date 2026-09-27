package win32

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procIsWindow                   = user32.NewProc("IsWindow")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procIsIconic                   = user32.NewProc("IsIconic")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procBringWindowToTop           = user32.NewProc("BringWindowToTop")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput          = user32.NewProc("AttachThreadInput")
	procSystemParametersInfoW      = user32.NewProc("SystemParametersInfoW")
	procKeybdEvent                 = user32.NewProc("keybd_event")
	procAllowSetForegroundWindow   = user32.NewProc("AllowSetForegroundWindow")
	procEnumWindows                = user32.NewProc("EnumWindows")

	procGetCurrentProcessId        = kernel32.NewProc("GetCurrentProcessId")
	procGetCurrentThreadId         = kernel32.NewProc("GetCurrentThreadId")
	procCreateToolhelp32Snapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = kernel32.NewProc("Process32FirstW")
	procProcess32NextW             = kernel32.NewProc("Process32NextW")
)

const (
	SW_RESTORE                   = 9
	SW_SHOW                      = 5
	SPI_GETFOREGROUNDLOCKTIMEOUT = 0x2000
	SPI_SETFOREGROUNDLOCKTIMEOUT = 0x2001
	SPIF_SENDCHANGE              = 0x0002
	VK_MENU                      = 0x12 // Alt key
	KEYEVENTF_KEYUP              = 0x0002
	KEYEVENTF_EXTENDEDKEY        = 0x0001
	ASFW_ANY                     = ^uintptr(0) // -1
	TH32CS_SNAPPROCESS           = 0x00000002
)

// HWND represents a Win32 window handle.
type HWND uintptr

// GetForegroundWindow returns the current foreground window handle.
func GetForegroundWindow() HWND {
	ret, _, _ := procGetForegroundWindow.Call()
	return HWND(ret)
}

// IsWindowValid checks whether an HWND exists and is an active window.
func IsWindowValid(hwnd HWND) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindow.Call(uintptr(hwnd))
	return ret != 0
}

// IsWindowVisible checks whether the window is visible.
func IsWindowVisible(hwnd HWND) bool {
	if !IsWindowValid(hwnd) {
		return false
	}
	ret, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return ret != 0
}

// GetWindowTitle returns the window's caption / title.
func GetWindowTitle(hwnd HWND) string {
	if !IsWindowValid(hwnd) {
		return ""
	}
	length, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return syscall.UTF16ToString(buf)
}

// GetWindowThreadAndPID retrieves the Thread ID and Process ID for an HWND.
func GetWindowThreadAndPID(hwnd HWND) (uint32, uint32) {
	var pid uint32
	tid, _, _ := procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))
	return uint32(tid), pid
}
