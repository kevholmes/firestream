package main

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	gabs "github.com/Jeffail/gabs/v2"
	"github.com/gorilla/websocket"
)

type NewWebsocketRequest struct {
	passiveKeepAlive   bool
	resumeCheckpoint   bool
	checkpointToResume float64
}

var emptyKeepAlive string = "{}"

// track the progress of our report ingestion
type WebsocketIngestionProgress struct {
	latestKeepalive time.Time
	checkpoint      float64
}

// update our checkpoint value from the stream api server
func (p *WebsocketIngestionProgress) updateCheckpoint(j *gabs.Container) (ok bool) {
	point, ok := j.Path("checkpoint").Data().(float64)
	if ok {
		p.checkpoint = point
		return true
	} else {
		log.Errorln("Unable to parse websocket ingestion checkpoint value from report packet!")
		return false
	}
}

// return a sorted, formatted string of url parameters for a CL API Websocket
func (nr *NewWebsocketRequest) params() (string, map[string]string, error) {
	// build optional params to websocket request
	paramKeyMap := make(map[string]string)
	//
	if nr.passiveKeepAlive {
		paramKeyMap["keepAlive"] = "passive"
	}
	if nr.resumeCheckpoint {
		point := fmt.Sprintf("%0.f", nr.checkpointToResume)
		paramKeyMap["checkpoint"] = point
	}

	// golang randomizes range map iterations
	// organize our parameters in a slice so we can test output
	keys := make([]string, len(paramKeyMap))
	t := 0
	for k := range paramKeyMap {
		keys[t] = k
		t++
	}
	sort.Strings(keys)

	// build full string of query parameters
	var b strings.Builder
	len := len(paramKeyMap)
	var i int = 0
	if len == 0 {
		params := b.String()
		return params, paramKeyMap, nil
	} else if len >= 1 {
		b.WriteString("?")
		// range over our sorted slice of param keys, inserting mapped values
		for _, param := range keys {
			i++
			b.WriteString(param + "=" + paramKeyMap[param])
			if i < len {
				b.WriteString("&")
			}
		}
		params := b.String()
		return params, paramKeyMap, nil
	}
	return "", paramKeyMap, errors.New("Unable to process websocket parameters!")
}

func Websocket(nr NewWebsocketRequest) (*websocket.Conn, error) {
	// build websocket parameters for our initial GET request
	params, pkmap, _ := nr.params()
	log.Debugf("websocket optional params() = %v, nil", params)
	// so we build an HTTP GET request here - but only as a helper to build our Authentication header
	req, _ := http.NewRequest("GET", authConf.url+params, nil)
	// assemble Authentication parameters
	oauthParams := oAuthParams()
	// add our optional query parameters to oauthParams for signing purposes
	for k, v := range pkmap {
		oauthParams[k] = v
	}
	signatureBase := signatureBase(req, oauthParams)
	signature, err := hmacSign(authConf.OauthConsumerSecret, OauthTokenSecret, signatureBase, sha1.New)
	if err != nil {
		log.Errorln("Error signing base Oauth1.0a request!")
	}
	// add our signature of the base query to Oauth param collection
	oauthParams[oauthSignatureParam] = signature
	// remove optional params from oauthParams map (otherwise they end up in header)
	for k := range pkmap {
		delete(oauthParams, k)
	}
	// set our auth header in our temporary http request helper
	req.Header.Set(authorizationHeaderParam, authHeaderValue(oauthParams))
	// extract our built headers from the http request helper
	h := req.Header
	// websocket dial now using our websocket URL (instead of http://) and assembled Authentication headers
	log.Debugf("Attempting to open websocket: %v, with headers: %v", authConf.wsUrl+params, h)
	c, resp, err := websocket.DefaultDialer.Dial(authConf.wsUrl+params, h)
	if err != nil {
		log.Errorf("firestream: error opening websocket conn, response code: %v\n", resp)
		return nil, err
	}
	return c, nil
}

