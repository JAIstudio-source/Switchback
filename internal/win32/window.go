package win32

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	shell32 = syscall.NewLazyDLL("shell32.dll")

	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procShowWindow               = user32.NewProc("ShowWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procAllowSetForegroundWindow = user32.NewProc("AllowSetForegroundWindow")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procSwitchToThisWindow       = user32.NewProc("SwitchToThisWindow")
	procOpenInputDesktop         = user32.NewProc("OpenInputDesktop")
	procSetThreadDesktop         = user32.NewProc("SetThreadDesktop")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetWindowLongW           = user32.NewProc("GetWindowLongW")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
	procMonitorFromWindow        = user32.NewProc("MonitorFromWindow")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procMapVirtualKeyW           = user32.NewProc("MapVirtualKeyW")

	procSHQueryUserNotificationState = shell32.NewProc("SHQueryUserNotificationState")
	procShellExecuteW                = shell32.NewProc("ShellExecuteW")

	procSendInput = user32.NewProc("SendInput")

	procGetCurrentProcessId      = kernel32.NewProc("GetCurrentProcessId")
	procGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
	procGetShortPathNameW        = kernel32.NewProc("GetShortPathNameW")
	procGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	procGlobalFree               = kernel32.NewProc("GlobalFree")
	procGlobalLock               = kernel32.NewProc("GlobalLock")
	procGlobalUnlock             = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory            = kernel32.NewProc("RtlMoveMemory")

	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procSetProcessDPIAware       = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
)

const (
	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3) // -4
)

// GetShortPath converts a path with spaces to an 8.3 short path without spaces.
func GetShortPath(path string) string {
	pUTF16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buf := make([]uint16, 1024)
	ret, _, _ := procGetShortPathNameW.Call(
		uintptr(unsafe.Pointer(pUTF16)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if ret == 0 || ret > uintptr(len(buf)) {
		return path
	}
	return syscall.UTF16ToString(buf[:ret])
}

var (
	lastEnsureDesktop time.Time
	ensureDesktopMu   sync.Mutex
	dpiAwareOnce      sync.Once
)

// EnsureDesktop attaches the current thread to the interactive desktop if needed.
// Also ensures High-DPI Per-Monitor V2 awareness is active for accurate window metrics.
func EnsureDesktop() {
	dpiAwareOnce.Do(func() {
		if procSetProcessDpiAwarenessContext.Find() == nil {
			procSetProcessDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
		} else if procSetProcessDPIAware.Find() == nil {
			procSetProcessDPIAware.Call()
		}
	})

	ensureDesktopMu.Lock()
	defer ensureDesktopMu.Unlock()

	if time.Since(lastEnsureDesktop) < 2*time.Second {
		return
	}
	lastEnsureDesktop = time.Now()

	desk, _, _ := procOpenInputDesktop.Call(0, 0, 0x0081)
	if desk != 0 {
		procSetThreadDesktop.Call(desk)
	}
}

const (
	SW_RESTORE                  = 9
	SW_SHOW                     = 5
	VK_MENU                     = 0x12 // Alt key
	VK_MEDIA_PLAY_PAUSE         = 0xB3 // Multimedia Play/Pause key
	VK_K                        = 0x4B // 'k' key (YouTube universal play/pause)
	WM_KEYDOWN                  = 0x0100
	WM_KEYUP                    = 0x0101
	WM_APPCOMMAND               = 0x0319
	APPCOMMAND_MEDIA_PLAY_PAUSE = 14
	KEYEVENTF_KEYUP             = 0x0002
	KEYEVENTF_EXTENDEDKEY       = 0x0001
	ASFW_ANY                    = ^uintptr(0) // -1
	TH32CS_SNAPPROCESS          = 0x00000002
	MONITOR_DEFAULTTONEAREST    = 2
	WS_CAPTION                  = 0x00C00000
	VK_SHIFT                    = 0x10
	VK_CONTROL                  = 0x11
	VK_ESCAPE                   = 0x1B
	VK_BACK                     = 0x08
	VK_A                        = 0x41
	VK_INSERT                   = 0x2D
	VK_RETURN                   = 0x0D
	VK_TAB                      = 0x09
	VK_SPACE                    = 0x20
	VK_DOWN                     = 0x28
	VK_V                        = 0x56
	VK_L                        = 0x4C
	VK_I                        = 0x49
	VK_B                        = 0x42
	VK_Y                        = 0x59
	VK_N                        = 0x4E
	CF_UNICODETEXT              = 13
	GMEM_MOVEABLE               = 0x0002
)

const (
	INPUT_KEYBOARD = 1
)

type keyboardInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type input64 struct {
	inputType uint32
	_         uint32
	ki        keyboardInput
	_         [8]byte
}

func sendInputKeyboardBatch(inputs []input64) error {
	if len(inputs) == 0 {
		return nil
	}
	EnsureDesktop()
	size := int32(unsafe.Sizeof(inputs[0]))
	ret, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(size),
	)
	if ret != uintptr(len(inputs)) {
		return fmt.Errorf("SendInput sent %d/%d: %v", ret, len(inputs), err)
	}
	return nil
}

// SendKey sends a key down and key up event atomically via Win32 SendInput.
func SendKey(vk uintptr) {
	inputs := []input64{
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(vk)}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(vk), dwFlags: KEYEVENTF_KEYUP}},
	}
	_ = sendInputKeyboardBatch(inputs)
}

