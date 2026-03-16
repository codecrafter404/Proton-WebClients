package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/codecrafter404/Proton-WebClients/backend/models"
	"github.com/codecrafter404/Proton-WebClients/backend/store"
)

// StoredKey represents a PGP key stored after key setup.
type StoredKey struct {
	ID          string
	PrivateKey  string
	PublicKey   string
	Fingerprint string
	Token       string
	Signature   string
	Primary     int
	Active      int
	Flags       int
}

// CalendarKeyData stores the key setup data for a calendar.
type CalendarKeyData struct {
	KeyID        string
	PassphraseID string
	PrivateKey   string
	MemberID     string
	Passphrase   string // DataPacket
	Signature    string
}

// Handler provides HTTP handler methods for the calendar API.
type Handler struct {
	Store *store.Store

	mu           sync.RWMutex
	userKeys     []StoredKey            // user-level keys
	addressKeys  map[string][]StoredKey // addressID -> keys
	keySalt      string                 // stored key salt
	calendarKeys map[string]*CalendarKeyData // calendarID -> key data
}

// New creates a new Handler.
func New(s *store.Store) *Handler {
	return &Handler{
		Store:        s,
		addressKeys:  make(map[string][]StoredKey),
		calendarKeys: make(map[string]*CalendarKeyData),
	}
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"Code":  status,
		"Error": message,
	})
}

// pathParam extracts a path parameter at the given segment index.
// For example, for path /calendar/v1/{calendarID}/events/{eventID},
// pathParam(r, 3) returns calendarID, pathParam(r, 5) returns eventID.
func pathParam(r *http.Request, index int) string {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if index < len(parts) {
		return parts[index]
	}
	return ""
}

// --- Calendar Handlers ---

// ListCalendars handles GET /calendar/v1
func (h *Handler) ListCalendars(w http.ResponseWriter, r *http.Request) {
	cals, err := h.Store.ListCalendars()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":      1000,
		"Calendars": cals,
	})
}

// CreateCalendar handles POST /calendar/v1
func (h *Handler) CreateCalendar(w http.ResponseWriter, r *http.Request) {
	var args models.CalendarCreateArguments
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if args.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if args.AddressID == "" {
		writeError(w, http.StatusBadRequest, "AddressID is required")
		return
	}

	cal, err := h.Store.CreateCalendar(args)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"Calendar": cal,
	})
}

