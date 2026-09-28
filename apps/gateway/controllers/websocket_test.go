package controllers

import (
	"testing"

	"github.com/zishang520/socket.io/v2/socket"
)

func TestRouteIDFromRoom(t *testing.T) {
	t.Run("route room", func(t *testing.T) {
		routeID, ok := routeIDFromRoom(routeRoom("abc"))
		if !ok || routeID != "abc" {
			t.Fatalf("got (%q, %v), want (\"abc\", true)", routeID, ok)
		}
	})

	t.Run("socket id room is ignored", func(t *testing.T) {
		if _, ok := routeIDFromRoom(socket.Room("I4IMp5ciJylqwgAAAAAAAAAA")); ok {
			t.Fatal("socket id room was treated as a route room")
		}
	})
}
