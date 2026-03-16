package handlers

import (
	"net/http"
)

// handleCoreUsers handles GET /core/v4/users
func (h *Handler) HandleCoreUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"User": map[string]interface{}{
			"ID":          "user-1",
			"Name":        "proton",
			"DisplayName": "Proton User",
			"Email":       "proton@proton.local",
			"Type":        1,
			"MaxSpace":    1073741824,
			"MaxUpload":   26214400,
			"UsedSpace":   0,
			"Role":        2,
			"Private":     1,
			"Subscribed":  0,
			"Services":    1,
			"Delinquent":  0,
			"Currency":    "USD",
			"Credit":      0,
			"MnemonicStatus": 0,
			"Keys":           []interface{}{},
			"ToMigrate":      0,
			"AccountRecovery": nil,
		},
	})
}

// handleCoreAddresses handles GET /core/v4/addresses
func (h *Handler) HandleCoreAddresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Addresses": []map[string]interface{}{
			{
				"ID":          "address-1",
				"DomainID":    "domain-1",
				"Email":       "proton@proton.local",
				"Send":        1,
				"Receive":     1,
				"Status":      1,
				"Type":        1,
				"Order":       1,
				"DisplayName": "Proton User",
				"Signature":   "",
				"HasKeys":     0,
				"Keys":        []interface{}{},
			},
		},
	})
}

// handleCoreKeySalts handles GET /core/v4/keys/salts
func (h *Handler) HandleCoreKeySalts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"KeySalts": []interface{}{},
	})
}

// handleCoreSettings handles GET /core/v4/settings
func (h *Handler) HandleCoreSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"UserSettings": map[string]interface{}{
			"Email": map[string]interface{}{
				"Value":  "proton@proton.local",
				"Status": 1,
				"Notify": 0,
				"Reset":  0,
			},
			"Phone":          map[string]interface{}{"Value": "", "Status": 0, "Notify": 0, "Reset": 0},
			"Password":       map[string]interface{}{"Mode": 1, "ExpirationTime": nil},
			"2FA":            map[string]interface{}{"Enabled": 0, "Allowed": 3, "ExpirationTime": nil, "RegisteredKeys": []interface{}{}},
			"News":           0,
			"Locale":         "en_US",
			"LogAuth":        1,
			"InvoiceText":    "",
			"Density":        0,
			"Theme":          "",
			"ThemeType":      1,
			"WeekStart":      1,
			"DateFormat":     0,
			"TimeFormat":     0,
			"Welcome":        0,
			"WelcomeFlag":    0,
			"EarlyAccess":    0,
			"Flags":          map[string]interface{}{},
			"Referral":       nil,
			"DeviceRecovery": 0,
			"Telemetry":      0,
			"CrashReports":   0,
			"HideSidePanel":  0,
			"HighSecurity": map[string]interface{}{
				"Eligible": 0,
				"Value":    0,
			},
			"SessionAccountRecovery": 0,
		},
	})
}

// handleCoreEventsLatest handles GET /core/v4/events/latest
func (h *Handler) HandleCoreEventsLatest(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":    1000,
		"EventID": "event-0",
	})
}

// handleCoreEvents handles GET /core/v4/events/{eventID}
func (h *Handler) HandleCoreEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":    1000,
		"EventID": "event-0",
		"Refresh": 0,
		"More":    0,
		"Calendars":         []interface{}{},
		"CalendarKeys":      []interface{}{},
		"CalendarAlarms":    []interface{}{},
		"Addresses":         []interface{}{},
		"Members":           []interface{}{},
		"MessageCounts":     []interface{}{},
		"ConversationCounts": []interface{}{},
		"Notices":           []interface{}{},
		"UsedSpace":         0,
	})
}

// handleCoreFeatures handles GET /core/v4/features
func (h *Handler) HandleCoreFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"Features": []interface{}{},
	})
}

// handleCalendarSettings handles GET /calendar/v1/settings
func (h *Handler) HandleCalendarSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"CalendarUserSettings": map[string]interface{}{
			"DefaultCalendarID":  nil,
			"WeekLength":         7,
			"DisplayWeekNumber":  0,
			"AutoDetectPrimaryTimezone": 1,
			"PrimaryTimezone":    "UTC",
			"DisplaySecondaryTimezone": 0,
			"SecondaryTimezone":  nil,
			"ViewPreference":     1,
			"InviteLocale":       nil,
			"AutoImportInvite":   0,
		},
	})
}

// handleSessions handles POST /auth/v4/sessions
func (h *Handler) HandleSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":         1000,
		"UID":          "",
		"AccessToken":  "",
		"RefreshToken": "",
		"ExpiresIn":    0,
		"TokenType":    "Bearer",
		"Scope":        "full",
	})
}

