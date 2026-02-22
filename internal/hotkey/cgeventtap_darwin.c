#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>

// Go callback — defined via //export in hotkey_darwin.go
extern void hotkeyEventCallback(int listenerID, int eventType, int64_t keycode, uint64_t flags);

// Per-listener storage for run loop and event tap references.
static CFRunLoopRef  runLoops[256];
static CFMachPortRef eventTaps[256];

static void reEnableEventTap(int id) {
	if (id >= 0 && id < 256 && eventTaps[id] != NULL) {
		CGEventTapEnable(eventTaps[id], true);
	}
}

// CGEventTap callback — forwards key-down/key-up events to Go.
static CGEventRef eventTapCallback(CGEventTapProxy proxy, CGEventType type,
                                   CGEventRef event, void *userInfo) {
	int id = (int)(uintptr_t)userInfo;

	// Re-enable the tap if macOS disabled it due to timeout.
	if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
		reEnableEventTap(id);
		return event;
	}

	// Only forward key-down, key-up, and flags-changed events.
	if (type != kCGEventKeyDown && type != kCGEventKeyUp && type != kCGEventFlagsChanged) {
		return event;
	}

	int64_t  keycode = CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
	uint64_t flags   = (uint64_t)CGEventGetFlags(event);
	hotkeyEventCallback(id, (int)type, keycode, flags);
	return event;
}

// startEventTap creates a listen-only CGEventTap, attaches it to the current
// thread's CFRunLoop, and blocks until the run loop is stopped.
// Returns 0 on success (run loop stopped normally), -1 on failure.
int startEventTap(int listenerID) {
	if (listenerID < 0 || listenerID >= 256) return -1;

	CGEventMask mask = ((uint64_t)1 << kCGEventKeyDown) | ((uint64_t)1 << kCGEventKeyUp) | ((uint64_t)1 << kCGEventFlagsChanged);
	CFMachPortRef tap = CGEventTapCreate(
		kCGSessionEventTap,
		kCGHeadInsertEventTap,
		kCGEventTapOptionListenOnly,
		mask,
		eventTapCallback,
		(void *)(uintptr_t)listenerID
	);
	if (tap == NULL) {
		return -1;
	}

	eventTaps[listenerID] = tap;
	runLoops[listenerID]  = CFRunLoopGetCurrent();

	CFRunLoopSourceRef src = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, tap, 0);
	CFRunLoopAddSource(runLoops[listenerID], src, kCFRunLoopCommonModes);
	CGEventTapEnable(tap, true);

	CFRunLoopRun(); // blocks until CFRunLoopStop is called

	// Cleanup.
	CGEventTapEnable(tap, false);
	CFRunLoopRemoveSource(runLoops[listenerID], src, kCFRunLoopCommonModes);
	CFRelease(src);
	CFRelease(tap);
	eventTaps[listenerID] = NULL;
	runLoops[listenerID]  = NULL;

	return 0;
}

// stopEventTap stops the run loop for the given listener, causing
// startEventTap to return.
void stopEventTap(int listenerID) {
	if (listenerID >= 0 && listenerID < 256 && runLoops[listenerID] != NULL) {
		CFRunLoopStop(runLoops[listenerID]);
	}
}
