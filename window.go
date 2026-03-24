package main

import (
	"encoding/hex"
	"image/color"
	"strings"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type compactTheme struct{ fyne.Theme }

func (m compactTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameText {
		return 10
	}
	if n == theme.SizeNamePadding {
		return 2
	}
	return theme.DefaultTheme().Size(n)
}

type AppUI struct {
	Window      fyne.Window
	Table       *widget.Table
	Packets     []Packet
	Mu          sync.Mutex
	IsRecording atomic.Bool
	IsInterrupt atomic.Bool
	Config      *Config
	SelectedRow int
	StatusLabel *widget.Label
}

func NewUI(cfg *Config) *AppUI {
	a := app.New()
	a.Settings().SetTheme(&compactTheme{Theme: theme.DefaultTheme()})
	ui := &AppUI{
		Window:      a.NewWindow("Net Analyzer"),
		Config:      cfg,
		SelectedRow: -1,
		StatusLabel: widget.NewLabelWithStyle("ОЖИДАНИЕ", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	}

	widths := []float32{95, 55, 70, 450}

	ui.Table = widget.NewTable(
		func() (int, int) { return len(ui.Packets) + 1, 4 },
		func() fyne.CanvasObject {
			bg := canvas.NewRectangle(color.Transparent)
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return container.NewStack(bg, lbl)
		},
		func(id widget.TableCellID, o fyne.CanvasObject) {
			stack := o.(*fyne.Container)
			bg := stack.Objects[0].(*canvas.Rectangle)
			l := stack.Objects[1].(*widget.Label)

			if id.Row == 0 {
				bg.FillColor = color.NRGBA{230, 230, 230, 255}
				l.TextStyle = fyne.TextStyle{Bold: true}
				l.Alignment = fyne.TextAlignCenter
				switch id.Col {
				case 0:
					l.SetText("Время")
				case 1:
					l.SetText("Длина")
				case 2:
					l.SetText("Опкод")
				case 3:
					l.SetText("Содержимое")
				}
				return
			}

			bg.FillColor = color.Transparent
			l.TextStyle = fyne.TextStyle{}
			l.Alignment = fyne.TextAlignLeading
			ui.Mu.Lock()
			idx := id.Row - 1
			if idx >= len(ui.Packets) {
				ui.Mu.Unlock()
				return
			}
			p := ui.Packets[idx]
			ui.Mu.Unlock()

			txt := ""
			switch id.Col {
			case 0:
				if ui.Config.ShowTime {
					txt = p.Time
				}
			case 1:
				if ui.Config.ShowLen {
					txt = p.Length
				}
			case 2:
				if ui.Config.ShowOpcode {
					txt = p.Opcode
				}
			case 3:
				if ui.Config.ShowData {
					txt = p.Body
				}
			}
			l.SetText(txt)
		},
	)

	for i, w := range widths {
		ui.Table.SetColumnWidth(i, w)
	}
	ui.Table.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 {
			ui.SelectedRow = id.Row - 1
		} else {
			ui.SelectedRow = -1
		}
	}

	btns := container.NewGridWithColumns(2,
		widget.NewButton("Начать запись", func() { ui.IsRecording.Store(true); ui.IsInterrupt.Store(false); ui.updateStatus("ЗАПИСЬ") }),
		widget.NewButton("Запись (ПРЕР)", func() { ui.IsRecording.Store(true); ui.IsInterrupt.Store(true); ui.updateStatus("ПАУЗА") }),
		widget.NewButton("Остановить", func() { ui.IsRecording.Store(false); ui.updateStatus("СТОП") }),
		widget.NewButton("Копировать", func() {
			ui.Mu.Lock()
			if ui.SelectedRow != -1 && ui.SelectedRow < len(ui.Packets) {
				ui.Window.Clipboard().SetContent(ui.Packets[ui.SelectedRow].FullHex)
			}
			ui.Mu.Unlock()
		}),
		widget.NewButton("Очередной", func() {
			ui.Mu.Lock()
			if len(ui.Packets) > 0 {
				SendToClient(ui.Packets[0].FullHex)
				ui.Packets = ui.Packets[1:]
			}
			ui.Mu.Unlock()
			ui.Table.Refresh()
		}),
		widget.NewButton("Отправить", func() {
			ui.Mu.Lock()
			if ui.SelectedRow != -1 && ui.SelectedRow < len(ui.Packets) {
				SendToClient(ui.Packets[ui.SelectedRow].FullHex)
			}
			ui.Mu.Unlock()
		}),
		widget.NewButton("Отправить (у)", func() {
			ui.Mu.Lock()
			if ui.SelectedRow != -1 && ui.SelectedRow < len(ui.Packets) {
				SendToClient(ui.Packets[ui.SelectedRow].FullHex)
				ui.Packets = append(ui.Packets[:ui.SelectedRow], ui.Packets[ui.SelectedRow+1:]...)
				ui.SelectedRow = -1
			}
			ui.Mu.Unlock()
			ui.Table.Refresh()
		}),
		widget.NewButton("Очистить", func() {
			ui.Mu.Lock()
			ui.Packets = nil
			ui.SelectedRow = -1
			ui.Mu.Unlock()
			ui.Table.Refresh()
			ui.Table.ScrollTo(widget.TableCellID{Row: 0, Col: 0})
		}),
		widget.NewButton("Отправить все", func() {
			ui.Mu.Lock()
			for _, p := range ui.Packets {
				SendToClient(p.FullHex)
			}
			ui.Packets = nil
			ui.IsInterrupt.Store(false)
			ui.Mu.Unlock()
			ui.Table.Refresh()
		}),
	)

	rightPanel := container.NewVBox(
		widget.NewLabelWithStyle("Статус", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ui.StatusLabel, widget.NewSeparator(),
		btns, widget.NewSeparator(),
		widget.NewLabelWithStyle("Отображение", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ui.createCheck("Время", &cfg.ShowTime), ui.createCheck("Длина", &cfg.ShowLen),
		ui.createCheck("Опкод", &cfg.ShowOpcode), ui.createCheck("Тело", &cfg.ShowData),
	)

	hexIn := widget.NewEntry()
	hexIn.SetPlaceHolder("Сырой Hex (1400...)")

	btnRawSend := widget.NewButton("Отправить RAW", func() {
		SendToClient(hexIn.Text)
		//hexIn.SetText("")
	})

	btnSendPlus := widget.NewButton("Отправить +", func() {
		data := HexToBytes(hexIn.Text)
		if len(data) >= 19 {
			SendToClient(hexIn.Text)
			data[18]++
			hexIn.SetText(strings.ToUpper(hex.EncodeToString(data)))
		}
	})

	bottomPanel := container.NewBorder(nil, nil, nil, container.NewHBox(btnRawSend, btnSendPlus), hexIn)

	ui.Window.SetContent(container.NewBorder(
		widget.NewLabelWithStyle("ПАКЕТЫ", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true}),
		container.NewPadded(bottomPanel), nil, container.NewPadded(rightPanel), container.NewPadded(ui.Table),
	))
	ui.Window.Resize(fyne.NewSize(900, 600))
	return ui
}

func (ui *AppUI) updateStatus(s string) { ui.StatusLabel.SetText(s) }
func (ui *AppUI) createCheck(n string, f *bool) *widget.Check {
	c := widget.NewCheck(n, func(v bool) { *f = v; ui.Config.Save(); ui.Table.Refresh() })
	c.Checked = *f
	return c
}
func (ui *AppUI) AddPacket(p Packet) {
	ui.Mu.Lock()
	ui.Packets = append(ui.Packets, p)
	ui.Mu.Unlock()
	ui.Table.Refresh()
}
