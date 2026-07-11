package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yeoblyv/graphite"
)

// =========================================================================
// COMPONENT PARSER
// =========================================================================

// ComponentManifest is the metadata a component folder describes itself
// with in manifest.json.
type ComponentManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	NeedsAdmin  bool   `json:"needs_admin"`
	Executable  string `json:"executable"`
}

// Component is one discovered subfolder of components/ with its parsed
// manifest.
type Component struct {
	FolderName string
	Path       string
	Manifest   ComponentManifest
}

// loadComponents scans baseDir for subfolders containing a manifest.json
// and returns the ones that parse successfully. Folders without a valid
// manifest are silently skipped rather than treated as an error, since
// baseDir is expected to contain arbitrary user-managed folders.
func loadComponents(baseDir string) []Component {
	var comps []Component
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return comps
	}

	for _, entry := range entries {
		if entry.IsDir() {
			compPath := filepath.Join(baseDir, entry.Name())
			manifestPath := filepath.Join(compPath, "manifest.json")

			data, err := os.ReadFile(manifestPath)
			if err == nil {
				var manifest ComponentManifest
				if err := json.Unmarshal(data, &manifest); err == nil {
					comps = append(comps, Component{
						FolderName: entry.Name(),
						Path:       compPath,
						Manifest:   manifest,
					})
				}
			}
		}
	}
	return comps
}

// =========================================================================
// REAL-TIME STREAMING EXECUTION (Terminal inside Terminal)
// =========================================================================

// streamComponent launches the executable at exePath in the background and
// streams its stdout/stderr into a modal window in real time, closing only
// once the process exits and the user dismisses the result. All widget
// mutations from the background goroutines go through app.Invoke, since
// those goroutines run concurrently with Application.Run's render loop.
func streamComponent(app *Graphite.Application, exePath string, compName string) {
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		app.ShowMessage(" ERROR ", fmt.Sprintf("Executable not found at:\n%s", exePath), Graphite.BtnDanger)
		return
	}

	mod := Graphite.NewWindow(80, 24, fmt.Sprintf(" Executing: %s ", compName))

	statusLbl := Graphite.NewLabel(2, 1, "Status: Starting process...")
	mod.AddWidget(statusLbl)

	outArea := Graphite.NewTextArea(2, 3, 74, 16)
	outArea.SetText("")
	mod.AddWidget(outArea)

	btnClose := Graphite.NewButton(34, 20, "Close", Graphite.BtnDefault, func() {
		app.CloseModal()
	})
	btnClose.SetVisible(false)
	mod.AddWidget(btnClose)

	app.SetModal(mod)

	go func() {
		cmd := exec.Command(exePath)
		cmd.Dir = filepath.Dir(exePath) // Run from the component's own folder.

		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			app.Invoke(func() {
				statusLbl.SetText(fmt.Sprintf("Status: Failed to start (%v)", err))
				btnClose.SetVisible(true)
			})
			return
		}

		app.Invoke(func() {
			statusLbl.SetText("Status: Running... (Streaming output)")
		})

		streamReader := func(r io.Reader) {
			buf := make([]byte, 128)
			for {
				n, err := r.Read(buf)
				if n > 0 {
					chunk := string(buf[:n])
					app.Invoke(func() {
						outArea.Text += chunk
						// Keep the cursor pinned to the end so the view
						// auto-scrolls as output arrives.
						outArea.CursorPos = len([]rune(outArea.Text))
					})
				}
				if err != nil {
					break // Process exited or the pipe was closed.
				}
			}
		}

		go streamReader(stdout)
		go streamReader(stderr)

		err := cmd.Wait()
		app.Invoke(func() {
			if err != nil {
				statusLbl.SetText(fmt.Sprintf("Status: Finished with error code: %v", err))
			} else {
				statusLbl.SetText("Status: Execution completed successfully.")
			}
			btnClose.SetVisible(true)
		})
	}()
}

