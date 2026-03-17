package models

// Nullable represents a value that can be null in JSON.
type Nullable[T any] *T

// --- Constants ---

type CalendarType int

const (
	CalendarTypePersonal   CalendarType = 0
	CalendarTypeSubscribed CalendarType = 1
	CalendarTypeHolidays   CalendarType = 2
)

type CalendarDisplay int

const (
	CalendarDisplayHidden CalendarDisplay = 0
	CalendarDisplayShown  CalendarDisplay = 1
)

type NotificationTypeAPI int

const (
	NotificationTypeDevice NotificationTypeAPI = 0
	NotificationTypeEmail  NotificationTypeAPI = 1
)

type AttendeeStatusAPI int

const (
	AttendeeStatusNeedsAction AttendeeStatusAPI = 0
	AttendeeStatusAccepted    AttendeeStatusAPI = 1
	AttendeeStatusDeclined    AttendeeStatusAPI = 2
	AttendeeStatusTentative   AttendeeStatusAPI = 3
)

type CalendarCardType int

const (
	CardTypeClear     CalendarCardType = 0
	CardTypeEncrypted CalendarCardType = 1
	CardTypeSigned    CalendarCardType = 2
	CardTypeBoth      CalendarCardType = 3
)

type DeletionReason int

const (
	DeletionReasonNormal         DeletionReason = 0
	DeletionReasonChangeCalendar DeletionReason = 1
)

type CalendarEventsQueryType int

const (
	QueryPartDayInsideWindow CalendarEventsQueryType = 0
	QueryPartDayBeforeWindow CalendarEventsQueryType = 1
	QueryFullDayInsideWindow CalendarEventsQueryType = 2
	QueryFullDayBeforeWindow CalendarEventsQueryType = 3
)

type SettingsView int

const (
	SettingsViewDay   SettingsView = 0
	SettingsViewWeek  SettingsView = 1
	SettingsViewMonth SettingsView = 2
	SettingsViewYear  SettingsView = 3
)

// --- Core Models ---

// Calendar represents a calendar container.
type Calendar struct {
	ID   string       `json:"ID"`
	Type CalendarType `json:"Type"`
}

// CalendarOwner represents the owner of a calendar.
type CalendarOwner struct {
	Email string `json:"Email"`
}

// CalendarMember represents a member of a calendar.
type CalendarMember struct {
	ID          string          `json:"ID"`
	Permissions int             `json:"Permissions"`
	Email       string          `json:"Email"`
	AddressID   string          `json:"AddressID"`
	CalendarID  string          `json:"CalendarID"`
	Name        string          `json:"Name"`
	Description string          `json:"Description"`
	Color       string          `json:"Color"`
	Display     CalendarDisplay `json:"Display"`
	Priority    int             `json:"Priority"`
	Flags       int             `json:"Flags"`
}

// CalendarWithMembers represents a calendar with its owner and members.
type CalendarWithMembers struct {
	Calendar
	Owner   CalendarOwner    `json:"Owner"`
	Members []CalendarMember `json:"Members"`
}

// VisualCalendar extends CalendarWithMembers with display properties.
type VisualCalendar struct {
	CalendarWithMembers
	Name        string          `json:"Name"`
	Description string          `json:"Description"`
	Color       string          `json:"Color"`
	Display     CalendarDisplay `json:"Display"`
	Email       string          `json:"Email"`
	Flags       int             `json:"Flags"`
	Permissions int             `json:"Permissions"`
	Priority    int             `json:"Priority"`
}

// --- Event Models ---

// CalendarEventData represents encrypted event content.
type CalendarEventData struct {
	Type      CalendarCardType `json:"Type"`
	Data      string           `json:"Data"`
	Signature *string          `json:"Signature"`
	Author    string           `json:"Author"`
}

// Attendee represents a calendar event attendee.
type Attendee struct {
	ID         string            `json:"ID"`
	Token      string            `json:"Token"`
	Status     AttendeeStatusAPI `json:"Status"`
	UpdateTime *int64            `json:"UpdateTime"`
}

// AttendeesInfo contains attendee data for an event.
type AttendeesInfo struct {
	Attendees     []Attendee `json:"Attendees"`
	MoreAttendees int        `json:"MoreAttendees"`
}

// CalendarNotificationSettings represents notification configuration.
type CalendarNotificationSettings struct {
	Type    NotificationTypeAPI `json:"Type"`
	Trigger string              `json:"Trigger"`
}