// SetupCalendarKeys handles POST /calendar/v1/{calendarID}/keys
// Stores the calendar's encryption keys (passphrase, private key).
func (h *Handler) SetupCalendarKeys(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	var req struct {
		AddressID  string `json:"AddressID"`
		Signature  string `json:"Signature"`
		PrivateKey string `json:"PrivateKey"`
		Passphrase struct {
			DataPacket string `json:"DataPacket"`
			KeyPacket  string `json:"KeyPacket"`
		} `json:"Passphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify the calendar exists
	cal, err := h.Store.GetCalendar(calendarID)
	if err != nil {
		writeError(w, http.StatusNotFound, "calendar not found")
		return
	}

	keyID := "calkey-" + calendarID
	passphraseID := "passphrase-" + calendarID

	// Find the member ID from the calendar
	memberID := ""
	if len(cal.Members) > 0 {
		memberID = cal.Members[0].ID
	}

	// Store the calendar key data for the bootstrap endpoint
	h.mu.Lock()
	h.calendarKeys[calendarID] = &CalendarKeyData{
		KeyID:        keyID,
		PassphraseID: passphraseID,
		PrivateKey:   req.PrivateKey,
		MemberID:     memberID,
		Passphrase:   req.Passphrase.DataPacket,
		Signature:    req.Signature,
	}
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Key": map[string]interface{}{
			"ID":           keyID,
			"CalendarID":   calendarID,
			"PrivateKey":   req.PrivateKey,
			"PassphraseID": passphraseID,
			"Flags":        3, // ACTIVE | PRIMARY
		},
		"Passphrase": map[string]interface{}{
			"ID":         passphraseID,
			"CalendarID": calendarID,
			"Flags":      1,
			"MemberPassphrases": []map[string]interface{}{
				{
					"MemberID":   memberID,
					"Passphrase": req.Passphrase.DataPacket,
					"Signature":  req.Signature,
				},
			},
		},
	})
}

// GetCalendarBootstrap handles GET /calendar/v2/{calendarID}/bootstrap
// Returns the full calendar data including keys, passphrase, members, and settings.
func (h *Handler) GetCalendarBootstrap(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	cal, err := h.Store.GetCalendar(calendarID)
	if err != nil {
		writeError(w, http.StatusNotFound, "calendar not found")
		return
	}

	h.mu.RLock()
	keyData := h.calendarKeys[calendarID]
	h.mu.RUnlock()

	keys := []interface{}{}
	passphrase := map[string]interface{}{
		"ID":                 "passphrase-" + calendarID,
		"Flags":              1,
		"MemberPassphrases":  []interface{}{},
		"Invitations":        []interface{}{},
	}

	if keyData != nil {
		keys = append(keys, map[string]interface{}{
			"ID":           keyData.KeyID,
			"CalendarID":   calendarID,
			"PrivateKey":   keyData.PrivateKey,
			"PassphraseID": keyData.PassphraseID,
			"Flags":        3, // ACTIVE | PRIMARY
		})
		passphrase = map[string]interface{}{
			"ID":    keyData.PassphraseID,
			"Flags": 1,
			"MemberPassphrases": []map[string]interface{}{
				{
					"MemberID":   keyData.MemberID,
					"Passphrase": keyData.Passphrase,
					"Signature":  keyData.Signature,
				},
			},
			"Invitations": []interface{}{},
		}
	}

	// Build members list
	members := []interface{}{}
	for _, m := range cal.Members {
		members = append(members, map[string]interface{}{
			"ID":          m.ID,
			"CalendarID":  calendarID,
			"AddressID":   m.AddressID,
			"Flags":       m.Flags,
			"Name":        m.Name,
			"Description": m.Description,
			"Email":       m.Email,
			"Permissions": m.Permissions,
			"Color":       m.Color,
			"Display":     m.Display,
			"Priority":    m.Priority,
		})
	}

	// Get calendar settings
	settings, _ := h.Store.GetCalendarSettings(calendarID)
	calSettings := map[string]interface{}{
		"ID":                            "settings-" + calendarID,
		"CalendarID":                    calendarID,
		"DefaultEventDuration":          1800,
		"DefaultPartDayNotifications":   []map[string]interface{}{{"Type": 1, "Trigger": "-PT15M"}},
		"DefaultFullDayNotifications":   []map[string]interface{}{{"Type": 1, "Trigger": "-PT15H"}},
		"MakesUserBusy":                 1,
	}
	if settings != nil {
		calSettings["ID"] = settings.ID
		calSettings["DefaultEventDuration"] = settings.DefaultEventDuration
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":             1000,
		"Keys":             keys,
		"Passphrase":       passphrase,
		"Members":          members,
		"CalendarSettings": calSettings,
	})
}

func (h *Handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	cal, err := h.Store.GetCalendar(calendarID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"Calendar": cal,
	})
}

// DeleteCalendar handles DELETE /calendar/v1/{calendarID}
func (h *Handler) DeleteCalendar(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	if err := h.Store.DeleteCalendar(calendarID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// UpdateCalendar handles PUT /calendar/v1/{calendarID}
func (h *Handler) UpdateCalendar(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	var args models.CalendarUpdateArguments
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cal, err := h.Store.UpdateCalendar(calendarID, args)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"Calendar": cal,
	})
}

// --- Event Handlers ---

// GetEventsCount handles GET /calendar/v1/{calendarID}/events/count
func (h *Handler) GetEventsCount(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	count, err := h.Store.CountEvents(calendarID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":  1000,
		"Total": count,
	})
}

// ListEvents handles GET /calendar/v1/{calendarID}/events
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	startStr := r.URL.Query().Get("Start")
	endStr := r.URL.Query().Get("End")

	start, _ := strconv.ParseInt(startStr, 10, 64)
	end, _ := strconv.ParseInt(endStr, 10, 64)

	// Default to a very wide range if not specified
	if end == 0 {
		end = 1<<62 - 1
	}

	events, err := h.Store.ListEvents(calendarID, start, end)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":   1000,
		"Events": events,
	})
}

// GetEventIDs handles GET /calendar/v1/{calendarID}/events/ids
func (h *Handler) GetEventIDs(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	limitStr := r.URL.Query().Get("Limit")
	afterID := r.URL.Query().Get("AfterID")

	limit, _ := strconv.Atoi(limitStr)

	ids, err := h.Store.ListEventIDs(calendarID, limit, afterID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"IDs":  ids,
	})
}

// GetEvent handles GET /calendar/v1/{calendarID}/events/{eventID}
func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	eventID := pathParam(r, 4)

	event, err := h.Store.GetEvent(calendarID, eventID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":  1000,
		"Event": event,
	})
}

// GetEventByUID handles GET /calendar/v1/events?UID=...
func (h *Handler) GetEventByUID(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("UID")
	if uid == "" {
		writeError(w, http.StatusBadRequest, "UID is required")
		return
	}

	event, err := h.Store.GetEventByUID(uid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":   1000,
		"Events": []*models.CalendarEvent{event},
	})
}

// DeleteEvent handles DELETE /calendar/v1/{calendarID}/events/{eventID}
func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	eventID := pathParam(r, 4)

	if err := h.Store.DeleteEvent(calendarID, eventID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// UpdateEventPersonal handles PUT /calendar/v1/{calendarID}/events/{eventID}/personal
func (h *Handler) UpdateEventPersonal(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	eventID := pathParam(r, 4)

	var data models.UpdatePersonalEventData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	event, err := h.Store.UpdateEventPersonal(calendarID, eventID, data)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":  1000,
		"Event": event,
	})
}

// SyncEvents handles PUT /calendar/v1/{calendarID}/events/sync
func (h *Handler) SyncEvents(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	var data models.SyncMultipleEventsData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	responses := make([]models.SyncEventResponse, len(data.Events))

	for i, syncEvt := range data.Events {
		resp := models.SyncEventResponse{Index: i}

		switch {
		case syncEvt.ID != "" && syncEvt.Event == nil:
			// Delete operation
			err := h.Store.DeleteEvent(calendarID, syncEvt.ID)
			if err != nil {
				resp.Response.Code = 2000
				resp.Response.Error = err.Error()
			} else {
				resp.Response.Code = 1000
			}

		case syncEvt.ID != "" && syncEvt.Event != nil:
			// Update operation
			event, err := h.Store.UpdateEvent(calendarID, syncEvt.ID, *syncEvt.Event)
			if err != nil {
				resp.Response.Code = 2000
				resp.Response.Error = err.Error()
			} else {
				resp.Response.Code = 1000
				resp.Response.Event = event
			}

		case syncEvt.ID == "" && syncEvt.Event != nil:
			// Create operation
			event, err := h.Store.CreateEvent(calendarID, *syncEvt.Event)
			if err != nil {
				resp.Response.Code = 2000
				resp.Response.Error = err.Error()
			} else {
				resp.Response.Code = 1000
				resp.Response.Event = event
			}

		default:
			resp.Response.Code = 2000
			resp.Response.Error = "invalid sync operation"
		}

		responses[i] = resp
	}

	writeJSON(w, http.StatusOK, models.SyncMultipleApiResponse{
		ApiResponse: models.ApiResponse{Code: 1000},
		Responses:   responses,
	})
}

// --- Attendee Handlers ---

// GetAttendees handles GET /calendar/v1/{calendarID}/events/{eventID}/attendees
func (h *Handler) GetAttendees(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	eventID := pathParam(r, 4)

	attendees, err := h.Store.GetEventAttendees(calendarID, eventID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":      1000,
		"Attendees": attendees,
	})
}

// UpdateAttendee handles PUT /calendar/v1/{calendarID}/events/{eventID}/attendees/{attendeeID}
func (h *Handler) UpdateAttendee(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	eventID := pathParam(r, 4)
	attendeeID := pathParam(r, 6)

	var data models.UpdateAttendeeData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	attendee, err := h.Store.UpdateAttendee(calendarID, eventID, attendeeID, data)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"Attendee": attendee,
	})
}

// --- Settings Handlers ---

// GetCalendarSettings handles GET /calendar/v1/{calendarID}/settings
func (h *Handler) GetCalendarSettings(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	settings, err := h.Store.GetCalendarSettings(calendarID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":             1000,
		"CalendarSettings": settings,
	})
}

// UpdateCalendarSettings handles PUT /calendar/v1/{calendarID}/settings
func (h *Handler) UpdateCalendarSettings(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	var update models.CalendarSettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.Store.UpdateCalendarSettings(calendarID, update)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":             1000,
		"CalendarSettings": settings,
	})
}

// GetUserSettings handles GET /settings/calendar
func (h *Handler) GetUserSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.Store.GetUserSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":                 1000,
		"CalendarUserSettings": settings,
	})
}

// UpdateUserSettings handles PUT /settings/calendar
func (h *Handler) UpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	var update models.CalendarUserSettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.Store.UpdateUserSettings(update)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":                 1000,
		"CalendarUserSettings": settings,
	})
}

// --- Alarm Handlers ---

// ListAlarms handles GET /calendar/v1/{calendarID}/alarms
func (h *Handler) ListAlarms(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	startStr := r.URL.Query().Get("Start")
	endStr := r.URL.Query().Get("End")

	start, _ := strconv.ParseInt(startStr, 10, 64)
	end, _ := strconv.ParseInt(endStr, 10, 64)

	if end == 0 {
		end = 1<<62 - 1
	}

	alarms, err := h.Store.ListAlarms(calendarID, start, end)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":   1000,
		"Alarms": alarms,
	})
}

// --- Member Handlers ---

// ListMembers handles GET /calendar/v1/{calendarID}/members
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	cal, err := h.Store.GetCalendar(calendarID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":    1000,
		"Members": cal.Members,
	})
}

// AddMember handles POST /calendar/v1/{calendarID}/members
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)

	var data models.CreateCalendarMemberData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if data.Email == "" {
		writeError(w, http.StatusBadRequest, "Email is required")
		return
	}

	member, err := h.Store.AddMember(calendarID, data)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":   1000,
		"Member": member,
	})
}

// UpdateMember handles PUT /calendar/v1/{calendarID}/members/{memberID}
func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	memberID := pathParam(r, 4)

	var data models.UpdateCalendarMemberData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	member, err := h.Store.UpdateMember(calendarID, memberID, data)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":   1000,
		"Member": member,
	})
}

// RemoveMember handles DELETE /calendar/v1/{calendarID}/members/{memberID}
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	memberID := pathParam(r, 4)

	if err := h.Store.RemoveMember(calendarID, memberID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// --- Mock Handlers ---

// GetTimezones handles GET /calendar/v1/timezones
func (h *Handler) GetTimezones(w http.ResponseWriter, r *http.Request) {
	tzs, err := h.Store.ListTimezones()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":      1000,
		"Timezones": tzs,
	})
}

// GetAlarm handles GET /calendar/v1/{calendarID}/alarms/{alarmID}
func (h *Handler) GetAlarm(w http.ResponseWriter, r *http.Request) {
	calendarID := pathParam(r, 2)
	alarmID := pathParam(r, 4)

	alarm, err := h.Store.GetAlarm(calendarID, alarmID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":  1000,
		"Alarm": alarm,
	})
}

// GetDirectory handles GET /calendar/v1/directory
func (h *Handler) GetDirectory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":      1000,
		"Calendars": []interface{}{},
	})
}
