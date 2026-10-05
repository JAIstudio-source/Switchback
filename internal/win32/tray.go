package win32

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

const (
	NIM_ADD        = 0x00000000
	NIM_MODIFY     = 0x00000001
	NIM_DELETE     = 0x00000002

	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	WM_USER_TRAY = 0x0400 + 101

	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800
	TPM_BOTTOMALIGN = 0x0020
	TPM_RIGHTBUTTON = 0x0002

	CMD_TRAY_OPEN    = 1001
	CMD_TRAY_MEDIA   = 1002
	CMD_TRAY_GAMING  = 1003
	CMD_TRAY_EXIT    = 1004
)

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

type POINT struct {
	X int32
	Y int32
}

type TrayController struct {
	mu         sync.Mutex
	hwnd       uintptr
	nid        NOTIFYICONDATAW
	port       int
	onExit     func()
	isAdded    bool
	stopChan   chan struct{}
}

var globalTray *TrayController
var trayOnce sync.Once
var trayWndProcCb uintptr

// StartTray initializes and runs the Windows System Tray icon in background.
func StartTray(port int, onExit func()) *TrayController {
	tc := &TrayController{
		port:     port,
		onExit:   onExit,
		stopChan: make(chan struct{}),
	}
	globalTray = tc

	go tc.run()
	return tc
}

func (tc *TrayController) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString(fmt.Sprintf("SwitchbackTrayWindow_%d", time.Now().UnixNano()))

	trayOnce.Do(func() {
		trayWndProcCb = syscall.NewCallback(trayWndProc)
	})

	hIcon, _, _ := procLoadIconW.Call(0, uintptr(32512)) // IDI_APPLICATION fallback

	wcex := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   trayWndProcCb,
		HInstance:     syscall.Handle(hInst),
		HIcon:         syscall.Handle(hIcon),
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0,
		0, 0, hInst, 0,
	)

	if hwnd == 0 {
		return
	}

	tc.mu.Lock()
	tc.hwnd = hwnd
	tc.nid = NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(NOTIFYICONDATAW{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		UCallbackMessage: WM_USER_TRAY,
		HIcon:            hIcon,
	}
	copy(tc.nid.SzTip[:], syscall.StringToUTF16("SwitchBack // Active"))
	tc.mu.Unlock()

	// Retry adding tray icon in case Windows Explorer is initializing
	for i := 0; i < 3; i++ {
		ret, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&tc.nid)))
		if ret != 0 {
			tc.mu.Lock()
			tc.isAdded = true
			tc.mu.Unlock()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Message loop for tray window
	var msg struct {
		HWnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      POINT
	}

	for {
		select {
		case <-tc.stopChan:
			tc.cleanup()
			return
		default:
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(ret) <= 0 {
				tc.cleanup()
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if globalTray == nil {
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}

	switch msg {
	case WM_USER_TRAY:
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			// Open browser dashboard
			url := fmt.Sprintf("http://127.0.0.1:%d", globalTray.port)
			_ = OpenURL(url)
			return 0

		case WM_RBUTTONUP:
			// Context menu
			var pt POINT
			procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

			hMenu, _, _ := procCreatePopupMenu.Call()
			if hMenu != 0 {
				strOpen, _ := syscall.UTF16PtrFromString("⚡ Open Dashboard")
				strMedia, _ := syscall.UTF16PtrFromString("⏯️ Toggle PC Media")
				strExit, _ := syscall.UTF16PtrFromString("❌ Exit SwitchBack")

				procAppendMenuW.Call(hMenu, MF_STRING, CMD_TRAY_OPEN, uintptr(unsafe.Pointer(strOpen)))
				procAppendMenuW.Call(hMenu, MF_STRING, CMD_TRAY_MEDIA, uintptr(unsafe.Pointer(strMedia)))
				procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
				procAppendMenuW.Call(hMenu, MF_STRING, CMD_TRAY_EXIT, uintptr(unsafe.Pointer(strExit)))

				procSetForegroundWindow.Call(hwnd)
				procTrackPopupMenu.Call(hMenu, TPM_RIGHTBUTTON|TPM_BOTTOMALIGN, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
				procPostMessageW.Call(hwnd, 0, 0, 0) // WM_NULL
				procDestroyMenu.Call(hMenu)
			}
			return 0
		}

	case 0x0111: // WM_COMMAND
		cmdID := int(wParam & 0xFFFF)
		switch cmdID {
		case CMD_TRAY_OPEN:
			url := fmt.Sprintf("http://127.0.0.1:%d", globalTray.port)
			_ = OpenURL(url)
		case CMD_TRAY_MEDIA:
			ToggleMediaPlayback(0)
		case CMD_TRAY_EXIT:
			if globalTray.onExit != nil {
				go globalTray.onExit()
			}
		}
		return 0

	case 0x0002: // WM_DESTROY
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func (tc *TrayController) cleanup() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.isAdded {
		procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&tc.nid)))
		tc.isAdded = false
	}
	if tc.hwnd != 0 {
		procDestroyWindow.Call(tc.hwnd)
		tc.hwnd = 0
	}
}

// StopTray unregisters the tray icon and terminates the message loop.
func StopTray() {
	if globalTray != nil {
		close(globalTray.stopChan)
		globalTray = nil
	}
}

// UpdateTrayStatus updates the tooltip in the notification area.
func UpdateTrayStatus(status string) {
	if globalTray == nil {
		return
	}
	globalTray.mu.Lock()
	defer globalTray.mu.Unlock()

	tip := fmt.Sprintf("SwitchBack // %s", status)
	var tipUTF16 [128]uint16
	copy(tipUTF16[:], syscall.StringToUTF16(tip))
	globalTray.nid.SzTip = tipUTF16

	if globalTray.isAdded {
		procShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&globalTray.nid)))
	}
}
