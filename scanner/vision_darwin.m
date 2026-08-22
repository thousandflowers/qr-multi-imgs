// Apple Vision QR backend (compiled only on darwin via the _darwin suffix).
// Declared in vision_darwin.go's cgo preamble; called from decodeWithVision.
//
// Strategy, cheapest first:
//   1. Normalize the long side to ~2200px — the single biggest win on real
//      photos. Tiny QRs get upscaled so the binarizer has enough modules;
//      huge phone shots get downscaled so detection is fast and stable.
//   2. Detect at native (normalized) resolution with CIDetector + Vision.
//   3. On a miss, a bounded enhancement ladder: document cleanup, Otsu
//      threshold, contrast, gamma, sharpen — then a few upscales of those.
// The ladder only runs when the fast path finds nothing, so clean images stay
// quick. QR error correction makes a false decode effectively impossible, so
// trying many variants is safe. (Region-cropping and tiling were measured to
// add nothing on a 89-image receipt set, so they're deliberately omitted.)
//
// Every variant returns ALL the codes it found, not just the first. The ladder
// still stops at the first variant that finds anything, so a variant that sees
// two of three codes ends the search — the caller's exhaustive mode does not
// reach in here. Closing that is the same finder-pattern accounting job as on
// the Go side.
//
// COORDINATES: points are emitted in ORIGINAL image pixels, origin top-left,
// matching the coordinate contract on Go's hit type. Vision and CIDetector
// both work bottom-left, on an image this code has already scaled, so every
// point is unscaled and Y-flipped before being written out. When an EXIF
// orientation had to be applied, Vision's frame no longer matches the frame
// Go's image.Decode sees, so points are omitted entirely rather than emitted
// in a frame the caller cannot reconcile — the caller then falls back to
// payload matching for those.

#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <CoreImage/CoreImage.h>
#import <ImageIO/ImageIO.h>
#import <CoreGraphics/CoreGraphics.h>
#include <string.h>

static const double kNormalizeLongSide = 2200.0;

// One decoded code: payload plus its corners in original-image pixels.
@interface QRRecord : NSObject
@property(nonatomic, copy) NSString *text;
@property(nonatomic, strong) NSMutableArray<NSNumber *> *points; // flat x,y,x,y…
@end
@implementation QRRecord
@end

// mapPoint converts a point in variant coordinates (origin bottom-left) into
// original-image pixels (origin top-left).
static void mapPoint(CGRect extent, double totalScale, double vx, double vy,
                     double *ox, double *oy) {
    double relX = vx - extent.origin.x;
    double relY = vy - extent.origin.y;
    *ox = relX / totalScale;
    *oy = (extent.size.height - relY) / totalScale;
}

static void addPoint(QRRecord *rec, CGRect extent, double totalScale, double vx, double vy) {
    double ox = 0, oy = 0;
    mapPoint(extent, totalScale, vx, vy, &ox, &oy);
    [rec.points addObject:@(ox)];
    [rec.points addObject:@(oy)];
}

