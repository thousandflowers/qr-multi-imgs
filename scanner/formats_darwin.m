//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <ImageIO/ImageIO.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdlib.h>
#include <string.h>

// Every raster type ImageIO can open, expressed as filename extensions. This is
// where a Mac's camera-raw support comes from: CR2, ARW, DNG, RAF, ORF and the
// rest are ImageIO types, and asking beats maintaining a list that goes stale
// the moment Apple adds a camera.
//
// Returns a malloc'd buffer of NUL-separated lowercase extensions with no
// leading dot, terminated by a second NUL so the caller can find the end
// without being told the length. NULL if the type list is unavailable.
char* systemReadableExtensions(void) {
    @autoreleasepool {
        CFArrayRef types = CGImageSourceCopyTypeIdentifiers();
        if (types == NULL) {
            return NULL;
        }

        NSMutableSet<NSString *> *exts = [NSMutableSet set];
        for (NSString *uti in (__bridge NSArray *)types) {
            UTType *type = [UTType typeWithIdentifier:uti];
            if (type == nil) {
                continue;
            }
            for (NSString *tag in type.tags[UTTagClassFilenameExtension]) {
                NSString *lower = tag.lowercaseString;
                if (lower.length > 0) {
                    [exts addObject:lower];
                }
            }
        }
        CFRelease(types);

        // PDF is not an ImageIO raster type and never appears in that list, but
        // this build renders its pages with Core Graphics and reads the codes
        // off them, so as far as a folder scan is concerned it is readable.
        [exts addObject:@"pdf"];

        // Size the buffer from the strings themselves: one NUL after each and
        // one more to terminate, so nothing assumes a bound on how many types
        // the OS reports.
        NSUInteger total = 1;
        for (NSString *ext in exts) {
            total += [ext lengthOfBytesUsingEncoding:NSUTF8StringEncoding] + 1;
        }

        char *buf = calloc(total, 1);
        if (buf == NULL) {
            return NULL;
        }
        size_t off = 0;
        for (NSString *ext in exts) {
            const char *c = ext.UTF8String;
            if (c == NULL) {
                continue;
            }
            size_t n = strlen(c);
            if (off + n + 2 > total) {
                break;
            }
            memcpy(buf + off, c, n);
            off += n + 1; // calloc already left the NUL in place
        }
        return buf;
    }
}