// CalendarEvent represents a complete calendar event.
type CalendarEvent struct {
	// Shared data
	ID                    string  `json:"ID"`
	SharedEventID         string  `json:"SharedEventID"`
	CalendarID            string  `json:"CalendarID"`
	CreateTime            int64   `json:"CreateTime"`
	ModifyTime            int64   `json:"ModifyTime"`
	Permissions           int     `json:"Permissions"`
	IsOrganizer           int     `json:"IsOrganizer"`
	IsProtonProtonInvite  int     `json:"IsProtonProtonInvite"`
	IsPersonalSingleEdit  bool    `json:"IsPersonalSingleEdit"`
	Author                string  `json:"Author"`
	Color                 *string `json:"Color"`
	// Blob data
	CalendarKeyPacket *string                         `json:"CalendarKeyPacket"`
	CalendarEvents    []CalendarEventData             `json:"CalendarEvents"`
	SharedKeyPacket   *string                         `json:"SharedKeyPacket"`
	AddressKeyPacket  *string                         `json:"AddressKeyPacket"`
	AddressID         *string                         `json:"AddressID"`
	SharedEvents      []CalendarEventData             `json:"SharedEvents"`
	Notifications     *[]CalendarNotificationSettings `json:"Notifications"`
	AttendeesEvents   []CalendarEventData             `json:"AttendeesEvents"`
	AttendeesInfo     AttendeesInfo                   `json:"AttendeesInfo"`
	// Metadata
	StartTime      int64   `json:"StartTime"`
	StartTimezone  string  `json:"StartTimezone"`
	EndTime        int64   `json:"EndTime"`
	EndTimezone    string  `json:"EndTimezone"`
	FullDay        int     `json:"FullDay"`
	RRule          *string `json:"RRule"`
	UID            string  `json:"UID"`
	RecurrenceID   *int64  `json:"RecurrenceID"`
	Exdates        []int64 `json:"Exdates"`
}

// --- Settings Models ---

// CalendarSettings represents per-calendar settings.
type CalendarSettings struct {
	ID                          string                         `json:"ID"`
	CalendarID                  string                         `json:"CalendarID"`
	DefaultEventDuration        int                            `json:"DefaultEventDuration"`
	DefaultPartDayNotifications []CalendarNotificationSettings `json:"DefaultPartDayNotifications"`
	DefaultFullDayNotifications []CalendarNotificationSettings `json:"DefaultFullDayNotifications"`
}

// CalendarUserSettings represents global calendar user settings.
type CalendarUserSettings struct {
	DefaultCalendarID        *string      `json:"DefaultCalendarID"`
	WeekLength               int          `json:"WeekLength"`
	DisplayWeekNumber        int          `json:"DisplayWeekNumber"`
	AutoDetectPrimaryTimezone int          `json:"AutoDetectPrimaryTimezone"`
	PrimaryTimezone          string       `json:"PrimaryTimezone"`
	DisplaySecondaryTimezone int          `json:"DisplaySecondaryTimezone"`
	SecondaryTimezone        *string      `json:"SecondaryTimezone"`
	ViewPreference           SettingsView `json:"ViewPreference"`
	InviteLocale             *string      `json:"InviteLocale"`
	AutoImportInvite         int          `json:"AutoImportInvite"`
}

// --- Alarm Model ---

// CalendarAlarm represents a triggered alarm.
type CalendarAlarm struct {
	ID         string `json:"ID"`
	CalendarID string `json:"CalendarID"`
	EventID    string `json:"EventID"`
	Occurrence int64  `json:"Occurrence"`
	Trigger    string `json:"Trigger"`
	IsEmail    bool   `json:"IsEmail"`
}

// --- API Request/Response Types ---

// ApiResponse is the base response structure.
type ApiResponse struct {
	Code int `json:"Code"`
}

// CalendarCreateArguments is the request body for creating a calendar.
type CalendarCreateArguments struct {
	Name        string          `json:"Name"`
	Description string          `json:"Description"`
	Color       string          `json:"Color"`
	Display     CalendarDisplay `json:"Display"`
	AddressID   string          `json:"AddressID"`
	URL         string          `json:"URL,omitempty"`
	IsImport    int             `json:"IsImport,omitempty"`
}

// CalendarUpdateArguments is the request body for updating a calendar.
type CalendarUpdateArguments struct {
	Name        *string          `json:"Name,omitempty"`
	Description *string          `json:"Description,omitempty"`
	Color       *string          `json:"Color,omitempty"`
	Display     *CalendarDisplay `json:"Display,omitempty"`
}

// CalendarEventsQuery is the query parameters for listing events.
type CalendarEventsQuery struct {
	Start        int64                   `json:"Start"`
	End          int64                   `json:"End"`
	Timezone     string                  `json:"Timezone"`
	Type         CalendarEventsQueryType `json:"Type"`
	MetaDataOnly int                     `json:"MetaDataOnly,omitempty"`
	Page         int                     `json:"Page,omitempty"`
	PageSize     int                     `json:"PageSize,omitempty"`
}

