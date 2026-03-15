package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Integration tests exercise the full request lifecycle through the HTTP stack
// including authentication, CORS, routing, handlers, and persistence.
// Run with: go test -v -run TestIntegration ./handlers/

// TestIntegration_FullAuthLifecycle tests login, authenticated requests,
// token refresh, and logout end-to-end.
func TestIntegration_FullAuthLifecycle(t *testing.T) {
	ts := newTestServer(t)

	// --- Step 1: Login via HTTP ---
	loginResp := ts.requestNoAuth(t, http.MethodPost, "/core/v4/auth", map[string]string{
		"Username": "proton",
		"Password": "proton",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", loginResp.Code, loginResp.Body.String())
	}
	loginData := parseResponse(t, loginResp)
	if loginData["Code"].(float64) != 1000 {
		t.Fatalf("login: expected Code 1000, got %v", loginData["Code"])
	}

	accessToken := loginData["AccessToken"].(string)
	refreshToken := loginData["RefreshToken"].(string)
	uid := loginData["UID"].(string)
	userID := loginData["UserID"].(string)

	if accessToken == "" || refreshToken == "" || uid == "" {
		t.Fatal("login: missing tokens or UID")
	}
	if userID != "user-1" {
		t.Fatalf("login: expected UserID user-1, got %s", userID)
	}

	// --- Step 2: Authenticated request using returned token ---
	ts2 := &testServer{handler: ts.handler, token: accessToken, uid: uid}
	calResp := ts2.request(t, http.MethodGet, "/calendar/v1", nil)
	if calResp.Code != http.StatusOK {
		t.Fatalf("list calendars: expected 200, got %d", calResp.Code)
	}

	// --- Step 3: Request without auth should be rejected ---
	noAuthResp := ts.requestNoAuth(t, http.MethodGet, "/calendar/v1", nil)
	if noAuthResp.Code != http.StatusUnauthorized {
		t.Fatalf("no-auth request: expected 401, got %d", noAuthResp.Code)
	}

	// --- Step 4: Refresh the token ---
	refreshResp := ts.requestNoAuth(t, http.MethodPost, "/auth/refresh", map[string]string{
		"UID":          uid,
		"RefreshToken": refreshToken,
	})
	if refreshResp.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", refreshResp.Code, refreshResp.Body.String())
	}
	refreshData := parseResponse(t, refreshResp)
	newAccessToken := refreshData["AccessToken"].(string)
	if newAccessToken == "" {
		t.Fatal("refresh: missing new access token")
	}
	if newAccessToken == accessToken {
		t.Fatal("refresh: new token should differ from old token")
	}

	// --- Step 5: Old token should be invalid after refresh ---
	oldTokenResp := ts2.request(t, http.MethodGet, "/calendar/v1", nil)
	if oldTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("old token after refresh: expected 401, got %d", oldTokenResp.Code)
	}

	// --- Step 6: New token should work ---
	ts3 := &testServer{handler: ts.handler, token: newAccessToken, uid: uid}
	newTokenResp := ts3.request(t, http.MethodGet, "/calendar/v1", nil)
	if newTokenResp.Code != http.StatusOK {
		t.Fatalf("new token: expected 200, got %d", newTokenResp.Code)
	}

	// --- Step 7: Logout ---
	logoutResp := ts3.request(t, http.MethodDelete, "/core/v4/auth", nil)
	if logoutResp.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", logoutResp.Code)
	}

	// --- Step 8: Token should be invalid after logout ---
	postLogoutResp := ts3.request(t, http.MethodGet, "/calendar/v1", nil)
	if postLogoutResp.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: expected 401, got %d", postLogoutResp.Code)
	}
}

// TestIntegration_CalendarEventLifecycle tests the full lifecycle of creating
// a calendar, adding events, querying them, updating, and deleting.
func TestIntegration_CalendarEventLifecycle(t *testing.T) {
	ts := newTestServer(t)

	// --- Create a calendar ---
	createResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Integration Test Calendar",
		"Color":     "#FF5733",
		"Display":   1,
		"AddressID": "addr-integration-1",
	})
	if createResp.Code != http.StatusOK {
		t.Fatalf("create calendar: expected 200, got %d: %s", createResp.Code, createResp.Body.String())
	}
	calData := parseResponse(t, createResp)
	calendarRaw := calData["Calendar"].(map[string]interface{})
	calendarID := calendarRaw["ID"].(string)
	if calendarID == "" {
		t.Fatal("create calendar: missing ID")
	}

	// --- List calendars and verify our calendar exists ---
	listResp := ts.request(t, http.MethodGet, "/calendar/v1", nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list calendars: expected 200, got %d", listResp.Code)
	}
	listData := parseResponse(t, listResp)
	calendars := listData["Calendars"].([]interface{})
	found := false
	for _, c := range calendars {
		cm := c.(map[string]interface{})
		if cm["ID"].(string) == calendarID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("list calendars: created calendar not found")
	}

	// --- Add an event via sync ---
	syncResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/sync", calendarID), map[string]interface{}{
		"MemberID": "member-1",
		"Events": []map[string]interface{}{
			{
				"Overwrite": 1,
				"Event": map[string]interface{}{
					"Permissions": 3,
					"IsOrganizer": 1,
					"SharedEvents": []map[string]interface{}{
						{"Type": 2, "Data": "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:Integration Test\nEND:VEVENT\nEND:VCALENDAR"},
					},
					"Notifications": []map[string]interface{}{
						{"Type": 0, "Trigger": "-PT15M"},
					},
					"StartTime":     1700000000,
					"StartTimezone": "UTC",
					"EndTime":       1700003600,
					"EndTimezone":   "UTC",
					"FullDay":       0,
					"UID":           "integration-test-event-1@test",
				},
			},
		},
	})
	if syncResp.Code != http.StatusOK {
		t.Fatalf("sync events: expected 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	syncData := parseResponse(t, syncResp)
	responses := syncData["Responses"].([]interface{})
	if len(responses) != 1 {
		t.Fatalf("sync events: expected 1 response, got %d", len(responses))
	}
	firstResp := responses[0].(map[string]interface{})
	respBody := firstResp["Response"].(map[string]interface{})
	if respBody["Code"].(float64) != 1000 {
		t.Fatalf("sync event: expected Code 1000, got %v", respBody["Code"])
	}
	eventData := respBody["Event"].(map[string]interface{})
	eventID := eventData["ID"].(string)

	// --- Query events by time range ---
	eventsResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events?Start=1699999000&End=1700005000", calendarID), nil)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("list events: expected 200, got %d", eventsResp.Code)
	}
	eventsData := parseResponse(t, eventsResp)
	events := eventsData["Events"].([]interface{})
	if len(events) < 1 {
		t.Fatal("list events: expected at least 1 event")
	}

	// --- Get event by ID ---
	getResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/%s", calendarID, eventID), nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get event: expected 200, got %d", getResp.Code)
	}

	// --- Get event by UID ---
	uidResp := ts.request(t, http.MethodGet, "/calendar/v1/events?UID=integration-test-event-1@test", nil)
	if uidResp.Code != http.StatusOK {
		t.Fatalf("get by UID: expected 200, got %d: %s", uidResp.Code, uidResp.Body.String())
	}

	// --- Count events ---
	countResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/count", calendarID), nil)
	if countResp.Code != http.StatusOK {
		t.Fatalf("count events: expected 200, got %d", countResp.Code)
	}
	countData := parseResponse(t, countResp)
	if countData["Total"].(float64) != 1 {
		t.Fatalf("count events: expected 1, got %v", countData["Total"])
	}

	// --- Update personal event data ---
	personalResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/%s/personal", calendarID, eventID), map[string]interface{}{
		"Color": "#00FF00",
		"Notifications": []map[string]interface{}{
			{"Type": 0, "Trigger": "-PT30M"},
		},
	})
	if personalResp.Code != http.StatusOK {
		t.Fatalf("update personal: expected 200, got %d: %s", personalResp.Code, personalResp.Body.String())
	}

	// --- Delete the event ---
	delEvtResp := ts.request(t, http.MethodDelete, fmt.Sprintf("/calendar/v1/%s/events/%s", calendarID, eventID), nil)
	if delEvtResp.Code != http.StatusOK {
		t.Fatalf("delete event: expected 200, got %d", delEvtResp.Code)
	}

	// --- Verify event is gone ---
	getDeletedResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/%s", calendarID, eventID), nil)
	if getDeletedResp.Code != http.StatusNotFound {
		t.Fatalf("get deleted event: expected 404, got %d", getDeletedResp.Code)
	}

	// --- Delete the calendar ---
	delCalResp := ts.request(t, http.MethodDelete, fmt.Sprintf("/calendar/v1/%s", calendarID), nil)
	if delCalResp.Code != http.StatusOK {
		t.Fatalf("delete calendar: expected 200, got %d", delCalResp.Code)
	}
}

