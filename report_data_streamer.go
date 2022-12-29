package main

import (
	"context"

	log "github.com/sirupsen/logrus"

	gabs "github.com/Jeffail/gabs/v2"
)

// When we receive a report in our websocket pipeline this is the initial struct
// accountId and cwAccountId originate in cartwheel, not from CLAPI
type ReportDataStreamV1 struct {
	reportType     string
	reportDataType string
	json           *gabs.Container
}

// Methods to convert raw ReportDataStreamV1 into more specific types (ie TransponderReportDataStreamV1)
func (r *ReportDataStreamV1) transponderReportDataStreamV1() (tr TransponderReportDataStreamV1) {
	tr.reportType = r.reportType
	tr.reportDataType = r.reportDataType
	tr.TransponderReportDataV1.json = r.json
	return tr
}
func (r *ReportDataStreamV1) videoReportDataStreamV1() (vr VideoReportDataStreamV1) {
	vr.reportType = r.reportType
	vr.reportDataType = r.reportDataType
	vr.VideoReportDataV1.json = r.json
	return vr
}
func (r *ReportDataStreamV1) eldReportDataStreamV1() (vr EldReportDataStreamV1) {
	vr.reportType = r.reportType
	vr.reportDataType = r.reportDataType
	vr.EldReportDataV1.json = r.json
	return vr
}

// once in the processing pipeline we identify the data being worked with
// and use specific embedded types for our json *gabs.Container, giving us
// methods specific to the kind of report data we are looking for
type TransponderReportDataStreamV1 struct {
	reportType     string
	reportDataType string
	cwAccountId    string
	cwDeviceWebId  string
	TransponderReportDataV1
}
type EldReportDataStreamV1 struct {
	reportType     string
	reportDataType string
	cwAccountId    string
	cwDeviceWebId  string
	clUserId       string
	EldReportDataV1
}
type VideoReportDataStreamV1 struct {
	reportType     string
	reportDataType string
	cwAccountId    string
	cwDeviceWebId  string
	VideoReportDataV1
}

// types of report data we receive thru the websocket
// REPORT_DATA_EVENT_TYPE, ELD_RECORD_TYPE, VIDEO_EVENT_TYPE
// these are embedded in a higher-level structure, giving
// us methods to use specific to report types
type TransponderReportDataV1 struct {
	json *gabs.Container
}
type EldReportDataV1 struct {
	json *gabs.Container
}
type VideoReportDataV1 struct {
	json *gabs.Container
}

// 1. Determine what kind if data we are working with.
// 2. Transfer data into a more specifically scoped structure.
// 3. Send new struct into pipeline
func firestoreAssemblyRouter(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// we've been instructed to return
			return
		default:
			// route firestoreAssembly channel messages based on report type/meta info
			rds := <-firestoreAssembly
			log.Debugf("Assembly router received a %s:%s report...\n", rds.reportType, rds.reportDataType)
			switch {
			case rds.reportType == "REPORT_DATA":
				log.Debugf("firestoreAssemblyRouter() pushing into transponderReportsV1 channel")
				transponderReportsV1 <- rds.transponderReportDataStreamV1()
			//case rds.reportType == "video_upload": // Disabled for now https://track.carmasys.com:8343/browse/FIRE-2
			//log.Debugf("firestoreAssemblyRouter() pushing into videoReportsV1 channel")
			//videoReportsV1 <- rds.videoReportDataStreamV1()
			case rds.reportType == "ELD_RECORD":
				log.Debugf("firestoreAssemblyRouter() pushing into eldReportsV1 channel")
				eldReportsV1 <- rds.eldReportDataStreamV1()
			default:
				// do not process other report types for now but log them for visibility
				log.Infof("Assembly router received an unhandled (type:dataType) (%s:%s) report: %s", rds.reportType, rds.reportDataType, rds.json.String())
			}
		}
	}
}

// Pub/sub message router, ...
