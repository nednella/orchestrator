//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// Wails builds the window menu natively and binds Minimize to Cmd+M. The app uses that
// key itself, so take it off the item.
static void stripMenuShortcuts(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		for (NSMenuItem *top in [[NSApp mainMenu] itemArray]) {
			for (NSMenuItem *item in [[top submenu] itemArray]) {
				if ([[item title] isEqualToString:@"Minimize"]) {
					[item setKeyEquivalent:@""];
				}
			}
		}
	});
}
*/
import "C"

func stripMenuShortcuts() { C.stripMenuShortcuts() }