// collectQR runs both detectors on one image variant and appends every code
// found to out. totalScale is the variant's size relative to the original
// image; keepGeometry is false when an EXIF orientation makes coordinates
// unreconcilable with the caller's frame.
static void collectQR(CIImage *image, CIContext *ctx, double totalScale,
                      BOOL keepGeometry, NSMutableArray<QRRecord *> *out) {
    CGRect extent = image.extent;

    CIDetector *detector =
        [CIDetector detectorOfType:CIDetectorTypeQRCode
                           context:ctx
                           options:@{CIDetectorAccuracy : CIDetectorAccuracyHigh}];
    for (CIFeature *f in [detector featuresInImage:image]) {
        if (![f isKindOfClass:[CIQRCodeFeature class]]) {
            continue;
        }
        CIQRCodeFeature *q = (CIQRCodeFeature *)f;
        if (q.messageString == nil) {
            continue;
        }
        QRRecord *rec = [[QRRecord alloc] init];
        rec.text = q.messageString;
        rec.points = [NSMutableArray array];
        if (keepGeometry) {
            addPoint(rec, extent, totalScale, q.topLeft.x, q.topLeft.y);
            addPoint(rec, extent, totalScale, q.topRight.x, q.topRight.y);
            addPoint(rec, extent, totalScale, q.bottomRight.x, q.bottomRight.y);
            addPoint(rec, extent, totalScale, q.bottomLeft.x, q.bottomLeft.y);
        }
        [out addObject:rec];
    }

    CGImageRef cg = [ctx createCGImage:image fromRect:extent];
    if (cg == NULL) {
        return;
    }
    VNImageRequestHandler *handler =
        [[VNImageRequestHandler alloc] initWithCGImage:cg options:@{}];
    CGImageRelease(cg);

    VNDetectBarcodesRequest *request = [[VNDetectBarcodesRequest alloc] init];
    request.symbologies = @[ VNBarcodeSymbologyQR ];
    if (![handler performRequests:@[ request ] error:nil]) {
        return;
    }
    for (VNBarcodeObservation *obs in request.results) {
        if (obs.payloadStringValue == nil) {
            continue;
        }
        QRRecord *rec = [[QRRecord alloc] init];
        rec.text = obs.payloadStringValue;
        rec.points = [NSMutableArray array];
        if (keepGeometry) {
            // Vision reports normalized coordinates against the image handed
            // to the handler, which is this variant at its own extent.
            CGPoint corners[4] = {obs.topLeft, obs.topRight, obs.bottomRight, obs.bottomLeft};
            for (int i = 0; i < 4; i++) {
                addPoint(rec, extent, totalScale,
                         extent.origin.x + corners[i].x * extent.size.width,
                         extent.origin.y + corners[i].y * extent.size.height);
            }
        }
        [out addObject:rec];
    }
}

// Apply a named single-input CIFilter, returning the input unchanged if the
// filter is unavailable (older macOS) or produces no output.
static CIImage *applyFilter(CIImage *image, NSString *name, NSDictionary *params) {
    CIFilter *filter = [CIFilter filterWithName:name];
    if (filter == nil) {
        return image;
    }
    [filter setValue:image forKey:kCIInputImageKey];
    for (NSString *k in params) {
        [filter setValue:params[k] forKey:k];
    }
    return filter.outputImage ?: image;
}

static CIImage *contrastGray(CIImage *image, double contrast) {
    return applyFilter(image, @"CIColorControls",
                       @{kCIInputContrastKey : @(contrast), kCIInputSaturationKey : @0.0});
}
static CIImage *gammaAdjust(CIImage *image, double power) {
    return applyFilter(image, @"CIGammaAdjust", @{@"inputPower" : @(power)});
}
static CIImage *documentEnhance(CIImage *image) {
    return applyFilter(image, @"CIDocumentEnhancer", @{@"inputAmount" : @1.0});
}
static CIImage *otsuThreshold(CIImage *image) {
    return applyFilter(image, @"CIColorThresholdOtsu", @{});
}
static CIImage *sharpen(CIImage *image) {
    return applyFilter(contrastGray(image, 1.4), @"CISharpenLuminance",
                       @{@"inputSharpness" : @1.0});
}
static CIImage *scaleImage(CIImage *image, double scale) {
    return [image imageByApplyingTransform:CGAffineTransformMakeScale(scale, scale)];
}

static void appendU32(NSMutableData *d, uint32_t v) { [d appendBytes:&v length:sizeof(v)]; }
static void appendF64(NSMutableData *d, double v) { [d appendBytes:&v length:sizeof(v)]; }

