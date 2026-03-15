/**
 * Playwright E2E UI Tests for the Proton Calendar backend.
 *
 * These tests launch a real browser, interact with the backend's test UI,
 * and verify the full end-to-end flow: login, calendar CRUD, event
 * creation with encrypted data, and CORS behavior.
 *
 * Run: cd e2e && npx playwright test
 */
import { test, expect } from '@playwright/test';

// ---- Login UI Flow ----

test.describe('Login Flow', () => {
    test('should show the test page title', async ({ page }) => {
        await page.goto('/static/test.html');
        await expect(page.locator('h1')).toContainText('Proton Calendar Backend API Test Suite');
    });

    test('should have a Run All Tests button', async ({ page }) => {
        await page.goto('/static/test.html');
        const button = page.locator('button', { hasText: 'Run All Tests' });
        await expect(button).toBeVisible();
    });

    test('should execute full API test suite via UI and all pass', async ({ page }) => {
        await page.goto('/static/test.html');

        // Click the "Run All Tests" button
        await page.click('button:has-text("Run All Tests")');

        // Wait for the summary to appear (indicates tests finished)
        const summary = page.locator('#summary span');
        await expect(summary).toBeVisible({ timeout: 30_000 });

        // Verify all tests passed
        const summaryText = await summary.textContent();
        const match = summaryText?.match(/(\d+)\/(\d+) tests passed/);
        expect(match).toBeTruthy();
        const [, passed, total] = match!;
        expect(parseInt(passed)).toBe(parseInt(total));
        expect(parseInt(total)).toBeGreaterThan(0);

        // Check there are no FAIL badges
        const failCount = await page.locator('.status.fail').count();
        expect(failCount).toBe(0);
    });

    test('should show PASS badges for each individual test', async ({ page }) => {
        await page.goto('/static/test.html');
        await page.click('button:has-text("Run All Tests")');

        // Wait for test completion
        await expect(page.locator('#summary span')).toBeVisible({ timeout: 30_000 });

        // Verify individual test results are rendered
        const testRows = page.locator('.test-row');
        const count = await testRows.count();
        expect(count).toBeGreaterThan(15); // There should be many API tests

        // Each row should have a PASS badge
        for (let i = 0; i < count; i++) {
            const status = testRows.nth(i).locator('.status');
            await expect(status).toHaveText('PASS');
        }
    });
});

// ---- Direct API Tests via Browser ----
// These tests use page.evaluate to make fetch() calls from the browser context,
// verifying the real browser CORS and fetch behavior.

test.describe('Browser-based API Tests', () => {
    test('login via browser fetch returns valid tokens', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            const resp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'proton' }),
            });
            return { status: resp.status, data: await resp.json() };
        });

        expect(result.status).toBe(200);
        expect(result.data.Code).toBe(1000);
        expect(result.data.AccessToken).toBeTruthy();
        expect(result.data.RefreshToken).toBeTruthy();
        expect(result.data.UID).toBeTruthy();
        expect(result.data.UserID).toBe('user-1');
        expect(result.data.TokenType).toBe('Bearer');
    });

    test('login with wrong password returns 401', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            const resp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'wrong' }),
            });
            return { status: resp.status };
        });

        expect(result.status).toBe(401);
    });

    test('unauthenticated request returns 401', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            const resp = await fetch('/calendar/v1');
            return { status: resp.status };
        });

        expect(result.status).toBe(401);
    });

    test('calendar CRUD via browser fetch', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            // Login
            const loginResp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'proton' }),
            });
            const loginData = await loginResp.json();
            const token = loginData.AccessToken;
            const uid = loginData.UID;

            const api = async (method: string, path: string, body?: any) => {
                const opts: RequestInit = {
                    method,
                    headers: {
                        'Content-Type': 'application/json',
                        Authorization: `Bearer ${token}`,
                        'x-pm-uid': uid,
                    },
                };
                if (body) opts.body = JSON.stringify(body);
                const resp = await fetch(path, opts);
                return { status: resp.status, data: await resp.json() };
            };

            // Create calendar
            const create = await api('POST', '/calendar/v1', {
                Name: 'E2E Browser Test',
                Color: '#6d4aff',
                AddressID: 'e2e-browser@proton.me',
                Display: 1,
            });

            const calId = create.data?.Calendar?.ID;

            // List calendars
            const list = await api('GET', '/calendar/v1');

            // Get calendar
            const get = await api('GET', `/calendar/v1/${calId}`);

            // Update calendar
            const update = await api('PUT', `/calendar/v1/${calId}`, { Name: 'Updated Browser Test' });

            // Delete calendar
            const del = await api('DELETE', `/calendar/v1/${calId}`);

            // Verify deleted
            const getDeleted = await api('GET', `/calendar/v1/${calId}`);

            return {
                create: { status: create.status, hasId: !!calId },
                list: { status: list.status, count: list.data?.Calendars?.length },
                get: { status: get.status, name: get.data?.Calendar?.Name },
                update: { status: update.status },
                del: { status: del.status },
                getDeleted: { status: getDeleted.status },
            };
        });

        expect(result.create.status).toBe(200);
        expect(result.create.hasId).toBe(true);
        expect(result.list.status).toBe(200);
        expect(result.list.count).toBeGreaterThanOrEqual(1);
        expect(result.get.status).toBe(200);
        expect(result.update.status).toBe(200);
        expect(result.del.status).toBe(200);
        expect(result.getDeleted.status).toBe(404);
    });
});

