package Graphite

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ShowFilePicker opens a modal dialog that lets the user browse the filesystem
// and select a file. The callback onSelect is invoked with the absolute path
// of the chosen file. If the user cancels, the modal is closed and onSelect is not called.
func ShowFilePicker(app *Application, initialDir string, onSelect func(path string)) {
	dir, err := filepath.Abs(initialDir)
	if err != nil {
		dir = "."
	}

	mod := NewWindow(0, 0, " "+app.T(KeyOpen)+" ")
	mod.SetPercentSize(90, 90)

	var history []string
	historyIdx := -1

	var loadDir func(target string, addToHistory bool)

	backBtn := NewButton(2, 1, "<", BtnDefault, func() {
		if historyIdx > 0 {
			historyIdx--
			loadDir(history[historyIdx], false)
		}
	})
	mod.AddWidget(backBtn)

	fwdBtn := NewButton(8, 1, ">", BtnDefault, func() {
		if historyIdx < len(history)-1 {
			historyIdx++
			loadDir(history[historyIdx], false)
		}
	})
	mod.AddWidget(fwdBtn)

	upBtn := NewButton(14, 1, "^", BtnDefault, func() {
		parent := filepath.Dir(dir)
		if parent != dir {
			loadDir(parent, true)
		}
	})
	mod.AddWidget(upBtn)

	pathInput := NewInputBox(20, 1, -2, app.T(KeyDirLabel))
	pathInput.Value = dir
	pathInput.CursorPos = len([]rune(dir))
	mod.AddWidget(pathInput)

	padRight := func(s string, w int) string {
		totalW := 0
		for _, r := range s {
			totalW += runeWidth(r)
		}
		if totalW > w {
			res := ""
			cur := 0
			for _, r := range s {
				rw := runeWidth(r)
				if cur+rw > w-1 {
					break
				}
				res += string(r)
				cur += rw
			}
			return res + "…" + strings.Repeat(" ", w-cur-1)
		}
		return s + strings.Repeat(" ", w-totalW)
	}

	padLeft := func(s string, w int) string {
		totalW := 0
		for _, r := range s {
			totalW += runeWidth(r)
		}
		if totalW > w {
			res := ""
			cur := 0
			for _, r := range s {
				rw := runeWidth(r)
				if cur+rw > w-1 {
					break
				}
				res += string(r)
				cur += rw
			}
			return res + "…" + strings.Repeat(" ", w-cur-1)
		}
		return strings.Repeat(" ", w-totalW) + s
	}

	headerLbl := NewLabel(2, 3, "")
	mod.AddWidget(headerLbl)

	var list *ListBox
	var entries []os.DirEntry
	var displayItems []string

	fileNameInput := NewInputBox(2, -2, -46, app.T(KeyFileNameLabel))
	mod.AddWidget(fileNameInput)

	filters := []string{app.T(KeyFilterAll), app.T(KeyFilterGph), app.T(KeyFilterImage), app.T(KeyFilterVideo)}
	filterBox := NewComboBox(-44, -2, 22, filters, func(idx int, item string) {
		loadDir(dir, false)
	})
	mod.AddWidget(filterBox)

	formatSize := func(bytes int64) string {
		if bytes < 1024 {
			return fmt.Sprintf("%d B", bytes)
		}
		kb := float64(bytes) / 1024.0
		if kb < 1024 {
			return fmt.Sprintf("%.0f KB", kb)
		}
		mb := kb / 1024.0
		if mb < 1024 {
			return fmt.Sprintf("%.1f MB", mb)
		}
		return fmt.Sprintf("%.1f GB", mb/1024.0)
	}

	loadDir = func(target string, addToHistory bool) {
		target, _ = filepath.Abs(target)
		d, err := os.ReadDir(target)
		if err == nil {
			if addToHistory {
				if historyIdx < len(history)-1 {
					history = history[:historyIdx+1]
				}
				history = append(history, target)
				historyIdx = len(history) - 1
			}

			dir = target
			pathInput.Value = dir
			pathInput.CursorPos = len([]rune(dir))

			filterGph := (filterBox.Selected == 1)
			filterImg := (filterBox.Selected == 2)
			filterVid := (filterBox.Selected == 3)

			var dirs, files []os.DirEntry
			for _, e := range d {
				if e.IsDir() {
					dirs = append(dirs, e)
				} else {
					ext := strings.ToLower(filepath.Ext(e.Name()))
					if filterGph {
						if ext == ".gph" {
							files = append(files, e)
						}
					} else if filterImg {
						if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
							files = append(files, e)
						}
					} else if filterVid {
						if ext == ".mp4" || ext == ".avi" || ext == ".mkv" || ext == ".webm" {
							files = append(files, e)
						}
					} else {
						files = append(files, e)
					}
				}
			}

			sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
			sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

			entries = make([]os.DirEntry, 0, len(dirs)+len(files)+1)
			displayItems = make([]string, 0, len(dirs)+len(files)+1)

			tW, _ := GetTerminalSize()
			cW := (tW * 90 / 100) - 8
			listW := cW - 4
			if listW < 40 {
				listW = 40
			}
			itemW := listW - 3
			dateW := 16
			typeW := 12
			sizeW := 10
			nameW := itemW - dateW - typeW - sizeW - 3
			if nameW < 10 {
				nameW = 10
			}

			headerLbl.SetText("  " + padRight(app.T(KeyColumnName), nameW) + " " + padRight(app.T(KeyColumnDate), dateW) + " " + padRight(app.T(KeyColumnType), typeW) + " " + padLeft(app.T(KeyColumnSize), sizeW))

			headerLbl.SetText("  " + padRight(app.T(KeyColumnName), nameW) + " " + padRight(app.T(KeyColumnDate), dateW) + " " + padRight(app.T(KeyColumnType), typeW) + " " + padLeft(app.T(KeyColumnSize), sizeW))

			for _, e := range dirs {
				entries = append(entries, e)
				info, _ := e.Info()
				dateStr := ""
				if info != nil {
					dateStr = info.ModTime().Format("02.01.2006 15:04")
				}
				name := "/" + e.Name()
				displayItems = append(displayItems, padRight(name, nameW)+" "+padRight(dateStr, dateW)+" "+padRight(app.T(KeyFileFolder), typeW)+" "+padLeft("", sizeW))
			}
			for _, e := range files {
				entries = append(entries, e)
				info, _ := e.Info()
				dateStr := ""
				sizeStr := ""
				if info != nil {
					dateStr = info.ModTime().Format("02.01.2006 15:04")
					sizeStr = formatSize(info.Size())
				}
				ext := filepath.Ext(e.Name())
				if len(ext) > 0 {
					ext = ext[1:] + " " + app.T(KeyFileGeneric)
				} else {
					ext = app.T(KeyFileGeneric)
				}
				displayItems = append(displayItems, padRight(e.Name(), nameW)+" "+padRight(dateStr, dateW)+" "+padRight(ext, typeW)+" "+padLeft(sizeStr, sizeW))
			}

			if list != nil {
				list.Items = displayItems
				list.Selected = 0
				list.Scroll = 0
			}
		}
	}

	pathInput.OnSubmit = func(val string) {
		info, err := os.Stat(val)
		if err == nil {
			if info.IsDir() {
				loadDir(val, true)
			} else {
				// It's a file
				loadDir(filepath.Dir(val), true)
				fileNameInput.Value = filepath.Base(val)
				fileNameInput.CursorPos = len([]rune(fileNameInput.Value))
				// Try to select the file in the list
				for i, e := range entries {
					if e != nil && !e.IsDir() && e.Name() == filepath.Base(val) {
						list.Selected = i
						if i >= list.LastH {
							list.Scroll = i - list.LastH + 1
						} else {
							list.Scroll = 0
						}
						break
					}
				}
			}
		}
	}

	loadDir(dir, true)

	onItemSelect := func(idx int, item string) {
		if idx < 0 || idx >= len(entries) {
			return
		}

		e := entries[idx]
		if e != nil && !e.IsDir() {
			fileNameInput.Value = e.Name()
			fileNameInput.CursorPos = len([]rune(e.Name()))
		}
	}

	list = NewListBox(2, 4, -2, -4, displayItems, onItemSelect)
	mod.AddWidget(list)

	list.OnDoubleClick = func(idx int, item string) {
		if idx < 0 || idx >= len(entries) {
			return
		}
		e := entries[idx]
		if e == nil {
			loadDir(filepath.Dir(dir), true)
			return
		}

		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			loadDir(path, true)
		} else {
			fileNameInput.Value = e.Name()
			app.CloseModal()
			onSelect(path)
		}
	}

	mod.AddWidget(NewButton(-20, -2, app.T(KeyOpen), BtnSuccess, func() {
		if fileNameInput.Value != "" {
			app.CloseModal()
			onSelect(filepath.Join(dir, fileNameInput.Value))
		}
	}))

	mod.AddWidget(NewButton(-10, -2, app.T(KeyCancel), BtnDefault, func() {
		app.CloseModal()
	}))

	app.SetModal(mod)
}
