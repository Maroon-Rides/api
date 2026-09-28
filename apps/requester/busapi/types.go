package busapi

import "encoding/json"

// Auth is the set of headers that authenticate a request, typically a session cookie.
type Auth map[string]string

type MapPattern struct {
	Key       string `json:"key"`
	IsDisplay bool   `json:"isDisplay"`
}

type MapDirection struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type MapDirectionList struct {
	Direction               MapDirection `json:"direction"`
	Destination             string       `json:"destination"`
	LineColor               string       `json:"lineColor"`
	TextColor               string       `json:"textColor"`
	PatternList             []MapPattern `json:"patternList"`
	ServiceInterruptionKeys []int        `json:"serviceInterruptionKeys"`
}

type MapStop struct {
	Name     string `json:"name"`
	StopCode string `json:"stopCode"`
	StopType int    `json:"stopType"`
}

type MapPatternPoint struct {
	Key       string   `json:"key"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Stop      *MapStop `json:"stop"`
}

type MapPatternPath struct {
	PatternKey    string            `json:"patternKey"`
	DirectionKey  string            `json:"directionKey"`
	PatternPoints []MapPatternPoint `json:"patternPoints"`
	SegmentPaths  []json.RawMessage `json:"segmentPaths"`
}

type MapRoute struct {
	Key           string             `json:"key"`
	Name          string             `json:"name"`
	ShortName     string             `json:"shortName"`
	DirectionList []MapDirectionList `json:"directionList"`
}

type MapServiceInterruption struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	TimeRangeString string `json:"timeRangeString"`
	StartDateUtc    string `json:"startDateUtc"`
	EndDateUtc      string `json:"endDateUtc"`
	DailyStartTime  string `json:"dailyStartTime"`
	DailyEndTime    string `json:"dailyEndTime"`
}

type DepartureTime struct {
	EstimatedDepartTimeUtc *string `json:"estimatedDepartTimeUtc"`
	ScheduledDepartTimeUtc *string `json:"scheduledDepartTimeUtc"`
	IsOffRoute             bool    `json:"isOffRoute"`
}

type RouteDirectionTime struct {
	DirectionKey  string          `json:"directionKey"`
	FrequencyInfo json.RawMessage `json:"frequencyInfo,omitempty"`
	NextDeparts   []DepartureTime `json:"nextDeparts"`
	RouteKey      string          `json:"routeKey"`
}

type BusLocation struct {
	Heading     float64 `json:"heading"`
	LastGpsDate string  `json:"lastGpsDate"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Speed       float64 `json:"speed"`
}

type Amenity struct {
	Name     string `json:"name"`
	IconName string `json:"iconName"`
}

type Vehicle struct {
	Amenities         []Amenity   `json:"amenities"`
	DirectionKey      string      `json:"directionKey"`
	DirectionName     string      `json:"directionName"`
	IsExtraTrip       bool        `json:"isExtraTrip"`
	Key               string      `json:"key"`
	Location          BusLocation `json:"location"`
	Name              string      `json:"name"`
	PassengerCapacity int         `json:"passengerCapacity"`
	PassengersOnboard int         `json:"passengersOnboard"`
	RouteKey          string      `json:"routeKey"`
}

type VehicleByDirection struct {
	DirectionKey string    `json:"directionKey"`
	Vehicles     []Vehicle `json:"vehicles"`
}

type NextStopTime struct {
	EstimatedDepartTimeUtc *string `json:"estimatedDepartTimeUtc"`
	ScheduledDepartTimeUtc *string `json:"scheduledDepartTimeUtc"`
	IsOffRoute             bool    `json:"isOffRoute"`
	IsRealtime             bool    `json:"isRealtime"`
}

type TimetableServiceInterruption struct {
	ExternalServiceInterruptionKey string `json:"externalServiceInterruptionKey"`
	ServiceInterruptionName        string `json:"serviceInterruptionName"`
	ServiceInterruptionTimeRange   string `json:"serviceInterruptionTimeRange"`
	IsStopClosed                   bool   `json:"isStopClosed"`
}

