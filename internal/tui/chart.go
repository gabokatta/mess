package tui

import "github.com/NimbleMarkets/ntcharts/v2/barchart"

// drawBars puts the labels down the left when horizontal.
func drawBars(bars []barchart.BarData, width, height int, horizontal bool) string {
	opts := []barchart.Option{barchart.WithDataSet(bars)}
	if horizontal {
		// WithDataSet has to come first: the origin recompute that makes
		// room for labels runs when horizontal mode is set.
		opts = append(opts, barchart.WithHorizontalBars())
	}
	chart := barchart.New(width, height, opts...)
	chart.Draw()
	return chart.View()
}

func chartWidth(termWidth int) int {
	return min(max(termWidth-8, 12), 62)
}
