package router

import (
	"net/http"
	"strings"

	"github.com/codecrafter404/Proton-WebClients/backend/auth"
	"github.com/codecrafter404/Proton-WebClients/backend/handlers"
)

// New creates a new HTTP router with all calendar API routes.
func New(h *handlers.Handler, authMgr *auth.Manager) http.Handler {
	mux := http.NewServeMux()

	// We use a custom dispatcher because the standard ServeMux doesn't
	// support path parameters. We route based on method + path pattern.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		route(h, authMgr, w, r)
	})

	return mux
}

func route(h *handlers.Handler, authMgr *auth.Manager, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")

	// ── Auth routes ──────────────────────────────────────────────────

	// GET /challenge/v4/html (captcha/challenge iframe)
	if strings.HasPrefix(path, "/challenge/") {
		h.HandleChallenge(w, r)
		return
	}

	// POST /core/v4/auth/info  (SRP auth info)
	if path == "/core/v4/auth/info" && method == http.MethodPost {
		authMgr.HandleAuthInfo(w, r)
		return
	}

	// GET /core/v4/auth/modulus
	if path == "/core/v4/auth/modulus" && method == http.MethodGet {
		authMgr.HandleAuthModulus(w, r)
		return
	}

	// POST /core/v4/auth  (SRP login)
	// DELETE /core/v4/auth (logout)
	if path == "/core/v4/auth" {
		switch method {
		case http.MethodPost:
			authMgr.HandleLogin(w, r)
		case http.MethodDelete:
			authMgr.HandleLogout(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// POST /auth/refresh
	if path == "/auth/refresh" && method == http.MethodPost {
		authMgr.HandleRefresh(w, r)
		return
	}

	// POST /auth/v4/sessions
	if path == "/auth/v4/sessions" && method == http.MethodPost {
		authMgr.HandleUnauthSession(w, r)
		return
	}

	// POST /core/v4/auth/cookies
	if path == "/core/v4/auth/cookies" && method == http.MethodPost {
		authMgr.HandleAuthCookies(w, r)
		return
	}

	// GET /auth/v4/sessions/local
	if path == "/auth/v4/sessions/local" && method == http.MethodGet {
		h.HandleLocalSessions(w, r)
		return
	}

	// GET|PUT /auth/v4/sessions/local/key
	if path == "/auth/v4/sessions/local/key" {
		h.HandleLocalKey(w, r)
		return
	}

	// ── Core API routes ──────────────────────────────────────────────

	if path == "/core/v4/users" && method == http.MethodGet {
		h.HandleCoreUsers(w, r)
		return
	}
	if path == "/core/v4/addresses" && method == http.MethodGet {
		h.HandleCoreAddresses(w, r)
		return
	}
	if path == "/core/v4/keys/salts" && method == http.MethodGet {
		h.HandleCoreKeySalts(w, r)
		return
	}
	if path == "/core/v4/keys/setup" && method == http.MethodPost {
		h.HandleKeysSetup(w, r)
		return
	}
	if path == "/core/v4/keys" && method == http.MethodPost {
		h.HandleKeysSetup(w, r)
		return
	}
	if path == "/core/v4/settings" && method == http.MethodGet {
		h.HandleCoreSettings(w, r)
		return
	}
	if path == "/core/v4/settings/password" && method == http.MethodPut {
		h.HandleCatchAll(w, r)
		return
	}
	if path == "/core/v4/features" && method == http.MethodGet {
		h.HandleCoreFeatures(w, r)
		return
	}
	if path == "/core/v4/events/latest" && method == http.MethodGet {
		h.HandleCoreEventsLatest(w, r)
		return
	}
	if strings.HasPrefix(path, "/core/v4/events/") && method == http.MethodGet {
		h.HandleCoreEvents(w, r)
		return
	}
	if strings.HasPrefix(path, "/core/v5/events/") && method == http.MethodGet {
		h.HandleCoreEvents(w, r)
		return
	}
	if path == "/core/v4/organizations" && method == http.MethodGet {
		h.HandleOrganization(w, r)
		return
	}
	if path == "/core/v4/plans/default" && method == http.MethodGet {
		h.HandlePlansDefault(w, r)
		return
	}

	// ── Feature flags (Unleash) ─────────────────────────────────────

	if strings.HasPrefix(path, "/feature/v2/frontend") {
		if strings.Contains(path, "/metrics") {
			h.HandleFeatureMetrics(w, r)
		} else {
			h.HandleFeatureFlags(w, r)
		}
		return
	}

	// ── Mail settings ───────────────────────────────────────────────

	if path == "/mail/v4/settings" && method == http.MethodGet {
		h.HandleMailSettings(w, r)
		return
	}

	// ── Contacts ────────────────────────────────────────────────────

	if strings.HasPrefix(path, "/contacts/") {
		h.HandleContactEmails(w, r)
		return
	}

	if strings.HasPrefix(path, "/core/v4/payments/") {
		switch {
		case strings.HasSuffix(path, "/methods"):
			h.HandlePaymentMethods(w, r)
		case strings.HasSuffix(path, "/status"):
			h.HandlePaymentStatus(w, r)
		case strings.HasSuffix(path, "/subscription"):
			h.HandleSubscription(w, r)
		default:
			h.HandleCatchAll(w, r)
		}
		return
	}

	// Payment routes (v4/v5 without /core/ prefix)
	if strings.HasPrefix(path, "/payments/") {
		switch {
		case strings.HasSuffix(path, "/plans/default"):
			h.HandlePaymentPlansDefault(w, r)
		case strings.HasSuffix(path, "/plans"):
			h.HandlePaymentPlans(w, r)
		case strings.HasSuffix(path, "/status"):
			h.HandlePaymentStatusV5(w, r)
		case strings.HasSuffix(path, "/methods"):
			h.HandlePaymentMethods(w, r)
		case strings.HasSuffix(path, "/subscription"):
			h.HandleSubscription(w, r)
		default:
			h.HandleCatchAll(w, r)
		}
		return
	}

	// ── Calendar settings ────────────────────────────────────────────

	if path == "/settings/calendar" {
		switch method {
		case http.MethodGet:
			h.GetUserSettings(w, r)
		case http.MethodPut:
			h.UpdateUserSettings(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if path == "/calendar/v1/settings" && method == http.MethodGet {
		h.HandleCalendarSettings(w, r)
		return
	}

	// ── Calendar API routes ──────────────────────────────────────────

	// Handle /calendar/v2/{calendarID}/bootstrap
	if len(parts) >= 4 && parts[0] == "calendar" && parts[1] == "v2" && parts[3] == "bootstrap" && method == http.MethodGet {
		h.GetCalendarBootstrap(w, r)
		return
	}

	if len(parts) < 2 || parts[0] != "calendar" || parts[1] != "v1" {
		// Unknown route – try catch-all for /core/ prefixed paths
		if strings.HasPrefix(path, "/core/") || strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/feature/") || strings.HasPrefix(path, "/settings/") || strings.HasPrefix(path, "/domains/") || strings.HasPrefix(path, "/payments/") {
			h.HandleCatchAll(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}

	// /calendar/v1
	if len(parts) == 2 {
		switch method {
		case http.MethodGet:
			h.ListCalendars(w, r)
		case http.MethodPost:
			h.CreateCalendar(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// /calendar/v1/timezones
	if len(parts) == 3 && parts[2] == "timezones" {
		h.GetTimezones(w, r)
		return
	}

	// /calendar/v1/bookings and /calendar/v1/booking
	if len(parts) == 3 && (parts[2] == "bookings" || parts[2] == "booking") {
		if method == http.MethodGet {
			h.HandleCatchAll(w, r)
			return
		}
	}

	// /calendar/v1/directory
	if len(parts) == 3 && parts[2] == "directory" {
		h.GetDirectory(w, r)
		return
	}

	// /calendar/v1/events?UID=...
	if len(parts) == 3 && parts[2] == "events" {
		if method == http.MethodGet {
			h.GetEventByUID(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Routes with calendarID: /calendar/v1/{calendarID}/...
	if len(parts) >= 3 {
		calendarID := parts[2]
		_ = calendarID

		// /calendar/v1/{calendarID}
		if len(parts) == 3 {
			switch method {
			case http.MethodGet:
				h.GetCalendar(w, r)
			case http.MethodPut:
				h.UpdateCalendar(w, r)
			case http.MethodDelete:
				h.DeleteCalendar(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		resource := parts[3]

		switch resource {
		case "keys":
			// /calendar/v1/{calendarID}/keys
			if len(parts) == 4 && method == http.MethodPost {
				h.SetupCalendarKeys(w, r)
				return
			}

		case "modelevents":
			// /calendar/v1/{calendarID}/modelevents/latest
			if len(parts) == 5 && parts[4] == "latest" && method == http.MethodGet {
				h.HandleModelEventsLatest(w, r)
				return
			}

		case "settings":
			// /calendar/v1/{calendarID}/settings
			if len(parts) == 4 {
				switch method {
				case http.MethodGet:
					h.GetCalendarSettings(w, r)
				case http.MethodPut:
					h.UpdateCalendarSettings(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

		case "members":
			if len(parts) == 4 {
				// /calendar/v1/{calendarID}/members
				switch method {
				case http.MethodGet:
					h.ListMembers(w, r)
				case http.MethodPost:
					h.AddMember(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}
			if len(parts) == 5 {
				// /calendar/v1/{calendarID}/members/{memberID}
				switch method {
				case http.MethodPut:
					h.UpdateMember(w, r)
				case http.MethodDelete:
					h.RemoveMember(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

		case "alarms":
			// /calendar/v1/{calendarID}/alarms
			if len(parts) == 4 && method == http.MethodGet {
				h.ListAlarms(w, r)
				return
			}
			// /calendar/v1/{calendarID}/alarms/{alarmID}
			if len(parts) == 5 && method == http.MethodGet {
				h.GetAlarm(w, r)
				return
			}

		case "events":
			if len(parts) == 4 {
				// /calendar/v1/{calendarID}/events
				switch method {
				case http.MethodGet:
					h.ListEvents(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

			if len(parts) == 5 {
				subResource := parts[4]

				switch subResource {
				case "count":
					// /calendar/v1/{calendarID}/events/count
					if method == http.MethodGet {
						h.GetEventsCount(w, r)
						return
					}
				case "ids":
					// /calendar/v1/{calendarID}/events/ids
					if method == http.MethodGet {
						h.GetEventIDs(w, r)
						return
					}
				case "sync":
					// /calendar/v1/{calendarID}/events/sync
					if method == http.MethodPut {
						h.SyncEvents(w, r)
						return
					}
				default:
					// /calendar/v1/{calendarID}/events/{eventID}
					switch method {
					case http.MethodGet:
						h.GetEvent(w, r)
					case http.MethodDelete:
						h.DeleteEvent(w, r)
					default:
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
					return
				}
			}

			if len(parts) == 6 {
				subResource := parts[5]

				switch subResource {
				case "personal":
					// /calendar/v1/{calendarID}/events/{eventID}/personal
					if method == http.MethodPut {
						h.UpdateEventPersonal(w, r)
						return
					}
				case "attendees":
					// /calendar/v1/{calendarID}/events/{eventID}/attendees
					if method == http.MethodGet {
						h.GetAttendees(w, r)
						return
					}
				}
			}

			if len(parts) == 7 && parts[5] == "attendees" {
				// /calendar/v1/{calendarID}/events/{eventID}/attendees/{attendeeID}
				if method == http.MethodPut {
					h.UpdateAttendee(w, r)
					return
				}
			}
		}
	}

	http.NotFound(w, r)
}