// SendKeyCombo sends a modifier + key combination atomically via Win32 SendInput.
func SendKeyCombo(mod uintptr, vk uintptr) {
	inputs := []input64{
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(mod)}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(vk)}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(vk), dwFlags: KEYEVENTF_KEYUP}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(mod), dwFlags: KEYEVENTF_KEYUP}},
	}
	_ = sendInputKeyboardBatch(inputs)
}

// SendKeyCombo3 sends two modifiers + key (e.g. Ctrl+Alt+B) atomically via Win32 SendInput.
func SendKeyCombo3(mod1, mod2, vk uintptr) {
	inputs := []input64{
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(mod1)}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(mod2)}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(vk)}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(vk), dwFlags: KEYEVENTF_KEYUP}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(mod2), dwFlags: KEYEVENTF_KEYUP}},
		{inputType: INPUT_KEYBOARD, ki: keyboardInput{wVk: uint16(mod1), dwFlags: KEYEVENTF_KEYUP}},
	}
	_ = sendInputKeyboardBatch(inputs)
}

type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

// HWND represents a Win32 window handle.
type HWND uintptr

// IsFullscreenActive checks whether the user is watching true fullscreen video or playing a fullscreen game.
func IsFullscreenActive() bool {
	EnsureDesktop()

	fg := GetForegroundWindow()
	if fg == 0 || !IsWindowValid(fg) {
		return false
	}

	title := strings.ToLower(GetWindowTitle(fg))
	// Never treat switchback dashboard, desktop, or taskbar as fullscreen game/video
	if strings.Contains(title, "switchback //") || strings.Contains(title, "focusmgr //") || strings.Contains(title, "program manager") {
		return false
	}

	// 1. Check Shell User Notification State
	// Only state 3 (QUNS_RUNNING_D3D_FULL_SCREEN) represents an exclusive fullscreen DirectX/D3D game.
	var state uint32
	ret, _, _ := procSHQueryUserNotificationState.Call(uintptr(unsafe.Pointer(&state)))
	if ret == 0 && state == 3 {
		return true
	}

	// 2. Direct bounds check on the foreground window against its monitor bounds
	var wRect RECT
	ret, _, _ = procGetWindowRect.Call(uintptr(fg), uintptr(unsafe.Pointer(&wRect)))
	if ret == 0 {
		return false
	}

	hMon, _, _ := procMonitorFromWindow.Call(uintptr(fg), MONITOR_DEFAULTTONEAREST)
	if hMon == 0 {
		return false
	}

	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ = procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return false
	}

	// If the window covers the entire monitor rect (including covering where the taskbar would be)
	coversMonitor := (wRect.Left <= mi.RcMonitor.Left &&
		wRect.Top <= mi.RcMonitor.Top &&
		wRect.Right >= mi.RcMonitor.Right &&
		wRect.Bottom >= mi.RcMonitor.Bottom)

	if !coversMonitor {
		return false
	}

	// If the monitor has a taskbar and the window covers it (wRect.Bottom >= mi.RcMonitor.Bottom > mi.RcWork.Bottom)
	if mi.RcWork.Bottom < mi.RcMonitor.Bottom {
		// If the window does NOT cover past the work area, it's just a regular maximized window!
		if wRect.Bottom <= mi.RcWork.Bottom+4 {
			return false
		}
	}

	// Check window style for WS_MAXIMIZE, WS_CAPTION, and WS_THICKFRAME
	var style uint32
	styleRet, _, _ := procGetWindowLongW.Call(uintptr(fg), uintptr(uint32(0xFFFFFFF0))) // GWL_STYLE (-16)
	style = uint32(styleRet)

	const WS_MAXIMIZE = 0x01000000
	const WS_THICKFRAME = 0x00040000

	// If the window is a standard maximized window with caption or resize border, it's NOT a fullscreen video or exclusive game!
	// (Common on laptops with auto-hide taskbar where RcWork == RcMonitor)
	if (style&WS_MAXIMIZE) != 0 && ((style&WS_CAPTION) == WS_CAPTION || (style&WS_THICKFRAME) != 0) {
		return false
	}

	// Taskbar position edge checks for Windows 10/11:
	// Bottom taskbar
	if mi.RcWork.Bottom < mi.RcMonitor.Bottom && wRect.Bottom <= mi.RcWork.Bottom+8 {
		return false
	}
	// Top taskbar
	if mi.RcWork.Top > mi.RcMonitor.Top && wRect.Top >= mi.RcWork.Top-8 {
		return false
	}
	// Left taskbar
	if mi.RcWork.Left > mi.RcMonitor.Left && wRect.Left >= mi.RcWork.Left-8 {
		return false
	}
	// Right taskbar
	if mi.RcWork.Right < mi.RcMonitor.Right && wRect.Right <= mi.RcWork.Right+8 {
		return false
	}

	// Standard title bar check
	if (style & WS_CAPTION) == WS_CAPTION {
		return false
	}

	return true
}

