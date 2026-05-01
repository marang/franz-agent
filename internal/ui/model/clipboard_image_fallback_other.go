//go:build !linux || android

package model

func readClipboardImageFallback() ([]byte, error) {
	return nil, errClipboardUnknownFormat
}