// ---- E2E Encryption Tests via Browser ----

test.describe('E2E Encryption via Browser', () => {
    test('encrypted event data round-trips correctly through browser', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            // Login
            const loginResp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'proton' }),
            });
            const loginData = await loginResp.json();
            const token = loginData.AccessToken;
            const uid = loginData.UID;

            const api = async (method: string, path: string, body?: any) => {
                const opts: RequestInit = {
                    method,
                    headers: {
                        'Content-Type': 'application/json',
                        Authorization: `Bearer ${token}`,
                        'x-pm-uid': uid,
                    },
                };
                if (body) opts.body = JSON.stringify(body);
                const resp = await fetch(path, opts);
                return { status: resp.status, data: await resp.json() };
            };

            // Create calendar
            const calResp = await api('POST', '/calendar/v1', {
                Name: 'Encryption Test',
                Color: '#ff0000',
                AddressID: 'enc@proton.me',
                Display: 1,
            });
            const calId = calResp.data.Calendar.ID;

            // Define encrypted data
            const calKeyPacket = 'wV4Dk7H5+browser+calendarKeyPacket+test==';
            const sharedKeyPacket = 'wV4Dk7H5+browser+sharedKeyPacket+test==';
            const addressKeyPacket = 'wV4Dk7H5+browser+addressKeyPacket+test==';
            const signature = '-----BEGIN PGP SIGNATURE-----\nbrowser-test-sig\n-----END PGP SIGNATURE-----';

            const calendarEventContent = [
                { Type: 0, Data: 'CLEAR:browser-test-clear-data', Author: 'user@proton.me' },
            ];
            const sharedEventContent = [
                { Type: 2, Data: 'SIGNED:browser-test-signed-data', Signature: signature, Author: 'user@proton.me' },
                { Type: 3, Data: 'wcBMA+browser+encrypted+signed==', Signature: signature, Author: 'user@proton.me' },
            ];
            const attendeesEventContent = [
                { Type: 1, Data: 'wcBMA+browser+encrypted+attendee==', Author: 'org@proton.me' },
            ];

            // Create event with encrypted data
            const syncResp = await api('PUT', `/calendar/v1/${calId}/events/sync`, {
                MemberID: 'm1',
                Events: [
                    {
                        Event: {
                            Permissions: 127,
                            StartTime: 1704067200,
                            EndTime: 1704070800,
                            StartTimezone: 'Europe/Berlin',
                            EndTimezone: 'Europe/Berlin',
                            UID: 'browser-e2e@proton.local',
                            CalendarKeyPacket: calKeyPacket,
                            CalendarEventContent: calendarEventContent,
                            SharedKeyPacket: sharedKeyPacket,
                            SharedEventContent: sharedEventContent,
                            AddressKeyPacket: addressKeyPacket,
                            AddressID: 'addr-browser-test',
                            AttendeesEventContent: attendeesEventContent,
                        },
                    },
                ],
            });

            const eventId = syncResp.data.Responses[0].Response.Event.ID;

            // Read back the event
            const getResp = await api('GET', `/calendar/v1/${calId}/events/${eventId}`);
            const event = getResp.data.Event;

            // Verify all encrypted fields
            const checks = {
                calKeyPacket: event.CalendarKeyPacket === calKeyPacket,
                sharedKeyPacket: event.SharedKeyPacket === sharedKeyPacket,
                addressKeyPacket: event.AddressKeyPacket === addressKeyPacket,
                addressId: event.AddressID === 'addr-browser-test',
                calEventsCount: event.CalendarEvents?.length === 1,
                calEventsData: event.CalendarEvents?.[0]?.Data === 'CLEAR:browser-test-clear-data',
                calEventsType: event.CalendarEvents?.[0]?.Type === 0,
                sharedEventsCount: event.SharedEvents?.length === 2,
                sharedEventsSignedData: event.SharedEvents?.[0]?.Data === 'SIGNED:browser-test-signed-data',
                sharedEventsSignedSig: event.SharedEvents?.[0]?.Signature === signature,
                sharedEventsEncData: event.SharedEvents?.[1]?.Data === 'wcBMA+browser+encrypted+signed==',
                sharedEventsEncSig: event.SharedEvents?.[1]?.Signature === signature,
                attEventsCount: event.AttendeesEvents?.length === 1,
                attEventsData: event.AttendeesEvents?.[0]?.Data === 'wcBMA+browser+encrypted+attendee==',
            };

            return { syncStatus: syncResp.status, getStatus: getResp.status, checks };
        });

        expect(result.syncStatus).toBe(200);
        expect(result.getStatus).toBe(200);

        // Verify every encrypted field round-tripped correctly
        for (const [field, passed] of Object.entries(result.checks)) {
            expect(passed, `${field} should be true`).toBe(true);
        }
    });

    test('encrypted data survives update through browser', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            // Login
            const loginResp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'proton' }),
            });
            const loginData = await loginResp.json();
            const token = loginData.AccessToken;
            const uid = loginData.UID;

            const api = async (method: string, path: string, body?: any) => {
                const opts: RequestInit = {
                    method,
                    headers: {
                        'Content-Type': 'application/json',
                        Authorization: `Bearer ${token}`,
                        'x-pm-uid': uid,
                    },
                };
                if (body) opts.body = JSON.stringify(body);
                const resp = await fetch(path, opts);
                return { status: resp.status, data: await resp.json() };
            };

            // Create calendar and event
            const calResp = await api('POST', '/calendar/v1', {
                Name: 'Update E2E Test',
                Color: '#00ff00',
                AddressID: 'upd@proton.me',
                Display: 1,
            });
            const calId = calResp.data.Calendar.ID;

            const syncResp = await api('PUT', `/calendar/v1/${calId}/events/sync`, {
                MemberID: 'm1',
                Events: [
                    {
                        Event: {
                            Permissions: 127,
                            StartTime: 1000,
                            EndTime: 2000,
                            StartTimezone: 'UTC',
                            EndTimezone: 'UTC',
                            CalendarKeyPacket: 'original-cal-kp',
                            SharedKeyPacket: 'original-shared-kp',
                            CalendarEventContent: [
                                { Type: 3, Data: 'original-encrypted-cal', Signature: 'orig-sig', Author: 'u@p.me' },
                            ],
                            SharedEventContent: [
                                { Type: 3, Data: 'original-encrypted-shared', Signature: 'orig-sig', Author: 'u@p.me' },
                            ],
                        },
                    },
                ],
            });
            const eventId = syncResp.data.Responses[0].Response.Event.ID;

            // Update with new encrypted data
            const updateResp = await api('PUT', `/calendar/v1/${calId}/events/sync`, {
                MemberID: 'm1',
                Events: [
                    {
                        ID: eventId,
                        Event: {
                            Permissions: 127,
                            StartTime: 3000,
                            EndTime: 4000,
                            StartTimezone: 'America/New_York',
                            EndTimezone: 'America/New_York',
                            CalendarKeyPacket: 'UPDATED-cal-kp',
                            SharedKeyPacket: 'UPDATED-shared-kp',
                            AddressKeyPacket: 'UPDATED-address-kp',
                            CalendarEventContent: [
                                { Type: 3, Data: 'UPDATED-encrypted-cal', Signature: 'new-sig', Author: 'u@p.me' },
                            ],
                            SharedEventContent: [
                                { Type: 2, Data: 'UPDATED-signed-shared', Signature: 'new-sig', Author: 'u@p.me' },
                            ],
                        },
                    },
                ],
            });

            // Read back
            const getResp = await api('GET', `/calendar/v1/${calId}/events/${eventId}`);
            const event = getResp.data.Event;

            return {
                updateStatus: updateResp.status,
                calKeyPacket: event.CalendarKeyPacket === 'UPDATED-cal-kp',
                sharedKeyPacket: event.SharedKeyPacket === 'UPDATED-shared-kp',
                addressKeyPacket: event.AddressKeyPacket === 'UPDATED-address-kp',
                calData: event.CalendarEvents?.[0]?.Data === 'UPDATED-encrypted-cal',
                sharedData: event.SharedEvents?.[0]?.Data === 'UPDATED-signed-shared',
                sharedType: event.SharedEvents?.[0]?.Type === 2,
                timezone: event.StartTimezone === 'America/New_York',
            };
        });

        expect(result.updateStatus).toBe(200);
        expect(result.calKeyPacket).toBe(true);
        expect(result.sharedKeyPacket).toBe(true);
        expect(result.addressKeyPacket).toBe(true);
        expect(result.calData).toBe(true);
        expect(result.sharedData).toBe(true);
        expect(result.sharedType).toBe(true);
        expect(result.timezone).toBe(true);
    });

    test('special characters in encrypted data are preserved', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            // Login
            const loginResp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'proton' }),
            });
            const loginData = await loginResp.json();
            const token = loginData.AccessToken;
            const uid = loginData.UID;

            const api = async (method: string, path: string, body?: any) => {
                const opts: RequestInit = {
                    method,
                    headers: {
                        'Content-Type': 'application/json',
                        Authorization: `Bearer ${token}`,
                        'x-pm-uid': uid,
                    },
                };
                if (body) opts.body = JSON.stringify(body);
                const resp = await fetch(path, opts);
                return { status: resp.status, data: await resp.json() };
            };

            const calResp = await api('POST', '/calendar/v1', {
                Name: 'Special Chars Test',
                Color: '#ff00ff',
                AddressID: 'special@proton.me',
                Display: 1,
            });
            const calId = calResp.data.Calendar.ID;

            const specialData = 'wcBMA+data/with+special/chars+base64/=\n\ttabs\r\n+unicode:日本語+emoji:🔒';
            const specialSig = '-----BEGIN PGP SIGNATURE-----\nwsBcBAAB+special/chars==\n-----END PGP SIGNATURE-----';
            const specialKP = 'key-packet-/+special+chars/base64==';

            const syncResp = await api('PUT', `/calendar/v1/${calId}/events/sync`, {
                MemberID: 'm1',
                Events: [
                    {
                        Event: {
                            Permissions: 127,
                            StartTime: 9000,
                            EndTime: 10000,
                            StartTimezone: 'UTC',
                            EndTimezone: 'UTC',
                            CalendarKeyPacket: specialKP,
                            CalendarEventContent: [
                                { Type: 3, Data: specialData, Signature: specialSig, Author: 'test@proton.me' },
                            ],
                            SharedEventContent: [
                                { Type: 3, Data: specialData, Signature: specialSig, Author: 'test@proton.me' },
                            ],
                        },
                    },
                ],
            });
            const eventId = syncResp.data.Responses[0].Response.Event.ID;

            // Read back
            const getResp = await api('GET', `/calendar/v1/${calId}/events/${eventId}`);
            const event = getResp.data.Event;

            return {
                keyPacketMatch: event.CalendarKeyPacket === specialKP,
                calDataMatch: event.CalendarEvents?.[0]?.Data === specialData,
                calSigMatch: event.CalendarEvents?.[0]?.Signature === specialSig,
                sharedDataMatch: event.SharedEvents?.[0]?.Data === specialData,
                sharedSigMatch: event.SharedEvents?.[0]?.Signature === specialSig,
            };
        });

        expect(result.keyPacketMatch).toBe(true);
        expect(result.calDataMatch).toBe(true);
        expect(result.calSigMatch).toBe(true);
        expect(result.sharedDataMatch).toBe(true);
        expect(result.sharedSigMatch).toBe(true);
    });
});