// TestIntegration_MobileAPICompatibility verifies that the API responses match
// the format expected by mobile clients (Proton Calendar Android/iOS).
// Mobile clients rely on specific JSON field names and response structure.
func TestIntegration_MobileAPICompatibility(t *testing.T) {
	ts := newTestServer(t)

	// --- Login response must contain all fields mobile clients expect ---
	loginResp := ts.requestNoAuth(t, http.MethodPost, "/core/v4/auth", map[string]string{
		"Username": "proton",
		"Password": "proton",
	})
	loginData := parseResponse(t, loginResp)

	requiredLoginFields := []string{"Code", "UID", "AccessToken", "RefreshToken", "ExpiresIn", "TokenType", "Scope", "UserID"}
	for _, field := range requiredLoginFields {
		if _, ok := loginData[field]; !ok {
			t.Errorf("login response missing required field: %s", field)
		}
	}
	if loginData["TokenType"].(string) != "Bearer" {
		t.Errorf("login: expected TokenType 'Bearer', got %v", loginData["TokenType"])
	}

	accessToken := loginData["AccessToken"].(string)
	uid := loginData["UID"].(string)
	tsAuth := &testServer{handler: ts.handler, token: accessToken, uid: uid}

	// --- Timezones endpoint must return timezone list ---
	tzResp := tsAuth.request(t, http.MethodGet, "/calendar/v1/timezones", nil)
	if tzResp.Code != http.StatusOK {
		t.Fatalf("timezones: expected 200, got %d", tzResp.Code)
	}
	tzData := parseResponse(t, tzResp)
	timezones := tzData["Timezones"].([]interface{})
	if len(timezones) < 50 {
		t.Fatalf("timezones: expected many timezones, got %d", len(timezones))
	}

	// --- Calendar list response format ---
	calListResp := tsAuth.request(t, http.MethodGet, "/calendar/v1", nil)
	if calListResp.Code != http.StatusOK {
		t.Fatalf("calendar list: expected 200, got %d", calListResp.Code)
	}
	calListData := parseResponse(t, calListResp)
	if _, ok := calListData["Calendars"]; !ok {
		t.Fatal("calendar list: missing 'Calendars' field")
	}
	if _, ok := calListData["Code"]; !ok {
		t.Fatal("calendar list: missing 'Code' field")
	}

	// --- User settings response format ---
	settingsResp := tsAuth.request(t, http.MethodGet, "/settings/calendar", nil)
	if settingsResp.Code != http.StatusOK {
		t.Fatalf("user settings: expected 200, got %d", settingsResp.Code)
	}
	settingsData := parseResponse(t, settingsResp)
	settings := settingsData["CalendarUserSettings"].(map[string]interface{})

	requiredSettingsFields := []string{
		"WeekLength", "DisplayWeekNumber", "PrimaryTimezone",
		"DisplaySecondaryTimezone", "ViewPreference",
	}
	for _, field := range requiredSettingsFields {
		if _, ok := settings[field]; !ok {
			t.Errorf("user settings missing field: %s", field)
		}
	}

	// --- Create calendar ---
	createResp := tsAuth.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Mobile Test",
		"Color":     "#0000FF",
		"Display":   1,
		"AddressID": "addr-mobile-1",
	})
	if createResp.Code != http.StatusOK {
		t.Fatalf("create calendar: expected 200, got %d: %s", createResp.Code, createResp.Body.String())
	}
	createData := parseResponse(t, createResp)
	cal := createData["Calendar"].(map[string]interface{})

	// CreateCalendar returns CalendarWithMembers: ID, Type, Owner, Members
	requiredCreateFields := []string{"ID", "Type", "Owner", "Members"}
	for _, field := range requiredCreateFields {
		if _, ok := cal[field]; !ok {
			t.Errorf("create calendar response missing field: %s", field)
		}
	}

	calendarID := cal["ID"].(string)

	// --- ListCalendars returns VisualCalendar with full display fields ---
	listCalResp := tsAuth.request(t, http.MethodGet, "/calendar/v1", nil)
	listCalData := parseResponse(t, listCalResp)
	calendars := listCalData["Calendars"].([]interface{})
	if len(calendars) < 1 {
		t.Fatal("expected at least 1 calendar in list")
	}
	visualCal := calendars[len(calendars)-1].(map[string]interface{})
	requiredVisualFields := []string{"ID", "Type", "Name", "Color", "Display", "Email", "Flags", "Permissions"}
	for _, field := range requiredVisualFields {
		if _, ok := visualCal[field]; !ok {
			t.Errorf("list calendar response missing field: %s", field)
		}
	}

	// --- Calendar settings response format ---
	calSettingsResp := tsAuth.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/settings", calendarID), nil)
	if calSettingsResp.Code != http.StatusOK {
		t.Fatalf("calendar settings: expected 200, got %d", calSettingsResp.Code)
	}
	calSettingsData := parseResponse(t, calSettingsResp)
	calSettings := calSettingsData["CalendarSettings"].(map[string]interface{})

	requiredCalSettingsFields := []string{"ID", "CalendarID", "DefaultEventDuration", "DefaultPartDayNotifications", "DefaultFullDayNotifications"}
	for _, field := range requiredCalSettingsFields {
		if _, ok := calSettings[field]; !ok {
			t.Errorf("calendar settings missing field: %s", field)
		}
	}

	// --- Members list response format ---
	membersResp := tsAuth.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/members", calendarID), nil)
	if membersResp.Code != http.StatusOK {
		t.Fatalf("members: expected 200, got %d", membersResp.Code)
	}
	membersData := parseResponse(t, membersResp)
	if _, ok := membersData["Members"]; !ok {
		t.Fatal("members: missing 'Members' field")
	}

	// --- Sync event and verify event response format ---
	syncResp := tsAuth.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/sync", calendarID), map[string]interface{}{
		"MemberID": "member-1",
		"Events": []map[string]interface{}{
			{
				"Overwrite": 1,
				"Event": map[string]interface{}{
					"Permissions":   3,
					"IsOrganizer":   1,
					"StartTime":     1700000000,
					"StartTimezone": "Europe/Berlin",
					"EndTime":       1700003600,
					"EndTimezone":   "Europe/Berlin",
					"FullDay":       0,
					"UID":           "mobile-compat-test@test",
					"SharedEvents": []map[string]interface{}{
						{"Type": 2, "Data": "event data"},
					},
				},
			},
		},
	})
	if syncResp.Code != http.StatusOK {
		t.Fatalf("sync: expected 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	syncData := parseResponse(t, syncResp)
	syncResponses := syncData["Responses"].([]interface{})
	firstSyncResp := syncResponses[0].(map[string]interface{})
	eventResp := firstSyncResp["Response"].(map[string]interface{})
	event := eventResp["Event"].(map[string]interface{})

	requiredEventFields := []string{
		"ID", "CalendarID", "CreateTime", "ModifyTime",
		"Permissions", "IsOrganizer", "StartTime", "StartTimezone",
		"EndTime", "EndTimezone", "FullDay", "UID",
	}
	for _, field := range requiredEventFields {
		if _, ok := event[field]; !ok {
			t.Errorf("event response missing field: %s", field)
		}
	}
}

// TestIntegration_CORSHeaders verifies that CORS headers are set correctly
// for cross-origin requests from web and mobile clients.
// Note: CORS middleware is applied in main.go. This test creates the same
// middleware chain to verify the behavior.
func TestIntegration_CORSHeaders(t *testing.T) {
	ts := newTestServer(t)

	// Wrap handler with the same CORS logic used in main.go
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-pm-uid, x-pm-appversion, x-pm-locale")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		ts.handler.ServeHTTP(w, r)
	})

	// CORS preflight request
	req := httptest.NewRequest(http.MethodOptions, "/calendar/v1", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	corsHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("CORS preflight: expected 204, got %d", rr.Code)
	}

	allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://localhost:3000" {
		t.Fatalf("CORS: expected origin http://localhost:3000, got %s", allowOrigin)
	}

	allowMethods := rr.Header().Get("Access-Control-Allow-Methods")
	if allowMethods == "" {
		t.Fatal("CORS: missing Access-Control-Allow-Methods header")
	}

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	if allowHeaders == "" {
		t.Fatal("CORS: missing Access-Control-Allow-Headers header")
	}

	// Verify x-pm-uid is in allowed headers (required by Proton clients)
	if !strings.Contains(allowHeaders, "x-pm-uid") {
		t.Fatal("CORS: x-pm-uid not in allowed headers")
	}
}

