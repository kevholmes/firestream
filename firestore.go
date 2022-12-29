package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"cloud.google.com/go/firestore"
	"google.golang.org/genproto/googleapis/type/latlng"
)

// Transponder generated reports (speeding, status, hard_accel, ...)
type FirestoreTransponderReportV1 struct {
	ConfigId           float64        `firestore:"configId,omitempty"`
	Duration           float64        `firestore:"duration,omitempty"`
	EventStart         time.Time      `firestore:"eventStart,omitempty"`
	InProgress         bool           `firestore:"inProgress,omitempty"`
	LocationAccuracy   float64        `firestore:"locationAccuracy,omitempty"`
	Heading            float64        `firestore:"heading,omitempty"`
	Address            string         `firestore:"address,omitempty"`
	DotOrientation     string         `firestore:"dotOrientation,omitempty"`
	LatLng             *latlng.LatLng `firestore:"latLng,omitempty"`
	BatteryVoltage     float64        `firestore:"batteryVoltage,omitempty"`
	CellSignalStrength float64        `firestore:"cellSignalStrength,omitempty"`
	IsLowBattery       bool           `firestore:"isLowBatteryVoltage,omitempty"`
	Odometer           float64        `firestore:"odometer,omitempty"`
	Speed              float64        `firestore:"speed,omitempty"`
	SpeedLimit         float64        `firestore:"speedLimit,omitempty"`
	ReportTimestamp    time.Time      `firestore:"reportTimestamp,omitempty"`
	Serial             float64        `firestore:"serial,omitempty"`
	Type               string         `firestore:"type"`                              // this is the "dataType" field of a streaming packet
	FirestoreCreation  time.Time      `firestore:"fsCreateTimestamp,serverTimestamp"` // if zero, Firestore sets this on their end
	GeoTags            []GeoTagV1     `firestore:"geoTags,omitempty"`                 // omit this whole object if nothing is here
}
type GeoTagV1 struct {
	GeoZoneId float64   `firestore:"zoneId"`                // unique ref to a Geo Zone ID, this updates if changes are made to a tag in CLAPI
	TagName   string    `firestore:"tagName"`               // name of Geo zone container report is within, geoZoneId can change
	TagSource string    `firestore:"scope"`                 // source of tag (account, global, ...)
	Timestamp time.Time `firestore:"zoneModifiedTimestamp"` // timestamp GeoTagId was last modified (boundary change etc.)
}

// ELD reports
type FirestoreEldReportV1 struct {
	UsDotNumber         string         `firestore:"usDotNumber,omitempty"`
	UserId              float64        `firestore:"userId,omitempty"`
	Username            string         `firestore:"userName,omitempty"`
	TransponderId       float64        `firestore:"transponderId,omitempty"`
	TerminalNumber      string         `firestore:"terminalNumber,omitempty"`
	ServerRxTimestamp   time.Time      `firestore:"serverRxTimestamp,omitempty"`
	EventId             string         `firestore:"eventId,omitempty"`
	RecordId            string         `firestore:"recordId,omitempty"`
	RecordTimestamp     time.Time      `firestore:"recordTimestamp,omitempty"`
	RecordStatus        string         `firestore:"recordStatus,omitempty"`
	RecordOrigin        string         `firestore:"recordOrigin,omitempty"`
	EventStartTimestamp time.Time      `firestore:"eventStartTimestamp,omitempty"`
	EventEndTimestamp   time.Time      `firestore:"eventEndTimestamp,omitempty"`
	NavigationEvent     string         `firestore:"navigationEvent,omitempty"`
	VehicleMode         string         `firestore:"vehicleMode,omitempty"`
	LocationType        string         `firestore:"locationType,omitempty"`
	Location            *latlng.LatLng `firestore:"location,omitempty"`
	GeoDescription      string         `firestore:"geoDescription,omitempty"`
	Meters              float64        `firestore:"meters,omitempty"`
	IsDiagnosticActive  bool           `firestore:"isDiagnosticActive,omitempty"`
	IsMalfunctionActive bool           `firestore:"isMalfunctionActive,omitempty"`
	Type                string         `firestore:"type"`                              // this is the "dataType" field of a streaming packet
	FirestoreCreation   time.Time      `firestore:"fsCreateTimestamp,serverTimestamp"` // time document was created in Firestore
}

// Dashcamera reports, triggered by transponder, driver upload, or requested footage
type FirestoreVideoReportV1 struct {
	VideoEventId      string         `firestore:"videoEventId,omitempty"`
	TerminalNumber    string         `firestore:"terminalNumber,omityempty"`
	TransponderId     float64        `firestore:"transponderId,omitempty"`
	EventTimestamp    time.Time      `firestore:"eventTimestamp,omitempty"`
	Username          string         `firestore:"username,omitempty"`
	EventType         []string       `firestore:"eventType,omitEmpty"` // is this correct?
	Location          *latlng.LatLng `firestore:"location,omitempty"`
	LocationAccuracy  float64        `firestore:"locationAccuracy,omitempty"`
	Speed             float64        `firestore:"speed,omitempty"`
	Heading           float64        `firestore:"heading,omitempty"`
	Type              string         `firestore:"type"`                              // this is the "dataType" field
	FirestoreCreation time.Time      `firestore:"fsCreateTimestamp,serverTimestamp"` // time document was created in Firestore
	Footage           []VideoReportFootageV1
}

// Individual footage element within a video report, one for each camera installed
type VideoReportFootageV1 struct {
	FootageId             string    `firestore:"footageId,omitempty"`
	GarminCameraName      string    `firestore:"garminCameraName"`
	GarminFootageFilename string    `firestore:"garminFootageFilename,omitempty"`
	Preview               bool      `firestore:"isPreview,omitempty"`
	FootageTimestamp      time.Time `firestore:"footageTimestamp,omitempty"`
	FootageDuration       float64   `firestore:"footageDuration,omitempty"`
	FootageAudioIncluded  bool      `firestore:"footageAudioIncluded,omitempty"`
	Size                  float64   `firestore:"size,omitempty"`
	Md5                   float64   `firestore:"md5,omitempty"`
	Crc32c                float64   `firestore:"crc32c,omitempty"`
	ContentType           string    `firestore:"contentType,omitempty"`
	FootagePath           string    `firestore:"footagePrivatePath,omitempty"`
	UploadStatus          string    `firestore:"uploadStatus,omitempty"`
	UploadStartTime       time.Time `firestore:"uploadStartTimestamp,omitempty"`
	LastChunkRxTime       time.Time `firestore:"uploadLastChunkRxTimestamp,omitempty"`
	BytesRemaining        float64   `firestore:"bytesRemaining,omitempt,omitemptyy"`
	UploadStatusReason    string    `firestore:"uploadStatusDetails,omitempty"`
}

func createFirestoreClient(ctx context.Context) (*firestore.Client, error) {
	client, err := firestore.NewClient(ctx, gcpProjectId)
	if err != nil {
		log.Infof("Failed to create client: %v\n", err)
		return nil, err
	}
	return client, nil
}