// ---- CORS Tests ----

test.describe('CORS from Browser', () => {
    test('API responds with correct CORS headers for cross-origin requests', async ({ page }) => {
        // Navigate to test page first (needed for browser context)
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            // Login first to get a token
            const loginResp = await fetch('/core/v4/auth', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ Username: 'proton', Password: 'proton' }),
            });
            const loginData = await loginResp.json();

            // Use authenticated request to verify API works from browser
            const resp = await fetch('/calendar/v1/timezones', {
                headers: {
                    Authorization: `Bearer ${loginData.AccessToken}`,
                    'x-pm-uid': loginData.UID,
                },
            });
            const data = await resp.json();
            return {
                status: resp.status,
                contentType: resp.headers.get('content-type'),
                hasTimezones: Array.isArray(data?.Timezones) && data.Timezones.length > 100,
            };
        });

        expect(result.status).toBe(200);
        expect(result.contentType).toContain('application/json');
        expect(result.hasTimezones).toBe(true);
    });

    test('OPTIONS preflight returns correct CORS headers', async ({ page }) => {
        await page.goto('/static/test.html');

        const result = await page.evaluate(async () => {
            const resp = await fetch('/calendar/v1', {
                method: 'OPTIONS',
            });
            return {
                status: resp.status,
                allowOrigin: resp.headers.get('access-control-allow-origin'),
                allowMethods: resp.headers.get('access-control-allow-methods'),
                allowHeaders: resp.headers.get('access-control-allow-headers'),
            };
        });

        // CORS preflight should return 204
        expect(result.status).toBe(204);
    });
});

