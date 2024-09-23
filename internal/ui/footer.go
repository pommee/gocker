package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func CreateFooter() *tview.TextView {
	header := tview.NewTextView().SetDynamicColors(true)
	header.SetBackgroundColor(tcell.ColorBlue)
	header.SetText("[white:#215ecf:b] ? [white:blue:B] help [white:#215ecf:b] ESC [white:#215ecf:b][white:blue:B] Quit [white:#215ecf:b] 1 [white:blue:B] running [white:#215ecf:b] 2 [white:blue:B] all")
	return header
}
