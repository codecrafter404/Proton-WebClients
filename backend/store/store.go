package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/codecrafter404/Proton-WebClients/backend/models"
)

// Store provides thread-safe in-memory storage for calendar data.
type Store struct {
	mu sync.RWMutex

	calendars      map[string]*models.CalendarWithMembers
	calendarMeta   map[string]*calendarMeta // extra display info keyed by calendar ID
	events         map[string]*models.CalendarEvent // keyed by event ID
	calendarEvents map[string][]string              // calendar ID -> event IDs
	settings       map[string]*models.CalendarSettings
	alarms         map[string]*models.CalendarAlarm
	userSettings   *models.CalendarUserSettings

	nextID int
}

type calendarMeta struct {
	Name        string
	Description string
	Color       string
	Display     models.CalendarDisplay
	Email       string
	Flags       int
	Permissions int
	Priority    int
}

// New creates a new Store with default user settings.
func New() *Store {
	defaultTZ := "UTC"
	return &Store{
		calendars:      make(map[string]*models.CalendarWithMembers),
		calendarMeta:   make(map[string]*calendarMeta),
		events:         make(map[string]*models.CalendarEvent),
		calendarEvents: make(map[string][]string),
		settings:       make(map[string]*models.CalendarSettings),
		alarms:         make(map[string]*models.CalendarAlarm),
		userSettings: &models.CalendarUserSettings{
			WeekLength:                7,
			DisplayWeekNumber:         0,
			AutoDetectPrimaryTimezone: 1,
			PrimaryTimezone:           defaultTZ,
			DisplaySecondaryTimezone:  0,
			ViewPreference:            models.SettingsViewWeek,
			AutoImportInvite:          0,
		},
		nextID: 1,
	}
}

func (s *Store) generateID() string {
	id := fmt.Sprintf("%d", s.nextID)
	s.nextID++
	return id
}

// --- Calendar Operations ---

// CreateCalendar creates a new calendar and returns it.
func (s *Store) CreateCalendar(args models.CalendarCreateArguments) (*models.CalendarWithMembers, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	calID := s.generateID()
	memberID := s.generateID()

	cal := &models.CalendarWithMembers{
		Calendar: models.Calendar{
			ID:   calID,
			Type: models.CalendarTypePersonal,
		},
		Owner: models.CalendarOwner{Email: args.AddressID},
		Members: []models.CalendarMember{
			{
				ID:          memberID,
				Permissions: 127, // full permissions for owner
				Email:       args.AddressID,
				AddressID:   args.AddressID,
				CalendarID:  calID,
				Name:        args.Name,
				Description: args.Description,
				Color:       args.Color,
				Display:     args.Display,
				Priority:    1,
				Flags:       1,
			},
		},
	}

	s.calendars[calID] = cal
	s.calendarMeta[calID] = &calendarMeta{
		Name:        args.Name,
		Description: args.Description,
		Color:       args.Color,
		Display:     args.Display,
		Email:       args.AddressID,
		Flags:       1,
		Permissions: 127,
		Priority:    1,
	}
	s.calendarEvents[calID] = []string{}

	// Create default settings
	settingsID := s.generateID()
	s.settings[calID] = &models.CalendarSettings{
		ID:                   settingsID,
		CalendarID:           calID,
		DefaultEventDuration: 1800, // 30 minutes
		DefaultPartDayNotifications: []models.CalendarNotificationSettings{
			{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
		},
		DefaultFullDayNotifications: []models.CalendarNotificationSettings{
			{Type: models.NotificationTypeDevice, Trigger: "-PT15H"},
		},
	}

	// Set as default calendar if it's the first one
	if s.userSettings.DefaultCalendarID == nil {
		s.userSettings.DefaultCalendarID = &calID
	}

	return cal, nil
}

// GetCalendar returns a calendar by ID.
func (s *Store) GetCalendar(calendarID string) (*models.CalendarWithMembers, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cal, ok := s.calendars[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}
	return cal, nil
}

// ListCalendars returns all calendars.
func (s *Store) ListCalendars() []*models.CalendarWithMembers {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.CalendarWithMembers, 0, len(s.calendars))
	for _, cal := range s.calendars {
		result = append(result, cal)
	}
	return result
}