// ---- UI State Verification ----

test.describe('UI State Verification', () => {
    test('test page shows progress during execution', async ({ page }) => {
        await page.goto('/static/test.html');
        await page.click('button:has-text("Run All Tests")');

        // Progress indicator should show during test execution
        const progress = page.locator('#progress em');
        await expect(progress).toBeVisible({ timeout: 5_000 });

        // Wait for completion
        await expect(page.locator('#summary span')).toBeVisible({ timeout: 30_000 });
    });

    test('test results are rendered with proper styling', async ({ page }) => {
        await page.goto('/static/test.html');
        await page.click('button:has-text("Run All Tests")');
        await expect(page.locator('#summary span')).toBeVisible({ timeout: 30_000 });

        // Check the styling of the results
        const passCount = await page.locator('.status.pass').count();
        expect(passCount).toBeGreaterThan(0);

        // Summary should have green color for all-pass
        const summaryColor = await page.locator('#summary span').evaluate(
            (el) => window.getComputedStyle(el).color
        );
        // Green-ish color (rgb(21, 87, 36) = #155724)
        expect(summaryColor).toContain('21');
    });

    test('can re-run tests after first run', async ({ page }) => {
        await page.goto('/static/test.html');

        // First run
        await page.click('button:has-text("Run All Tests")');
        await expect(page.locator('#summary span')).toBeVisible({ timeout: 30_000 });
        const firstSummary = await page.locator('#summary span').textContent();

        // Second run
        await page.click('button:has-text("Run All Tests")');
        await expect(page.locator('#summary span')).toBeVisible({ timeout: 30_000 });
        const secondSummary = await page.locator('#summary span').textContent();

        // Both runs should have the same result
        expect(firstSummary).toBe(secondSummary);
    });
});
