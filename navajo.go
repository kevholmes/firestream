package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Jeffail/gabs/v2"
)

type NavajoAuthConfig struct {
	host string
	user string
	pass string
}

type NavajoAccountData struct {
	// clApiAccountId:cwAccountId
	clAccountIdMap map[string]string
	// clApiDeviceId:cwDeviceId
	clDeviceIdMap map[string]string
	// track how many times we've updated our internal maps
	successfulUpdates int
	failedUpdates     int
	mutex             sync.Mutex
}

// run in background, firing every N seconds
// queries navajo devices and accounts endpoints
// to build a clApi:cwApi map of like-minded ids
func keepNavajoIdMapsUpdated(ctx context.Context) {
	ticker := time.NewTicker(navajoRebuildTimer)
	for {
		select {
		case <-ticker.C:
			log.Debugf("Automatic re-cache of Navajo data in progress...\n")
			buildNavajoIdMaps() // run for all accounts
		case <-navajoUpdater:
			log.Debugf("Requested re-cache of Navajo data in progress\n")
			buildNavajoIdMaps() // run for all accounts
		case <-ctx.Done():
			log.Debugln("KeepNavajoIdMapsUpdated(): context.Done() received")
			ticker.Stop()
			return
		}
	}
}

// make requests to navajo api, build deviceId and accountId maps from response
func buildNavajoIdMaps() {
	// mutex for our global navajo ref map
	navajoReferenceIds.mutex.Lock()
	defer navajoReferenceIds.mutex.Unlock()

	// get device data out of navajo
	devicesEndpoint := "/v1/devices"
	dResp, err := navajoHttpClient(devicesEndpoint)
	if err != nil {
		log.Warnf("Building Navajo ID maps not successful, unable to make request to server: %s\n", err)
		navajoReferenceIds.failedUpdates++
		return
	}
	dJson, err := gabs.ParseJSONBuffer(dResp.Body)
	for _, child := range dJson.Children() {
		state, stateOk := child.Path("state.state").Data().(string)
		webIdf, webIdOk := child.Path("webId").Data().(float64)
		traIdf, traIdOk := child.Path("currentTransponder.transponderId").Data().(float64)

		// convert webid, transponder id from float64 to strings for use in our map
		// determine what's active / valid and assign it to our in-memory map
		if state == "DEACTIVATED" {
			continue // it's archived, move on
		} else if stateOk && webIdOk && traIdOk && state == "ACTIVE" {
			webId := fmt.Sprintf("%.0f", webIdf)
			traId := fmt.Sprintf("%.0f", traIdf)
			// add to our map
			navajoReferenceIds.clDeviceIdMap[traId] = webId
		} else {
			log.Warnf("Encountered bad data when building Navajo ID Device maps, incomplete data: state:%t, webId:%t, or transponderId:%t", stateOk, webIdOk, traIdOk)
			continue
		}
	}

	// get account data out of navajo
	accountsEndpoint := "/v1/accounts"
	aResp, err := navajoHttpClient(accountsEndpoint)
	if err != nil {
		log.Warnf("Building Navajo ID maps not successful, unable to make request to server: %s\n", err)
		navajoReferenceIds.failedUpdates++
		return
	}
	aJson, err := gabs.ParseJSONBuffer(aResp.Body)
	for _, child := range aJson.Children() {
		cwAcctIdf, cwAcctIdOk := child.Search("accountId").Data().(float64)
		clAcctIdf, clAcctIdOk := child.Search("apiId").Data().(float64)
		// cwAcctId can't be nil, but clAcctId might be? validate
		if cwAcctIdOk && clAcctIdOk {
			cwAcctId := fmt.Sprintf("%.0f", cwAcctIdf)
			clAcctId := fmt.Sprintf("%.0f", clAcctIdf)
			// add to map
			navajoReferenceIds.clAccountIdMap[clAcctId] = cwAcctId
		} else if !clAcctIdOk && cwAcctIdOk {
			log.Warnf("This Navajo account doesn't have a valid CL API ID: %v", cwAcctIdf)
			continue
		}
	}
	log.Debugf("Navajo ID mapping updated successfully\n")
	navajoReferenceIds.successfulUpdates++
	return
}

// set up our navajo api client and pass it back
func navajoHttpClient(endpoint string) (*http.Response, error) {
	client := &http.Client{}
	limit := "?limit=10000"
	// append our request endpoint and params to our host
	url := navajoAuthConf.host + endpoint + limit
	// build request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("Unable to create http client for navajo request: %s\n", err)
		return nil, err
	}
	// insert our http basic auth credentials provided as env vars
	req.SetBasicAuth(navajoAuthConf.user, navajoAuthConf.pass)

	// send out request
	resp, err := client.Do(req)
	if err != nil {
		return &http.Response{}, err
	}
	if resp.StatusCode != 200 {
		log.Errorln("Navajo API returning non-200 responses!")
		errContext := fmt.Sprintf("ERROR: Navajo API response code: %v", resp.StatusCode)
		return resp, errors.New(errContext)
	}
	return resp, nil
}
