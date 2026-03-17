package store

import (
	"testing"

	"github.com/codecrafter404/Proton-WebClients/backend/models"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// ---------- Calendar Tests ----------

func TestCreateAndGetCalendar(t *testing.T) {
	s := newTestStore(t)

	cal, err := s.CreateCalendar(models.CalendarCreateArguments{
		Name:      "Work",
		Color:     "#ff0000",
		AddressID: "user@proton.me",
		Display:   models.CalendarDisplayShown,
	})
	if err != nil {
		t.Fatalf("CreateCalendar: %v", err)
	}
	if cal.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if cal.Owner.Email != "user@proton.me" {
		t.Errorf("owner email = %q, want %q", cal.Owner.Email, "user@proton.me")
	}
	if len(cal.Members) != 1 {
		t.Fatalf("members count = %d, want 1", len(cal.Members))
	}
	if cal.Members[0].Name != "Work" {
		t.Errorf("member name = %q, want %q", cal.Members[0].Name, "Work")
	}

	// Get should return the same
	got, err := s.GetCalendar(cal.ID)
	if err != nil {
		t.Fatalf("GetCalendar: %v", err)
	}
	if got.ID != cal.ID {
		t.Errorf("got ID = %q, want %q", got.ID, cal.ID)
	}
}

func TestListCalendars(t *testing.T) {
	s := newTestStore(t)

	s.CreateCalendar(models.CalendarCreateArguments{Name: "A", Color: "#aaa", AddressID: "u@p.me", Display: 1})
	s.CreateCalendar(models.CalendarCreateArguments{Name: "B", Color: "#bbb", AddressID: "u@p.me", Display: 1})

	cals, err := s.ListCalendars()
	if err != nil {
		t.Fatalf("ListCalendars: %v", err)
	}
	if len(cals) != 2 {
		t.Errorf("len = %d, want 2", len(cals))
	}
}

func TestDeleteCalendar(t *testing.T) {
	s := newTestStore(t)

	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "Del", Color: "#000", AddressID: "u@p.me", Display: 1})

	if err := s.DeleteCalendar(cal.ID); err != nil {
		t.Fatalf("DeleteCalendar: %v", err)
	}
	if _, err := s.GetCalendar(cal.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestDeleteCalendar_NotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteCalendar("nonexistent"); err == nil {
		t.Error("expected error for nonexistent calendar")
	}
}

func TestUpdateCalendar(t *testing.T) {
	s := newTestStore(t)

	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "Old", Color: "#000", AddressID: "u@p.me", Display: 1})

	newName := "New"
	newColor := "#fff"
	updated, err := s.UpdateCalendar(cal.ID, models.CalendarUpdateArguments{Name: &newName, Color: &newColor})
	if err != nil {
		t.Fatalf("UpdateCalendar: %v", err)
	}
	if updated.Members[0].Name != "New" {
		t.Errorf("member name = %q, want %q", updated.Members[0].Name, "New")
	}
	if updated.Members[0].Color != "#fff" {
		t.Errorf("member color = %q, want %q", updated.Members[0].Color, "#fff")
	}
}

func TestDefaultCalendarID_SetOnFirst(t *testing.T) {
	s := newTestStore(t)

	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "First", Color: "#000", AddressID: "u@p.me", Display: 1})

	us, err := s.GetUserSettings()
	if err != nil {
		t.Fatalf("GetUserSettings: %v", err)
	}
	if us.DefaultCalendarID == nil || *us.DefaultCalendarID != cal.ID {
		t.Errorf("default calendar = %v, want %q", us.DefaultCalendarID, cal.ID)
	}
}

func TestDeleteCalendar_ResetsDefault(t *testing.T) {
	s := newTestStore(t)

	cal1, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "A", Color: "#a", AddressID: "u@p.me", Display: 1})
	s.CreateCalendar(models.CalendarCreateArguments{Name: "B", Color: "#b", AddressID: "u@p.me", Display: 1})

	s.DeleteCalendar(cal1.ID)

	us, _ := s.GetUserSettings()
	if us.DefaultCalendarID == nil {
		t.Error("expected default calendar to be reset to another calendar")
	}
}

// ---------- Event Tests ----------

func TestCreateAndGetEvent(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, err := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		Permissions:   127,
		StartTime:     1000,
		StartTimezone: "UTC",
		EndTime:       2000,
		EndTimezone:   "UTC",
		UID:           "test-uid@proton.local",
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if event.ID == "" {
		t.Fatal("expected non-empty event ID")
	}
	if event.UID != "test-uid@proton.local" {
		t.Errorf("UID = %q, want %q", event.UID, "test-uid@proton.local")
	}

	got, err := s.GetEvent(cal.ID, event.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.StartTime != 1000 || got.EndTime != 2000 {
		t.Errorf("times = [%d, %d], want [1000, 2000]", got.StartTime, got.EndTime)
	}
}

func TestCreateEvent_AutoUID(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC",
	})
	if event.UID == "" {
		t.Error("expected auto-generated UID")
	}
}

