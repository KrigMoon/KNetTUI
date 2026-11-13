package KNetTUI

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/KrigMoon/KCommandParser"
	"github.com/KrigMoon/KJournal"
	"github.com/KrigMoon/KNet/KNet/Commands"
	"github.com/KrigMoon/KNet/Proto"
	knetpb "github.com/KrigMoon/KNet/Proto/gen"
)

//region PrintText

func (knt *KNetTUI) PrintConsole(line string) {
	knt.app.QueueUpdateDraw(func() {
		fmt.Fprintln(knt.console, line)
		knt.console.ScrollToEnd()
	})
}

//endregion

//region Formating Text

type Color string

const (
	Gray            Color = "#808080"
	Silver          Color = "#C0C0C0"
	White           Color = "#FFFFFF::b"
	Red             Color = "#E05252::b"
	Orange          Color = "#E08A2E::b"
	Yellow          Color = "#E0C040"
	Green           Color = "#4CAF50::b"
	Cyan            Color = "#3BB8C4"
	Blue            Color = "#4A7FE0"
	Magenta         Color = "#B05AC4::b"
	Reset           Color = "-:-:-"
	Background            = "#000000"
	InputBackground       = "#1a1a1a"
	InputForeground       = "#FFFFFF"
	Border                = "#C0C0C0"
)

func ColoredText(color Color, line string) string {
	return fmt.Sprintf("[%s]%s[%s]", color, line, Reset)
}

func RegularText(line string) string {
	return ColoredText(White, line)
}

func SecondaryText(line string) string {
	return ColoredText(Silver, line)
}

func LevelText(level, line string) string {
	switch level {
	case "INFO":
		return ColoredText(Blue, line)
	case "WARNING":
		return ColoredText(Orange, line)
	case "ERROR":
		return ColoredText(Red, line)
	default:
		return SecondaryText(line)
	}
}

func CropText(text string, width int) string {
	if width <= 3 {
		return text
	}

	runes := []rune(text)
	if len(runes) > width {
		return string(runes[:(width-3)]) + "..."
	}
	return text + strings.Repeat(" ", width-len(runes))
}

func FillChanText(length, capacity int) string {
	fill := 0
	if capacity > 0 {
		fill = int(math.Round(float64(length) / float64(capacity) * 100))
	}

	color := Gray

	switch {
	case fill >= 10 && fill <= 50:
		color = Green
	case fill > 50 && fill <= 70:
		color = Yellow
	case fill > 70 && fill <= 90:
		color = Orange
	case fill > 90:
		color = Red
	}

	return ColoredText(color, fmt.Sprintf("%d/%d", length, capacity))
}

func ChanText(label string, length, capacity int) string {
	return fmt.Sprintf("%s%s%s",
		ColoredText(Cyan, CropText(label, 10)),
		RegularText(": "),
		FillChanText(length, capacity),
	)
}

//endregion

//region Convert Stucture To Formating Text

func coloredFTime(t time.Time) string {
	return fmt.Sprintf("%s%s%s",
		RegularText("("),
		ColoredText(Gray, t.Format("15:04:05")),
		RegularText(")"),
	)
}

func coloredRecord(record *KJournal.Record) string {

	return fmt.Sprintf("%s %s %s%s%s%s%s %s",
		coloredFTime(record.Time),
		SecondaryText(CropText(record.Author, 8)),
		RegularText("| ("),
		LevelText(record.Level, record.Level),
		RegularText("|"),
		SecondaryText(record.Code),
		RegularText(")"),
		LevelText(record.Level, record.Message),
	)
}

func coloredResponse(response *KCommandParser.Response) string {
	return fmt.Sprintf("%s %s%s%s %s",
		coloredFTime(response.Time),
		RegularText("("),
		LevelText(response.Level, response.Level),
		RegularText(")"),
		LevelText(response.Level, response.ResponseMessage),
	)
}

func coloredPackage(pkg *knetpb.Package) string {
	return fmt.Sprintf("%s %s %s\n\t%s %s\n\t%s %s",
		ColoredText(Cyan, pkg.From),
		RegularText("~>"),
		ColoredText(Blue, pkg.To),
		RegularText("Metadata:"),
		SecondaryText(pkg.Metadata),
		RegularText("Message: "),
		SecondaryText(pkg.GetText()),
	)
}

//endregion

//region TUI Commands Map

func (knt *KNetTUI) setTUICommands() {
	knt.CommandsMap["/clear"] = Commands.CommandContainer{
		Check: func(i Commands.Interface, c *KCommandParser.Command) (*KCommandParser.Response, bool) {
			return nil, true
		},
		Body: func(i Commands.Interface, c *KCommandParser.Command) *KCommandParser.Response {
			knt.app.QueueUpdateDraw(func() {
				knt.console.Clear()
			})
			return Proto.NewResponse(c.ID, knetpb.RecordLevel_INFO, "clear")
		},
	}
}

//endregion
