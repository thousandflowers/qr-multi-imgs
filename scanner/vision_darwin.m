// Apple Vision QR backend (compiled only on darwin via the _darwin suffix).
// Declared in vision_darwin.go's cgo preamble; called from decodeWithVision.
//
// Strategy, cheapest first, stopping at the first decode:
//   1. Normalize the long side to ~2200px — the single biggest win on real
//      photos. Tiny QRs get upscaled so the binarizer has enough modules;
//      huge phone shots get downscaled so detection is fast and stable.
//   2. Detect at native (normalized) resolution with CIDetector + Vision.
//   3. On a miss, a bounded enhancement ladder: document cleanup, Otsu
//      threshold, contrast, gamma, sharpen — then a few upscales of those.
// The ladder only runs when the fast path misses, so clean images stay quick.
// QR error correction makes a false decode effectively impossible, so trying
// many variants is safe. (Region-cropping and tiling were measured to add
// nothing on a 89-image receipt set, so they're deliberately omitted.)

#import <Foundation/Foundation.h>
#import <Vision/Vision.h>
#import <CoreImage/CoreImage.h>
#import <ImageIO/ImageIO.h>
#import <CoreGraphics/CoreGraphics.h>
#include <string.h>

static const double kNormalizeLongSide = 2200.0;

// Run both detectors on one image variant. Returns a strdup'd payload or NULL.
static char *detectQR(CIImage *image, CIContext *ctx) {
    CIDetector *detector =
        [CIDetector detectorOfType:CIDetectorTypeQRCode
                           context:ctx
                           options:@{CIDetectorAccuracy : CIDetectorAccuracyHigh}];
    for (CIFeature *f in [detector featuresInImage:image]) {
        if ([f isKindOfClass:[CIQRCodeFeature class]]) {
            NSString *msg = ((CIQRCodeFeature *)f).messageString;
            if (msg != nil) {
                return strdup(msg.UTF8String);
            }
        }
    }

    CGImageRef cg = [ctx createCGImage:image fromRect:image.extent];
    if (cg == NULL) {
        return NULL;
    }
    VNImageRequestHandler *handler =
        [[VNImageRequestHandler alloc] initWithCGImage:cg options:@{}];
    CGImageRelease(cg);

    VNDetectBarcodesRequest *request = [[VNDetectBarcodesRequest alloc] init];
    request.symbologies = @[ VNBarcodeSymbologyQR ];
    char *result = NULL;
    if ([handler performRequests:@[ request ] error:nil]) {
        for (VNBarcodeObservation *obs in request.results) {
            if (obs.payloadStringValue != nil) {
                result = strdup(obs.payloadStringValue.UTF8String);
                break;
            }
        }
    }
    return result;
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

char *decodeQRVision(const char *cpath) {
    @autoreleasepool {
        NSString *path = [NSString stringWithUTF8String:cpath];
        NSURL *url = [NSURL fileURLWithPath:path];

        CIImage *base = [CIImage imageWithContentsOfURL:url];
        if (base == nil) {
            return NULL;
        }

        // Apply EXIF orientation so rotated phone photos are read upright.
        CGImageSourceRef src = CGImageSourceCreateWithURL((__bridge CFURLRef)url, NULL);
        if (src != NULL) {
            NSDictionary *props =
                (NSDictionary *)CFBridgingRelease(CGImageSourceCopyPropertiesAtIndex(src, 0, NULL));
            CFRelease(src);
            NSNumber *o = props[(NSString *)kCGImagePropertyOrientation];
            if (o != nil) {
                base = [base imageByApplyingOrientation:o.intValue];
            }
        }

        // Normalize the long side so module size lands in the detector's sweet
        // spot regardless of source resolution.
        double maxSide = MAX(base.extent.size.width, base.extent.size.height);
        if (maxSide > 0) {
            base = scaleImage(base, kNormalizeLongSide / maxSide);
        }

        CIContext *ctx = [CIContext contextWithOptions:nil];

        // Fast path: normalized native resolution.
        char *result = detectQR(base, ctx);
        if (result != NULL) {
            return result;
        }

        // Enhancement ladder at native size.
        CIImage *globals[] = {
            documentEnhance(base), otsuThreshold(base), contrastGray(base, 2.5),
            gammaAdjust(base, 0.6), sharpen(base),
        };
        for (int i = 0; i < 5; i++) {
            result = detectQR(globals[i], ctx);
            if (result != NULL) {
                return result;
            }
        }

        // Upscale ladder: plain, document-cleaned, and contrast-stretched.
        const double scales[] = {2.0, 3.0};
        for (int s = 0; s < 2; s++) {
            CIImage *up = scaleImage(base, scales[s]);
            CIImage *variants[] = {up, documentEnhance(up), contrastGray(up, 2.0)};
            for (int v = 0; v < 3; v++) {
                result = detectQR(variants[v], ctx);
                if (result != NULL) {
                    return result;
                }
            }
        }
        return NULL;
    }
}
