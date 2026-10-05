package win32

var (
	procMessageBeep = user32.NewProc("MessageBeep")
)

const (
	MB_OK              = 0x00000000
	MB_ICONHAND        = 0x00000010 // Stop / Error
	MB_ICONEXCLAMATION = 0x00000030 // Warning / Attention needed
	MB_ICONASTERISK    = 0x00000040 // Information / Success
)

// PlayNativeChime plays a gentle, optional standard Windows sound.
// Hardware motherboard speaker beeps (kernel32 Beep) have been completely removed.
func PlayNativeChime(chimeType string) {
	// Intentionally a no-op or gentle standard Windows sound to prevent unwanted noise
}
