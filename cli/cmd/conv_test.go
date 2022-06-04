package cmd

import (
	"testing"
)

func TestConvJSONtoYAML(t *testing.T) {
	in := `
{
  "widget": {
  "debug": "on",
  "window": {
      "title": "Sample Konfabulator Widget",
      "name": "main_window",
      "width": 500,
      "height": 500
  },
  "image": {
      "src": "Images/Sun.png",
      "name": "sun1",
      "hOffset": 250,
      "vOffset": 250,
      "alignment": "center"
  },
  "text": {
      "data": "Click Here",
      "size": 36,
      "style": "bold",
      "name": "text1",
      "hOffset": 250,
      "vOffset": 100,
      "alignment": "center",
      "onMouseUp": "sun1.opacity = (sun1.opacity / 100) * 90;"
  }
}}`
	exp := `widget:
  debug: on
  image:
    alignment: center
    hOffset: 250.0
    name: sun1
    src: Images/Sun.png
    vOffset: 250.0
  text:
    alignment: center
    data: Click Here
    hOffset: 250.0
    name: text1
    onMouseUp: sun1.opacity = (sun1.opacity / 100) * 90;
    size: 36.0
    style: bold
    vOffset: 100.0
  window:
    height: 500.0
    name: main_window
    title: Sample Konfabulator Widget
    width: 500.0
`
	out := execute(t, convCmd, []byte(in), "-a", "json", "-b", "yaml")
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestConvJSONtoTOML(t *testing.T) {
	in := `
{
  "widget": {
  "debug": "on",
  "window": {
      "title": "Sample Konfabulator Widget",
      "name": "main_window",
      "width": 500,
      "height": 500
  },
  "image": {
      "src": "Images/Sun.png",
      "name": "sun1",
      "hOffset": 250,
      "vOffset": 250,
      "alignment": "center"
  },
  "text": {
      "data": "Click Here",
      "size": 36,
      "style": "bold",
      "name": "text1",
      "hOffset": 250,
      "vOffset": 100,
      "alignment": "center",
      "onMouseUp": "sun1.opacity = (sun1.opacity / 100) * 90;"
  }
}}`
	exp := `[widget]
debug = 'on'
[widget.image]
alignment = 'center'
hOffset = 250.0
name = 'sun1'
src = 'Images/Sun.png'
vOffset = 250.0

[widget.text]
alignment = 'center'
data = 'Click Here'
hOffset = 250.0
name = 'text1'
onMouseUp = 'sun1.opacity = (sun1.opacity / 100) * 90;'
size = 36.0
style = 'bold'
vOffset = 100.0

[widget.window]
height = 500.0
name = 'main_window'
title = 'Sample Konfabulator Widget'
width = 500.0


`
	out := execute(t, convCmd, []byte(in), "-a", "json", "-b", "toml")
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}

func TestConvYAMLtoJSON(t *testing.T) {
	in := `widget:
  debug: on
  image:
    alignment: center
    hOffset: 250.0
    name: sun1
    src: Images/Sun.png
    vOffset: 250.0
  text:
    alignment: center
    data: Click Here
    hOffset: 250.0
    name: text1
    onMouseUp: sun1.opacity = (sun1.opacity / 100) * 90;
    size: 36.0
    style: bold
    vOffset: 100.0
  window:
    height: 500.0
    name: main_window
    title: Sample Konfabulator Widget
    width: 500.0
`
	exp := `{"widget":{"debug":"on","image":{"alignment":"center","hOffset":250,"name":"sun1","src":"Images/Sun.png","vOffset":250},"text":{"alignment":"center","data":"Click Here","hOffset":250,"name":"text1","onMouseUp":"sun1.opacity = (sun1.opacity / 100) * 90;","size":36,"style":"bold","vOffset":100},"window":{"height":500,"name":"main_window","title":"Sample Konfabulator Widget","width":500}}}`
	out := execute(t, convCmd, []byte(in), "-a", "yaml", "-b", "json")
	if string(out) != exp {
		t.Fatalf("expecting \"%s\", got \"%s\"", exp, string(out))
	}
}