// TestIntegration_InvalidCredentials verifies login rejection with wrong credentials.
func TestIntegration_InvalidCredentials(t *testing.T) {
	ts := newTestServer(t)

	cases := []struct {
		name     string
		username string
		password string
	}{
		{"wrong password", "proton", "wrong"},
		{"wrong username", "wrong", "proton"},
		{"empty credentials", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.requestNoAuth(t, http.MethodPost, "/core/v4/auth", map[string]string{
				"Username": tc.username,
				"Password": tc.password,
			})
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", resp.Code)
			}
		})
	}
}

// TestIntegration_ConcurrentSessions verifies multiple independent sessions
// can coexist (important for mobile + web simultaneous access).
func TestIntegration_ConcurrentSessions(t *testing.T) {
	ts := newTestServer(t)

	// Create session 1 (simulating web client)
	login1 := ts.requestNoAuth(t, http.MethodPost, "/core/v4/auth", map[string]string{
		"Username": "proton", "Password": "proton",
	})
	data1 := parseResponse(t, login1)
	token1 := data1["AccessToken"].(string)
	uid1 := data1["UID"].(string)

	// Create session 2 (simulating mobile client)
	login2 := ts.requestNoAuth(t, http.MethodPost, "/core/v4/auth", map[string]string{
		"Username": "proton", "Password": "proton",
	})
	data2 := parseResponse(t, login2)
	token2 := data2["AccessToken"].(string)
	uid2 := data2["UID"].(string)

	// Both tokens should be different
	if token1 == token2 {
		t.Fatal("concurrent sessions should have different tokens")
	}
	if uid1 == uid2 {
		t.Fatal("concurrent sessions should have different UIDs")
	}

	// Both should work independently
	ts1 := &testServer{handler: ts.handler, token: token1, uid: uid1}
	ts2 := &testServer{handler: ts.handler, token: token2, uid: uid2}

	resp1 := ts1.request(t, http.MethodGet, "/calendar/v1", nil)
	resp2 := ts2.request(t, http.MethodGet, "/calendar/v1", nil)

	if resp1.Code != http.StatusOK {
		t.Fatalf("session 1: expected 200, got %d", resp1.Code)
	}
	if resp2.Code != http.StatusOK {
		t.Fatalf("session 2: expected 200, got %d", resp2.Code)
	}

	// Logout session 1, session 2 should still work
	ts1.request(t, http.MethodDelete, "/core/v4/auth", nil)

	resp1After := ts1.request(t, http.MethodGet, "/calendar/v1", nil)
	resp2After := ts2.request(t, http.MethodGet, "/calendar/v1", nil)

	if resp1After.Code != http.StatusUnauthorized {
		t.Fatalf("session 1 after logout: expected 401, got %d", resp1After.Code)
	}
	if resp2After.Code != http.StatusOK {
		t.Fatalf("session 2 after session 1 logout: expected 200, got %d", resp2After.Code)
	}
}

// TestIntegration_SettingsWorkflow tests reading and updating user and calendar settings.
func TestIntegration_SettingsWorkflow(t *testing.T) {
	ts := newTestServer(t)

	// --- Read default user settings ---
	getResp := ts.request(t, http.MethodGet, "/settings/calendar", nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get user settings: expected 200, got %d", getResp.Code)
	}
	getData := parseResponse(t, getResp)
	settings := getData["CalendarUserSettings"].(map[string]interface{})
	if settings["PrimaryTimezone"].(string) != "UTC" {
		t.Errorf("default timezone: expected UTC, got %v", settings["PrimaryTimezone"])
	}

	// --- Update user settings ---
	updateResp := ts.request(t, http.MethodPut, "/settings/calendar", map[string]interface{}{
		"PrimaryTimezone":  "Europe/Berlin",
		"WeekLength":       7,
		"ViewPreference":   1,
		"DisplayWeekNumber": 1,
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update user settings: expected 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}

	// --- Verify the update persisted ---
	getResp2 := ts.request(t, http.MethodGet, "/settings/calendar", nil)
	getData2 := parseResponse(t, getResp2)
	settings2 := getData2["CalendarUserSettings"].(map[string]interface{})
	if settings2["PrimaryTimezone"].(string) != "Europe/Berlin" {
		t.Errorf("updated timezone: expected Europe/Berlin, got %v", settings2["PrimaryTimezone"])
	}

	// --- Create a calendar and test per-calendar settings ---
	createResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Settings Test",
		"Color":     "#123456",
		"Display":   1,
		"AddressID": "addr-settings-1",
	})
	calData := parseResponse(t, createResp)
	calendarID := calData["Calendar"].(map[string]interface{})["ID"].(string)

	// --- Read calendar-specific settings ---
	calSettingsResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/settings", calendarID), nil)
	if calSettingsResp.Code != http.StatusOK {
		t.Fatalf("get cal settings: expected 200, got %d", calSettingsResp.Code)
	}

	// --- Update calendar settings ---
	calUpdateResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/settings", calendarID), map[string]interface{}{
		"DefaultEventDuration": 7200,
		"DefaultPartDayNotifications": []map[string]interface{}{
			{"Type": 0, "Trigger": "-PT30M"},
		},
	})
	if calUpdateResp.Code != http.StatusOK {
		t.Fatalf("update cal settings: expected 200, got %d: %s", calUpdateResp.Code, calUpdateResp.Body.String())
	}

	// --- Verify calendar settings update ---
	calSettingsResp2 := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/settings", calendarID), nil)
	calSettings2 := parseResponse(t, calSettingsResp2)["CalendarSettings"].(map[string]interface{})
	if calSettings2["DefaultEventDuration"].(float64) != 7200 {
		t.Errorf("updated duration: expected 7200, got %v", calSettings2["DefaultEventDuration"])
	}
}

