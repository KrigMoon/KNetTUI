package KNetTUI

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

//region Pages

// desc: Adds a page and binds a hotkey to switch to it (thread-safe)
func (knt *KNetTUI) AddPage(key string, item tview.Primitive, resize, visible bool, hotkey tcell.Key) {
	if knt.app == nil || knt.pages == nil {
		return
	}

	handler := func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == hotkey {
			knt.pages.SwitchToPage(key)
			return nil
		}
		return event
	}

	knt.inputHandlers.Store(key, []func(*tcell.EventKey) *tcell.EventKey{handler})

	knt.app.QueueUpdateDraw(func() {
		knt.pages.AddPage(key, item, resize, visible)
	})
}

// desc: Removes a page and its hotkey handler (thread-safe)
func (knt *KNetTUI) DelPage(key string) {
	if knt.app == nil || knt.pages == nil {
		return
	}

	knt.inputHandlers.Delete(key)

	knt.app.QueueUpdateDraw(func() {
		knt.pages.RemovePage(key)
	})
}

//endregion

//region App

// desc: Returns the underlying tview application (thread-safe access via QueueUpdateDraw)
func (knt *KNetTUI) App() *tview.Application {
	return knt.app
}

//endregion