func TestListEvents_TimeRange(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 1000, EndTime: 2000, StartTimezone: "UTC", EndTimezone: "UTC",
	})
	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 3000, EndTime: 4000, StartTimezone: "UTC", EndTimezone: "UTC",
	})
	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 5000, EndTime: 6000, StartTimezone: "UTC", EndTimezone: "UTC",
	})

	// Query [500, 2500] should return only the first event
	events, err := s.ListEvents(cal.ID, 500, 2500)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("len = %d, want 1", len(events))
	}

	// Query [0, 10000] should return all three
	events, _ = s.ListEvents(cal.ID, 0, 10000)
	if len(events) != 3 {
		t.Errorf("len = %d, want 3", len(events))
	}
}

func TestGetEventByUID(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC", UID: "unique@test",
	})

	event, err := s.GetEventByUID("unique@test")
	if err != nil {
		t.Fatalf("GetEventByUID: %v", err)
	}
	if event.UID != "unique@test" {
		t.Errorf("UID = %q, want %q", event.UID, "unique@test")
	}

	_, err = s.GetEventByUID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent UID")
	}
}

func TestUpdateEvent(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		Permissions: 127, StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC",
	})

	updated, err := s.UpdateEvent(cal.ID, event.ID, models.CreateOrUpdateCalendarEventData{
		Permissions: 63, StartTime: 300, EndTime: 400, StartTimezone: "Europe/Berlin", EndTimezone: "Europe/Berlin",
	})
	if err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}
	if updated.StartTime != 300 || updated.EndTime != 400 {
		t.Errorf("times = [%d, %d], want [300, 400]", updated.StartTime, updated.EndTime)
	}
	if updated.StartTimezone != "Europe/Berlin" {
		t.Errorf("timezone = %q, want %q", updated.StartTimezone, "Europe/Berlin")
	}
}

func TestDeleteEvent(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC",
	})
	if err := s.DeleteEvent(cal.ID, event.ID); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := s.GetEvent(cal.ID, event.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestCountEvents(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC"})
	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{StartTime: 300, EndTime: 400, StartTimezone: "UTC", EndTimezone: "UTC"})

	count, err := s.CountEvents(cal.ID)
	if err != nil {
		t.Fatalf("CountEvents: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestListEventIDs(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	e1, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC"})
	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{StartTime: 300, EndTime: 400, StartTimezone: "UTC", EndTimezone: "UTC"})

	ids, err := s.ListEventIDs(cal.ID, 0, "")
	if err != nil {
		t.Fatalf("ListEventIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("len = %d, want 2", len(ids))
	}

	// With limit
	ids, _ = s.ListEventIDs(cal.ID, 1, "")
	if len(ids) != 1 {
		t.Errorf("len with limit = %d, want 1", len(ids))
	}

	// With afterID
	ids, _ = s.ListEventIDs(cal.ID, 0, e1.ID)
	if len(ids) != 1 {
		t.Errorf("len with afterID = %d, want 1", len(ids))
	}
}

// ---------- Attendee Tests ----------

func TestEventAttendees(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC",
		Attendees: []models.AttendeeInput{
			{Token: "token1", Status: models.AttendeeStatusNeedsAction},
			{Token: "token2", Status: models.AttendeeStatusAccepted},
		},
	})

	attendees, err := s.GetEventAttendees(cal.ID, event.ID)
	if err != nil {
		t.Fatalf("GetEventAttendees: %v", err)
	}
	if len(attendees) != 2 {
		t.Fatalf("len = %d, want 2", len(attendees))
	}

	// Update attendee status
	updated, err := s.UpdateAttendee(cal.ID, event.ID, attendees[0].ID, models.UpdateAttendeeData{
		Status: models.AttendeeStatusDeclined,
	})
	if err != nil {
		t.Fatalf("UpdateAttendee: %v", err)
	}
	if updated.Status != models.AttendeeStatusDeclined {
		t.Errorf("status = %d, want %d", updated.Status, models.AttendeeStatusDeclined)
	}
	if updated.UpdateTime == nil {
		t.Error("expected UpdateTime to be set")
	}
}

// ---------- Settings Tests ----------