// IsMeetingActive detects whether an active Zoom, Discord call, Microsoft Teams, or Google Meet call window is active.
func IsMeetingActive() bool {
	EnsureDesktop()
	meetingPatterns := []string{
		"zoom meeting", "zoom webinar",
		"discord voice", "discord stream",
		"microsoft teams meeting", "teams call",
		"slack huddle", "webex meeting",
	}
	h := FindWindowByTitlePattern(meetingPatterns)
	return h != 0 && IsWindowValid(h)
}

// SendMediaPlayPause broadcasts a hardware media Play/Pause keypress (VK_MEDIA_PLAY_PAUSE 0xB3)
// with the KEYEVENTF_EXTENDEDKEY flag required by Windows 10/11 SMTC and modern browsers.
func SendMediaPlayPause() {
	EnsureDesktop()
	scanCode, _, _ := procMapVirtualKeyW.Call(VK_MEDIA_PLAY_PAUSE, 0)
	procKeybdEvent.Call(VK_MEDIA_PLAY_PAUSE, scanCode, KEYEVENTF_EXTENDEDKEY, 0)
	time.Sleep(35 * time.Millisecond)
	procKeybdEvent.Call(VK_MEDIA_PLAY_PAUSE, scanCode, KEYEVENTF_EXTENDEDKEY|KEYEVENTF_KEYUP, 0)
}

// ToggleMediaPlayback pulses the playback state for the target window or system media:
// If target window accepts WM_APPCOMMAND, we send it directly; otherwise we broadcast global hardware key.
func ToggleMediaPlayback(targetHWND HWND) {
	EnsureDesktop()

	if targetHWND != 0 && IsWindowValid(targetHWND) {
		ret, _, _ := procSendMessageW.Call(
			uintptr(targetHWND),
			WM_APPCOMMAND,
			uintptr(targetHWND),
			uintptr(APPCOMMAND_MEDIA_PLAY_PAUSE<<16),
		)
		if ret != 0 {
			return
		}
	}

	SendMediaPlayPause()
}

// GetForegroundWindow returns the current foreground window handle.
func GetForegroundWindow() HWND {
	EnsureDesktop()
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

// OpenURL opens a URL in the user's default browser via Windows ShellExecuteW.
func OpenURL(targetURL string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	pURL, err := syscall.UTF16PtrFromString(targetURL)
	if err != nil {
		return err
	}
	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(pURL)),
		0,
		0,
		1, // SW_SHOWNORMAL
	)
	if ret <= 32 {
		return exec.Command("cmd.exe", "/c", "start", "", targetURL).Start()
	}
	return nil
}

// SetClipboardText sets Unicode text on the Windows clipboard natively using Win32 API.
func SetClipboardText(text string) error {
	EnsureDesktop()
	u16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	size := len(u16) * 2

	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("GlobalLock failed")
	}

	procRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&u16[0])), uintptr(size))
	procGlobalUnlock.Call(hMem)

	var opened bool
	for i := 0; i < 10; i++ {
		ret, _, _ := procOpenClipboard.Call(0)
		if ret != 0 {
			opened = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !opened {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	ret, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, hMem)
	if ret == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("SetClipboardData failed")
	}
	return nil
}