func readPump(ws *websocket.Conn, ctx context.Context, progress *WebsocketIngestionProgress) bool {
	// make sure we have a valid websocket to work on to avoid panics
	if ws == nil {
		return false // send us back into originating for loop and try to acquire a valid websocket conn
	}
	// keep our websocket open until function returns or the universe-at-large closes it for us..
	defer func() {
		ws.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			// upstream telling us to bail out
			ws.Close()
			return true
		default:
			// set up websocket read timeout, handler
			keepAliveWait := websocketTimeout                 // global with default, also user configurable
			ws.SetReadDeadline(time.Now().Add(keepAliveWait)) // set our first ws read deadline
			ws.SetPongHandler(func(string) error {            // handle future deadlines
				deadline := time.Now().Add(keepAliveWait)
				log.Debugf("websocket PongHandler() called, ReadDeadline updated to :%v", deadline)
				ws.SetReadDeadline(deadline) //
				return nil
			})
			// read messages until we loose our websocket or are told to quit
			_, message, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Infof("websocket.IsUnexpectedCloseError(): %v\n", err)
				}
				log.Debugf("ws.ReadMessage() = err, %v", err)
				return false
			}
			jsonParsed, err := gabs.ParseJSON(message)
			if err != nil {
				imetrics.unparseableSamples++
				if imetrics.unparseableSamples > maxJSONParseErrors {
					log.Warnf("More than %v errors, restarting connection..\n", maxJSONParseErrors)
					return false
				}
			}

			// look for server's keep-alive sent to us, otherwise process the data
			if jsonParsed.String() == emptyKeepAlive {
				progress.latestKeepalive = now()
				log.Debugf("Websocket keep-alive received: %v", progress.latestKeepalive)
				// send one back
				emptyJson := []byte(emptyKeepAlive)
				err := ws.WriteMessage(1, emptyJson)
				if err != nil {
					log.Errorln("Unable to send server-requested keep-alive back into websocket")
					return false
				}
			} else {
				// look for reports we care about, verify any keys, push into processing pipeline
				ok := processStreamingJSON(jsonParsed)
				if !ok {
					log.Debugf("processStreamingJSON() = false")
				}
				// mark stream checkpoint even if processStreamingJSON rejected the packet (we don't want to replay them .. I assume?)
				progress.updateCheckpoint(jsonParsed)
				log.Debugf("progress checkpoint set: %.0f", progress.checkpoint)
				// collectIngestionMetrics(jsonParsed) // disable these basic metrics, not needed for now
			}
		}
	}
}

func websocketIngestor(mainContext context.Context) {
	readPumpCtx, cancelReadPump := context.WithCancel(mainContext)
	progress := &WebsocketIngestionProgress{}
	nr := NewWebsocketRequest{}
	for {
		select {
		case <-mainContext.Done():
			// main context telling us to return
			cancelReadPump()
			return
		default:
			// ingestion loop
			if progress.checkpoint == 0 {
				log.Debugf("websocketIngestor() has no progress checkpoint...")
				nr.passiveKeepAlive = false
				nr.resumeCheckpoint = false
			} else {
				log.Debugf("resuming websocket checkpoint : %.0f", progress.checkpoint)
				nr.resumeCheckpoint = true
				nr.checkpointToResume = progress.checkpoint
			}
			ws, err := Websocket(nr)
			if err != nil {
				log.Errorln(err)
				log.Error("websocketIngestor() cannot get a websocket... sleeping and retrying...")
				time.Sleep(5 * time.Second)
			} else {
				log.Debugln("websocketIngestor() ready to call readPump() ...")
				ok := readPump(ws, readPumpCtx, progress)
				if !ok {
					log.Warnln("readPump() = false, restarting...")
				}
			}
		}
	}
}

func processStreamingJSON(pj *gabs.Container) bool {
	// we require a report's "type" and "dataType" if we are
	// to do any kind of routing and processing
	reportType, rTypeOk := pj.Path("type").Data().(string)
	reportDataType, rDTypeOk := pj.Path("dataType").Data().(string)

	// validate required items that will be used as keys later
	if !rTypeOk || !rDTypeOk {
		log.Warnf("processStreamingJson(): Report packet doesn't satisfy type/dataType checks: %s\n", pj.String())
		return false
	}
	// build type ReportDataStreamV1 as "rds"
	rds := ReportDataStreamV1{}
	rds.reportType = reportType
	rds.reportDataType = reportDataType
	rds.json = pj // json report packet

	// send into firestoreAssembly pipeline
	firestoreAssembly <- rds
	// send into other pipelines here (cloud pub/sub, ...)

	return true
}
