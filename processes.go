package KNetTUI

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/KrigMoon/KNet/KNet/Commands"
	knetpb "github.com/KrigMoon/KNet/Proto/gen"
)

//region TUI

func (knt *KNetTUI) readerRecords() {
	defer knt.Journal.DelReader("tuiJournal")

	// Auto-cancel: exit the loop once writers stop
	done := make(chan struct{})
	go func() {
		knt.Journal.AwaitWritersStop()
		close(done)
	}()

	for {
		select {
		case <-done:
			// read the remaining records and exit
			records, _ := knt.Journal.ReadGroup("tuiJournal")
			for _, record := range records {
				knt.PrintConsole(coloredRecord(record))
			}
			return

		default:
			records, err := knt.Journal.ReadGroup("tuiJournal")
			if err != nil {
				return
			}
			for _, record := range records {
				knt.PrintConsole(coloredRecord(record))
			}
		}
	}
}

func (knt *KNetTUI) updMetrics() {
	knt.processesTUI.Add(1)
	defer knt.processesTUI.Done()

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-knt.ctxTUI.Done():
			return
		case <-ticker.C:
			kjm := knt.Journal.GetMetrics()
			knm := knt.kn.GetMetrics()
			kncm := knm.Core

			//* Journal
			line := ColoredText(Magenta, "Journal\n")
			line += fmt.Sprintf("\t%s%s\n", RegularText("Status: "), SecondaryText(kjm.Status))
			line += fmt.Sprintf("\t%s%s\n", RegularText("Writers: "), SecondaryText(strconv.Itoa(kjm.Writers)))
			line += fmt.Sprintf("\t%s%s\n", RegularText("Readers: "), SecondaryText(strconv.Itoa(kjm.Readers)))
			for id, fill := range kjm.ReadersFill {
				line += fmt.Sprintf("\t\t%s\n", ChanText(id, fill.Len, fill.Cap))
			}

			//* KNet
			line += ColoredText(Magenta, "KNet\n")
			line += fmt.Sprintf("\t%s%s\n", RegularText("Status: "), SecondaryText(knm.Status.String()))
			line += fmt.Sprintf("\t%s%s\n", RegularText("Cursor queue: "), FillChanText(knm.Cursor.Len, knm.Cursor.Cap))
			line += fmt.Sprintf("\t\t%s%s\n", SecondaryText("Cursor: "), ColoredText(Gray, knm.Cursor.Cursor.ID))
			line += fmt.Sprintf("\t%s%s\n", SecondaryText("Executer: "), ColoredText(Gray, knm.Cursor.Cursor.Executer))
			line += fmt.Sprintf("\t\t%s%s\n", SecondaryText("Local program: "), ColoredText(Gray, strconv.Itoa(knm.Cursor.Cursor.LenLoc)))
			line += fmt.Sprintf("\t\t%s%s\n", SecondaryText("External program: "), ColoredText(Gray, strconv.Itoa(knm.Cursor.Cursor.LenExt)))
			line += fmt.Sprintf("\t%s%s\n", RegularText("Responses: "), FillChanText(knm.Responses.Len, knm.Responses.Cap))
			line += fmt.Sprintf("\t%s%s\n", RegularText("Readers: "), SecondaryText(strconv.Itoa(knm.ReadersCount)))
			for _, r := range knm.Readers {
				line += fmt.Sprintf("\t\t%s\n", ChanText(r.ID, r.Len, r.Cap))
			}

			//* KNetCore
			line += ColoredText(Magenta, "\tKNetCore\n")
			line += fmt.Sprintf("\t\t%s%s\n", RegularText("Status: "), SecondaryText(kncm.Status.String()))
			line += fmt.Sprintf("\t\t%s%s\n", RegularText("Entry: "), FillChanText(kncm.Entry.Len, kncm.Entry.Cap))
			line += fmt.Sprintf("\t\t%s%s\n", RegularText("Exit: "), FillChanText(kncm.Exit.Len, kncm.Exit.Cap))
			line += fmt.Sprintf("\t\t%s%s\n", RegularText("Peers: "), SecondaryText(strconv.Itoa(kncm.PeersCount)))

			knt.app.QueueUpdateDraw(func() {
				knt.metrics.Clear()
				fmt.Fprintln(knt.metrics, line)
			})
		}
	}
}

//endregion

//region API