// CreateOrUpdateCalendarEventData is the request body for creating/updating an event.
type CreateOrUpdateCalendarEventData struct {
	Permissions          int                              `json:"Permissions"`
	IsOrganizer          *int                             `json:"IsOrganizer,omitempty"`
	IsPersonalSingleEdit *bool                            `json:"IsPersonalSingleEdit,omitempty"`
	CalendarKeyPacket    *string                          `json:"CalendarKeyPacket,omitempty"`
	CalendarEventContent []CalendarEventData              `json:"CalendarEventContent,omitempty"`
	SharedKeyPacket      *string                          `json:"SharedKeyPacket,omitempty"`
	SharedEventContent   []CalendarEventData              `json:"SharedEventContent,omitempty"`
	AddressKeyPacket     *string                          `json:"AddressKeyPacket,omitempty"`
	AddressID            *string                          `json:"AddressID,omitempty"`
	Notifications        *[]CalendarNotificationSettings  `json:"Notifications"`
	Color                *string                          `json:"Color"`
	AttendeesEventContent []CalendarEventData             `json:"AttendeesEventContent,omitempty"`
	Attendees            []AttendeeInput                  `json:"Attendees,omitempty"`
	// Event metadata
	StartTime     int64   `json:"StartTime"`
	StartTimezone string  `json:"StartTimezone"`
	EndTime       int64   `json:"EndTime"`
	EndTimezone   string  `json:"EndTimezone"`
	FullDay       int     `json:"FullDay"`
	RRule         *string `json:"RRule,omitempty"`
	UID           string  `json:"UID,omitempty"`
	RecurrenceID  *int64  `json:"RecurrenceID,omitempty"`
	Exdates       []int64 `json:"Exdates,omitempty"`
}

// AttendeeInput is the attendee data without server-generated fields.
type AttendeeInput struct {
	Token  string            `json:"Token"`
	Status AttendeeStatusAPI `json:"Status"`
}

// SyncEvent represents a single event operation in a sync request.
type SyncEvent struct {
	ID             string                           `json:"ID,omitempty"`
	Overwrite      *int                             `json:"Overwrite,omitempty"`
	Event          *CreateOrUpdateCalendarEventData `json:"Event,omitempty"`
	DeletionReason *DeletionReason                  `json:"DeletionReason,omitempty"`
}

// SyncMultipleEventsData is the request body for syncing multiple events.
type SyncMultipleEventsData struct {
	MemberID string      `json:"MemberID"`
	IsImport int         `json:"IsImport,omitempty"`
	Events   []SyncEvent `json:"Events"`
}

// SyncEventResponse represents the response for a single sync operation.
type SyncEventResponse struct {
	Index    int `json:"Index"`
	Response struct {
		Code  int            `json:"Code"`
		Event *CalendarEvent `json:"Event,omitempty"`
		Error string         `json:"Error,omitempty"`
	} `json:"Response"`
}

// SyncMultipleApiResponse is the response for syncing multiple events.
type SyncMultipleApiResponse struct {
	ApiResponse
	Responses []SyncEventResponse `json:"Responses"`
}

// UpdateAttendeeData is the request body for updating an attendee.
type UpdateAttendeeData struct {
	Status     AttendeeStatusAPI `json:"Status"`
	UpdateTime *int64            `json:"UpdateTime,omitempty"`
}

// UpdatePersonalEventData is the request body for updating personal event data.
type UpdatePersonalEventData struct {
	Notifications *[]CalendarNotificationSettings `json:"Notifications"`
	Color         *string                         `json:"Color"`
}

// CalendarSettingsUpdate is the request body for updating calendar settings.
type CalendarSettingsUpdate struct {
	DefaultEventDuration        *int                            `json:"DefaultEventDuration,omitempty"`
	DefaultPartDayNotifications *[]CalendarNotificationSettings `json:"DefaultPartDayNotifications,omitempty"`
	DefaultFullDayNotifications *[]CalendarNotificationSettings `json:"DefaultFullDayNotifications,omitempty"`
}

// CalendarUserSettingsUpdate is the request body for updating user settings.
type CalendarUserSettingsUpdate struct {
	DefaultCalendarID         *string       `json:"DefaultCalendarID,omitempty"`
	WeekLength                *int          `json:"WeekLength,omitempty"`
	DisplayWeekNumber         *int          `json:"DisplayWeekNumber,omitempty"`
	AutoDetectPrimaryTimezone *int          `json:"AutoDetectPrimaryTimezone,omitempty"`
	PrimaryTimezone           *string       `json:"PrimaryTimezone,omitempty"`
	DisplaySecondaryTimezone  *int          `json:"DisplaySecondaryTimezone,omitempty"`
	SecondaryTimezone         *string       `json:"SecondaryTimezone,omitempty"`
	ViewPreference            *SettingsView `json:"ViewPreference,omitempty"`
	InviteLocale              *string       `json:"InviteLocale,omitempty"`
	AutoImportInvite          *int          `json:"AutoImportInvite,omitempty"`
}

// QueryCalendarAlarms is the query parameters for listing alarms.
type QueryCalendarAlarms struct {
	Start    int64 `json:"Start"`
	End      int64 `json:"End"`
	PageSize int   `json:"PageSize"`
}

// CreateCalendarMemberData is the request body for adding a member.
type CreateCalendarMemberData struct {
	Email                string `json:"Email"`
	PassphraseKeyPacket  string `json:"PassphraseKeyPacket"`
	Permissions          int    `json:"Permissions"`
}

// UpdateCalendarMemberData is the request body for updating a member.
type UpdateCalendarMemberData struct {
	Permissions int             `json:"Permissions,omitempty"`
	Name        string          `json:"Name,omitempty"`
	Description string          `json:"Description,omitempty"`
	Color       string          `json:"Color,omitempty"`
	Display     CalendarDisplay `json:"Display,omitempty"`
}