func main() {
	app := Graphite.NewApplication() // Uses Graphite.DefaultTheme(); see showcase/main.go for a custom theme.
	app.SetStatus(" F1: Help | Arrows/Mouse: Navigate | Tab: Next Widget | Enter: Select | Esc: Exit ")

	win := Graphite.NewWindow(100, 30, " System Launcher Console ")
	win.SetPercentSize(90, 90)

	navTabs := Graphite.NewTabView(0, 0, 90, Graphite.TabDefault)
	win.AddWidget(navTabs)

	components := loadComponents("components")
	var folderNames []string
	for _, c := range components {
		folderNames = append(folderNames, c.FolderName)
	}

	// =========================================================
	// TAB 1: TWEAKS
	// =========================================================
	var tTweaks []Graphite.Widget
	colList := Graphite.NewPanel(0, 2, 0, 0)
	colList.SetPercentLayout(0, 0, 35, 90)

	colAction := Graphite.NewPanel(0, 2, 0, 0)
	colAction.SetPercentLayout(40, 0, 55, 90)

	lblCompName := Graphite.NewLabel(0, 0, "Select a component...")
	lblCompVer := Graphite.NewLabel(0, 2, "Version: -")
	lblCompAuth := Graphite.NewLabel(0, 3, "Author: -")
	lblCompAdmin := Graphite.NewLabel(0, 4, "")

	descArea := Graphite.NewTextArea(0, 6, 0, 8)
	descArea.SetEnabled(false)
	descArea.SetText("Description will appear here.")

	var selectedExePath string
	var selectedCompName string

	btnRun := Graphite.NewButton(0, 15, "Execute Component", Graphite.BtnSuccess, func() {
		if selectedExePath != "" {
			streamComponent(app, selectedExePath, selectedCompName)
		}
	})
	btnRun.SetEnabled(false)

	colAction.AddWidget(lblCompName)
	colAction.AddWidget(lblCompVer)
	colAction.AddWidget(lblCompAuth)
	colAction.AddWidget(lblCompAdmin)
	colAction.AddWidget(descArea)
	colAction.AddWidget(btnRun)

	list := Graphite.NewListBox(0, 0, 0, 20, folderNames, nil)

	if len(folderNames) == 0 {
		lblCompName.SetText("No components found in ./components/")
	}
	colList.AddWidget(list)
	tTweaks = append(tTweaks, colList, colAction)

	// =========================================================
	// DYNAMIC UI UPDATE CALLBACK
	// =========================================================
	lastSelectedIdx := -1
	app.SetIdleCallback(func() {
		if list.Selected != lastSelectedIdx && list.Selected >= 0 && list.Selected < len(components) {
			lastSelectedIdx = list.Selected
			c := components[list.Selected]

			lblCompName.SetText("Name: " + c.Manifest.Name)
			lblCompVer.SetText("Version: " + c.Manifest.Version)
			lblCompAuth.SetText("Author: " + c.Manifest.Author)

			if c.Manifest.NeedsAdmin {
				lblCompAdmin.SetText("[!] REQUIRES ADMINISTRATOR PRIVILEGES")
			} else {
				lblCompAdmin.SetText("Runs as standard user")
			}
			descArea.SetText(c.Manifest.Description)

			absPath, _ := filepath.Abs(c.Path)
			selectedExePath = filepath.Join(absPath, c.Manifest.Executable)
			selectedCompName = c.Manifest.Name

			btnRun.SetEnabled(true)
		}
	})

	// =========================================================
	// TABS 2 & 3
	// =========================================================
	var tParams []Graphite.Widget
	pnlParams := Graphite.NewPanel(2, 2, 0, 0)
	pnlParams.SetPercentLayout(0, 0, 90, 90)
	pnlParams.AddWidget(Graphite.NewCheckbox(0, 0, "Run launcher with Admin privileges", false))
	pnlParams.AddWidget(Graphite.NewCheckbox(0, 2, "Check for updates on startup", true))
	tParams = append(tParams, pnlParams)

	var tVersion []Graphite.Widget
	pnlVersion := Graphite.NewPanel(2, 2, 0, 0)
	pnlVersion.SetPercentLayout(0, 0, 90, 90)
	pnlVersion.AddWidget(Graphite.NewLabel(0, 0, "Launcher Version: v3.0.0-STREAMING-FIX"))
	tVersion = append(tVersion, pnlVersion)

	navTabs.AddTab("Tweaks", tTweaks)
	for _, w := range tTweaks {
		win.AddWidget(w)
	}
	navTabs.AddTab("Parameters", tParams)
	for _, w := range tParams {
		win.AddWidget(w)
	}
	navTabs.AddTab("Version", tVersion)
	for _, w := range tVersion {
		win.AddWidget(w)
	}

	app.SetWindow(win)
	app.Run()
}
