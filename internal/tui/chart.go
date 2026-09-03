package tui

import "github.com/NimbleMarkets/ntcharts/v2/barchart"

func drawBars(bars []barchart.BarData, width, height int, horizontal bool) string {
	opts := []barchart.Option{barchart.WithDataSet(bars)}
	if horizontal {
		// WithDataSet must come first: setting horizontal recomputes the origin.
		opts = append(opts, barchart.WithHorizontalBars())
	}
	chart := barchart.New(width, height, opts...)
	chart.Draw()
	return chart.View()
}

func chartWidth(termWidth int) int {
	return min(max(termWidth-8, 12), 62)
}