// DeleteCalendar removes a calendar and its events.
func (s *Store) DeleteCalendar(calendarID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.calendars[calendarID]; !ok {
		return fmt.Errorf("calendar not found: %s", calendarID)
	}

	// Delete all events in this calendar
	if eventIDs, ok := s.calendarEvents[calendarID]; ok {
		for _, eid := range eventIDs {
			delete(s.events, eid)
		}
	}
	delete(s.calendarEvents, calendarID)
	delete(s.calendars, calendarID)
	delete(s.calendarMeta, calendarID)
	delete(s.settings, calendarID)

	// Clear default calendar if it was this one
	if s.userSettings.DefaultCalendarID != nil && *s.userSettings.DefaultCalendarID == calendarID {
		s.userSettings.DefaultCalendarID = nil
		// Set to first remaining calendar
		for id := range s.calendars {
			s.userSettings.DefaultCalendarID = &id
			break
		}
	}

	return nil
}

// UpdateCalendar updates calendar display properties.
func (s *Store) UpdateCalendar(calendarID string, args models.CalendarUpdateArguments) (*models.CalendarWithMembers, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cal, ok := s.calendars[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	meta := s.calendarMeta[calendarID]

	if args.Name != nil {
		meta.Name = *args.Name
	}
	if args.Description != nil {
		meta.Description = *args.Description
	}
	if args.Color != nil {
		meta.Color = *args.Color
	}
	if args.Display != nil {
		meta.Display = *args.Display
	}

	// Update members to reflect new metadata
	for i := range cal.Members {
		if args.Name != nil {
			cal.Members[i].Name = *args.Name
		}
		if args.Description != nil {
			cal.Members[i].Description = *args.Description
		}
		if args.Color != nil {
			cal.Members[i].Color = *args.Color
		}
		if args.Display != nil {
			cal.Members[i].Display = *args.Display
		}
	}

	return cal, nil
}

// GetCalendarMeta returns the visual metadata for a calendar.
func (s *Store) GetCalendarMeta(calendarID string) (*calendarMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.calendarMeta[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}
	return meta, nil
}

// --- Event Operations ---

// CreateEvent creates a new event in a calendar.
func (s *Store) CreateEvent(calendarID string, data models.CreateOrUpdateCalendarEventData) (*models.CalendarEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.calendars[calendarID]; !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	eventID := s.generateID()
	sharedEventID := s.generateID()
	now := time.Now().Unix()

	uid := data.UID
	if uid == "" {
		uid = fmt.Sprintf("event-%s@proton.local", eventID)
	}

	isOrganizer := 1
	if data.IsOrganizer != nil {
		isOrganizer = *data.IsOrganizer
	}

	attendees := make([]models.Attendee, len(data.Attendees))
	for i, a := range data.Attendees {
		attendees[i] = models.Attendee{
			ID:     s.generateID(),
			Token:  a.Token,
			Status: a.Status,
		}
	}

	event := &models.CalendarEvent{
		ID:                   eventID,
		SharedEventID:        sharedEventID,
		CalendarID:           calendarID,
		CreateTime:           now,
		ModifyTime:           now,
		Permissions:          data.Permissions,
		IsOrganizer:          isOrganizer,
		IsProtonProtonInvite: 0,
		Author:               "author@proton.local",
		Color:                data.Color,
		CalendarKeyPacket:    data.CalendarKeyPacket,
		CalendarEvents:       data.CalendarEventContent,
		SharedKeyPacket:      data.SharedKeyPacket,
		SharedEvents:         data.SharedEventContent,
		Notifications:        data.Notifications,
		AttendeesEvents:      data.AttendeesEventContent,
		AttendeesInfo: models.AttendeesInfo{
			Attendees:     attendees,
			MoreAttendees: 0,
		},
		StartTime:     data.StartTime,
		StartTimezone:  data.StartTimezone,
		EndTime:       data.EndTime,
		EndTimezone:    data.EndTimezone,
		FullDay:       data.FullDay,
		RRule:         data.RRule,
		UID:           uid,
		RecurrenceID:  data.RecurrenceID,
		Exdates:       data.Exdates,
	}

	// Ensure slice fields are not nil
	if event.CalendarEvents == nil {
		event.CalendarEvents = []models.CalendarEventData{}
	}
	if event.SharedEvents == nil {
		event.SharedEvents = []models.CalendarEventData{}
	}
	if event.AttendeesEvents == nil {
		event.AttendeesEvents = []models.CalendarEventData{}
	}
	if event.Exdates == nil {
		event.Exdates = []int64{}
	}

	s.events[eventID] = event
	s.calendarEvents[calendarID] = append(s.calendarEvents[calendarID], eventID)

	return event, nil
}

// GetEvent returns an event by ID.
func (s *Store) GetEvent(calendarID, eventID string) (*models.CalendarEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[eventID]
	if !ok || event.CalendarID != calendarID {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}
	return event, nil
}

// ListEvents returns events in a calendar matching the query.
func (s *Store) ListEvents(calendarID string, start, end int64) ([]*models.CalendarEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.calendars[calendarID]; !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	eventIDs := s.calendarEvents[calendarID]
	var result []*models.CalendarEvent

	for _, eid := range eventIDs {
		event := s.events[eid]
		if event == nil {
			continue
		}
		// Check time overlap: event overlaps [start, end] if event.StartTime < end && event.EndTime > start
		if event.StartTime < end && event.EndTime > start {
			result = append(result, event)
		}
	}

	return result, nil
}

// GetEventByUID finds an event by its UID across all calendars.
func (s *Store) GetEventByUID(uid string) (*models.CalendarEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, event := range s.events {
		if event.UID == uid {
			return event, nil
		}
	}
	return nil, fmt.Errorf("event not found with UID: %s", uid)
}

// UpdateEvent updates an existing event.
func (s *Store) UpdateEvent(calendarID, eventID string, data models.CreateOrUpdateCalendarEventData) (*models.CalendarEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[eventID]
	if !ok || event.CalendarID != calendarID {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	now := time.Now().Unix()
	event.ModifyTime = now
	event.Permissions = data.Permissions
	event.Color = data.Color
	event.Notifications = data.Notifications

	if data.CalendarKeyPacket != nil {
		event.CalendarKeyPacket = data.CalendarKeyPacket
	}
	if data.SharedKeyPacket != nil {
		event.SharedKeyPacket = data.SharedKeyPacket
	}
	if data.CalendarEventContent != nil {
		event.CalendarEvents = data.CalendarEventContent
	}
	if data.SharedEventContent != nil {
		event.SharedEvents = data.SharedEventContent
	}
	if data.AttendeesEventContent != nil {
		event.AttendeesEvents = data.AttendeesEventContent
	}
	if data.IsOrganizer != nil {
		event.IsOrganizer = *data.IsOrganizer
	}
	if data.IsPersonalSingleEdit != nil {
		event.IsPersonalSingleEdit = *data.IsPersonalSingleEdit
	}

	event.StartTime = data.StartTime
	event.StartTimezone = data.StartTimezone
	event.EndTime = data.EndTime
	event.EndTimezone = data.EndTimezone
	event.FullDay = data.FullDay
	event.RRule = data.RRule

	if data.Exdates != nil {
		event.Exdates = data.Exdates
	}

	if data.Attendees != nil {
		attendees := make([]models.Attendee, len(data.Attendees))
		for i, a := range data.Attendees {
			attendees[i] = models.Attendee{
				ID:     s.generateID(),
				Token:  a.Token,
				Status: a.Status,
			}
		}
		event.AttendeesInfo.Attendees = attendees
	}

	return event, nil
}

// DeleteEvent removes an event.
func (s *Store) DeleteEvent(calendarID, eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[eventID]
	if !ok || event.CalendarID != calendarID {
		return fmt.Errorf("event not found: %s", eventID)
	}

	delete(s.events, eventID)

	// Remove from calendar's event list
	eventIDs := s.calendarEvents[calendarID]
	for i, eid := range eventIDs {
		if eid == eventID {
			s.calendarEvents[calendarID] = append(eventIDs[:i], eventIDs[i+1:]...)
			break
		}
	}

	return nil
}

// CountEvents returns the number of events in a calendar.
func (s *Store) CountEvents(calendarID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.calendars[calendarID]; !ok {
		return 0, fmt.Errorf("calendar not found: %s", calendarID)
	}
	return len(s.calendarEvents[calendarID]), nil
}

// UpdateEventPersonal updates the personal part of an event (notifications, color).
func (s *Store) UpdateEventPersonal(calendarID, eventID string, data models.UpdatePersonalEventData) (*models.CalendarEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[eventID]
	if !ok || event.CalendarID != calendarID {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	event.ModifyTime = time.Now().Unix()
	event.Notifications = data.Notifications
	event.Color = data.Color

	return event, nil
}

// --- Attendee Operations ---

// GetEventAttendees returns attendees for an event.
func (s *Store) GetEventAttendees(calendarID, eventID string) ([]models.Attendee, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[eventID]
	if !ok || event.CalendarID != calendarID {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	return event.AttendeesInfo.Attendees, nil
}

// UpdateAttendee updates an attendee's status.
func (s *Store) UpdateAttendee(calendarID, eventID, attendeeID string, data models.UpdateAttendeeData) (*models.Attendee, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[eventID]
	if !ok || event.CalendarID != calendarID {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	for i := range event.AttendeesInfo.Attendees {
		if event.AttendeesInfo.Attendees[i].ID == attendeeID {
			event.AttendeesInfo.Attendees[i].Status = data.Status
			if data.UpdateTime != nil {
				event.AttendeesInfo.Attendees[i].UpdateTime = data.UpdateTime
			} else {
				now := time.Now().Unix()
				event.AttendeesInfo.Attendees[i].UpdateTime = &now
			}
			return &event.AttendeesInfo.Attendees[i], nil
		}
	}

	return nil, fmt.Errorf("attendee not found: %s", attendeeID)
}

// --- Settings Operations ---

// GetCalendarSettings returns settings for a calendar.
func (s *Store) GetCalendarSettings(calendarID string) (*models.CalendarSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings, ok := s.settings[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar settings not found: %s", calendarID)
	}
	return settings, nil
}

// UpdateCalendarSettings updates settings for a calendar.
func (s *Store) UpdateCalendarSettings(calendarID string, update models.CalendarSettingsUpdate) (*models.CalendarSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, ok := s.settings[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar settings not found: %s", calendarID)
	}

	if update.DefaultEventDuration != nil {
		settings.DefaultEventDuration = *update.DefaultEventDuration
	}
	if update.DefaultPartDayNotifications != nil {
		settings.DefaultPartDayNotifications = *update.DefaultPartDayNotifications
	}
	if update.DefaultFullDayNotifications != nil {
		settings.DefaultFullDayNotifications = *update.DefaultFullDayNotifications
	}

	return settings, nil
}

// GetUserSettings returns the global user calendar settings.
func (s *Store) GetUserSettings() *models.CalendarUserSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userSettings
}

// UpdateUserSettings updates the global user calendar settings.
func (s *Store) UpdateUserSettings(update models.CalendarUserSettingsUpdate) *models.CalendarUserSettings {
	s.mu.Lock()
	defer s.mu.Unlock()

	if update.DefaultCalendarID != nil {
		s.userSettings.DefaultCalendarID = update.DefaultCalendarID
	}
	if update.WeekLength != nil {
		s.userSettings.WeekLength = *update.WeekLength
	}
	if update.DisplayWeekNumber != nil {
		s.userSettings.DisplayWeekNumber = *update.DisplayWeekNumber
	}
	if update.AutoDetectPrimaryTimezone != nil {
		s.userSettings.AutoDetectPrimaryTimezone = *update.AutoDetectPrimaryTimezone
	}
	if update.PrimaryTimezone != nil {
		s.userSettings.PrimaryTimezone = *update.PrimaryTimezone
	}
	if update.DisplaySecondaryTimezone != nil {
		s.userSettings.DisplaySecondaryTimezone = *update.DisplaySecondaryTimezone
	}
	if update.SecondaryTimezone != nil {
		s.userSettings.SecondaryTimezone = update.SecondaryTimezone
	}
	if update.ViewPreference != nil {
		s.userSettings.ViewPreference = *update.ViewPreference
	}
	if update.InviteLocale != nil {
		s.userSettings.InviteLocale = update.InviteLocale
	}
	if update.AutoImportInvite != nil {
		s.userSettings.AutoImportInvite = *update.AutoImportInvite
	}

	return s.userSettings
}

// --- Alarm Operations ---

// ListAlarms returns alarms in a time range.
func (s *Store) ListAlarms(calendarID string, start, end int64) ([]*models.CalendarAlarm, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.calendars[calendarID]; !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	var result []*models.CalendarAlarm
	for _, alarm := range s.alarms {
		if alarm.CalendarID == calendarID && alarm.Occurrence >= start && alarm.Occurrence <= end {
			result = append(result, alarm)
		}
	}
	return result, nil
}

// --- Member Operations ---

// AddMember adds a member to a calendar.
func (s *Store) AddMember(calendarID string, data models.CreateCalendarMemberData) (*models.CalendarMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cal, ok := s.calendars[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	memberID := s.generateID()
	meta := s.calendarMeta[calendarID]

	member := models.CalendarMember{
		ID:          memberID,
		Permissions: data.Permissions,
		Email:       data.Email,
		AddressID:   data.Email,
		CalendarID:  calendarID,
		Name:        meta.Name,
		Description: meta.Description,
		Color:       meta.Color,
		Display:     meta.Display,
		Priority:    len(cal.Members) + 1,
		Flags:       1,
	}

	cal.Members = append(cal.Members, member)
	return &member, nil
}

// RemoveMember removes a member from a calendar.
func (s *Store) RemoveMember(calendarID, memberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cal, ok := s.calendars[calendarID]
	if !ok {
		return fmt.Errorf("calendar not found: %s", calendarID)
	}

	for i, m := range cal.Members {
		if m.ID == memberID {
			cal.Members = append(cal.Members[:i], cal.Members[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("member not found: %s", memberID)
}

// UpdateMember updates a member's properties.
func (s *Store) UpdateMember(calendarID, memberID string, data models.UpdateCalendarMemberData) (*models.CalendarMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cal, ok := s.calendars[calendarID]
	if !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	for i := range cal.Members {
		if cal.Members[i].ID == memberID {
			if data.Permissions != 0 {
				cal.Members[i].Permissions = data.Permissions
			}
			if data.Name != "" {
				cal.Members[i].Name = data.Name
			}
			if data.Description != "" {
				cal.Members[i].Description = data.Description
			}
			if data.Color != "" {
				cal.Members[i].Color = data.Color
			}
			if data.Display != 0 {
				cal.Members[i].Display = data.Display
			}
			return &cal.Members[i], nil
		}
	}

	return nil, fmt.Errorf("member not found: %s", memberID)
}

// ListEventIDs returns all event IDs in a calendar.
func (s *Store) ListEventIDs(calendarID string, limit int, afterID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.calendars[calendarID]; !ok {
		return nil, fmt.Errorf("calendar not found: %s", calendarID)
	}

	eventIDs := s.calendarEvents[calendarID]
	if afterID != "" {
		found := false
		for i, eid := range eventIDs {
			if eid == afterID {
				eventIDs = eventIDs[i+1:]
				found = true
				break
			}
		}
		if !found {
			eventIDs = nil
		}
	}

	if limit > 0 && len(eventIDs) > limit {
		eventIDs = eventIDs[:limit]
	}

	return eventIDs, nil
}
