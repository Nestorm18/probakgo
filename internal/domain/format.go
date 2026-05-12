package domain

import "fmt"

// FormatBytes formats b using SI units (base 1000), 2 decimal places. Returns "–" for zero or negative values.
func FormatBytes(b int64) string {
	if b <= 0 {
		return "–"
	}
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
