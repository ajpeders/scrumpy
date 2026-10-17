package viz

import "testing"

func TestDiscoverTargetOnlyCider(t *testing.T) {
	const dump = `[
 {"type":"PipeWire:Interface:Metadata","metadata":[{"subject":0,"key":"default.audio.sink","value":{"name":"alsa_output.speakers"}}]},
 {"id":42,"type":"PipeWire:Interface:Node","info":{"props":{"node.name":"alsa_output.speakers","media.class":"Audio/Sink"}}},
 {"id":83,"type":"PipeWire:Interface:Node","info":{"props":{"application.name":"Cider","media.class":"Stream/Output/Audio","object.serial":12001}}}
]`
	if got := discoverTarget([]byte(dump)); got != "12001" {
		t.Fatalf("stream target = %q", got)
	}
	// Real Cider (Electron) stream: application.name is "Chromium", the binary
	// is what identifies it.
	const electron = `[
 {"type":"PipeWire:Interface:Node","info":{"props":{"application.name":"Chromium","application.process.binary":"Cider","media.class":"Stream/Output/Audio","object.serial":2307}}}
]`
	if got := discoverTarget([]byte(electron)); got != "2307" {
		t.Fatalf("electron cider target = %q", got)
	}
	// A non-Cider Chromium stream must not be captured.
	const otherChromium = `[
 {"type":"PipeWire:Interface:Node","info":{"props":{"application.name":"Chromium","application.process.binary":"chrome","media.class":"Stream/Output/Audio","object.serial":2308}}}
]`
	if got := discoverTarget([]byte(otherChromium)); got != "" {
		t.Fatalf("unexpected chromium target = %q", got)
	}
	// Without a Cider stream nothing is recorded: targeting the default sink
	// makes pw-record fall back to the microphone.
	const sinkOnly = `[
 {"type":"PipeWire:Interface:Metadata","metadata":[{"subject":0,"key":"default.audio.sink","value":{"name":"alsa_output.speakers"}}]},
 {"id":42,"type":"PipeWire:Interface:Node","info":{"props":{"node.name":"alsa_output.speakers","media.class":"Audio/Sink"}}}
]`
	if got := discoverTarget([]byte(sinkOnly)); got != "" {
		t.Fatalf("sink-only target = %q, want none", got)
	}
	if got := discoverTarget([]byte(`[{"id":99,"type":"PipeWire:Interface:Node","info":{"props":{"application.name":"Cider","media.class":"Audio/Source"}}}]`)); got != "" {
		t.Fatalf("unexpected source target = %q", got)
	}
}
