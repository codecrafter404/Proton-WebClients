package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codecrafter404/Proton-WebClients/backend/auth"
	"github.com/codecrafter404/Proton-WebClients/backend/handlers"
	"github.com/codecrafter404/Proton-WebClients/backend/router"
	"github.com/codecrafter404/Proton-WebClients/backend/store"
)

type testServer struct {
	handler      http.Handler
	authMgr      *auth.Manager
	token        string
	refreshToken string
	uid          string
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	authMgr, err := auth.NewManager()
	if err != nil {
		t.Fatalf("auth.NewManager: %v", err)
	}
	h := handlers.New(s)
	mux := router.New(h, authMgr)
	handler := authMgr.Middleware(mux)

	// Create session directly (bypassing SRP for test convenience)
	sess := authMgr.CreateSession("proton")

	return &testServer{handler: handler, authMgr: authMgr, token: sess.AccessToken, refreshToken: sess.RefreshToken, uid: sess.UID}
}

func (ts *testServer) request(t *testing.T, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ts.token)
	req.Header.Set("x-pm-uid", ts.uid)

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	return rr
}

func (ts *testServer) requestNoAuth(t *testing.T, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	ts.handler.ServeHTTP(rr, req)
	return rr
}

func parseResponse(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse response: %v\nbody: %s", err, rr.Body.String())
	}
	return result
}

// ---------- Auth Tests ----------

func TestAuth_LoginEndpoint(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	authMgr, err := auth.NewManager()
	if err != nil {
		t.Fatalf("auth.NewManager: %v", err)
	}
	h := handlers.New(s)
	mux := router.New(h, authMgr)
	handler := authMgr.Middleware(mux)

	// Test auth/info endpoint first
	body, _ := json.Marshal(map[string]string{"Username": "proton"})
	req := httptest.NewRequest(http.MethodPost, "/core/v4/auth/info", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("auth/info status = %d, want 200", rr.Code)
	}
	result := parseResponse(t, rr)
	if result["ServerEphemeral"] == nil || result["ServerEphemeral"].(string) == "" {
		t.Error("expected ServerEphemeral in response")
	}
	if result["Salt"] == nil || result["Salt"].(string) == "" {
		t.Error("expected Salt in response")
	}
	if result["Modulus"] == nil || result["Modulus"].(string) == "" {
		t.Error("expected Modulus in response")
	}
}