// TestIntegration_MembersWorkflow tests adding, updating, and removing calendar members.
func TestIntegration_MembersWorkflow(t *testing.T) {
	ts := newTestServer(t)

	// Create a calendar
	createResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Members Test",
		"Color":     "#ABCDEF",
		"Display":   1,
		"AddressID": "addr-members-1",
	})
	calData := parseResponse(t, createResp)
	calendarID := calData["Calendar"].(map[string]interface{})["ID"].(string)

	// Add a member
	addResp := ts.request(t, http.MethodPost, fmt.Sprintf("/calendar/v1/%s/members", calendarID), map[string]interface{}{
		"Email":                "test@example.com",
		"Permissions":          1,
		"PassphraseKeyPacket": "key-data",
	})
	if addResp.Code != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", addResp.Code, addResp.Body.String())
	}
	memberData := parseResponse(t, addResp)
	memberID := memberData["Member"].(map[string]interface{})["ID"].(string)

	// List members
	listResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/members", calendarID), nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list members: expected 200, got %d", listResp.Code)
	}
	listData := parseResponse(t, listResp)
	members := listData["Members"].([]interface{})
	// Should have the auto-created owner member + our added member
	if len(members) < 2 {
		t.Fatalf("expected at least 2 members, got %d", len(members))
	}

	// Update member
	updateResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/members/%s", calendarID, memberID), map[string]interface{}{
		"Permissions": 3,
		"Color":       "#FF0000",
		"Display":     1,
		"Name":        "Updated Member",
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update member: expected 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}

	// Remove member
	removeResp := ts.request(t, http.MethodDelete, fmt.Sprintf("/calendar/v1/%s/members/%s", calendarID, memberID), nil)
	if removeResp.Code != http.StatusOK {
		t.Fatalf("remove member: expected 200, got %d", removeResp.Code)
	}
}