type TimetableNearbyStops struct {
	DirectionKey         string                         `json:"directionKey"`
	DirectionName        string                         `json:"directionName"`
	Distance             float64                        `json:"distance"`
	StopCode             string                         `json:"stopCode"`
	StopName             *string                        `json:"stopName"`
	IsTemporary          bool                           `json:"isTemporary"`
	NextStopTimes        []NextStopTime                 `json:"nextStopTimes"`
	FrequencyInfo        json.RawMessage                `json:"frequencyInfo"`
	ServiceInterruptions []TimetableServiceInterruption `json:"serviceInterruptions"`
	Amenities            []Amenity                      `json:"amenities"`
}

type TimetableRoute struct {
	RouteKey       string                 `json:"routeKey"`
	RouteNumber    *string                `json:"routeNumber"`
	RouteName      *string                `json:"routeName"`
	DistanceString *string                `json:"distanceString"`
	Distance       *float64               `json:"distance"`
	NearbyStops    []TimetableNearbyStops `json:"nearbyStops"`
}

type BaseDataResponse struct {
	Routes               []MapRoute               `json:"routes"`
	ServiceInterruptions []MapServiceInterruption `json:"serviceInterruptions"`
}

type PatternPathsResponse struct {
	RouteKey             string               `json:"routeKey"`
	PatternPaths         []MapPatternPath     `json:"patternPaths"`
	VehiclesByDirections []VehicleByDirection `json:"vehiclesByDirections"`
}

type NextDepartureTimesResponse struct {
	Amenities           []Amenity            `json:"amenities"`
	RouteDirectionTimes []RouteDirectionTime `json:"routeDirectionTimes"`
	StopCode            string               `json:"stopCode"`
}

type VehicleResponse struct {
	RouteKey             string               `json:"routeKey"`
	VehiclesByDirections []VehicleByDirection `json:"vehiclesByDirections"`
}

type StopTime struct {
	ScheduledDepartTimeUtc string  `json:"scheduledDepartTimeUtc"`
	EstimatedDepartTimeUtc *string `json:"estimatedDepartTimeUtc"`
	IsRealtime             bool    `json:"isRealtime"`
	TripPointID            string  `json:"tripPointId"`
	IsLastPoint            *bool   `json:"isLastPoint"`
	IsCancelled            bool    `json:"isCancelled"`
	IsOffRoute             bool    `json:"isOffRoute"`
}

type RouteStopSchedule struct {
	RouteName            string          `json:"routeName"`
	RouteNumber          string          `json:"routeNumber"`
	DirectionName        string          `json:"directionName"`
	StopTimes            []StopTime      `json:"stopTimes"`
	FrequencyInfo        json.RawMessage `json:"frequencyInfo"`
	HasTrips             bool            `json:"hasTrips"`
	HasSchedule          bool            `json:"hasSchedule"`
	IsEndOfRoute         bool            `json:"isEndOfRoute"`
	IsTemporaryStopOnly  bool            `json:"isTemporaryStopOnly"`
	IsClosedRegularStop  bool            `json:"isClosedRegularStop"`
	ServiceInterruptions json.RawMessage `json:"serviceInterruptions"`
}

type StopEstimatesResponse struct {
	Amenities          []Amenity           `json:"amenities"`
	Date               string              `json:"date"`
	RouteStopSchedules []RouteStopSchedule `json:"routeStopSchedules"`
}

type StopSchedulesResponse struct {
	Amenities          []Amenity           `json:"amenities"`
	Date               string              `json:"date"`
	RouteStopSchedules []RouteStopSchedule `json:"routeStopSchedules"`
}

type NearbyRoutesResponse struct {
	Longitude           float64           `json:"longitude"`
	Latitude            float64           `json:"latitude"`
	StopCode            json.RawMessage   `json:"stopCode"`
	BusStopRouteResults []json.RawMessage `json:"busStopRouteResults"`
	RouteResults        []TimetableRoute  `json:"routeResults"`
	NextMinRadius       float64           `json:"nextMinRadius"`
	NextMaxRadius       float64           `json:"nextMaxRadius"`
	CanLoadMore         bool              `json:"canLoadMore"`
}

