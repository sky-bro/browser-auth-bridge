package nativehost

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/sky-bro/browser-auth-bridge/bridge"
)

func TestNativeRoundTrip(t *testing.T) {
	want := bridge.Message{Type: "stdin", Session: "", Data: "hello\n"}
	var buffer bytes.Buffer
	if err := WriteMessage(&buffer, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMessage(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
