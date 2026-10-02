package win32

import (
	"fmt"
	"time"
)

// SetForegroundWindowWithBypass brings a target window to foreground cleanly and reliably,
// using proactive input simulation and thread attachment to bypass Windows Focus Stealing Prevention.
func SetForegroundWindowWithBypass(target HWND) error {
	if !IsWindowValid(target) {
		return fmt.Errorf("target window %d is not valid", target)
	}

	EnsureDesktop()
	procAllowSetForegroundWindow.Call(ASFW_ANY)

	currentForeground := GetForegroundWindow()
	if currentForeground == target {
		// Already in foreground
		return nil
	}

	// 1. If target window is minimized, restore it
	isIconic, _, _ := procIsIconic.Call(uintptr(target))
	if isIconic != 0 {
		procShowWindow.Call(uintptr(target), SW_RESTORE)
	} else {
		procShowWindow.Call(uintptr(target), SW_SHOW)
	}

	// 2. Attach thread input
	myTID, _, _ := procGetCurrentThreadId.Call()
	targetTID, _ := GetWindowThreadAndPID(target)
	currentForegroundTID, _ := GetWindowThreadAndPID(currentForeground)

	attachedForeground := false
	attachedTarget := false

	if currentForegroundTID != 0 && uintptr(currentForegroundTID) != myTID {
		ret, _, _ := procAttachThreadInput.Call(myTID, uintptr(currentForegroundTID), 1)
		attachedForeground = (ret != 0)
	}
	if targetTID != 0 && uintptr(targetTID) != myTID {
		ret, _, _ := procAttachThreadInput.Call(myTID, uintptr(targetTID), 1)
		attachedTarget = (ret != 0)
	}

	defer func() {
		if attachedTarget {
			procAttachThreadInput.Call(myTID, uintptr(targetTID), 0)
		}
		if attachedForeground {
			procAttachThreadInput.Call(myTID, uintptr(currentForegroundTID), 0)
		}
	}()

	// 3. Proactive Alt-key pulse satisfying Windows foreground lock timer
	procKeybdEvent.Call(VK_MENU, 0, KEYEVENTF_EXTENDEDKEY, 0)
	procKeybdEvent.Call(VK_MENU, 0, KEYEVENTF_EXTENDEDKEY|KEYEVENTF_KEYUP, 0)

	// 4. Bring window to top and switch
	procBringWindowToTop.Call(uintptr(target))
	procSwitchToThisWindow.Call(uintptr(target), 1)
	ret, _, _ := procSetForegroundWindow.Call(uintptr(target))

	if ret == 0 {
		time.Sleep(20 * time.Millisecond)
		procBringWindowToTop.Call(uintptr(target))
		procSetForegroundWindow.Call(uintptr(target))
		procSwitchToThisWindow.Call(uintptr(target), 1)
	}

	return nil
}