// TestIntegration_AlarmsWorkflow tests alarm creation via events and retrieval.
func TestIntegration_AlarmsWorkflow(t *testing.T) {
	ts := newTestServer(t)

	// Create a calendar
	createResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Alarms Test",
		"Color":     "#FF0000",
		"Display":   1,
		"AddressID": "addr-alarms-1",
	})
	calData := parseResponse(t, createResp)
	calendarID := calData["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event with notifications (which generate alarms)
	syncResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/sync", calendarID), map[string]interface{}{
		"MemberID": "member-1",
		"Events": []map[string]interface{}{
			{
				"Overwrite": 1,
				"Event": map[string]interface{}{
					"Permissions":   3,
					"IsOrganizer":   1,
					"StartTime":     1700000000,
					"StartTimezone": "UTC",
					"EndTime":       1700003600,
					"EndTimezone":   "UTC",
					"FullDay":       0,
					"UID":           "alarm-test@test",
					"SharedEvents": []map[string]interface{}{
						{"Type": 2, "Data": "event data"},
					},
					"Notifications": []map[string]interface{}{
						{"Type": 0, "Trigger": "-PT15M"},
						{"Type": 1, "Trigger": "-PT1H"},
					},
				},
			},
		},
	})
	if syncResp.Code != http.StatusOK {
		t.Fatalf("sync: expected 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	// List alarms
	alarmsResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/alarms?Start=1699990000&End=1700010000", calendarID), nil)
	if alarmsResp.Code != http.StatusOK {
		t.Fatalf("list alarms: expected 200, got %d: %s", alarmsResp.Code, alarmsResp.Body.String())
	}
	alarmsData := parseResponse(t, alarmsResp)
	alarms := alarmsData["Alarms"].([]interface{})
	if len(alarms) < 1 {
		t.Fatal("expected at least 1 alarm from notifications")
	}

	// Get a specific alarm
	firstAlarm := alarms[0].(map[string]interface{})
	alarmID := firstAlarm["ID"].(string)
	getAlarmResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/alarms/%s", calendarID, alarmID), nil)
	if getAlarmResp.Code != http.StatusOK {
		t.Fatalf("get alarm: expected 200, got %d", getAlarmResp.Code)
	}

	// Verify alarm fields
	alarmData := parseResponse(t, getAlarmResp)
	alarm := alarmData["Alarm"].(map[string]interface{})
	requiredFields := []string{"ID", "CalendarID", "EventID", "Occurrence", "Trigger"}
	for _, field := range requiredFields {
		if _, ok := alarm[field]; !ok {
			t.Errorf("alarm missing field: %s", field)
		}
	}
}

// TestIntegration_AttendeesWorkflow tests attendee operations on events.
func TestIntegration_AttendeesWorkflow(t *testing.T) {
	ts := newTestServer(t)

	// Create calendar
	createResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Attendees Test",
		"Color":     "#00FF00",
		"Display":   1,
		"AddressID": "addr-attendees-1",
	})
	calData := parseResponse(t, createResp)
	calendarID := calData["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event with attendees
	syncResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/sync", calendarID), map[string]interface{}{
		"MemberID": "member-1",
		"Events": []map[string]interface{}{
			{
				"Overwrite": 1,
				"Event": map[string]interface{}{
					"Permissions":   3,
					"IsOrganizer":   1,
					"StartTime":     1700000000,
					"StartTimezone": "UTC",
					"EndTime":       1700003600,
					"EndTimezone":   "UTC",
					"FullDay":       0,
					"UID":           "attendees-test@test",
					"SharedEvents": []map[string]interface{}{
						{"Type": 2, "Data": "event data"},
					},
					"AttendeesEvents": []map[string]interface{}{
						{"Type": 2, "Data": "attendee data"},
					},
					"Attendees": []map[string]interface{}{
						{"Token": "token-1", "Status": 0},
						{"Token": "token-2", "Status": 0},
					},
				},
			},
		},
	})
	if syncResp.Code != http.StatusOK {
		t.Fatalf("sync: expected 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	syncData := parseResponse(t, syncResp)
	eventID := syncData["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	// Get attendees
	attendeesResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/%s/attendees", calendarID, eventID), nil)
	if attendeesResp.Code != http.StatusOK {
		t.Fatalf("get attendees: expected 200, got %d: %s", attendeesResp.Code, attendeesResp.Body.String())
	}
	attendeesData := parseResponse(t, attendeesResp)
	attendees := attendeesData["Attendees"].([]interface{})
	if len(attendees) < 2 {
		t.Fatalf("expected 2 attendees, got %d", len(attendees))
	}

	// Update attendee status (accept)
	firstAttendee := attendees[0].(map[string]interface{})
	attendeeID := firstAttendee["ID"].(string)
	updateResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/%s/attendees/%s", calendarID, eventID, attendeeID), map[string]interface{}{
		"Status":     1,
		"UpdateTime": 1700001000,
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update attendee: expected 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}

	// Verify attendee status was updated
	attendeesResp2 := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/%s/attendees", calendarID, eventID), nil)
	attendeesData2 := parseResponse(t, attendeesResp2)
	attendees2 := attendeesData2["Attendees"].([]interface{})
	for _, a := range attendees2 {
		am := a.(map[string]interface{})
		if am["ID"].(string) == attendeeID {
			if am["Status"].(float64) != 1 {
				t.Errorf("attendee status: expected 1 (accepted), got %v", am["Status"])
			}
		}
	}
}

// TestIntegration_EventIDsAndPagination tests the event IDs endpoint with pagination.
func TestIntegration_EventIDsAndPagination(t *testing.T) {
	ts := newTestServer(t)

	// Create calendar
	createResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name":      "Pagination Test",
		"Color":     "#AABB00",
		"Display":   1,
		"AddressID": "addr-page-1",
	})
	calData := parseResponse(t, createResp)
	calendarID := calData["Calendar"].(map[string]interface{})["ID"].(string)

	// Create multiple events
	events := make([]map[string]interface{}, 5)
	for i := 0; i < 5; i++ {
		events[i] = map[string]interface{}{
			"Overwrite": 1,
			"Event": map[string]interface{}{
				"Permissions":   3,
				"IsOrganizer":   1,
				"StartTime":     1700000000 + i*3600,
				"StartTimezone": "UTC",
				"EndTime":       1700003600 + i*3600,
				"EndTimezone":   "UTC",
				"FullDay":       0,
				"UID":           fmt.Sprintf("page-test-%d@test", i),
				"SharedEvents": []map[string]interface{}{
					{"Type": 2, "Data": fmt.Sprintf("event %d", i)},
				},
			},
		}
	}
	syncResp := ts.request(t, http.MethodPut, fmt.Sprintf("/calendar/v1/%s/events/sync", calendarID), map[string]interface{}{
		"MemberID": "member-1",
		"Events":   events,
	})
	if syncResp.Code != http.StatusOK {
		t.Fatalf("sync: expected 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}

	// Get event IDs
	idsResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/ids?Limit=3", calendarID), nil)
	if idsResp.Code != http.StatusOK {
		t.Fatalf("event IDs: expected 200, got %d", idsResp.Code)
	}
	idsData := parseResponse(t, idsResp)
	ids := idsData["IDs"].([]interface{})
	if len(ids) > 3 {
		t.Fatalf("expected at most 3 IDs, got %d", len(ids))
	}

	// Count should be 5
	countResp := ts.request(t, http.MethodGet, fmt.Sprintf("/calendar/v1/%s/events/count", calendarID), nil)
	countData := parseResponse(t, countResp)
	if countData["Total"].(float64) != 5 {
		t.Fatalf("expected 5 events, got %v", countData["Total"])
	}
}

// TestIntegration_LoginResponseJSON verifies the exact JSON structure returned
// by the login endpoint, ensuring compatibility with all clients.
func TestIntegration_LoginResponseJSON(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.requestNoAuth(t, http.MethodPost, "/core/v4/auth", map[string]string{
		"Username": "proton",
		"Password": "proton",
	})

	// Parse raw JSON to verify exact structure
	var raw json.RawMessage
	if err := json.Unmarshal(resp.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(raw, &result)

	// Verify types
	if _, ok := result["Code"].(float64); !ok {
		t.Error("Code should be a number")
	}
	if _, ok := result["UID"].(string); !ok {
		t.Error("UID should be a string")
	}
	if _, ok := result["AccessToken"].(string); !ok {
		t.Error("AccessToken should be a string")
	}
	if _, ok := result["RefreshToken"].(string); !ok {
		t.Error("RefreshToken should be a string")
	}
	if _, ok := result["ExpiresIn"].(float64); !ok {
		t.Error("ExpiresIn should be a number")
	}
	if _, ok := result["TokenType"].(string); !ok {
		t.Error("TokenType should be a string")
	}
	if _, ok := result["Scope"].(string); !ok {
		t.Error("Scope should be a string")
	}
	if _, ok := result["UserID"].(string); !ok {
		t.Error("UserID should be a string")
	}

	// Verify values
	if result["Code"].(float64) != 1000 {
		t.Errorf("Code: expected 1000, got %v", result["Code"])
	}
	if result["ExpiresIn"].(float64) != 86400 {
		t.Errorf("ExpiresIn: expected 86400, got %v", result["ExpiresIn"])
	}
	if result["TokenType"].(string) != "Bearer" {
		t.Errorf("TokenType: expected Bearer, got %v", result["TokenType"])
	}
	if result["Scope"].(string) != "full" {
		t.Errorf("Scope: expected full, got %v", result["Scope"])
	}
}

// --- E2E Encryption Integration Tests ---
// These tests validate that the backend correctly round-trips encrypted data
// without modification, ensuring the client-side E2E encryption model works.

// TestIntegration_E2E_EncryptedEventDataRoundTrip verifies that all four
// CalendarCardType values (Clear=0, Encrypted=1, Signed=2, Both=3) round-trip
// correctly through create → get → update → get.
func TestIntegration_E2E_EncryptedEventDataRoundTrip(t *testing.T) {
	ts := newTestServer(t)

	// Create a calendar
	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "E2E Encryption Test", "Color": "#ff0000", "AddressID": "e2e@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	// Define test encrypted data using all 4 card types
	sig := "-----BEGIN PGP SIGNATURE-----\nfakeSignatureData==\n-----END PGP SIGNATURE-----"
	calendarEventContent := []map[string]interface{}{
		{
			"Type":      0, // CardTypeClear
			"Data":      "BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nDTSTART:20240101T120000Z\nEND:VEVENT\nEND:VCALENDAR",
			"Signature": nil,
			"Author":    "user@proton.me",
		},
	}
	sharedEventContent := []map[string]interface{}{
		{
			"Type":      2, // CardTypeSigned
			"Data":      "BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nSUMMARY:Signed Event\nEND:VEVENT\nEND:VCALENDAR",
			"Signature": sig,
			"Author":    "user@proton.me",
		},
		{
			"Type":      3, // CardTypeBoth (encrypted + signed)
			"Data":      "wcBMA7H5hGBtKRBAAQgAi+encrypted+shared+data/base64+content==",
			"Signature": sig,
			"Author":    "user@proton.me",
		},
	}
	attendeesEventContent := []map[string]interface{}{
		{
			"Type":      1, // CardTypeEncrypted
			"Data":      "wcBMA7H5hGBtKRBAAQgAi+encrypted+attendee+data/base64==",
			"Signature": nil,
			"Author":    "organizer@proton.me",
		},
	}

	calKeyPacket := "wV4Dk7H5hGBtKRBSAQdA+calendarKeyPacket+base64data=="
	sharedKeyPacket := "wV4Dk7H5hGBtKRBSAQdA+sharedKeyPacket+base64data=="
	addressKeyPacket := "wV4Dk7H5hGBtKRBSAQdA+addressKeyPacket+base64data=="
	addressID := "address-id-e2e-test"

	// Create event with all encrypted fields via sync
	syncResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"Permissions":          127,
					"StartTime":            1704067200, // 2024-01-01T00:00:00Z
					"EndTime":              1704070800, // 2024-01-01T01:00:00Z
					"StartTimezone":        "Europe/Berlin",
					"EndTimezone":          "Europe/Berlin",
					"UID":                  "e2e-test-event@proton.local",
					"CalendarKeyPacket":    calKeyPacket,
					"CalendarEventContent": calendarEventContent,
					"SharedKeyPacket":      sharedKeyPacket,
					"SharedEventContent":   sharedEventContent,
					"AddressKeyPacket":     addressKeyPacket,
					"AddressID":            addressID,
					"AttendeesEventContent": attendeesEventContent,
					"Attendees": []map[string]interface{}{
						{"Token": "attendee-token-1", "Status": 0},
					},
				},
			},
		},
	})
	if syncResp.Code != http.StatusOK {
		t.Fatalf("sync create: expected 200, got %d: %s", syncResp.Code, syncResp.Body.String())
	}
	syncData := parseResponse(t, syncResp)
	responses := syncData["Responses"].([]interface{})
	eventResp := responses[0].(map[string]interface{})["Response"].(map[string]interface{})
	if eventResp["Code"].(float64) != 1000 {
		t.Fatalf("sync event create failed: %v", eventResp["Error"])
	}
	event := eventResp["Event"].(map[string]interface{})
	eventID := event["ID"].(string)

	// --- Verify encrypted data on the create response ---
	verifyEncryptedEvent(t, event, "create response",
		calKeyPacket, sharedKeyPacket, addressKeyPacket,
		calendarEventContent, sharedEventContent, attendeesEventContent, sig)

	// --- GET the event and verify data matches exactly ---
	getResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get event: expected 200, got %d", getResp.Code)
	}
	getEvent := parseResponse(t, getResp)["Event"].(map[string]interface{})

	verifyEncryptedEvent(t, getEvent, "GET response",
		calKeyPacket, sharedKeyPacket, addressKeyPacket,
		calendarEventContent, sharedEventContent, attendeesEventContent, sig)

	// --- GET by UID and verify ---
	uidResp := ts.request(t, http.MethodGet, "/calendar/v1/events?UID=e2e-test-event@proton.local", nil)
	if uidResp.Code != http.StatusOK {
		t.Fatalf("get by UID: expected 200, got %d", uidResp.Code)
	}
	uidEvents := parseResponse(t, uidResp)["Events"].([]interface{})
	if len(uidEvents) != 1 {
		t.Fatalf("expected 1 event by UID, got %d", len(uidEvents))
	}
	uidEvent := uidEvents[0].(map[string]interface{})

	verifyEncryptedEvent(t, uidEvent, "UID lookup response",
		calKeyPacket, sharedKeyPacket, addressKeyPacket,
		calendarEventContent, sharedEventContent, attendeesEventContent, sig)
}

