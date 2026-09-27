package win32

import (
	"fmt"
	"time"
	"unsafe"
)

// SetForegroundWindowWithBypass aggressively brings a target window to foreground,
// bypassing Windows 10/11 focus-stealing lock restrictions.
func SetForegroundWindowWithBypass(target HWND) error {
	if !IsWindowValid(target) {
		return fmt.Errorf("target window %d is not valid", target)
	}

	currentForeground := GetForegroundWindow()
	if currentForeground == target {
		// Already in foreground
		return nil
	}

	// 1. Grant permission via AllowSetForegroundWindow
	procAllowSetForegroundWindow.Call(ASFW_ANY)

	targetTID, _ := GetWindowThreadAndPID(target)
	currentForegroundTID, _ := GetWindowThreadAndPID(currentForeground)
	myTID, _, _ := procGetCurrentThreadId.Call()

	// 2. Attach thread inputs if threads differ
	attachedTarget := false
	attachedForeground := false

	if targetTID != 0 && uintptr(targetTID) != myTID {
		ret, _, _ := procAttachThreadInput.Call(myTID, uintptr(targetTID), 1)
		attachedTarget = (ret != 0)
	}

	if currentForegroundTID != 0 && uintptr(currentForegroundTID) != myTID {
		ret, _, _ := procAttachThreadInput.Call(myTID, uintptr(currentForegroundTID), 1)
		attachedForeground = (ret != 0)
	}

	defer func() {
		// Clean up thread attachments
		if attachedTarget {
			procAttachThreadInput.Call(myTID, uintptr(targetTID), 0)
		}
		if attachedForeground {
			procAttachThreadInput.Call(myTID, uintptr(currentForegroundTID), 0)
		}
	}()

	// 3. Temporarily bypass foreground lock timeout
	var oldTimeout uint32
	procSystemParametersInfoW.Call(
		SPI_GETFOREGROUNDLOCKTIMEOUT,
		0,
		uintptr(unsafe.Pointer(&oldTimeout)),
		0,
	)
	procSystemParametersInfoW.Call(
		SPI_SETFOREGROUNDLOCKTIMEOUT,
		0,
		0,
		SPIF_SENDCHANGE,
	)
	defer procSystemParametersInfoW.Call(
		SPI_SETFOREGROUNDLOCKTIMEOUT,
		0,
		uintptr(oldTimeout),
		SPIF_SENDCHANGE,
	)

	// 4. Send a harmless simulated Alt key down/up to trick Windows into allowing foreground switch
	procKeybdEvent.Call(VK_MENU, 0, KEYEVENTF_EXTENDEDKEY, 0)
	procKeybdEvent.Call(VK_MENU, 0, KEYEVENTF_EXTENDEDKEY|KEYEVENTF_KEYUP, 0)

	// 5. Restore if minimized
	isIconic, _, _ := procIsIconic.Call(uintptr(target))
	if isIconic != 0 {
		procShowWindow.Call(uintptr(target), SW_RESTORE)
	} else {
		procShowWindow.Call(uintptr(target), SW_SHOW)
	}

	// 6. Invoke SetForegroundWindow and BringWindowToTop
	ret, _, _ := procSetForegroundWindow.Call(uintptr(target))
	procBringWindowToTop.Call(uintptr(target))

	if ret == 0 {
		// Retry once after brief pause
		time.Sleep(15 * time.Millisecond)
		ret, _, _ = procSetForegroundWindow.Call(uintptr(target))
		if ret == 0 {
			return fmt.Errorf("failed to set foreground window for HWND %d", target)
		}
	}

	return nil
}
