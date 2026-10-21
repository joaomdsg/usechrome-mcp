package browser

// ImageFormat is the encoding of a screenshot.
type ImageFormat string

// The screenshot encodings.
const (
	PNG  ImageFormat = "png"
	JPEG ImageFormat = "jpeg"
)

// ScreenshotRequest captures what a tab shows: the viewport by default, the
// whole page with FullPage, or one element with Selector. Quality applies to
// JPEG only, 0 to 100.
type ScreenshotRequest struct {
	Tab      string
	Selector string
	FullPage bool
	Format   ImageFormat
	Quality  int
}

// Image is an encoded screenshot and its size in image pixels. Truncated
// says the capture was cut to keep each side within the size limit.
type Image struct {
	Data      []byte
	Format    ImageFormat
	Width     int
	Height    int
	Truncated bool
}