func TestAuth_LoginFail(t *testing.T) {
	s, _ := store.New(":memory:")
	defer s.Close()
	authMgr, err := auth.NewManager()
	if err != nil {
		t.Fatalf("auth.NewManager: %v", err)
	}
	h := handlers.New(s)
	mux := router.New(h, authMgr)
	handler := authMgr.Middleware(mux)

	// SRP login with invalid proof should fail
	body, _ := json.Marshal(map[string]string{
		"Username":        "proton",
		"ClientEphemeral": "AAAA",
		"ClientProof":     "AAAA",
		"SRPSession":      "nonexistent",
	})
	req := httptest.NewRequest(http.MethodPost, "/core/v4/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAuth_UnauthorizedWithoutToken(t *testing.T) {
	ts := newTestServer(t)
	rr := ts.requestNoAuth(t, http.MethodGet, "/calendar/v1", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

// ---------- Calendar Tests ----------

func TestHTTP_CalendarCRUD(t *testing.T) {
	ts := newTestServer(t)

	// Create
	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "Work", "Color": "#ff0000", "AddressID": "user@proton.me", "Display": 1,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	result := parseResponse(t, rr)
	calMap := result["Calendar"].(map[string]interface{})
	calID := calMap["ID"].(string)
	if calID == "" {
		t.Fatal("expected non-empty calendar ID")
	}

	// Get
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get status = %d", rr.Code)
	}

	// List
	rr = ts.request(t, http.MethodGet, "/calendar/v1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d", rr.Code)
	}
	result = parseResponse(t, rr)
	cals := result["Calendars"].([]interface{})
	if len(cals) != 1 {
		t.Errorf("calendars count = %d, want 1", len(cals))
	}

	// Update
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID, map[string]interface{}{
		"Name": "Personal",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d", rr.Code)
	}

	// Delete
	rr = ts.request(t, http.MethodDelete, "/calendar/v1/"+calID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rr.Code)
	}

	// Verify deleted
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHTTP_CalendarCreate_Validation(t *testing.T) {
	ts := newTestServer(t)

	// Missing Name
	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"AddressID": "u@p.me",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}

	// Missing AddressID
	rr = ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "Test",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ---------- Event Tests ----------

func TestHTTP_EventCRUD(t *testing.T) {
	ts := newTestServer(t)

	// Create calendar first
	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"Permissions":   127,
					"StartTime":     1000,
					"StartTimezone": "UTC",
					"EndTime":       2000,
					"EndTimezone":   "UTC",
					"UID":           "test-uid@local",
				},
			},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("sync create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	syncResult := parseResponse(t, rr)
	responses := syncResult["Responses"].([]interface{})
	eventResp := responses[0].(map[string]interface{})["Response"].(map[string]interface{})
	eventMap := eventResp["Event"].(map[string]interface{})
	eventID := eventMap["ID"].(string)

	// Get event
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get event status = %d", rr.Code)
	}

	// List events
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events?Start=0&End=3000", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list events status = %d", rr.Code)
	}
	events := parseResponse(t, rr)["Events"].([]interface{})
	if len(events) != 1 {
		t.Errorf("events count = %d, want 1", len(events))
	}

	// Count events
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/count", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("count status = %d", rr.Code)
	}
	total := parseResponse(t, rr)["Total"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}

	// Event IDs
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/ids", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("ids status = %d", rr.Code)
	}
	ids := parseResponse(t, rr)["IDs"].([]interface{})
	if len(ids) != 1 {
		t.Errorf("ids count = %d, want 1", len(ids))
	}

	// Get by UID
	rr = ts.request(t, http.MethodGet, "/calendar/v1/events?UID=test-uid@local", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get by UID status = %d", rr.Code)
	}

	// Delete via sync
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{"ID": eventID},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("sync delete status = %d", rr.Code)
	}

	// Verify deleted
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHTTP_EventSync_Update(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"StartTime": 100, "EndTime": 200, "StartTimezone": "UTC", "EndTimezone": "UTC",
				},
			},
		},
	})
	eventID := parseResponse(t, rr)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	// Update via sync
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"ID": eventID,
				"Event": map[string]interface{}{
					"StartTime": 300, "EndTime": 400, "StartTimezone": "Europe/Berlin", "EndTimezone": "Europe/Berlin",
				},
			},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d", rr.Code)
	}

	// Verify
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID, nil)
	event := parseResponse(t, rr)["Event"].(map[string]interface{})
	if event["StartTimezone"] != "Europe/Berlin" {
		t.Errorf("tz = %v, want Europe/Berlin", event["StartTimezone"])
	}
}

// ---------- Settings Tests ----------

func TestHTTP_CalendarSettings(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// Get settings
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/settings", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get settings status = %d", rr.Code)
	}
	settings := parseResponse(t, rr)["CalendarSettings"].(map[string]interface{})
	if settings["DefaultEventDuration"].(float64) != 1800 {
		t.Errorf("duration = %v, want 1800", settings["DefaultEventDuration"])
	}

	// Update settings
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/settings", map[string]interface{}{
		"DefaultEventDuration": 3600,
		"DefaultPartDayNotifications": []map[string]interface{}{
			{"Type": 1, "Trigger": "-PT30M"},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update settings status = %d", rr.Code)
	}
	settings = parseResponse(t, rr)["CalendarSettings"].(map[string]interface{})
	if settings["DefaultEventDuration"].(float64) != 3600 {
		t.Errorf("updated duration = %v, want 3600", settings["DefaultEventDuration"])
	}
}

func TestHTTP_UserSettings(t *testing.T) {
	ts := newTestServer(t)

	// Get
	rr := ts.request(t, http.MethodGet, "/settings/calendar", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get user settings status = %d", rr.Code)
	}

	// Update
	rr = ts.request(t, http.MethodPut, "/settings/calendar", map[string]interface{}{
		"PrimaryTimezone": "America/New_York",
		"WeekLength":      5,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update user settings status = %d", rr.Code)
	}
	settings := parseResponse(t, rr)["CalendarUserSettings"].(map[string]interface{})
	if settings["PrimaryTimezone"] != "America/New_York" {
		t.Errorf("tz = %v, want America/New_York", settings["PrimaryTimezone"])
	}
}

// ---------- Timezone Tests ----------

func TestHTTP_Timezones(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodGet, "/calendar/v1/timezones", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}

	result := parseResponse(t, rr)
	tzs := result["Timezones"].([]interface{})
	if len(tzs) < 100 {
		t.Errorf("expected >100 timezones, got %d", len(tzs))
	}
}

