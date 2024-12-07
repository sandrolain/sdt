package main

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Smart Developer Tools")

	myWindow.Resize(fyne.NewSize(480, 320))

	textAreaIn := widget.NewMultiLineEntry()

	commandsNum := len(commandsMap)
	commandNames := make([]string, commandsNum)
	i := 0
	for k := range commandsMap {
		commandNames[i] = k
		i++
	}

	commandSelect := widget.NewSelect(commandNames, func(value string) {
		log.Println("Select set to", value)
	})
	textAreaOut := widget.NewMultiLineEntry()
	textAreaOut.Disable()

	form := &widget.Form{
		Items: []*widget.FormItem{ // we can specify items in the constructor
			{Text: "Input", Widget: textAreaIn},
			{Text: "Command", Widget: commandSelect},
			{Text: "Output", Widget: textAreaOut},
		},
		OnSubmit: func() { // optional, handle form submission
			val := textAreaIn.Text

			out, err := callCommand(commandSelect.Selected, val)

			if err != nil {
				textAreaOut.Text = fmt.Sprintf("%v\n%v", out, err.Error())
			} else {
				textAreaOut.Text = out
			}

			textAreaOut.Refresh()
		},
	}

	myWindow.SetContent(form)
	myWindow.ShowAndRun()
}

type CommandFunc func(in string) (*exec.Cmd, error)

var commandsMap = map[string][]string{
	"Base 64 Encode": {"sdt", "b64"},
	"Base 64 Decode": {"sdt", "b64", "dec"},
}

func callCommand(command string, in string) (string, error) {
	args, ok := commandsMap[command]
	if !ok {
		return "", fmt.Errorf("invalid command")
	}

	//#nosec G204 -- implementation of generic utility
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(in)
	var out bytes.Buffer
	cmd.Stdout = &out

	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	err := cmd.Run()
	if err != nil {
		return errOut.String(), err
	}
	return out.String(), nil
}
