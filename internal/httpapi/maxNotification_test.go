package httpapi

import "testing"

func TestMaxNotificationRoutePermission(t *testing.T) {
	routes := BuildRoutes()
	for _, route := range routes {
		if route.Name == "max.notifications.send" {
			if route.Permission != "maxNotification.send" {
				t.Fatalf("permission = %q", route.Permission)
			}
			return
		}
	}
	t.Fatal("max.notifications.send route not found")
}
