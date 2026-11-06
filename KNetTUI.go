package KNetTUI

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KrigMoon/KJournal"
	"github.com/KrigMoon/KNet/KNet"
	"github.com/KrigMoon/KNet/KNet/Commands"
	"github.com/KrigMoon/KNet/Proto"
	knetpb "github.com/KrigMoon/KNet/Proto/gen"
	"github.com/admpub/once"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

//region Structures, types, settings and constructor

//region^ KNetTUI's Interface

type AppLogic func(Interface, context.Context, *sync.WaitGroup)

type Interface struct {
	KNetTUI  InterfaceKNetTUI
	KNet     Commands.InterfaceKNet
	KNetCore Commands.InterfaceKNetCore
}

type InterfaceKNetTUI interface {
	PrintConsole(line string)
	AddPage(key string, item tview.Primitive, resize, visible bool, hotkey tcell.Key)
	DelPage(name string)
	App() *tview.Application
}

//endregion^

//region^ KNetTUI's Settings

type KNTSettings struct {
	Journal KJournal.KJSettings
	API     KNet.KNSettings
}

// desc: Validates settings
func (knts *KNTSettings) validate() error {
	return nil
}

//endregion^

//region^ KNetTUI

type KNetTUI struct {
	//* Settings, Status and Journal
	settings KNTSettings
	status   knetpb.Status
	Journal  KJournal.KJournal
	//* Lifecycle
	stopOnce      once.Once
	awaitStopOnce sync.WaitGroup
	stopTwice     once.Once
	//* API
	kn KNet.KNet
	//* tview
	app     *tview.Application
	pages   *tview.Pages
	console *tview.TextView
	peers   *tview.TextView
	metrics *tview.TextView
	node    *tview.TextView
	input   *tview.InputField
	//* Processes
	ctxTUI       context.Context
	cancelTUI    context.CancelFunc
	processesTUI sync.WaitGroup
	ctxAPI       context.Context
	cancelAPI    context.CancelFunc
	processesAPI sync.WaitGroup
	//* Maps
	CommandsMap   map[string]Commands.CommandContainer
	inputHandlers sync.Map
}

func NewKNetTUI(commandsMap map[string]Commands.CommandContainer) KNetTUI {
	return KNetTUI{
		CommandsMap: commandsMap,
	}
}

//endregion^

//endregion

//region Lifecycle

func (knt *KNetTUI) Run(settings KNTSettings, appWorkstation AppLogic) error {
	if knt.status != knetpb.Status_OFF {
		return fmt.Errorf("KNetTUI is already running")
	}

	//* Initialization and check settings
	knt.settings = settings
	if err := knt.settings.validate(); err != nil {
		return err
	}

	//* Initialization lifecycle and design
	knt.stopOnce.Reset()
	knt.stopTwice.Reset()
	knt.design()

	//* Initialization journal
	knt.Journal = KJournal.NewKJournal()
	knt.Journal.Run(knt.settings.Journal)
	knt.Journal.NewWriter()
	readerID := "tuiJournal"
	readerID, err := knt.Journal.NewReader(&readerID, 0, nil)
	if err != nil {
		knt.PrintConsole(err.Error())
	}
	knt.settings.API.Journal = &knt.Journal
	knt.settings.API.Core.Journal = &knt.Journal

	knt.Journal.Write(Proto.NewRecord("KNetTUI", knetpb.RecordLevel_INFO, knetpb.RecordCode_LIFECIRCLE, "KNetTUI RUNNING"))
	knt.status = knetpb.Status_RUNNING

	//* Running TUI processes
	knt.ctxTUI, knt.cancelTUI = context.WithCancel(context.Background())
	go knt.readerRecords()

	//* Initialization Commands Map
	knt.setTUICommands()
	extCM := make(map[string]struct{})
	for i := range knt.CommandsMap {
		extCM[i] = struct{}{}
	}
	knt.settings.API.ExternalCommandsMap = extCM

	knt.Journal.Write(Proto.NewRecord("KNetTUI", knetpb.RecordLevel_INFO, knetpb.RecordCode_LIFECIRCLE, "KNetTUI ON"))
	knt.status = knetpb.Status_ON

	//* Running API
	go func() {
		time.Sleep(1000 * time.Millisecond)

		//* Running API
		knt.kn = KNet.NewKNet()
		if err := knt.kn.Run(knt.settings.API); err != nil {
			knt.PrintConsole(err.Error())
			return
		}

		//* Running API processes
		knt.ctxAPI, knt.cancelAPI = context.WithCancel(context.Background())
		go knt.readerResponse()
		go knt.executerPrograms()
		go knt.readerPackages()
		go knt.updPeers()
		go knt.updMetrics()

		knt.updNode()

		//* Launching custom logic
		i := knt.kn.GetInterface()
		appWorkstation(Interface{
			KNetTUI:  knt,
			KNet:     i.KNet,
			KNetCore: i.KNetCore,
		},
			knt.ctxAPI,
			&knt.processesAPI,
		)
	}()

	//* Running TUI
	if err := knt.app.Run(); err != nil {
		knt.PrintConsole(err.Error())
	}

	return nil
}