func (knt *KNetTUI) readerResponse() {
	knt.processesAPI.Add(1)
	defer knt.processesAPI.Done()

	for {
		select {
		case <-knt.ctxAPI.Done():
			return
		default:
			response := knt.kn.Response()
			if response == nil || response.ResponseMessage == "" {
				time.Sleep(50 * time.Millisecond)
				continue
			}

			knt.PrintConsole(coloredResponse(response))
		}
	}
}

func (knt *KNetTUI) executerPrograms() {
	knt.processesAPI.Add(1)
	defer knt.processesAPI.Done()

	oldCursor := Commands.CursorNil
	for {
		select {
		case <-knt.ctxAPI.Done():
			return
		default:
			newCursor := knt.kn.GetCursor()

			if oldCursor == newCursor {
				time.Sleep(50 * time.Millisecond)
				continue
			}

			if newCursor.Executer == Commands.ExecuterExternal || newCursor.Executer == Commands.ExecuterBoth {
				ext := knt.kn.ExternalPrograms()
				var wait sync.WaitGroup
				for _, item := range ext {
					wait.Add(1)
					go func() {
						defer wait.Done()
						container := knt.CommandsMap[item.Name]
						resp, ok := container.Check(knt.kn.GetInterface(), item)
						if ok {
							resp = container.Body(knt.kn.GetInterface(), item)
						}
						knt.kn.WriteResponse(resp)
					}()
				}
				wait.Wait()
			}

			oldCursor = newCursor
		}
	}
}

func (knt *KNetTUI) readerPackages() {
	knt.processesAPI.Add(1)
	defer knt.processesAPI.Done()
	id := "tuiPackageReader"
	knt.kn.AddReaderPackages(&id, 512, func(p *knetpb.Package) bool {
		return p.DataType == knetpb.DataType_STRING
	})
	defer knt.kn.DelReaderPackages(id)

	for {
		select {
		case <-knt.ctxAPI.Done():
			return
		default:
			pkg := knt.kn.Package(id)
			if pkg == nil {
				time.Sleep(50 * time.Millisecond)
				continue
			}

			knt.PrintConsole(coloredPackage(pkg))
		}
	}
}

//region^ Update panels

func (knt *KNetTUI) updPeers() {
	knt.processesAPI.Add(1)
	defer knt.processesAPI.Done()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-knt.ctxAPI.Done():
			return
		case <-ticker.C:
			peers := knt.kn.GetPeerList()

			var line string
			for _, peer := range peers {
				line += fmt.Sprintf("%s %s%s%s %s\n",
					ColoredText(Magenta, peer.Node.Id),
					RegularText("("),
					ColoredText(Magenta, peer.Node.Type.String()),
					RegularText(")"),
					ColoredText(Magenta, peer.Node.Name),
				)
				if len(peer.PeerList) != 0 {
					for _, subPeer := range peer.PeerList {
						line += fmt.Sprintf("\t%s %s%s%s %s\n",
							ColoredText(Cyan, subPeer.Id),
							RegularText("("),
							ColoredText(Cyan, subPeer.Type.String()),
							RegularText(")"),
							ColoredText(Cyan, subPeer.Name),
						)
					}
				}
			}

			knt.app.QueueUpdateDraw(func() {
				if line == "" && knt.peers.GetText(true) == "" {
					return
				}
				knt.peers.Clear()
				fmt.Fprintln(knt.peers, line)
			})
		}
	}
}

func (knt *KNetTUI) updNode() {
	var TypeColor Color = Green
	if knt.settings.API.Core.MyNode.Type == knetpb.TypeNode_SERVER {
		TypeColor = Blue
	}
	knt.node.SetText(fmt.Sprintf("%s%s\n%s%s\n%s%s\n%s%s\n%s%s",
		RegularText(CropText("Node: ", 17)),
		SecondaryText(knt.settings.API.Core.MyNode.Name),
		RegularText(CropText("Type: ", 17)),
		ColoredText(TypeColor, knt.settings.API.Core.MyNode.Type.String()),
		RegularText(CropText("ID: ", 17)),
		SecondaryText(knt.settings.API.Core.MyNode.Id),
		RegularText(CropText("Listening IP: ", 17)),
		SecondaryText(knt.settings.API.Core.MyNode.ListeningIP),
		RegularText(CropText("Listening Port: ", 17)),
		SecondaryText(strconv.Itoa(int(knt.settings.API.Core.MyNode.ListeningPort))),
	))
}

//endregion^

//endregion
