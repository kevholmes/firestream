package main

import (
	"fmt"
	"math/rand"
	"testing"

	gabs "github.com/Jeffail/gabs/v2"
)

// initialize relevant global variables for usage in our tests
func init() {
	// stream report processing channel for Stream API Reports -> Firestore
	firestoreAssembly = make(chan ReportDataStreamV1)
}

// Look for a "type" and "dataType" object - in our ingested JSON
// We cannot route packets downstream without these values
func TestProcessStreamingJSON(t *testing.T) {
	// spin up goroutine to wait for our assembly channel output
	go func() {
		select {
		case <-firestoreAssembly:
			t.Logf("firestoreAssembly channel received ReportDataStreamV1{}")
		default:
			t.Logf("firestoreAssembly channel waiting for processStreamingJSON() to send us our report...")
		}
	}()
	// set up our test *gabs.Container to pass in as JSON object from websocket
	json := gabs.New()
	json.Set("REPORT_DATA", "type")
	json.Set("status", "dataType")
	if ok := processStreamingJSON(json); !ok {
		t.Errorf("processStreamingJSON(%s) = false", json.String())
	} else {
		t.Logf("processStreamingJSON() = true")
		return
	}
}

func TestWebsocketConnParams(t *testing.T) {
	randChkPtf := float64(rand.Uint32())
	randChkPt := fmt.Sprintf("%.0f", randChkPtf)
	t.Log(randChkPt)
	tests := []struct {
		name    string
		want    string
		request NewWebsocketRequest
	}{
		{name: "no params", want: "", request: NewWebsocketRequest{passiveKeepAlive: false, resumeCheckpoint: false}},
		{name: "passive keep-alive", want: "?keepAlive=passive", request: NewWebsocketRequest{passiveKeepAlive: true, resumeCheckpoint: false}},
		{name: "resume checkpoint", want: "?checkpoint=" + randChkPt, request: NewWebsocketRequest{resumeCheckpoint: true, checkpointToResume: randChkPtf}},
		{name: "resume point and passive", want: "?checkpoint=" + randChkPt + "&" + "keepAlive=passive",
			request: NewWebsocketRequest{resumeCheckpoint: true, passiveKeepAlive: true, checkpointToResume: randChkPtf}},
	}
	for _, tc := range tests {
		got, _, err := tc.request.params()
		if err != nil {
			t.Errorf("NewWebsocketRequest.params() returned err != nil")
		}
		if got != tc.want {
			t.Errorf("NewWebsocketRequest.params() = %s, want: %s", got, tc.want)
		}
	}

}