// InjectTextToAgent copies text to clipboard, focuses agent AI chat input, pastes it, and sends Enter.
func InjectTextToAgent(agentHWND HWND, text string, mobilePrimary bool) error {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return nil
	}

	EnsureDesktop()

	if agentHWND == 0 || !IsWindowValid(agentHWND) {
		return fmt.Errorf("invalid agent HWND %d", agentHWND)
	}

	origFG := GetForegroundWindow()
	title := strings.ToLower(GetWindowTitle(agentHWND))

	// 1. Ensure window is restored if minimized
	procShowWindow.Call(uintptr(agentHWND), uintptr(SW_RESTORE))
	time.Sleep(100 * time.Millisecond)

	// 2. Bring window to top & foreground with thread attachment bypass
	_ = SetForegroundWindowWithBypass(agentHWND)
	time.Sleep(250 * time.Millisecond)

	// 3. Put text onto Windows clipboard natively
	if err := SetClipboardText(clean); err != nil {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Set-Clipboard", "-Value", clean)
		_ = cmd.Run()
	}
	time.Sleep(80 * time.Millisecond)

	isTerminal := strings.Contains(title, "cmd") ||
		strings.Contains(title, "powershell") ||
		strings.Contains(title, "terminal") ||
		strings.Contains(title, "bash") ||
		strings.Contains(title, "conhost") ||
		strings.Contains(title, "mintty")

	if isTerminal {
		// Terminal: Paste prompt text via Ctrl+V
		SendKeyCombo(VK_CONTROL, VK_V)
		time.Sleep(100 * time.Millisecond)
		SendKey(VK_RETURN)
	} else {
		// IDE (Antigravity, Cursor, VS Code, Windsurf):
		// 1. Focus Chat / Agent input: Ctrl+Alt+I (official VS Code / Antigravity binding)
		SendKeyCombo3(VK_CONTROL, VK_MENU, VK_I)
		time.Sleep(200 * time.Millisecond)
		SendKeyCombo(VK_CONTROL, VK_L)
		time.Sleep(300 * time.Millisecond)

		// 2. Paste prompt text: Ctrl+V
		SendKeyCombo(VK_CONTROL, VK_V)
		time.Sleep(200 * time.Millisecond)

		// 3. Submit prompt: send Enter
		SendKey(VK_RETURN)
	}

	// 4. Window focus handling:
	// If Mobile Primary is active, keep the agent window in view so user sees it working!
	// Only switch back if mobile primary is OFF, and user was on another window (e.g. VLC)
	if !mobilePrimary && origFG != 0 && origFG != agentHWND && IsWindowValid(origFG) {
		time.Sleep(600 * time.Millisecond)
		_ = SetForegroundWindowWithBypass(origFG)
	}

	return nil
}

// InjectApprovalToAgent submits approval/answers to prompts waiting in the agent window.
func InjectApprovalToAgent(agentHWND HWND, decision string, answer string, index int, mobilePrimary bool) error {
	if agentHWND == 0 || !IsWindowValid(agentHWND) {
		return nil
	}
	EnsureDesktop()
	origFG := GetForegroundWindow()
	title := strings.ToLower(GetWindowTitle(agentHWND))

	// Only send raw terminal keystrokes (number keys, Y/N) to actual terminals or CLI sessions.
	// In GUI IDEs (VS Code, JetBrains, Cursor, Windsurf, Antigravity), raw numbers/letters
	// would type directly into active code editor files.
	isTerminal := strings.Contains(title, "cmd") ||
		strings.Contains(title, "powershell") ||
		strings.Contains(title, "pwsh") ||
		strings.Contains(title, "terminal") ||
		strings.Contains(title, "bash") ||
		strings.Contains(title, "conhost") ||
		strings.Contains(title, "mintty") ||
		strings.Contains(title, "alacritty") ||
		strings.Contains(title, "wezterm")

	if !isTerminal {
		// For GUI IDEs, do not inject raw numbers into code editor buffers
		return nil
	}

	_ = SetForegroundWindowWithBypass(agentHWND)
	time.Sleep(200 * time.Millisecond)

	lowerDecision := strings.ToLower(strings.TrimSpace(decision))

	if index >= 1 && index <= 9 {
		// Send number key (0x30 + index) and Enter
		SendKey(uintptr(0x30 + index))
		time.Sleep(60 * time.Millisecond)
		SendKey(VK_RETURN)
	} else if lowerDecision == "allow" || lowerDecision == "yes" {
		trimmedAnswer := strings.TrimSpace(answer)
		if trimmedAnswer != "" && trimmedAnswer != "Allowed by user" && trimmedAnswer != "Allow Action" && trimmedAnswer != "Yes, proceed with action" {
			_ = SetClipboardText(trimmedAnswer)
			SendKeyCombo(VK_CONTROL, VK_V)
			time.Sleep(100 * time.Millisecond)
		} else {
			SendKey(VK_Y)
			time.Sleep(60 * time.Millisecond)
		}
		SendKey(VK_RETURN)
	} else {
		// Deny: send "n" + Enter + Escape
		SendKey(VK_N)
		time.Sleep(60 * time.Millisecond)
		SendKey(VK_RETURN)
		time.Sleep(60 * time.Millisecond)
		SendKey(VK_ESCAPE)
	}

	if !mobilePrimary && origFG != 0 && origFG != agentHWND && IsWindowValid(origFG) {
		time.Sleep(400 * time.Millisecond)
		_ = SetForegroundWindowWithBypass(origFG)
	}
	return nil
}