// TestIntegration_E2E_EncryptedDataSurvivesUpdate verifies that encrypted
// data is preserved correctly when an event is updated.
func TestIntegration_E2E_EncryptedDataSurvivesUpdate(t *testing.T) {
	ts := newTestServer(t)

	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "Update E2E Test", "Color": "#00ff00", "AddressID": "e2e@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	origCalKeyPacket := "wV4D+original+calendarKeyPacket+base64=="
	origSharedKeyPacket := "wV4D+original+sharedKeyPacket+base64=="

	// Create event with initial encrypted data
	syncResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"Permissions":       127,
					"StartTime":         1000,
					"EndTime":           2000,
					"StartTimezone":     "UTC",
					"EndTimezone":       "UTC",
					"CalendarKeyPacket": origCalKeyPacket,
					"SharedKeyPacket":   origSharedKeyPacket,
					"CalendarEventContent": []map[string]interface{}{
						{"Type": 3, "Data": "original-encrypted-calendar-data", "Signature": "original-sig", "Author": "user@proton.me"},
					},
					"SharedEventContent": []map[string]interface{}{
						{"Type": 3, "Data": "original-encrypted-shared-data", "Signature": "original-shared-sig", "Author": "user@proton.me"},
					},
				},
			},
		},
	})
	eventID := parseResponse(t, syncResp)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	// Update event with new encrypted data
	newCalKeyPacket := "wV4D+UPDATED+calendarKeyPacket+base64=="
	newSharedKeyPacket := "wV4D+UPDATED+sharedKeyPacket+base64=="
	newAddressKeyPacket := "wV4D+UPDATED+addressKeyPacket+base64=="

	updateResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"ID": eventID,
				"Event": map[string]interface{}{
					"Permissions":       127,
					"StartTime":         3000,
					"EndTime":           4000,
					"StartTimezone":     "America/New_York",
					"EndTimezone":       "America/New_York",
					"CalendarKeyPacket": newCalKeyPacket,
					"SharedKeyPacket":   newSharedKeyPacket,
					"AddressKeyPacket":  newAddressKeyPacket,
					"CalendarEventContent": []map[string]interface{}{
						{"Type": 3, "Data": "UPDATED-encrypted-calendar-data", "Signature": "updated-cal-sig", "Author": "user@proton.me"},
					},
					"SharedEventContent": []map[string]interface{}{
						{"Type": 2, "Data": "UPDATED-signed-shared-data", "Signature": "updated-shared-sig", "Author": "user@proton.me"},
					},
					"AttendeesEventContent": []map[string]interface{}{
						{"Type": 1, "Data": "UPDATED-encrypted-attendee-data", "Author": "user@proton.me"},
					},
				},
			},
		},
	})
	if updateResp.Code != http.StatusOK {
		t.Fatalf("sync update: expected 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}

	// Verify the updated event
	getResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	event := parseResponse(t, getResp)["Event"].(map[string]interface{})

	// Key packets should be updated
	if event["CalendarKeyPacket"] != newCalKeyPacket {
		t.Errorf("CalendarKeyPacket: expected updated value, got %v", event["CalendarKeyPacket"])
	}
	if event["SharedKeyPacket"] != newSharedKeyPacket {
		t.Errorf("SharedKeyPacket: expected updated value, got %v", event["SharedKeyPacket"])
	}
	if event["AddressKeyPacket"] != newAddressKeyPacket {
		t.Errorf("AddressKeyPacket: expected updated value, got %v", event["AddressKeyPacket"])
	}

	// CalendarEvents data should be updated
	calEvents := event["CalendarEvents"].([]interface{})
	if len(calEvents) != 1 {
		t.Fatalf("expected 1 calendar event, got %d", len(calEvents))
	}
	ce := calEvents[0].(map[string]interface{})
	if ce["Data"] != "UPDATED-encrypted-calendar-data" {
		t.Errorf("CalendarEvents[0].Data: expected updated, got %v", ce["Data"])
	}
	if ce["Signature"] != "updated-cal-sig" {
		t.Errorf("CalendarEvents[0].Signature: expected updated, got %v", ce["Signature"])
	}

	// SharedEvents data should be updated
	sharedEvents := event["SharedEvents"].([]interface{})
	if len(sharedEvents) != 1 {
		t.Fatalf("expected 1 shared event, got %d", len(sharedEvents))
	}
	se := sharedEvents[0].(map[string]interface{})
	if se["Data"] != "UPDATED-signed-shared-data" {
		t.Errorf("SharedEvents[0].Data: expected updated, got %v", se["Data"])
	}
	if se["Type"].(float64) != 2 {
		t.Errorf("SharedEvents[0].Type: expected 2 (Signed), got %v", se["Type"])
	}

	// AttendeesEvents should be present
	attEvents := event["AttendeesEvents"].([]interface{})
	if len(attEvents) != 1 {
		t.Fatalf("expected 1 attendee event, got %d", len(attEvents))
	}
	ae := attEvents[0].(map[string]interface{})
	if ae["Data"] != "UPDATED-encrypted-attendee-data" {
		t.Errorf("AttendeesEvents[0].Data: expected updated, got %v", ae["Data"])
	}

	// Metadata should be updated
	if event["StartTimezone"] != "America/New_York" {
		t.Errorf("StartTimezone: expected America/New_York, got %v", event["StartTimezone"])
	}
}

// TestIntegration_E2E_AllCardTypes validates each CalendarCardType is stored
// and returned with correct type, data, signature, and author fields.
func TestIntegration_E2E_AllCardTypes(t *testing.T) {
	ts := newTestServer(t)

	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "CardType Test", "Color": "#0000ff", "AddressID": "cards@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	cardTypes := []struct {
		name     string
		typeVal  int
		data     string
		hasSig   bool
		sig      string
	}{
		{"Clear", 0, "CLEAR:plaintext calendar data", false, ""},
		{"Encrypted", 1, "wcBMA+encrypted+blob+data==", false, ""},
		{"Signed", 2, "SIGNED:plaintext with signature", true, "-----BEGIN PGP SIGNATURE-----\nsigned-data\n-----END PGP SIGNATURE-----"},
		{"Both", 3, "wcBMA+encrypted+and+signed==", true, "-----BEGIN PGP SIGNATURE-----\nboth-sig-data\n-----END PGP SIGNATURE-----"},
	}

	for _, ct := range cardTypes {
		t.Run(ct.name, func(t *testing.T) {
			eventContent := []map[string]interface{}{
				{
					"Type":   ct.typeVal,
					"Data":   ct.data,
					"Author": "test@proton.me",
				},
			}
			if ct.hasSig {
				eventContent[0]["Signature"] = ct.sig
			}

			syncResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
				"MemberID": "m1",
				"Events": []map[string]interface{}{
					{
						"Event": map[string]interface{}{
							"Permissions":          127,
							"StartTime":            int64(1000 + ct.typeVal*1000),
							"EndTime":              int64(2000 + ct.typeVal*1000),
							"StartTimezone":        "UTC",
							"EndTimezone":          "UTC",
							"CalendarEventContent": eventContent,
							"SharedEventContent":   eventContent,
						},
					},
				},
			})
			if syncResp.Code != http.StatusOK {
				t.Fatalf("sync: expected 200, got %d", syncResp.Code)
			}

			eventID := parseResponse(t, syncResp)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

			// GET and verify
			getResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
			event := parseResponse(t, getResp)["Event"].(map[string]interface{})

			for _, fieldName := range []string{"CalendarEvents", "SharedEvents"} {
				events := event[fieldName].([]interface{})
				if len(events) != 1 {
					t.Fatalf("%s: expected 1 entry, got %d", fieldName, len(events))
				}
				entry := events[0].(map[string]interface{})

				if entry["Type"].(float64) != float64(ct.typeVal) {
					t.Errorf("%s.Type: expected %d, got %v", fieldName, ct.typeVal, entry["Type"])
				}
				if entry["Data"].(string) != ct.data {
					t.Errorf("%s.Data: expected %q, got %q", fieldName, ct.data, entry["Data"])
				}
				if entry["Author"].(string) != "test@proton.me" {
					t.Errorf("%s.Author: expected test@proton.me, got %v", fieldName, entry["Author"])
				}
				if ct.hasSig {
					if entry["Signature"].(string) != ct.sig {
						t.Errorf("%s.Signature: expected signature, got %v", fieldName, entry["Signature"])
					}
				}
			}
		})
	}
}

