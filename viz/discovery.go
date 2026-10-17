package viz

import (
	"encoding/json"
	"strconv"
	"strings"
)

type pipeObject struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Info struct {
		Props map[string]json.RawMessage `json:"props"`
	} `json:"info"`
}

func property(props map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(props[key], &value)
	return value
}

// discoverTarget returns Cider's playback stream serial, or "" when Cider has
// no stream (it removes it some time after pausing). There is deliberately no
// fallback: pw-record given a sink as --target records the default source,
// which on a headset is the microphone.
func discoverTarget(data []byte) string {
	var objects []pipeObject
	if json.Unmarshal(data, &objects) != nil {
		return ""
	}
	for _, object := range objects {
		if object.Type != "PipeWire:Interface:Node" || property(object.Info.Props, "media.class") != "Stream/Output/Audio" {
			continue
		}
		app := property(object.Info.Props, "application.name")
		// Cider is Electron: its audio stream reports application.name
		// "Chromium" but application.process.binary "Cider". Match the binary
		// first, then the name, so other Chromium apps are not captured.
		binary := property(object.Info.Props, "application.process.binary")
		if !strings.Contains(strings.ToLower(binary), "cider") &&
			!strings.Contains(strings.ToLower(app), "cider") {
			continue
		}
		if serial := object.Info.Props["object.serial"]; len(serial) > 0 {
			var number json.Number
			if json.Unmarshal(serial, &number) == nil && number.String() != "0" {
				return number.String()
			}
		}
		if object.ID > 0 {
			return strconv.Itoa(object.ID)
		}
	}
	return ""
}