// ---------- Alarm Tests ----------

func TestHTTP_Alarms(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event with notifications
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"StartTime":     10000,
					"EndTime":       11000,
					"StartTimezone": "UTC",
					"EndTimezone":   "UTC",
					"Notifications": []map[string]interface{}{
						{"Type": 1, "Trigger": "-PT15M"},
						{"Type": 0, "Trigger": "-PT1H"},
					},
				},
			},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("sync status = %d", rr.Code)
	}

	// List alarms
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/alarms?Start=0&End=20000", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("alarms status = %d", rr.Code)
	}
	alarms := parseResponse(t, rr)["Alarms"].([]interface{})
	if len(alarms) != 2 {
		t.Errorf("alarms count = %d, want 2", len(alarms))
	}

	// Get specific alarm
	alarmID := alarms[0].(map[string]interface{})["ID"].(string)
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/alarms/"+alarmID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get alarm status = %d", rr.Code)
	}
}

// ---------- Member Tests ----------

func TestHTTP_Members(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// List members
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/members", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list members status = %d", rr.Code)
	}
	members := parseResponse(t, rr)["Members"].([]interface{})
	if len(members) != 1 {
		t.Errorf("members = %d, want 1", len(members))
	}

	// Add member
	rr = ts.request(t, http.MethodPost, "/calendar/v1/"+calID+"/members", map[string]interface{}{
		"Email": "other@p.me", "Permissions": 63,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("add member status = %d", rr.Code)
	}
	memberID := parseResponse(t, rr)["Member"].(map[string]interface{})["ID"].(string)

	// Update member
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/members/"+memberID, map[string]interface{}{
		"Name": "New Name",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update member status = %d", rr.Code)
	}

	// Delete member
	rr = ts.request(t, http.MethodDelete, "/calendar/v1/"+calID+"/members/"+memberID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete member status = %d", rr.Code)
	}
}

// ---------- Attendee Tests ----------

func TestHTTP_Attendees(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event with attendees
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"StartTime": 100, "EndTime": 200, "StartTimezone": "UTC", "EndTimezone": "UTC",
					"Attendees": []map[string]interface{}{
						{"Token": "tok1", "Status": 0},
						{"Token": "tok2", "Status": 1},
					},
				},
			},
		},
	})
	eventID := parseResponse(t, rr)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	// Get attendees
	rr = ts.request(t, http.MethodGet, "/calendar/v1/"+calID+"/events/"+eventID+"/attendees", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("get attendees status = %d", rr.Code)
	}
	attendees := parseResponse(t, rr)["Attendees"].([]interface{})
	if len(attendees) != 2 {
		t.Errorf("attendees = %d, want 2", len(attendees))
	}

	// Update attendee
	attID := attendees[0].(map[string]interface{})["ID"].(string)
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/"+eventID+"/attendees/"+attID, map[string]interface{}{
		"Status": 3,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update attendee status = %d", rr.Code)
	}
}

// ---------- Personal Event Update ----------

func TestHTTP_UpdatePersonal(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodPost, "/calendar/v1", map[string]interface{}{
		"Name": "C", "Color": "#c", "AddressID": "u@p.me", "Display": 1,
	})
	calID := parseResponse(t, rr)["Calendar"].(map[string]interface{})["ID"].(string)

	// Create event
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/sync", map[string]interface{}{
		"MemberID": "m1",
		"Events": []map[string]interface{}{
			{
				"Event": map[string]interface{}{
					"StartTime": 100, "EndTime": 200, "StartTimezone": "UTC", "EndTimezone": "UTC",
				},
			},
		},
	})
	eventID := parseResponse(t, rr)["Responses"].([]interface{})[0].(map[string]interface{})["Response"].(map[string]interface{})["Event"].(map[string]interface{})["ID"].(string)

	// Update personal
	rr = ts.request(t, http.MethodPut, "/calendar/v1/"+calID+"/events/"+eventID+"/personal", map[string]interface{}{
		"Color": "#ff0000",
		"Notifications": []map[string]interface{}{
			{"Type": 1, "Trigger": "-PT5M"},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("update personal status = %d", rr.Code)
	}
}

// ---------- 404 Tests ----------

func TestHTTP_NotFound(t *testing.T) {
	ts := newTestServer(t)

	rr := ts.request(t, http.MethodGet, "/nonexistent", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
