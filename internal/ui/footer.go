package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func CreateFooter() *tview.TextView {
	header := tview.NewTextView().SetDynamicColors(true)
	header.SetBackgroundColor(tcell.GetColor("#24292f"))
	header.SetText("[orange:#24292f:b] ? [#989a9c:#24292f:B] help [orange:#24292f:b] ESC [orange:#24292f:b][#989a9c:#24292f:B] Quit [orange:#24292f:b] 1 [#989a9c:#24292f:B] running [orange:#24292f:b] 2 [#989a9c:#24292f:B] all")
	return header
}