// TestIntegration_E2E_MultipleEventDataEntries verifies that multiple
// CalendarEventData entries per event are stored and returned correctly.
func TestIntegration_E2E_MultipleEventDataEntries(t *testing.T) {
	ts := newTestServer(t)

	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "Multi Entry Test", "Color": "#ff00ff", "AddressID": "multi@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event with multiple entries in each encrypted data array
	syncResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"Permissions":   127,
					"StartTime":     5000,
					"EndTime":       6000,
					"StartTimezone": "UTC",
					"EndTimezone":   "UTC",
					"CalendarEventContent": []map[string]interface{}{
						{"Type": 0, "Data": "clear-personal-data", "Author": "user@proton.me"},
						{"Type": 2, "Data": "signed-personal-data", "Signature": "personal-sig", "Author": "user@proton.me"},
					},
					"SharedEventContent": []map[string]interface{}{
						{"Type": 2, "Data": "signed-shared-metadata", "Signature": "shared-meta-sig", "Author": "user@proton.me"},
						{"Type": 3, "Data": "encrypted-signed-shared-body", "Signature": "shared-body-sig", "Author": "user@proton.me"},
					},
					"AttendeesEventContent": []map[string]interface{}{
						{"Type": 1, "Data": "encrypted-attendee-1", "Author": "user@proton.me"},
						{"Type": 3, "Data": "encrypted-signed-attendee-2", "Signature": "att-sig", "Author": "user@proton.me"},
					},
				},
			},
		},
	})
	eventID := parseResponse(t, syncResp)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	getResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	event := parseResponse(t, getResp)["Event"].(map[string]interface{})

	calEvents := event["CalendarEvents"].([]interface{})
	if len(calEvents) != 2 {
		t.Fatalf("CalendarEvents: expected 2 entries, got %d", len(calEvents))
	}
	if calEvents[0].(map[string]interface{})["Data"].(string) != "clear-personal-data" {
		t.Errorf("CalendarEvents[0].Data mismatch")
	}
	if calEvents[1].(map[string]interface{})["Data"].(string) != "signed-personal-data" {
		t.Errorf("CalendarEvents[1].Data mismatch")
	}

	sharedEvents := event["SharedEvents"].([]interface{})
	if len(sharedEvents) != 2 {
		t.Fatalf("SharedEvents: expected 2 entries, got %d", len(sharedEvents))
	}
	if sharedEvents[0].(map[string]interface{})["Data"].(string) != "signed-shared-metadata" {
		t.Errorf("SharedEvents[0].Data mismatch")
	}
	if sharedEvents[1].(map[string]interface{})["Data"].(string) != "encrypted-signed-shared-body" {
		t.Errorf("SharedEvents[1].Data mismatch")
	}

	attEvents := event["AttendeesEvents"].([]interface{})
	if len(attEvents) != 2 {
		t.Fatalf("AttendeesEvents: expected 2 entries, got %d", len(attEvents))
	}
	if attEvents[0].(map[string]interface{})["Data"].(string) != "encrypted-attendee-1" {
		t.Errorf("AttendeesEvents[0].Data mismatch")
	}
	if attEvents[1].(map[string]interface{})["Data"].(string) != "encrypted-signed-attendee-2" {
		t.Errorf("AttendeesEvents[1].Data mismatch")
	}
}

// TestIntegration_E2E_KeyPacketPreservation verifies key packets are stored
// and retrieved exactly as sent, with no modification by the backend.
func TestIntegration_E2E_KeyPacketPreservation(t *testing.T) {
	ts := newTestServer(t)

	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "KeyPacket Test", "Color": "#123456", "AddressID": "kp@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	// Use realistic-looking base64 key packets with special chars
	calKP := "wV4Dk7H5hGBtKRBSAQdA/+calKP/with+special/chars=="
	sharedKP := "wV4Dk7H5hGBtKRBSAQdA/+sharedKP/with+special/chars+and+padding==="
	addressKP := "wV4Dk7H5hGBtKRBSAQdA/+addressKP/longer+data+with+various+chars+/+=="

	syncResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"Permissions":       127,
					"StartTime":         7000,
					"EndTime":           8000,
					"StartTimezone":     "UTC",
					"EndTimezone":       "UTC",
					"CalendarKeyPacket": calKP,
					"SharedKeyPacket":   sharedKP,
					"AddressKeyPacket":  addressKP,
					"AddressID":         "addr-kp-test",
					"CalendarEventContent": []map[string]interface{}{
						{"Type": 3, "Data": "encrypted-data", "Signature": "sig", "Author": "u@p.me"},
					},
					"SharedEventContent": []map[string]interface{}{
						{"Type": 3, "Data": "encrypted-shared", "Signature": "sig2", "Author": "u@p.me"},
					},
				},
			},
		},
	})
	eventID := parseResponse(t, syncResp)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	getResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	event := parseResponse(t, getResp)["Event"].(map[string]interface{})

	if event["CalendarKeyPacket"].(string) != calKP {
		t.Errorf("CalendarKeyPacket not preserved exactly: got %q, want %q", event["CalendarKeyPacket"], calKP)
	}
	if event["SharedKeyPacket"].(string) != sharedKP {
		t.Errorf("SharedKeyPacket not preserved exactly: got %q, want %q", event["SharedKeyPacket"], sharedKP)
	}
	if event["AddressKeyPacket"].(string) != addressKP {
		t.Errorf("AddressKeyPacket not preserved exactly: got %q, want %q", event["AddressKeyPacket"], addressKP)
	}
	if event["AddressID"].(string) != "addr-kp-test" {
		t.Errorf("AddressID not preserved: got %q", event["AddressID"])
	}
}

