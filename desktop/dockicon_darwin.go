//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// The bytes are copied before this returns, so the caller's slice is free once it does.
static void setDockIcon(const void *png, int length) {
	NSData *data = [[NSData alloc] initWithBytes:png length:length];
	dispatch_async(dispatch_get_main_queue(), ^{
		NSImage *image = [[NSImage alloc] initWithData:data];
		[data release];
		[NSApp setApplicationIconImage:image];
		[image release];
	});
}
*/
import "C"

import "unsafe"

func setDockIcon(png []byte) { C.setDockIcon(unsafe.Pointer(&png[0]), C.int(len(png))) }
