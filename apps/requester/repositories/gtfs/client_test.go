package gtfs

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func feedArchive(files map[string]string) []byte {
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := archive.Create(name)
		Expect(err).NotTo(HaveOccurred())
		_, err = w.Write([]byte(content))
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(archive.Close()).To(Succeed())
	return buf.Bytes()
}

func newTestClient(status int, body []byte) *Client {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write(body)
	}))
	DeferCleanup(server.Close)

	return NewClient(ClientConfig{FeedURL: server.URL, HTTPClient: server.Client()})
}

var validFeed = map[string]string{
	fileRoutes: "route_id,agency_id,route_short_name\n" +
		"r12,a,12\n",
	fileStops: byteOrderMark + "stop_id,stop_code,stop_name\n" +
		"s1,1200,MSC\n" +
		"s2,1202,Lincoln\n",
	fileTrips: "route_id,service_id,trip_id\n" +
		"r12,weekday,t1\n",
	fileStopTimes: "trip_id,arrival_time,departure_time,stop_id,stop_sequence,timepoint\n" +
		"t1,07:54:00,07:54:00,s1,0,1\n" +
		"t1,07:55:18,07:55:18,s2,1,0\n",
}

var _ = Describe("Client", Label("unit"), func() {
	ctx := context.Background()

	It("reads columns by header name", func() {
		client := newTestClient(http.StatusOK, feedArchive(validFeed))

		feed, err := client.GetFeed(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(feed.Routes).To(Equal([]Route{{RouteID: "r12", ShortName: "12"}}))
		Expect(feed.Stops).To(Equal([]Stop{{StopID: "s1", StopCode: "1200"}, {StopID: "s2", StopCode: "1202"}}))
		Expect(feed.Trips).To(Equal([]Trip{{TripID: "t1", RouteID: "r12"}}))
		Expect(feed.StopTimes).To(Equal([]StopTime{
			{TripID: "t1", StopID: "s1", Timepoint: true},
			{TripID: "t1", StopID: "s2", Timepoint: false},
		}))
	})

	It("fails when a required file is missing", func() {
		files := map[string]string{fileRoutes: validFeed[fileRoutes]}
		client := newTestClient(http.StatusOK, feedArchive(files))

		_, err := client.GetFeed(ctx)
		Expect(err).To(MatchError(ContainSubstring(fileStops)))
	})

	It("fails on a non-2xx response", func() {
		client := newTestClient(http.StatusNotFound, nil)

		_, err := client.GetFeed(ctx)
		Expect(err).To(HaveOccurred())
	})
})

var _ = DescribeTable("isTimepoint",
	func(timepoint, arrivalTime string, expected bool) {
		Expect(isTimepoint(timepoint, arrivalTime)).To(Equal(expected))
	},
	Entry("exact", "1", "07:54:00", true),
	Entry("approximate", "0", "07:55:18", false),
	Entry("blank with a time is exact", "", "07:54:00", true),
	Entry("blank without a time is interpolated", "", "", false),
	Label("unit"),
)
