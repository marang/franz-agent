//go:build linux && !android

package model

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

func readClipboardImageFallback() ([]byte, error) {
	// Wayland: discover offered MIME types first, then fetch best image type.
	if out, err := exec.CommandContext(context.Background(), "wl-paste", "--list-types").Output(); err == nil {
		types := strings.Fields(string(out))
		preferred := []string{
			"image/png",
			"image/webp",
			"image/jpeg",
			"image/jpg",
			"image/gif",
			"image/bmp",
		}
		var candidates []string
		for _, p := range preferred {
			for _, t := range types {
				if t == p {
					candidates = append(candidates, p)
				}
			}
		}
		for _, t := range types {
			if strings.HasPrefix(t, "image/") && !containsString(candidates, t) {
				candidates = append(candidates, t)
			}
		}
		for _, mime := range candidates {
			img, imgErr := exec.CommandContext(context.Background(), "wl-paste", "--no-newline", "--type", mime).Output()
			if imgErr == nil && len(bytes.TrimSpace(img)) > 0 {
				return img, nil
			}
		}
	}

	commands := [][]string{
		// X11 fallbacks.
		{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"},
		{"xclip", "-selection", "clipboard", "-t", "image/jpeg", "-o"},
		{"xclip", "-selection", "clipboard", "-t", "image/webp", "-o"},
	}

	var errs []error
	for _, cmd := range commands {
		out, err := exec.CommandContext(context.Background(), cmd[0], cmd[1:]...).Output()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(bytes.TrimSpace(out)) == 0 {
			continue
		}
		return out, nil
	}

	if len(errs) == 0 {
		return nil, errClipboardUnknownFormat
	}
	return nil, errors.Join(errs...)
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
