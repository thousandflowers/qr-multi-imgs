package scanner

// Decoding a viewfinder frame, where latency is the whole product.
//
// The file path and the live path want opposite things. A folder scan runs
// every strategy because the image will not come round again: a code missed is
// a code missed. A viewfinder gets another frame in fifty milliseconds, so the
// cost of missing one is nothing and the cost of being slow is everything -
// a scanner that thinks for a third of a second cannot be aimed, and the
// person waves it around while it works on a picture of where the code used
// to be.
//
// Measured on a 720x540 frame with the full cascade: 64ms when a code is in
// shot, and 320ms when there is nothing - 3 frames a second, and the empty
// frame is the one you spend all your time on while aiming. The reason is the
// early exit: with no finder patterns located there is no count to stop at, so
// every strategy runs, including the two upscales and the three preprocessing
// rungs. Exactly the right behaviour for a file. Exactly the wrong one here.

import (
	"image"

	"github.com/makiuchi-d/gozxing"
	multidetector "github.com/makiuchi-d/gozxing/multi/qrcode/detector"
	"github.com/makiuchi-d/gozxing/qrcode/detector"
)

// liveStrategies is the short cascade. Luminance, both binarizers, native
// scale: the two that between them find 64% of everything the full list finds
// on the benchmark, at a fraction of the cost. Nothing that upscales, nothing
// that transforms pixels first.
//
// A code these two cannot read from a moving frame is a code the person should
// hold the camera steadier for - or freeze and box by hand, which is what the
// freeze button is for.
var liveStrategies = []decodeStrategy{
	{channel: "lum", scale: 1},
	{channel: "lum", scale: 1, global: true},
}

// ScanLive decodes one viewfinder frame and locates what it could not read.
//
// It returns the same Detail as the other entry points, so the page draws
// boxes from it the same way, and it carries no Metadata: nothing on a live
// frame is going to be measured against a corpus, and the image measures cost
// a full pass over the pixels.
func ScanLive(img image.Image) Detail {
	infos := liveFinders(img)
	var hits []hit
	for _, s := range liveStrategies {
		hits = mergeHits(hits, decodeAttempt(img, s))
		if len(infos) > 0 && len(hits) >= len(infos) {
			break
		}
	}
	sortHits(hits)
	codes := hitTexts(hits)

	d := Detail{Codes: codes, Classification: classify(codes, infos)}
	if d.Classification == QRDetectedDecodeFailed {
		d.Detections = detectionsFrom(infos, img.Bounds())
	} else {
		d.Detections = detectionsFromHits(hits, img.Bounds())
	}
	return d
}

// liveFinders is detectFinders over luminance alone.
//
// The full version tries every colour projection and keeps the best, which is
// what recovers a coloured code whose contrast lives in one channel. That is
// six passes over the frame for a case a person can fix in half a second by
// moving the camera, and paying it sixty times a second is how the viewfinder
// stopped being aimable.
func liveFinders(img image.Image) []*detector.FinderPatternInfo {
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	bmp, err := gozxing.NewBinaryBitmap(gozxing.NewGlobalHistgramBinarizer(
		gozxing.NewLuminanceSourceFromImage(img)))
	if err != nil || bmp == nil {
		return nil
	}
	matrix, merr := bmp.GetBlackMatrix()
	if merr != nil || matrix == nil {
		return nil
	}
	infos, ferr := multidetector.NewMultiFinderPatternFinder(matrix, nil).FindMulti(hints)
	if ferr != nil {
		return nil
	}
	return infos
}
