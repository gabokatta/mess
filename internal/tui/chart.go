package tui

import "github.com/NimbleMarkets/ntcharts/v2/barchart"

func drawBars(bars []barchart.BarData, width, height int) string {
	chart := barchart.New(width, height, barchart.WithDataSet(bars))
	chart.Draw()
	return chart.View()
}

func chartWidth(termWidth int) int {
	return min(max(termWidth-8, 12), 62)
}
