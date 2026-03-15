package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/codecrafter404/Proton-WebClients/backend/models"
)

// Store provides SQLite-backed storage for calendar data.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath and runs migrations.
// Use ":memory:" for an in-memory database (useful for tests).
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// For in-memory databases, restrict to a single connection since each
	// connection would get its own separate database.
	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
	}
	// Enable WAL mode and foreign keys
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("pragma %s: %w", pragma, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.seedDefaults(); err != nil {
		return nil, fmt.Errorf("seed defaults: %w", err)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS calendars (
			id TEXT PRIMARY KEY,
			type INTEGER NOT NULL DEFAULT 0,
			owner_email TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '#657ee4',
			display INTEGER NOT NULL DEFAULT 1,
			flags INTEGER NOT NULL DEFAULT 1,
			permissions INTEGER NOT NULL DEFAULT 127,
			priority INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_members (
			id TEXT PRIMARY KEY,
			calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
			permissions INTEGER NOT NULL DEFAULT 127,
			email TEXT NOT NULL,
			address_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			color TEXT NOT NULL DEFAULT '#657ee4',
			display INTEGER NOT NULL DEFAULT 1,
			priority INTEGER NOT NULL DEFAULT 1,
			flags INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS calendar_settings (
			id TEXT PRIMARY KEY,
			calendar_id TEXT NOT NULL UNIQUE REFERENCES calendars(id) ON DELETE CASCADE,
			default_event_duration INTEGER NOT NULL DEFAULT 1800,
			default_part_day_notifications TEXT NOT NULL DEFAULT '[]',
			default_full_day_notifications TEXT NOT NULL DEFAULT '[]'
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			shared_event_id TEXT NOT NULL,
			calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
			create_time INTEGER NOT NULL,
			modify_time INTEGER NOT NULL,
			permissions INTEGER NOT NULL DEFAULT 127,
			is_organizer INTEGER NOT NULL DEFAULT 1,
			is_proton_proton_invite INTEGER NOT NULL DEFAULT 0,
			is_personal_single_edit INTEGER NOT NULL DEFAULT 0,
			author TEXT NOT NULL DEFAULT '',
			color TEXT,
			calendar_key_packet TEXT,
			calendar_events TEXT NOT NULL DEFAULT '[]',
			shared_key_packet TEXT,
			address_key_packet TEXT,
			address_id TEXT,
			shared_events TEXT NOT NULL DEFAULT '[]',
			notifications TEXT,
			attendees_events TEXT NOT NULL DEFAULT '[]',
			start_time INTEGER NOT NULL,
			start_timezone TEXT NOT NULL DEFAULT 'UTC',
			end_time INTEGER NOT NULL,
			end_timezone TEXT NOT NULL DEFAULT 'UTC',
			full_day INTEGER NOT NULL DEFAULT 0,
			rrule TEXT,
			uid TEXT NOT NULL,
			recurrence_id INTEGER,
			exdates TEXT NOT NULL DEFAULT '[]'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_calendar ON events(calendar_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_time ON events(start_time, end_time)`,
		`CREATE INDEX IF NOT EXISTS idx_events_uid ON events(uid)`,
		`CREATE TABLE IF NOT EXISTS attendees (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			token TEXT NOT NULL,
			status INTEGER NOT NULL DEFAULT 0,
			update_time INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS alarms (
			id TEXT PRIMARY KEY,
			calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
			event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			occurrence INTEGER NOT NULL,
			trigger_value TEXT NOT NULL,
			is_email INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alarms_time ON alarms(calendar_id, occurrence)`,
		`CREATE TABLE IF NOT EXISTS user_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			default_calendar_id TEXT,
			week_length INTEGER NOT NULL DEFAULT 7,
			display_week_number INTEGER NOT NULL DEFAULT 0,
			auto_detect_primary_tz INTEGER NOT NULL DEFAULT 1,
			primary_timezone TEXT NOT NULL DEFAULT 'UTC',
			display_secondary_tz INTEGER NOT NULL DEFAULT 0,
			secondary_timezone TEXT,
			view_preference INTEGER NOT NULL DEFAULT 1,
			invite_locale TEXT,
			auto_import_invite INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS timezones (
			id TEXT PRIMARY KEY
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

func (s *Store) seedDefaults() error {
	// Seed user settings if not present
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM user_settings").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := s.db.Exec(`INSERT INTO user_settings (id, primary_timezone, view_preference) VALUES (1, 'UTC', 1)`)
		if err != nil {
			return err
		}
	}
	// Seed timezones if not present
	if err := s.db.QueryRow("SELECT COUNT(*) FROM timezones").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		stmt, err := tx.Prepare("INSERT INTO timezones (id) VALUES (?)")
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, tz := range ianaTimezones {
			if _, err := stmt.Exec(tz); err != nil {
				return err
			}
		}
		return tx.Commit()
	}
	return nil
}

func newID() string { return uuid.New().String() }

// ---------- Calendar Operations ----------

func (s *Store) CreateCalendar(args models.CalendarCreateArguments) (*models.CalendarWithMembers, error) {
	calID := newID()
	memberID := newID()
	settingsID := newID()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO calendars (id, type, owner_email, name, description, color, display) VALUES (?,?,?,?,?,?,?)`,
		calID, models.CalendarTypePersonal, args.AddressID, args.Name, args.Description, args.Color, args.Display); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`INSERT INTO calendar_members (id, calendar_id, permissions, email, address_id, name, description, color, display, priority, flags) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		memberID, calID, 127, args.AddressID, args.AddressID, args.Name, args.Description, args.Color, args.Display, 1, 1); err != nil {
		return nil, err
	}

	partDay, _ := json.Marshal([]models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15M"},
	})
	fullDay, _ := json.Marshal([]models.CalendarNotificationSettings{
		{Type: models.NotificationTypeDevice, Trigger: "-PT15H"},
	})
	if _, err := tx.Exec(`INSERT INTO calendar_settings (id, calendar_id, default_event_duration, default_part_day_notifications, default_full_day_notifications) VALUES (?,?,?,?,?)`,
		settingsID, calID, 1800, string(partDay), string(fullDay)); err != nil {
		return nil, err
	}

	// Set as default if it's the first calendar
	var defCal sql.NullString
	if err := tx.QueryRow("SELECT default_calendar_id FROM user_settings WHERE id=1").Scan(&defCal); err != nil {
		return nil, err
	}
	if !defCal.Valid || defCal.String == "" {
		if _, err := tx.Exec("UPDATE user_settings SET default_calendar_id=? WHERE id=1", calID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetCalendar(calID)
}

func (s *Store) GetCalendar(calendarID string) (*models.CalendarWithMembers, error) {
	row := s.db.QueryRow("SELECT id, type, owner_email, name, description, color, display, flags, permissions, priority FROM calendars WHERE id=?", calendarID)
	var cal models.CalendarWithMembers
	var name, desc, color, email string
	var display, flags, perm, prio int
	if err := row.Scan(&cal.ID, &cal.Type, &email, &name, &desc, &color, &display, &flags, &perm, &prio); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("calendar not found: %s", calendarID)
		}
		return nil, err
	}
	cal.Owner = models.CalendarOwner{Email: email}

	members, err := s.listMembers(calendarID)
	if err != nil {
		return nil, err
	}
	cal.Members = members
	return &cal, nil
}

func (s *Store) listMembers(calendarID string) ([]models.CalendarMember, error) {
	rows, err := s.db.Query("SELECT id, calendar_id, permissions, email, address_id, name, description, color, display, priority, flags FROM calendar_members WHERE calendar_id=?", calendarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []models.CalendarMember
	for rows.Next() {
		var m models.CalendarMember
		if err := rows.Scan(&m.ID, &m.CalendarID, &m.Permissions, &m.Email, &m.AddressID, &m.Name, &m.Description, &m.Color, &m.Display, &m.Priority, &m.Flags); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []models.CalendarMember{}
	}
	return members, rows.Err()
}

func (s *Store) ListCalendars() ([]*models.CalendarWithMembers, error) {
	rows, err := s.db.Query("SELECT id FROM calendars ORDER BY priority")
	if err != nil {
		return nil, err
	}
	// Collect IDs first to avoid holding the connection during GetCalendar calls
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]*models.CalendarWithMembers, 0, len(ids))
	for _, id := range ids {
		cal, err := s.GetCalendar(id)
		if err != nil {
			return nil, err
		}
		result = append(result, cal)
	}
	return result, nil
}

func (s *Store) DeleteCalendar(calendarID string) error {
	res, err := s.db.Exec("DELETE FROM calendars WHERE id=?", calendarID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("calendar not found: %s", calendarID)
	}
	// Reset default calendar if needed
	var defCal sql.NullString
	if err := s.db.QueryRow("SELECT default_calendar_id FROM user_settings WHERE id=1").Scan(&defCal); err == nil {
		if defCal.Valid && defCal.String == calendarID {
			var newDefault sql.NullString
			s.db.QueryRow("SELECT id FROM calendars LIMIT 1").Scan(&newDefault)
			s.db.Exec("UPDATE user_settings SET default_calendar_id=? WHERE id=1", newDefault)
		}
	}
	return nil
}

func (s *Store) UpdateCalendar(calendarID string, args models.CalendarUpdateArguments) (*models.CalendarWithMembers, error) {
	// Check existence
	if _, err := s.GetCalendar(calendarID); err != nil {
		return nil, err
	}

	if args.Name != nil {
		if _, err := s.db.Exec("UPDATE calendars SET name=? WHERE id=?", *args.Name, calendarID); err != nil {
			return nil, fmt.Errorf("update calendar name: %w", err)
		}
		if _, err := s.db.Exec("UPDATE calendar_members SET name=? WHERE calendar_id=?", *args.Name, calendarID); err != nil {
			return nil, fmt.Errorf("update member name: %w", err)
		}
	}
	if args.Description != nil {
		if _, err := s.db.Exec("UPDATE calendars SET description=? WHERE id=?", *args.Description, calendarID); err != nil {
			return nil, fmt.Errorf("update calendar description: %w", err)
		}
		if _, err := s.db.Exec("UPDATE calendar_members SET description=? WHERE calendar_id=?", *args.Description, calendarID); err != nil {
			return nil, fmt.Errorf("update member description: %w", err)
		}
	}
	if args.Color != nil {
		if _, err := s.db.Exec("UPDATE calendars SET color=? WHERE id=?", *args.Color, calendarID); err != nil {
			return nil, fmt.Errorf("update calendar color: %w", err)
		}
		if _, err := s.db.Exec("UPDATE calendar_members SET color=? WHERE calendar_id=?", *args.Color, calendarID); err != nil {
			return nil, fmt.Errorf("update member color: %w", err)
		}
	}
	if args.Display != nil {
		if _, err := s.db.Exec("UPDATE calendars SET display=? WHERE id=?", *args.Display, calendarID); err != nil {
			return nil, fmt.Errorf("update calendar display: %w", err)
		}
		if _, err := s.db.Exec("UPDATE calendar_members SET display=? WHERE calendar_id=?", *args.Display, calendarID); err != nil {
			return nil, fmt.Errorf("update member display: %w", err)
		}
	}

	return s.GetCalendar(calendarID)
}

// ---------- Event Operations ----------

func (s *Store) CreateEvent(calendarID string, data models.CreateOrUpdateCalendarEventData) (*models.CalendarEvent, error) {
	// Verify calendar exists
	if _, err := s.GetCalendar(calendarID); err != nil {
		return nil, err
	}

	eventID := newID()
	sharedEventID := newID()
	now := time.Now().Unix()

	uid := data.UID
	if uid == "" {
		uid = fmt.Sprintf("event-%s@proton.local", eventID)
	}

	isOrganizer := 1
	if data.IsOrganizer != nil {
		isOrganizer = *data.IsOrganizer
	}
	isPersonalSingleEdit := false
	if data.IsPersonalSingleEdit != nil {
		isPersonalSingleEdit = *data.IsPersonalSingleEdit
	}

	calEvents, _ := json.Marshal(nonNilSlice(data.CalendarEventContent))
	sharedEvents, _ := json.Marshal(nonNilSlice(data.SharedEventContent))
	attendeesEvents, _ := json.Marshal(nonNilSlice(data.AttendeesEventContent))
	exdates, _ := json.Marshal(nonNilInt64Slice(data.Exdates))
	var notifJSON *string
	if data.Notifications != nil {
		b, _ := json.Marshal(data.Notifications)
		s := string(b)
		notifJSON = &s
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO events (
		id, shared_event_id, calendar_id, create_time, modify_time,
		permissions, is_organizer, is_proton_proton_invite, is_personal_single_edit,
		author, color, calendar_key_packet, calendar_events, shared_key_packet,
		address_key_packet, address_id, shared_events, notifications, attendees_events,
		start_time, start_timezone, end_time, end_timezone, full_day, rrule, uid, recurrence_id, exdates
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		eventID, sharedEventID, calendarID, now, now,
		data.Permissions, isOrganizer, 0, isPersonalSingleEdit,
		"author@proton.local", data.Color, data.CalendarKeyPacket,
		string(calEvents), data.SharedKeyPacket, nil, nil,
		string(sharedEvents), notifJSON, string(attendeesEvents),
		data.StartTime, data.StartTimezone, data.EndTime, data.EndTimezone,
		data.FullDay, data.RRule, uid, data.RecurrenceID, string(exdates),
	)
	if err != nil {
		return nil, err
	}

	// Insert attendees
	for _, a := range data.Attendees {
		attID := newID()
		if _, err := tx.Exec("INSERT INTO attendees (id, event_id, token, status) VALUES (?,?,?,?)",
			attID, eventID, a.Token, a.Status); err != nil {
			return nil, err
		}
	}

	// Compute and insert alarms for this event
	if err := s.insertAlarmsForEvent(tx, calendarID, eventID, data); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetEvent(calendarID, eventID)
}

// insertAlarmsForEvent computes alarm occurrences from notifications and inserts them.
func (s *Store) insertAlarmsForEvent(tx *sql.Tx, calendarID, eventID string, data models.CreateOrUpdateCalendarEventData) error {
	// Delete existing alarms for this event
	if _, err := tx.Exec("DELETE FROM alarms WHERE event_id=?", eventID); err != nil {
		return err
	}

	if data.Notifications == nil {
		return nil
	}

	for _, notif := range *data.Notifications {
		offsetSec := parseTriggerToSeconds(notif.Trigger)
		occurrence := data.StartTime + int64(offsetSec)

		alarmID := newID()
		isEmail := 0
		if notif.Type == models.NotificationTypeEmail {
			isEmail = 1
		}
		if _, err := tx.Exec("INSERT INTO alarms (id, calendar_id, event_id, occurrence, trigger_value, is_email) VALUES (?,?,?,?,?,?)",
			alarmID, calendarID, eventID, occurrence, notif.Trigger, isEmail); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetEvent(calendarID, eventID string) (*models.CalendarEvent, error) {
	row := s.db.QueryRow(`SELECT
		id, shared_event_id, calendar_id, create_time, modify_time,
		permissions, is_organizer, is_proton_proton_invite, is_personal_single_edit,
		author, color, calendar_key_packet, calendar_events, shared_key_packet,
		address_key_packet, address_id, shared_events, notifications, attendees_events,
		start_time, start_timezone, end_time, end_timezone, full_day, rrule, uid, recurrence_id, exdates
	FROM events WHERE id=? AND calendar_id=?`, eventID, calendarID)
	return s.scanEvent(row)
}

func (s *Store) scanEvent(row *sql.Row) (*models.CalendarEvent, error) {
	var e models.CalendarEvent
	var calEventsJSON, sharedEventsJSON, attendeesEventsJSON, exdatesJSON string
	var notifJSON sql.NullString

	err := row.Scan(
		&e.ID, &e.SharedEventID, &e.CalendarID, &e.CreateTime, &e.ModifyTime,
		&e.Permissions, &e.IsOrganizer, &e.IsProtonProtonInvite, &e.IsPersonalSingleEdit,
		&e.Author, &e.Color, &e.CalendarKeyPacket, &calEventsJSON, &e.SharedKeyPacket,
		&e.AddressKeyPacket, &e.AddressID, &sharedEventsJSON, &notifJSON, &attendeesEventsJSON,
		&e.StartTime, &e.StartTimezone, &e.EndTime, &e.EndTimezone, &e.FullDay,
		&e.RRule, &e.UID, &e.RecurrenceID, &exdatesJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, err
	}

	json.Unmarshal([]byte(calEventsJSON), &e.CalendarEvents)
	json.Unmarshal([]byte(sharedEventsJSON), &e.SharedEvents)
	json.Unmarshal([]byte(attendeesEventsJSON), &e.AttendeesEvents)
	json.Unmarshal([]byte(exdatesJSON), &e.Exdates)

	if notifJSON.Valid {
		var notifs []models.CalendarNotificationSettings
		json.Unmarshal([]byte(notifJSON.String), &notifs)
		e.Notifications = &notifs
	}

	// Ensure non-nil slices
	if e.CalendarEvents == nil {
		e.CalendarEvents = []models.CalendarEventData{}
	}
	if e.SharedEvents == nil {
		e.SharedEvents = []models.CalendarEventData{}
	}
	if e.AttendeesEvents == nil {
		e.AttendeesEvents = []models.CalendarEventData{}
	}
	if e.Exdates == nil {
		e.Exdates = []int64{}
	}

	// Load attendees
	attendees, err := s.loadAttendees(e.ID)
	if err != nil {
		return nil, err
	}
	e.AttendeesInfo = models.AttendeesInfo{Attendees: attendees, MoreAttendees: 0}

	return &e, nil
}

func (s *Store) loadAttendees(eventID string) ([]models.Attendee, error) {
	rows, err := s.db.Query("SELECT id, token, status, update_time FROM attendees WHERE event_id=?", eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.Attendee
	for rows.Next() {
		var a models.Attendee
		if err := rows.Scan(&a.ID, &a.Token, &a.Status, &a.UpdateTime); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if result == nil {
		result = []models.Attendee{}
	}
	return result, rows.Err()
}

func (s *Store) ListEvents(calendarID string, start, end int64) ([]*models.CalendarEvent, error) {
	if _, err := s.GetCalendar(calendarID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query("SELECT id FROM events WHERE calendar_id=? AND start_time < ? AND end_time > ? ORDER BY start_time", calendarID, end, start)
	if err != nil {
		return nil, err
	}
	// Collect IDs first to avoid holding the connection during GetEvent calls
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]*models.CalendarEvent, 0, len(ids))
	for _, id := range ids {
		e, err := s.GetEvent(calendarID, id)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, nil
}

func (s *Store) GetEventByUID(uid string) (*models.CalendarEvent, error) {
	var eventID, calendarID string
	err := s.db.QueryRow("SELECT id, calendar_id FROM events WHERE uid=? LIMIT 1", uid).Scan(&eventID, &calendarID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found with UID: %s", uid)
		}
		return nil, err
	}
	return s.GetEvent(calendarID, eventID)
}

func (s *Store) UpdateEvent(calendarID, eventID string, data models.CreateOrUpdateCalendarEventData) (*models.CalendarEvent, error) {
	// Verify event exists
	if _, err := s.GetEvent(calendarID, eventID); err != nil {
		return nil, err
	}

	now := time.Now().Unix()

	calEvents, _ := json.Marshal(nonNilSlice(data.CalendarEventContent))
	sharedEvents, _ := json.Marshal(nonNilSlice(data.SharedEventContent))
	attendeesEvents, _ := json.Marshal(nonNilSlice(data.AttendeesEventContent))
	exdates, _ := json.Marshal(nonNilInt64Slice(data.Exdates))

	var notifJSON *string
	if data.Notifications != nil {
		b, _ := json.Marshal(data.Notifications)
		str := string(b)
		notifJSON = &str
	}

	isOrganizer := 1
	if data.IsOrganizer != nil {
		isOrganizer = *data.IsOrganizer
	}
	isPersonalSingleEdit := false
	if data.IsPersonalSingleEdit != nil {
		isPersonalSingleEdit = *data.IsPersonalSingleEdit
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE events SET
		modify_time=?, permissions=?, is_organizer=?, is_personal_single_edit=?,
		color=?, calendar_key_packet=COALESCE(?, calendar_key_packet),
		calendar_events=?, shared_key_packet=COALESCE(?, shared_key_packet),
		shared_events=?, notifications=?, attendees_events=?,
		start_time=?, start_timezone=?, end_time=?, end_timezone=?,
		full_day=?, rrule=?, exdates=?
	WHERE id=? AND calendar_id=?`,
		now, data.Permissions, isOrganizer, isPersonalSingleEdit,
		data.Color, data.CalendarKeyPacket,
		string(calEvents), data.SharedKeyPacket,
		string(sharedEvents), notifJSON, string(attendeesEvents),
		data.StartTime, data.StartTimezone, data.EndTime, data.EndTimezone,
		data.FullDay, data.RRule, string(exdates),
		eventID, calendarID,
	)
	if err != nil {
		return nil, err
	}

	// Replace attendees
	if data.Attendees != nil {
		if _, err := tx.Exec("DELETE FROM attendees WHERE event_id=?", eventID); err != nil {
			return nil, fmt.Errorf("delete attendees: %w", err)
		}
		for _, a := range data.Attendees {
			attID := newID()
			if _, err := tx.Exec("INSERT INTO attendees (id, event_id, token, status) VALUES (?,?,?,?)",
				attID, eventID, a.Token, a.Status); err != nil {
				return nil, err
			}
		}
	}

	// Recompute alarms
	if err := s.insertAlarmsForEvent(tx, calendarID, eventID, data); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetEvent(calendarID, eventID)
}

func (s *Store) DeleteEvent(calendarID, eventID string) error {
	res, err := s.db.Exec("DELETE FROM events WHERE id=? AND calendar_id=?", eventID, calendarID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("event not found: %s", eventID)
	}
	return nil
}

func (s *Store) CountEvents(calendarID string) (int, error) {
	if _, err := s.GetCalendar(calendarID); err != nil {
		return 0, err
	}
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM events WHERE calendar_id=?", calendarID).Scan(&count)
	return count, err
}

func (s *Store) UpdateEventPersonal(calendarID, eventID string, data models.UpdatePersonalEventData) (*models.CalendarEvent, error) {
	if _, err := s.GetEvent(calendarID, eventID); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	var notifJSON *string
	if data.Notifications != nil {
		b, _ := json.Marshal(data.Notifications)
		str := string(b)
		notifJSON = &str
	}

	_, err := s.db.Exec("UPDATE events SET modify_time=?, notifications=?, color=? WHERE id=? AND calendar_id=?",
		now, notifJSON, data.Color, eventID, calendarID)
	if err != nil {
		return nil, err
	}

	return s.GetEvent(calendarID, eventID)
}

func (s *Store) ListEventIDs(calendarID string, limit int, afterID string) ([]string, error) {
	if _, err := s.GetCalendar(calendarID); err != nil {
		return nil, err
	}

	var rows *sql.Rows
	var err error
	if afterID != "" {
		// Use rowid of the afterID event as cursor
		if limit > 0 {
			rows, err = s.db.Query("SELECT id FROM events WHERE calendar_id=? AND rowid > (SELECT rowid FROM events WHERE id=?) ORDER BY rowid LIMIT ?", calendarID, afterID, limit)
		} else {
			rows, err = s.db.Query("SELECT id FROM events WHERE calendar_id=? AND rowid > (SELECT rowid FROM events WHERE id=?) ORDER BY rowid", calendarID, afterID)
		}
	} else {
		if limit > 0 {
			rows, err = s.db.Query("SELECT id FROM events WHERE calendar_id=? ORDER BY rowid LIMIT ?", calendarID, limit)
		} else {
			rows, err = s.db.Query("SELECT id FROM events WHERE calendar_id=? ORDER BY rowid", calendarID)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, rows.Err()
}

// ---------- Attendee Operations ----------

func (s *Store) GetEventAttendees(calendarID, eventID string) ([]models.Attendee, error) {
	if _, err := s.GetEvent(calendarID, eventID); err != nil {
		return nil, err
	}
	return s.loadAttendees(eventID)
}

func (s *Store) UpdateAttendee(calendarID, eventID, attendeeID string, data models.UpdateAttendeeData) (*models.Attendee, error) {
	if _, err := s.GetEvent(calendarID, eventID); err != nil {
		return nil, err
	}

	updateTime := time.Now().Unix()
	if data.UpdateTime != nil {
		updateTime = *data.UpdateTime
	}

	res, err := s.db.Exec("UPDATE attendees SET status=?, update_time=? WHERE id=? AND event_id=?",
		data.Status, updateTime, attendeeID, eventID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("attendee not found: %s", attendeeID)
	}

	var a models.Attendee
	err = s.db.QueryRow("SELECT id, token, status, update_time FROM attendees WHERE id=?", attendeeID).
		Scan(&a.ID, &a.Token, &a.Status, &a.UpdateTime)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ---------- Settings Operations ----------

func (s *Store) GetCalendarSettings(calendarID string) (*models.CalendarSettings, error) {
	row := s.db.QueryRow("SELECT id, calendar_id, default_event_duration, default_part_day_notifications, default_full_day_notifications FROM calendar_settings WHERE calendar_id=?", calendarID)
	var cs models.CalendarSettings
	var partJSON, fullJSON string
	if err := row.Scan(&cs.ID, &cs.CalendarID, &cs.DefaultEventDuration, &partJSON, &fullJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("calendar settings not found: %s", calendarID)
		}
		return nil, err
	}
	json.Unmarshal([]byte(partJSON), &cs.DefaultPartDayNotifications)
	json.Unmarshal([]byte(fullJSON), &cs.DefaultFullDayNotifications)
	if cs.DefaultPartDayNotifications == nil {
		cs.DefaultPartDayNotifications = []models.CalendarNotificationSettings{}
	}
	if cs.DefaultFullDayNotifications == nil {
		cs.DefaultFullDayNotifications = []models.CalendarNotificationSettings{}
	}
	return &cs, nil
}

func (s *Store) UpdateCalendarSettings(calendarID string, update models.CalendarSettingsUpdate) (*models.CalendarSettings, error) {
	if _, err := s.GetCalendarSettings(calendarID); err != nil {
		return nil, err
	}

	if update.DefaultEventDuration != nil {
		if _, err := s.db.Exec("UPDATE calendar_settings SET default_event_duration=? WHERE calendar_id=?", *update.DefaultEventDuration, calendarID); err != nil {
			return nil, fmt.Errorf("update default_event_duration: %w", err)
		}
	}
	if update.DefaultPartDayNotifications != nil {
		b, _ := json.Marshal(update.DefaultPartDayNotifications)
		if _, err := s.db.Exec("UPDATE calendar_settings SET default_part_day_notifications=? WHERE calendar_id=?", string(b), calendarID); err != nil {
			return nil, fmt.Errorf("update default_part_day_notifications: %w", err)
		}
	}
	if update.DefaultFullDayNotifications != nil {
		b, _ := json.Marshal(update.DefaultFullDayNotifications)
		if _, err := s.db.Exec("UPDATE calendar_settings SET default_full_day_notifications=? WHERE calendar_id=?", string(b), calendarID); err != nil {
			return nil, fmt.Errorf("update default_full_day_notifications: %w", err)
		}
	}

	return s.GetCalendarSettings(calendarID)
}

func (s *Store) GetUserSettings() (*models.CalendarUserSettings, error) {
	row := s.db.QueryRow(`SELECT default_calendar_id, week_length, display_week_number,
		auto_detect_primary_tz, primary_timezone, display_secondary_tz, secondary_timezone,
		view_preference, invite_locale, auto_import_invite FROM user_settings WHERE id=1`)
	var us models.CalendarUserSettings
	err := row.Scan(&us.DefaultCalendarID, &us.WeekLength, &us.DisplayWeekNumber,
		&us.AutoDetectPrimaryTimezone, &us.PrimaryTimezone, &us.DisplaySecondaryTimezone,
		&us.SecondaryTimezone, &us.ViewPreference, &us.InviteLocale, &us.AutoImportInvite)
	if err != nil {
		return nil, err
	}
	return &us, nil
}

func (s *Store) UpdateUserSettings(update models.CalendarUserSettingsUpdate) (*models.CalendarUserSettings, error) {
	if update.DefaultCalendarID != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET default_calendar_id=? WHERE id=1", *update.DefaultCalendarID); err != nil {
			return nil, fmt.Errorf("update default_calendar_id: %w", err)
		}
	}
	if update.WeekLength != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET week_length=? WHERE id=1", *update.WeekLength); err != nil {
			return nil, fmt.Errorf("update week_length: %w", err)
		}
	}
	if update.DisplayWeekNumber != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET display_week_number=? WHERE id=1", *update.DisplayWeekNumber); err != nil {
			return nil, fmt.Errorf("update display_week_number: %w", err)
		}
	}
	if update.AutoDetectPrimaryTimezone != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET auto_detect_primary_tz=? WHERE id=1", *update.AutoDetectPrimaryTimezone); err != nil {
			return nil, fmt.Errorf("update auto_detect_primary_tz: %w", err)
		}
	}
	if update.PrimaryTimezone != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET primary_timezone=? WHERE id=1", *update.PrimaryTimezone); err != nil {
			return nil, fmt.Errorf("update primary_timezone: %w", err)
		}
	}
	if update.DisplaySecondaryTimezone != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET display_secondary_tz=? WHERE id=1", *update.DisplaySecondaryTimezone); err != nil {
			return nil, fmt.Errorf("update display_secondary_tz: %w", err)
		}
	}
	if update.SecondaryTimezone != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET secondary_timezone=? WHERE id=1", *update.SecondaryTimezone); err != nil {
			return nil, fmt.Errorf("update secondary_timezone: %w", err)
		}
	}
	if update.ViewPreference != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET view_preference=? WHERE id=1", *update.ViewPreference); err != nil {
			return nil, fmt.Errorf("update view_preference: %w", err)
		}
	}
	if update.InviteLocale != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET invite_locale=? WHERE id=1", *update.InviteLocale); err != nil {
			return nil, fmt.Errorf("update invite_locale: %w", err)
		}
	}
	if update.AutoImportInvite != nil {
		if _, err := s.db.Exec("UPDATE user_settings SET auto_import_invite=? WHERE id=1", *update.AutoImportInvite); err != nil {
			return nil, fmt.Errorf("update auto_import_invite: %w", err)
		}
	}
	return s.GetUserSettings()
}

// ---------- Alarm Operations ----------

func (s *Store) ListAlarms(calendarID string, start, end int64) ([]*models.CalendarAlarm, error) {
	if _, err := s.GetCalendar(calendarID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query("SELECT id, calendar_id, event_id, occurrence, trigger_value, is_email FROM alarms WHERE calendar_id=? AND occurrence >= ? AND occurrence <= ? ORDER BY occurrence", calendarID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.CalendarAlarm
	for rows.Next() {
		var a models.CalendarAlarm
		var isEmail int
		if err := rows.Scan(&a.ID, &a.CalendarID, &a.EventID, &a.Occurrence, &a.Trigger, &isEmail); err != nil {
			return nil, err
		}
		a.IsEmail = isEmail != 0
		result = append(result, &a)
	}
	if result == nil {
		result = []*models.CalendarAlarm{}
	}
	return result, rows.Err()
}

func (s *Store) GetAlarm(calendarID, alarmID string) (*models.CalendarAlarm, error) {
	row := s.db.QueryRow("SELECT id, calendar_id, event_id, occurrence, trigger_value, is_email FROM alarms WHERE id=? AND calendar_id=?", alarmID, calendarID)
	var a models.CalendarAlarm
	var isEmail int
	if err := row.Scan(&a.ID, &a.CalendarID, &a.EventID, &a.Occurrence, &a.Trigger, &isEmail); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("alarm not found: %s", alarmID)
		}
		return nil, err
	}
	a.IsEmail = isEmail != 0
	return &a, nil
}

// ---------- Member Operations ----------

func (s *Store) AddMember(calendarID string, data models.CreateCalendarMemberData) (*models.CalendarMember, error) {
	if _, err := s.GetCalendar(calendarID); err != nil {
		return nil, err
	}

	memberID := newID()
	// Get calendar meta
	var name, desc, color string
	var display int
	err := s.db.QueryRow("SELECT name, description, color, display FROM calendars WHERE id=?", calendarID).
		Scan(&name, &desc, &color, &display)
	if err != nil {
		return nil, err
	}

	var maxPrio int
	s.db.QueryRow("SELECT COALESCE(MAX(priority),0) FROM calendar_members WHERE calendar_id=?", calendarID).Scan(&maxPrio)

	_, err = s.db.Exec(`INSERT INTO calendar_members (id, calendar_id, permissions, email, address_id, name, description, color, display, priority, flags) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		memberID, calendarID, data.Permissions, data.Email, data.Email, name, desc, color, display, maxPrio+1, 1)
	if err != nil {
		return nil, err
	}

	m := &models.CalendarMember{
		ID: memberID, CalendarID: calendarID, Permissions: data.Permissions,
		Email: data.Email, AddressID: data.Email, Name: name, Description: desc,
		Color: color, Display: models.CalendarDisplay(display), Priority: maxPrio + 1, Flags: 1,
	}
	return m, nil
}

func (s *Store) RemoveMember(calendarID, memberID string) error {
	res, err := s.db.Exec("DELETE FROM calendar_members WHERE id=? AND calendar_id=?", memberID, calendarID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("member not found: %s", memberID)
	}
	return nil
}

func (s *Store) UpdateMember(calendarID, memberID string, data models.UpdateCalendarMemberData) (*models.CalendarMember, error) {
	// Verify existence
	var exists int
	if err := s.db.QueryRow("SELECT 1 FROM calendar_members WHERE id=? AND calendar_id=?", memberID, calendarID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("member not found: %s", memberID)
	}

	if data.Permissions != 0 {
		if _, err := s.db.Exec("UPDATE calendar_members SET permissions=? WHERE id=?", data.Permissions, memberID); err != nil {
			return nil, fmt.Errorf("update member permissions: %w", err)
		}
	}
	if data.Name != "" {
		if _, err := s.db.Exec("UPDATE calendar_members SET name=? WHERE id=?", data.Name, memberID); err != nil {
			return nil, fmt.Errorf("update member name: %w", err)
		}
	}
	if data.Description != "" {
		if _, err := s.db.Exec("UPDATE calendar_members SET description=? WHERE id=?", data.Description, memberID); err != nil {
			return nil, fmt.Errorf("update member description: %w", err)
		}
	}
	if data.Color != "" {
		if _, err := s.db.Exec("UPDATE calendar_members SET color=? WHERE id=?", data.Color, memberID); err != nil {
			return nil, fmt.Errorf("update member color: %w", err)
		}
	}
	if data.Display != 0 {
		if _, err := s.db.Exec("UPDATE calendar_members SET display=? WHERE id=?", data.Display, memberID); err != nil {
			return nil, fmt.Errorf("update member display: %w", err)
		}
	}

	var m models.CalendarMember
	err := s.db.QueryRow("SELECT id, calendar_id, permissions, email, address_id, name, description, color, display, priority, flags FROM calendar_members WHERE id=?", memberID).
		Scan(&m.ID, &m.CalendarID, &m.Permissions, &m.Email, &m.AddressID, &m.Name, &m.Description, &m.Color, &m.Display, &m.Priority, &m.Flags)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ---------- Timezone Operations ----------

func (s *Store) ListTimezones() ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM timezones ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var tz string
		if err := rows.Scan(&tz); err != nil {
			return nil, err
		}
		result = append(result, tz)
	}
	return result, rows.Err()
}

// ---------- Helpers ----------

func nonNilSlice(s []models.CalendarEventData) []models.CalendarEventData {
	if s == nil {
		return []models.CalendarEventData{}
	}
	return s
}

func nonNilInt64Slice(s []int64) []int64 {
	if s == nil {
		return []int64{}
	}
	return s
}

// parseTriggerToSeconds parses an ISO 8601 duration trigger string like "-PT15M"
// and returns the offset in seconds (negative means before the event).
func parseTriggerToSeconds(trigger string) int {
	if trigger == "" {
		return 0
	}

	isNegative := false
	s := trigger
	if s[0] == '-' {
		isNegative = true
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}

	// Expect 'P'
	if len(s) == 0 || s[0] != 'P' {
		return 0
	}
	s = s[1:]

	weeks, days, hours, minutes, seconds := 0, 0, 0, 0, 0
	inTime := false
	num := 0
	hasNum := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == 'T' {
			inTime = true
			num = 0
			hasNum = false
			continue
		}
		if ch >= '0' && ch <= '9' {
			num = num*10 + int(ch-'0')
			hasNum = true
			continue
		}
		if !hasNum {
			continue
		}
		switch {
		case ch == 'W':
			weeks = num
		case ch == 'D':
			days = num
		case ch == 'H' && inTime:
			hours = num
		case ch == 'M' && inTime:
			minutes = num
		case ch == 'S' && inTime:
			seconds = num
		}
		num = 0
		hasNum = false
	}

	total := weeks*7*24*3600 + days*24*3600 + hours*3600 + minutes*60 + seconds
	if isNegative {
		return -total
	}
	return total
}