// handleAuthCookies handles POST /core/v4/auth/cookies
func (h *Handler) HandleAuthCookies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// handleLocalSessions handles GET /auth/v4/sessions/local
func (h *Handler) HandleLocalSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":     1000,
		"Sessions": []interface{}{},
	})
}

// handleLocalKey handles GET/PUT /auth/v4/sessions/local/key
func (h *Handler) HandleLocalKey(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"Code":      1000,
			"ClientKey": "",
		})
	case http.MethodPut:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"Code": 1000,
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleOrganization handles GET /core/v4/organizations
func (h *Handler) HandleOrganization(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Organization": map[string]interface{}{
			"Name":        "",
			"DisplayName": "",
			"PlanName":    "free",
			"VPNPlanName": "",
			"TwoFactorRequired": 0,
			"MaxMembers":  1,
			"MaxSpace":    1073741824,
			"UsedMembers": 1,
			"UsedSpace":   0,
			"Features":    0,
			"Flags":       0,
		},
	})
}

// handlePlansDefault handles GET /core/v4/plans/default
func (h *Handler) HandlePlansDefault(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Plans": []interface{}{},
	})
}

// handlePaymentMethods handles GET /core/v4/payments/methods
func (h *Handler) HandlePaymentMethods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":           1000,
		"PaymentMethods": []interface{}{},
	})
}

// handlePaymentStatus handles GET /core/v4/payments/status
func (h *Handler) HandlePaymentStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Stripe":  true,
		"Paypal":  false,
		"Apple":   false,
		"Cash":    false,
		"Bitcoin": false,
	})
}

// handleSubscription handles GET /core/v4/payments/subscription
func (h *Handler) HandleSubscription(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Subscription": map[string]interface{}{
			"ID":           "sub-1",
			"InvoiceID":    "",
			"Cycle":        1,
			"PeriodStart":  0,
			"PeriodEnd":    0,
			"CouponCode":   nil,
			"Currency":     "USD",
			"Amount":       0,
			"Plans":        []interface{}{},
		},
		"UpcomingSubscription": nil,
	})
}

// handleCatchAll returns a generic success response for unhandled endpoints.
func (h *Handler) HandleCatchAll(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// HandleFeatureFlags returns an empty Unleash feature toggles response.
func (h *Handler) HandleFeatureFlags(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"toggles": []interface{}{},
	})
}

// HandleFeatureMetrics accepts Unleash client metrics.
func (h *Handler) HandleFeatureMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
	})
}

// HandleChallenge returns a minimal challenge HTML page.
// The Proton frontend loads this in an iframe for anti-bot measures.
// The ChallengeFrame component expects these message stages:
//   init -> (parent sends load) -> onload -> (parent sends env.loaded + submit.broadcast) -> child.message.data
func (h *Handler) HandleChallenge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html><head><script>
(function() {
  var name = new URLSearchParams(window.location.search).get('Name') || 'challenge';

  window.addEventListener('message', function(e) {
    if (!e.data || !e.data.type) return;

    if (e.data.type === 'load') {
      // Stage 2: parent sent styles/config, acknowledge load complete
      window.parent.postMessage({type: 'onload'}, e.origin);
    }

    if (e.data.type === 'submit.broadcast') {
      // Stage 3: parent requests challenge result
      window.parent.postMessage({
        type: 'child.message.data',
        data: {
          id: name,
          fingerprint: ''
        }
      }, e.origin);
    }
  });

  // Stage 1: signal initialization to parent
  if (window.parent) {
    window.parent.postMessage({type: 'init'}, '*');
  }
})();
</script></head><body></body></html>`))
}

// HandlePaymentPlans handles GET /payments/v4/plans and /payments/v5/plans
func (h *Handler) HandlePaymentPlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code":  1000,
		"Plans": []interface{}{},
	})
}

// HandlePaymentPlansDefault handles GET /payments/v4/plans/default
func (h *Handler) HandlePaymentPlansDefault(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"Plans": map[string]interface{}{
			"free": map[string]interface{}{
				"ID":        "free",
				"Type":      0,
				"Name":      "free",
				"Title":     "Free",
				"MaxMembers": 1,
				"MaxSpace":   1073741824,
				"Features":   0,
				"State":      1,
			},
		},
	})
}

// HandlePaymentStatusV5 handles GET /payments/v5/status
func (h *Handler) HandlePaymentStatusV5(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"Code": 1000,
		"VendorStates": map[string]interface{}{
			"Card":    true,
			"Paypal":  false,
			"Apple":   false,
			"Cash":    true,
			"Bitcoin": false,
			"Google":  false,
		},
		"CountryCode": "US",
		"State":       nil,
		"ZipCode":     nil,
	})
}