func TestCalendarSettings(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	settings, err := s.GetCalendarSettings(cal.ID)
	if err != nil {
		t.Fatalf("GetCalendarSettings: %v", err)
	}
	if settings.DefaultEventDuration != 1800 {
		t.Errorf("duration = %d, want 1800", settings.DefaultEventDuration)
	}
	if len(settings.DefaultPartDayNotifications) != 1 {
		t.Errorf("part-day notifs = %d, want 1", len(settings.DefaultPartDayNotifications))
	}

	// Update
	newDuration := 3600
	newNotifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT30M"},
		{Type: models.NotificationTypeEmail, Trigger: "-PT1H"},
	}
	updated, err := s.UpdateCalendarSettings(cal.ID, models.CalendarSettingsUpdate{
		DefaultEventDuration:        &newDuration,
		DefaultPartDayNotifications: &newNotifs,
	})
	if err != nil {
		t.Fatalf("UpdateCalendarSettings: %v", err)
	}
	if updated.DefaultEventDuration != 3600 {
		t.Errorf("duration = %d, want 3600", updated.DefaultEventDuration)
	}
	if len(updated.DefaultPartDayNotifications) != 2 {
		t.Errorf("part-day notifs = %d, want 2", len(updated.DefaultPartDayNotifications))
	}
}

func TestUserSettings(t *testing.T) {
	s := newTestStore(t)

	us, err := s.GetUserSettings()
	if err != nil {
		t.Fatalf("GetUserSettings: %v", err)
	}
	if us.PrimaryTimezone != "UTC" {
		t.Errorf("primary tz = %q, want %q", us.PrimaryTimezone, "UTC")
	}

	newTZ := "America/New_York"
	newWeekLen := 5
	updated, err := s.UpdateUserSettings(models.CalendarUserSettingsUpdate{
		PrimaryTimezone: &newTZ,
		WeekLength:      &newWeekLen,
	})
	if err != nil {
		t.Fatalf("UpdateUserSettings: %v", err)
	}
	if updated.PrimaryTimezone != "America/New_York" {
		t.Errorf("tz = %q, want %q", updated.PrimaryTimezone, "America/New_York")
	}
	if updated.WeekLength != 5 {
		t.Errorf("week len = %d, want 5", updated.WeekLength)
	}
}

// ---------- Alarm Tests ----------

func TestAlarms_CreatedFromNotifications(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	notifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
		{Type: models.NotificationTypeEmail, Trigger: "-PT1H"},
	}
	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime:     10000,
		EndTime:       11000,
		StartTimezone: "UTC",
		EndTimezone:   "UTC",
		Notifications: &notifs,
	})

	// Query alarms around event start time
	alarms, err := s.ListAlarms(cal.ID, 0, 20000)
	if err != nil {
		t.Fatalf("ListAlarms: %v", err)
	}
	if len(alarms) != 2 {
		t.Fatalf("len alarms = %d, want 2", len(alarms))
	}

	// First alarm: 15 min before = 10000 - 900 = 9100
	found9100 := false
	found6400 := false
	for _, a := range alarms {
		if a.EventID != event.ID {
			t.Errorf("alarm event ID = %q, want %q", a.EventID, event.ID)
		}
		if a.Occurrence == 9100 {
			found9100 = true
			if a.IsEmail {
				t.Error("expected device alarm for -PT15M")
			}
		}
		if a.Occurrence == 6400 { // 10000 - 3600
			found6400 = true
			if !a.IsEmail {
				t.Error("expected email alarm for -PT1H")
			}
		}
	}
	if !found9100 {
		t.Error("expected alarm at occurrence 9100")
	}
	if !found6400 {
		t.Error("expected alarm at occurrence 6400")
	}
}

func TestAlarms_TimeRangeFilter(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	notifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
	}
	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 10000, EndTime: 11000, StartTimezone: "UTC", EndTimezone: "UTC",
		Notifications: &notifs,
	})

	// Query range that excludes the alarm (alarm at 9100)
	alarms, _ := s.ListAlarms(cal.ID, 0, 5000)
	if len(alarms) != 0 {
		t.Errorf("expected 0 alarms, got %d", len(alarms))
	}

	// Query range that includes the alarm
	alarms, _ = s.ListAlarms(cal.ID, 9000, 9200)
	if len(alarms) != 1 {
		t.Errorf("expected 1 alarm, got %d", len(alarms))
	}
}

func TestAlarms_UpdateRecreatesAlarms(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	notifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
	}
	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 10000, EndTime: 11000, StartTimezone: "UTC", EndTimezone: "UTC",
		Notifications: &notifs,
	})

	// Update event time — alarms should be recalculated
	newNotifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT30M"},
	}
	s.UpdateEvent(cal.ID, event.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 20000, EndTime: 21000, StartTimezone: "UTC", EndTimezone: "UTC",
		Notifications: &newNotifs,
	})

	// Old alarm at 9100 should be gone, new at 20000 - 1800 = 18200
	alarms, _ := s.ListAlarms(cal.ID, 0, 30000)
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(alarms))
	}
	if alarms[0].Occurrence != 18200 {
		t.Errorf("occurrence = %d, want 18200", alarms[0].Occurrence)
	}
}