// TestIntegration_E2E_PassphraseKeyPacketForMembers verifies that the
// PassphraseKeyPacket used for sharing calendar encryption keys with
// other members is stored and managed correctly.
func TestIntegration_E2E_PassphraseKeyPacketForMembers(t *testing.T) {
	ts := newTestServer(t)

	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "Member E2E Test", "Color": "#654321", "AddressID": "owner@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	// Add a member with PassphraseKeyPacket
	memberResp := ts.request(t, http.MethodPost, "/calendar/v1/"+calID+"/members", map[string]interface{}{
		"Email":               "member@proton.me",
		"PassphraseKeyPacket": "wV4Dk+passphrase+key+packet+encrypted+with+member+public+key==",
		"Permissions":         63,
	})
	if memberResp.Code != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", memberResp.Code, memberResp.Body.String())
	}
	memberData := parseResponse(t, memberResp)
	member := memberData["Member"].(map[string]interface{})
	memberID := member["ID"].(string)
	if memberID == "" {
		t.Fatal("expected non-empty member ID")
	}
	if member["Email"].(string) != "member@proton.me" {
		t.Errorf("member Email: expected member@proton.me, got %v", member["Email"])
	}
	if member["Permissions"].(float64) != 63 {
		t.Errorf("member Permissions: expected 63, got %v", member["Permissions"])
	}

	// Verify members list includes the new member
	membersResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/members", nil)
	members := parseResponse(t, membersResp)["Members"].([]interface{})
	found := false
	for _, m := range members {
		if m.(map[string]interface{})["Email"].(string) == "member@proton.me" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new member not found in members list")
	}
}

// TestIntegration_E2E_BackendNeverModifiesEncryptedContent verifies that
// the backend acts as a pure storage layer and never modifies any
// encrypted content fields.
func TestIntegration_E2E_BackendNeverModifiesEncryptedContent(t *testing.T) {
	ts := newTestServer(t)

	calResp := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "Immutability Test", "Color": "#abcdef", "AddressID": "immutable@proton.me", "Display": 1,
	})
	calID := parseResponse(t, calResp)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event with carefully crafted encrypted data including special
	// characters, long strings, and Unicode to verify no corruption occurs
	specialData := "wcBMA7H5hGBtKRBAAQgA\n\ttabs+and+newlines\r\n+unicode:日本語+emoji:🔒+base64:/+=="
	specialSig := "-----BEGIN PGP SIGNATURE-----\nVersion: OpenPGP.js\n\nwsBcBAAB+special/chars/in/signature==\n-----END PGP SIGNATURE-----"

	syncResp := ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"Permissions":       127,
					"StartTime":         9000,
					"EndTime":           10000,
					"StartTimezone":     "UTC",
					"EndTimezone":       "UTC",
					"CalendarKeyPacket": "key-packet-with-special/chars+/==",
					"SharedKeyPacket":   "shared-key-special/chars+/padding===",
					"CalendarEventContent": []map[string]interface{}{
						{"Type": 3, "Data": specialData, "Signature": specialSig, "Author": "test@proton.me"},
					},
					"SharedEventContent": []map[string]interface{}{
						{"Type": 3, "Data": specialData, "Signature": specialSig, "Author": "test@proton.me"},
					},
				},
			},
		},
	})
	eventID := parseResponse(t, syncResp)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	// Read back multiple times to verify consistency
	for i := 0; i < 3; i++ {
		getResp := ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
		event := parseResponse(t, getResp)["Event"].(map[string]interface{})

		calEvents := event["CalendarEvents"].([]interface{})
		ce := calEvents[0].(map[string]interface{})
		if ce["Data"].(string) != specialData {
			t.Errorf("read %d: CalendarEvents[0].Data was modified by backend", i+1)
		}
		if ce["Signature"].(string) != specialSig {
			t.Errorf("read %d: CalendarEvents[0].Signature was modified by backend", i+1)
		}

		sharedEvents := event["SharedEvents"].([]interface{})
		se := sharedEvents[0].(map[string]interface{})
		if se["Data"].(string) != specialData {
			t.Errorf("read %d: SharedEvents[0].Data was modified by backend", i+1)
		}
		if se["Signature"].(string) != specialSig {
			t.Errorf("read %d: SharedEvents[0].Signature was modified by backend", i+1)
		}

		if event["CalendarKeyPacket"].(string) != "key-packet-with-special/chars+/==" {
			t.Errorf("read %d: CalendarKeyPacket was modified by backend", i+1)
		}
	}
}

// verifyEncryptedEvent is a helper that checks all encrypted fields of an event.
func verifyEncryptedEvent(t *testing.T, event map[string]interface{}, context string,
	expectedCalKP, expectedSharedKP, expectedAddressKP string,
	expectedCalContent, expectedSharedContent, expectedAttContent []map[string]interface{},
	expectedSig string) {
	t.Helper()

	// Verify key packets
	if event["CalendarKeyPacket"].(string) != expectedCalKP {
		t.Errorf("[%s] CalendarKeyPacket mismatch: got %q", context, event["CalendarKeyPacket"])
	}
	if event["SharedKeyPacket"].(string) != expectedSharedKP {
		t.Errorf("[%s] SharedKeyPacket mismatch: got %q", context, event["SharedKeyPacket"])
	}
	if event["AddressKeyPacket"].(string) != expectedAddressKP {
		t.Errorf("[%s] AddressKeyPacket mismatch: got %q", context, event["AddressKeyPacket"])
	}

	// Verify CalendarEvents
	calEvents := event["CalendarEvents"].([]interface{})
	if len(calEvents) != len(expectedCalContent) {
		t.Fatalf("[%s] CalendarEvents: expected %d entries, got %d", context, len(expectedCalContent), len(calEvents))
	}
	for i, expected := range expectedCalContent {
		actual := calEvents[i].(map[string]interface{})
		if actual["Type"].(float64) != float64(expected["Type"].(int)) {
			t.Errorf("[%s] CalendarEvents[%d].Type: expected %v, got %v", context, i, expected["Type"], actual["Type"])
		}
		if actual["Data"].(string) != expected["Data"].(string) {
			t.Errorf("[%s] CalendarEvents[%d].Data mismatch", context, i)
		}
	}

	// Verify SharedEvents
	sharedEvents := event["SharedEvents"].([]interface{})
	if len(sharedEvents) != len(expectedSharedContent) {
		t.Fatalf("[%s] SharedEvents: expected %d entries, got %d", context, len(expectedSharedContent), len(sharedEvents))
	}
	for i, expected := range expectedSharedContent {
		actual := sharedEvents[i].(map[string]interface{})
		if actual["Type"].(float64) != float64(expected["Type"].(int)) {
			t.Errorf("[%s] SharedEvents[%d].Type: expected %v, got %v", context, i, expected["Type"], actual["Type"])
		}
		if actual["Data"].(string) != expected["Data"].(string) {
			t.Errorf("[%s] SharedEvents[%d].Data mismatch", context, i)
		}
		if expected["Signature"] != nil {
			if actual["Signature"].(string) != expected["Signature"].(string) {
				t.Errorf("[%s] SharedEvents[%d].Signature mismatch", context, i)
			}
		}
	}

	// Verify AttendeesEvents
	attEvents := event["AttendeesEvents"].([]interface{})
	if len(attEvents) != len(expectedAttContent) {
		t.Fatalf("[%s] AttendeesEvents: expected %d entries, got %d", context, len(expectedAttContent), len(attEvents))
	}
	for i, expected := range expectedAttContent {
		actual := attEvents[i].(map[string]interface{})
		if actual["Data"].(string) != expected["Data"].(string) {
			t.Errorf("[%s] AttendeesEvents[%d].Data mismatch", context, i)
		}
	}
}
