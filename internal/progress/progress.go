// Package progress provides reusable progress bar utilities for Ivaldi VCS
package progress

import (
	"fmt"
	"os"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Bar wraps progressbar.ProgressBar with Ivaldi-specific styling
type Bar struct {
	bar *progressbar.ProgressBar
}

// NewDownloadBar creates a progress bar for download operations
func NewDownloadBar(total int, description string) *Bar {
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintln(os.Stderr)
		}),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
	bar.RenderBlank()
	return &Bar{bar: bar}
}

// NewUploadBar creates a progress bar for upload operations
func NewUploadBar(total int, description string) *Bar {
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintln(os.Stderr)
		}),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
	bar.RenderBlank()
	return &Bar{bar: bar}
}

// NewSpinner creates a spinner-style progress indicator for unknown totals
func NewSpinner(description string) *Bar {
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintln(os.Stderr)
		}),
	)
	return &Bar{bar: bar}
}

// Increment advances the progress bar by one
func (b *Bar) Increment() error {
	return b.bar.Add(1)
}

// Add advances the progress bar by n
func (b *Bar) Add(n int) error {
	return b.bar.Add(n)
}

// Set sets the progress bar to a specific value
func (b *Bar) Set(n int) error {
	return b.bar.Set(n)
}

// Finish completes the progress bar
func (b *Bar) Finish() error {
	return b.bar.Finish()
}

// Clear clears the progress bar from the terminal
func (b *Bar) Clear() error {
	return b.bar.Clear()
}

// Describe updates the description of the progress bar
func (b *Bar) Describe(description string) {
	b.bar.Describe(description)
}

// GetMax returns the maximum value of the progress bar
func (b *Bar) GetMax() int {
	return b.bar.GetMax()
}

// ChangeMax changes the maximum value of the progress bar
func (b *Bar) ChangeMax(max int) {
	b.bar.ChangeMax(max)
}