func (knt *KNetTUI) stop() {
	oOrT := true
	knt.stopOnce.Do(func() {
		knt.awaitStopOnce.Add(1)

		//* Stoping API processes
		knt.cancelAPI()
		knt.processesAPI.Wait()

		//* Stoping API
		knt.kn.Endwork()

		knt.awaitStopOnce.Done()
		oOrT = false
	})
	if oOrT {
		knt.stopTwice.Do(func() {
			knt.awaitStopOnce.Wait()

			knt.Journal.Write(Proto.NewRecord("KNetTUI", knetpb.RecordLevel_INFO, knetpb.RecordCode_LIFECIRCLE, "KNetTUI STOP"))
			knt.status = knetpb.Status_STOP

			//* Stoping TUI processes
			knt.cancelTUI()
			knt.processesTUI.Wait()

			knt.Journal.Write(Proto.NewRecord("KNetTUI", knetpb.RecordLevel_INFO, knetpb.RecordCode_LIFECIRCLE, "KNetTUI OFF"))
			knt.status = knetpb.Status_OFF

			knt.Journal.DelWriter()
			knt.Journal.Endwork()

			time.Sleep(1000 * time.Millisecond)

			//* Stoping TUI
			knt.app.Stop()
		})
	}
}

//endregion

//region Design

func (knt *KNetTUI) consoleWindow(cn *tview.Flex) {
	knt.console = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	knt.console.SetBorder(true).SetTitle(" Console ")
	knt.console.SetBackgroundColor(tcell.GetColor(Background))
	knt.console.SetBorderColor(tcell.GetColor(Border))

	page := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(knt.console, 0, 1, false).
		AddItem(cn, 7, 0, false).
		AddItem(knt.input, 3, 0, true)

	knt.pages.AddPage("console", page, true, true)
}

func (knt *KNetTUI) infoWindow(cn *tview.Flex) {
	knt.peers = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	knt.peers.SetBorder(true).SetTitle(" Peers ")
	knt.peers.SetBackgroundColor(tcell.GetColor(Background))
	knt.peers.SetBorderColor(tcell.GetColor(Border))

	knt.metrics = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	knt.metrics.SetBorder(true).SetTitle(" Metrics ")
	knt.metrics.SetBackgroundColor(tcell.GetColor(Background))
	knt.metrics.SetBorderColor(tcell.GetColor(Border))

	top := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(knt.peers, 0, 1, false).
		AddItem(knt.metrics, 0, 1, false)

	page := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(top, 0, 1, false).
		AddItem(cn, 7, 0, false).
		AddItem(knt.input, 3, 0, true)

	knt.pages.AddPage("info", page, true, false)
}

func (knt *KNetTUI) design() {
	knt.app = tview.NewApplication()
	knt.app.EnableMouse(true)

	knt.pages = tview.NewPages()

	knt.input = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0)
	knt.input.SetBorder(true).SetTitle(" Input ")
	knt.input.SetBackgroundColor(tcell.GetColor(Background))
	knt.input.SetBorderColor(tcell.GetColor(Border))
	knt.input.SetFieldBackgroundColor(tcell.GetColor(InputBackground))
	knt.input.SetFieldTextColor(tcell.GetColor(InputForeground))

	knt.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := knt.input.GetText()
			if text != "" {
				go func() {
					if err := knt.kn.Execute(text); err != nil {
						knt.Journal.Write(Proto.NewRecord("KNet", knetpb.RecordLevel_ERROR, knetpb.RecordCode_CommandCenter, "Execute error: "+err.Error()))
					}
				}()
				knt.input.SetText("")
			}
		}
	})

	knt.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		knt.inputHandlers.Range(func(_, v any) bool {
			for _, h := range v.([]func(*tcell.EventKey) *tcell.EventKey) {
				if ev := h(event); ev == nil {
					return false
				}
			}
			return true
		})

		switch event.Key() {
		case tcell.KeyEscape:
			go knt.stop()
			return nil
		case tcell.KeyF1:
			knt.pages.SwitchToPage("console")
			return nil
		case tcell.KeyF2:
			knt.pages.SwitchToPage("info")
			return nil
		}
		return event
	})

	controls := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	controls.SetBorder(true).SetTitle(" Controls ")
	controls.SetText(fmt.Sprintf("%s%s%s%s\n%s%s%s%s\n%s%s%s%s",
		RegularText("("),
		ColoredText(Magenta, "F1"),
		RegularText(") - "),
		SecondaryText("Console Window"),
		RegularText("("),
		ColoredText(Magenta, "F2"),
		RegularText(") - "),
		SecondaryText("Info Window"),
		RegularText("("),
		ColoredText(Magenta, "Esc"),
		RegularText(") - "),
		SecondaryText("Exit"),
	))
	controls.SetBackgroundColor(tcell.GetColor(Background))
	controls.SetBorderColor(tcell.GetColor(Border))

	knt.node = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	knt.node.SetBorder(true).SetTitle(" Node ")
	knt.node.SetBackgroundColor(tcell.GetColor(Background))
	knt.node.SetBorderColor(tcell.GetColor(Border))

	cn := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(controls, 0, 1, false).
		AddItem(knt.node, 0, 1, false)

	knt.consoleWindow(cn)
	knt.infoWindow(cn)

	knt.app.SetRoot(knt.pages, true).SetFocus(knt.input)
}

//endregion