type FoundStop struct {
	StopCode  string  `json:"stopCode"`
	StopName  string  `json:"stopName"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

type MatchedSubstring struct {
	Length int `json:"length"`
	Offset int `json:"offset"`
}

type LocationTerms struct {
	Offset int    `json:"offset"`
	Value  string `json:"value"`
}

type StructuredFormatting struct {
	MainText                  string             `json:"main_text"`
	MainTextMatchedSubstrings []MatchedSubstring `json:"main_text_matched_substrings"`
	SecondaryText             string             `json:"secondary_text"`
}

type FoundLocation struct {
	Description          string               `json:"description"`
	MatchedSubstrings    []MatchedSubstring   `json:"matchedSubstrings"`
	PlaceID              string               `json:"place_id"`
	Reference            string               `json:"reference"`
	StructuredFormatting StructuredFormatting `json:"structured_formatting"`
	Terms                []LocationTerms      `json:"terms"`
	Types                []string             `json:"types"`
}

// Endpoint is one end of a trip: a searched place, a bus stop, or a coordinate.
type Endpoint struct {
	Title     string
	Subtitle  string
	Latitude  *float64
	Longitude *float64
	StopCode  *string
	PlaceID   *string
}

type PlanBlock struct {
	ClassName      string  `json:"className"`
	IconString     string  `json:"iconString"`
	LeftPosition   float64 `json:"leftPosition"`
	RouteShortName string  `json:"routeShortName"`
	StepType       int     `json:"stepType"`
	TopPosition    float64 `json:"topPosition"`
	Width          float64 `json:"width"`
}

type ChartLinePosition struct {
	LeftPosition float64 `json:"leftPosition"`
	TimeLabel    string  `json:"timeLabel"`
}

type OptionBlock struct {
	LeavingIn    string  `json:"leavingIn"`
	LeftPosition float64 `json:"leftPosition"`
	TopPosition  float64 `json:"topPosition"`
	TotalMinute  string  `json:"totalMinute"`
	Width        float64 `json:"width"`
}

type WalkingInstruction struct {
	Index       int    `json:"index"`
	Instruction string `json:"instruction"`
	Polyline    string `json:"polyline"`
}

type InstructionStep struct {
	ClassName           string               `json:"className"`
	Duration            string               `json:"duration"`
	IconClassName       *string              `json:"iconClassName"`
	Instruction         string               `json:"instruction"`
	Latitude            float64              `json:"latitude"`
	Longitude           float64              `json:"longitude"`
	Polyline            string               `json:"polyline"`
	RouteShortName      *string              `json:"routeShortName"`
	StartTime           string               `json:"startTime"`
	StepType            int                  `json:"stepType"`
	WalkingInstructions []WalkingInstruction `json:"walkingInstructions"`
}

type Agency struct {
	AgencyName string  `json:"agencyName"`
	AgencyURL  *string `json:"agencyUrl"`
}

type MapBounds struct {
	NeLatitude  float64 `json:"neLatitude"`
	NeLongitude float64 `json:"neLongitude"`
	SwLatitude  float64 `json:"swLatitude"`
	SwLongitude float64 `json:"swLongitude"`
}

type OptionDetail struct {
	Agencies             []Agency          `json:"agencies"`
	Copyrights           string            `json:"copyrights"`
	EndTime              int64             `json:"endTime"`
	EndTimeText          string            `json:"endTimeText"`
	Instructions         []InstructionStep `json:"instructions"`
	MapBounds            MapBounds         `json:"mapBounds"`
	OptionIndex          int               `json:"optionIndex"`
	StartTime            int64             `json:"startTime"`
	StartTimeText        string            `json:"startTimeText"`
	TotalTime            string            `json:"totalTime"`
	TotalWalkingDistance string            `json:"totalWalkingDistance"`
	TotalWalkingTime     string            `json:"totalWalkingTime"`
	Warnings             []string          `json:"warnings"`
}

type OptionPosition struct {
	OptionIndex   int     `json:"optionIndex"`
	OptionSummary string  `json:"optionSummary"`
	TopPosition   float64 `json:"topPosition"`
}

type TripPlan struct {
	Blocks             []PlanBlock         `json:"blocks"`
	ChartHeight        float64             `json:"chartHeight"`
	ChartLinePositions []ChartLinePosition `json:"chartLinePositions"`
	HeaderHeight       float64             `json:"headerHeight"`
	OptionBlocks       []OptionBlock       `json:"optionBlocks"`
	OptionDetails      []OptionDetail      `json:"optionDetails"`
	OptionHeight       float64             `json:"optionHeight"`
	OptionPositions    []OptionPosition    `json:"optionPositions"`
	ResultCount        int                 `json:"resultCount"`
}
