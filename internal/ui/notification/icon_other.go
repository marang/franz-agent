//go:build !darwin

package notification

// Icon is empty on non-darwin to avoid shipping legacy branded raster assets.
var Icon any = ""
