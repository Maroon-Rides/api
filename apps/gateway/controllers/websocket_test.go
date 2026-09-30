package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx/fxtest"

	"github.com/MaroonRides/api/apps/gateway/controllers"
	"github.com/MaroonRides/api/apps/gateway/dtos"
	"github.com/MaroonRides/api/apps/gateway/services"
)

func dialWebsocket(server *httptest.Server) *websocket.Conn {
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	DeferCleanup(conn.Close)
	return conn
}

var _ = Describe("WebsocketController", Label("unit"), func() {
	It("answers a ping with a pong", func() {
		server := fuego.NewServer()
		svc := services.NewWebsocketService(fxtest.NewLifecycle(GinkgoT()), nil)
		controllers.NewWebsocketController(svc).Register(fuego.Group(server, "/api"))
		httpServer := httptest.NewServer(server.Mux)
		DeferCleanup(httpServer.Close)
		conn := dialWebsocket(httpServer)

		Expect(conn.WriteJSON(dtos.WebsocketClientMessage{Type: dtos.WebsocketMessageTypes.Ping})).To(Succeed())

		var reply dtos.WebsocketPongMessage
		Expect(conn.ReadJSON(&reply)).To(Succeed())
		Expect(reply.Type).To(Equal(dtos.WebsocketMessageTypes.Pong))
	})
})

var _ = Describe("websocket client", Label("unit"), func() {
	It("is dropped instead of blocking once its outbox is full", func() {
		accepted := make(chan *websocket.Conn, 1)
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			Expect(err).NotTo(HaveOccurred())
			accepted <- conn
		}))
		DeferCleanup(httpServer.Close)
		dialWebsocket(httpServer)

		// No write loop runs, so nothing drains the outbox.
		client := controllers.NewWebsocketClient(<-accepted)
		for range controllers.ClientOutboxSize {
			Expect(client.Send("queued")).To(Succeed())
		}

		Expect(client.Send("overflow")).To(MatchError(controllers.ErrClientTooSlow))
		Expect(client.Send("after")).To(MatchError(controllers.ErrClientClosed))
	})
})