func TestGetAlarm(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	notifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
	}
	s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 10000, EndTime: 11000, StartTimezone: "UTC", EndTimezone: "UTC",
		Notifications: &notifs,
	})

	alarms, _ := s.ListAlarms(cal.ID, 0, 20000)
	if len(alarms) == 0 {
		t.Fatal("expected at least 1 alarm")
	}

	alarm, err := s.GetAlarm(cal.ID, alarms[0].ID)
	if err != nil {
		t.Fatalf("GetAlarm: %v", err)
	}
	if alarm.ID != alarms[0].ID {
		t.Errorf("alarm ID = %q, want %q", alarm.ID, alarms[0].ID)
	}
}

// ---------- Timezone Tests ----------

func TestListTimezones(t *testing.T) {
	s := newTestStore(t)

	tzs, err := s.ListTimezones()
	if err != nil {
		t.Fatalf("ListTimezones: %v", err)
	}
	if len(tzs) < 100 {
		t.Errorf("expected >100 timezones, got %d", len(tzs))
	}

	// Check some well-known timezones
	tzSet := make(map[string]bool)
	for _, tz := range tzs {
		tzSet[tz] = true
	}
	for _, expected := range []string{"UTC", "America/New_York", "Europe/Berlin", "Asia/Tokyo"} {
		if !tzSet[expected] {
			t.Errorf("missing timezone %q", expected)
		}
	}
}

// ---------- Member Tests ----------

func TestAddAndRemoveMember(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "owner@p.me", Display: 1})

	member, err := s.AddMember(cal.ID, models.CreateCalendarMemberData{
		Email:       "other@p.me",
		Permissions: 63,
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if member.Email != "other@p.me" {
		t.Errorf("email = %q, want %q", member.Email, "other@p.me")
	}

	// Verify calendar now has 2 members
	cal2, _ := s.GetCalendar(cal.ID)
	if len(cal2.Members) != 2 {
		t.Errorf("members = %d, want 2", len(cal2.Members))
	}

	// Remove member
	if err := s.RemoveMember(cal.ID, member.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	cal3, _ := s.GetCalendar(cal.ID)
	if len(cal3.Members) != 1 {
		t.Errorf("members = %d, want 1", len(cal3.Members))
	}
}

func TestUpdateMember(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	member := cal.Members[0]
	updated, err := s.UpdateMember(cal.ID, member.ID, models.UpdateCalendarMemberData{
		Name:  "Updated",
		Color: "#aabbcc",
	})
	if err != nil {
		t.Fatalf("UpdateMember: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("name = %q, want %q", updated.Name, "Updated")
	}
	if updated.Color != "#aabbcc" {
		t.Errorf("color = %q, want %q", updated.Color, "#aabbcc")
	}
}

// ---------- Cascade Delete Tests ----------

func TestDeleteCalendar_CascadesEvents(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC",
	})

	s.DeleteCalendar(cal.ID)

	if _, err := s.GetEvent(cal.ID, event.ID); err == nil {
		t.Error("expected event to be deleted with calendar")
	}
}

func TestDeleteEvent_CascadesAlarms(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	notifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
	}
	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 10000, EndTime: 11000, StartTimezone: "UTC", EndTimezone: "UTC",
		Notifications: &notifs,
	})

	s.DeleteEvent(cal.ID, event.ID)

	alarms, _ := s.ListAlarms(cal.ID, 0, 20000)
	if len(alarms) != 0 {
		t.Errorf("expected 0 alarms after event delete, got %d", len(alarms))
	}
}

// ---------- Personal Event Update Tests ----------

func TestUpdateEventPersonal(t *testing.T) {
	s := newTestStore(t)
	cal, _ := s.CreateCalendar(models.CalendarCreateArguments{Name: "C", Color: "#c", AddressID: "u@p.me", Display: 1})

	event, _ := s.CreateEvent(cal.ID, models.CreateOrUpdateCalendarEventData{
		StartTime: 100, EndTime: 200, StartTimezone: "UTC", EndTimezone: "UTC",
	})

	newColor := "#ff0000"
	newNotifs := []models.CalendarNotificationSettings{
		{Type: models.NotificationTypeEmail, Trigger: "-PT5M"},
	}
	updated, err := s.UpdateEventPersonal(cal.ID, event.ID, models.UpdatePersonalEventData{
		Color:         &newColor,
		Notifications: &newNotifs,
	})
	if err != nil {
		t.Fatalf("UpdateEventPersonal: %v", err)
	}
	if updated.Color == nil || *updated.Color != "#ff0000" {
		t.Errorf("color = %v, want #ff0000", updated.Color)
	}
	if updated.Notifications == nil || len(*updated.Notifications) != 1 {
		t.Error("expected 1 notification")
	}
}