// serialize packs records into the length-prefixed wire format the Go side
// parses. A delimiter is unusable here: QR payloads carry arbitrary bytes,
// including any separator that might be chosen.
//
//   uint32 count
//   per record: uint32 textLen, textLen bytes,
//               uint32 pointCount, pointCount * (double x, double y)
//
// Host byte order on both sides; this buffer never leaves the process.
static char *serialize(NSArray<QRRecord *> *records, int *outLen) {
    NSMutableData *buf = [NSMutableData data];
    appendU32(buf, (uint32_t)records.count);
    for (QRRecord *rec in records) {
        NSData *utf8 = [rec.text dataUsingEncoding:NSUTF8StringEncoding];
        appendU32(buf, (uint32_t)utf8.length);
        [buf appendData:utf8];
        uint32_t nPoints = (uint32_t)(rec.points.count / 2);
        appendU32(buf, nPoints);
        for (uint32_t i = 0; i < nPoints * 2; i++) {
            appendF64(buf, rec.points[i].doubleValue);
        }
    }
    char *out = malloc(buf.length);
    if (out == NULL) {
        return NULL;
    }
    memcpy(out, buf.bytes, buf.length);
    *outLen = (int)buf.length;
    return out;
}

char *decodeQRVisionAll(const char *cpath, int *outLen) {
    @autoreleasepool {
        *outLen = 0;
        NSString *path = [NSString stringWithUTF8String:cpath];
        NSURL *url = [NSURL fileURLWithPath:path];

        CIImage *base = [CIImage imageWithContentsOfURL:url];
        if (base == nil) {
            // NULL means this decoder could not read the file at all, which is
            // what lets the caller tell "unreadable" apart from "no QR here".
            return NULL;
        }
        double originalLongSide = MAX(base.extent.size.width, base.extent.size.height);

        // Apply EXIF orientation so rotated phone photos are read upright.
        // Doing so moves Vision into a frame the Go decoder never sees, so
        // geometry is dropped for those images; see the header comment.
        BOOL keepGeometry = YES;
        CGImageSourceRef src = CGImageSourceCreateWithURL((__bridge CFURLRef)url, NULL);
        if (src != NULL) {
            NSDictionary *props =
                (NSDictionary *)CFBridgingRelease(CGImageSourceCopyPropertiesAtIndex(src, 0, NULL));
            CFRelease(src);
            NSNumber *o = props[(NSString *)kCGImagePropertyOrientation];
            if (o != nil && o.intValue != 1) {
                base = [base imageByApplyingOrientation:o.intValue];
                keepGeometry = NO;
            }
        }

        // Normalize the long side so module size lands in the detector's sweet
        // spot regardless of source resolution.
        double normalizeScale = 1.0;
        double maxSide = MAX(base.extent.size.width, base.extent.size.height);
        if (maxSide > 0 && originalLongSide > 0) {
            normalizeScale = kNormalizeLongSide / maxSide;
            base = scaleImage(base, normalizeScale);
        }

        CIContext *ctx = [CIContext contextWithOptions:nil];
        NSMutableArray<QRRecord *> *found = [NSMutableArray array];

        // Fast path: normalized native resolution.
        collectQR(base, ctx, normalizeScale, keepGeometry, found);
        if (found.count > 0) {
            return serialize(found, outLen);
        }

        // Enhancement ladder at native size.
        CIImage *globals[] = {
            documentEnhance(base), otsuThreshold(base), contrastGray(base, 2.5),
            gammaAdjust(base, 0.6), sharpen(base),
        };
        for (int i = 0; i < 5; i++) {
            collectQR(globals[i], ctx, normalizeScale, keepGeometry, found);
            if (found.count > 0) {
                return serialize(found, outLen);
            }
        }

        // Upscale ladder: plain, document-cleaned, and contrast-stretched.
        const double scales[] = {2.0, 3.0};
        for (int s = 0; s < 2; s++) {
            CIImage *up = scaleImage(base, scales[s]);
            CIImage *variants[] = {up, documentEnhance(up), contrastGray(up, 2.0)};
            for (int v = 0; v < 3; v++) {
                collectQR(variants[v], ctx, normalizeScale * scales[s], keepGeometry, found);
                if (found.count > 0) {
                    return serialize(found, outLen);
                }
            }
        }
        // Read fine, found nothing: an empty list, not a failure. Core Image
        // opens HEIC and other formats Go cannot, so this answer is what tells
        // the caller a raster decode failure was not the end of the story.
        return serialize(found, outLen);
    }
}
