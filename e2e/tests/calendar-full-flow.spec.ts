/**
 * Comprehensive Playwright E2E tests for the Proton Calendar static UI.
 *
 * Tests every user-facing feature of /static/calendar.html:
 *  1. Login (valid + invalid)
 *  2. Calendar creation
 *  3. Event creation
 *  4. Event detail view
 *  5. Event editing
 *  6. Event deletion
 *  7. Week navigation
 *  8. Multiple calendars
 *  9. Logout
 *  10. Full end-to-end flow
 *  11. Modal interactions
 *  12. Encryption indicators
 *
 * Each test takes named screenshots saved to e2e/screenshots/.
 * Uses unique names per test to avoid conflicts from shared backend state.
 *
 * Run:  cd e2e && npx playwright test tests/calendar-full-flow.spec.ts
 */
import { test, expect, Page } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';

const SCREENSHOTS_DIR = path.join(__dirname, '..', 'screenshots');

// Unique suffix per test run to avoid name collisions
const RUN_ID = Date.now().toString(36);
let testCounter = 0;
function uid(prefix: string) {
    return `${prefix}-${RUN_ID}-${++testCounter}`;
}

// Ensure screenshots directory exists
test.beforeAll(() => {
    if (!fs.existsSync(SCREENSHOTS_DIR)) {
        fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
    }
});

/** Helper: take a named screenshot and save to screenshots/ */
async function snap(page: Page, name: string) {
    await page.screenshot({ path: path.join(SCREENSHOTS_DIR, `${name}.png`), fullPage: true });
}

/** Helper: login and return the authenticated page */
async function login(page: Page) {
    await page.goto('/static/calendar.html');
    await page.fill('#username', 'proton');
    await page.fill('#password', 'proton');
    await page.click('#login-btn');
    await expect(page.locator('#app-screen')).toBeVisible({ timeout: 10_000 });
}

/** Helper: create a calendar and return its name */
async function createCalendar(page: Page, calName?: string): Promise<string> {
    const name = calName ?? uid('Cal');
    await page.click('#btn-new-calendar');
    await page.fill('#new-cal-name', name);
    await page.click('[data-testid="save-calendar"]');
    await expect(page.locator('#cal-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
    await expect(page.locator('#calendar-list')).toContainText(name);
    return name;
}

/** Helper: create an event in the current calendar */
async function createEvent(page: Page, title: string, hour = 10): Promise<string> {
    await page.click('#btn-new-event');
    await expect(page.locator('#event-modal')).toHaveClass(/open/);

    const today = new Date().toISOString().split('T')[0];
    await page.fill('[data-testid="event-title"]', title);
    await page.fill('[data-testid="event-date"]', today);
    await page.fill('[data-testid="event-start-time"]', `${String(hour).padStart(2, '0')}:00`);
    await page.fill('[data-testid="event-end-time"]', `${String(hour + 1).padStart(2, '0')}:00`);
    await page.click('[data-testid="save-event"]');
    await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
    await page.waitForTimeout(500);
    return title;
}

// ═══════════════════════════════════════════════════════════════════════════
// 1. LOGIN FLOW
// ═══════════════════════════════════════════════════════════════════════════

test.describe('1. Login Flow', () => {
    test('shows login screen on load', async ({ page }) => {
        await page.goto('/static/calendar.html');
        await expect(page.locator('#login-screen')).toBeVisible();
        await expect(page.locator('#app-screen')).toBeHidden();
        await expect(page.locator('.login-card h1')).toContainText('Proton Calendar');
        await snap(page, '01-login-screen');
    });

    test('rejects invalid credentials', async ({ page }) => {
        await page.goto('/static/calendar.html');
        await page.fill('#username', 'proton');
        await page.fill('#password', 'wrongpassword');
        await page.click('#login-btn');
        await expect(page.locator('#login-error')).toBeVisible({ timeout: 5_000 });
        await snap(page, '02-login-error');
    });

    test('successful login shows app screen', async ({ page }) => {
        await login(page);
        await expect(page.locator('#user-label')).toHaveText('proton');
        await expect(page.locator('#login-screen')).toBeHidden();
        await snap(page, '03-logged-in');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 2. CALENDAR CREATION
// ═══════════════════════════════════════════════════════════════════════════

test.describe('2. Calendar Creation', () => {
    test('create a new calendar', async ({ page }) => {
        await login(page);
        const calName = uid('WorkCal');
        await page.click('#btn-new-calendar');
        await expect(page.locator('#cal-modal')).toHaveClass(/open/);
        await snap(page, '04-new-calendar-modal');

        await page.fill('#new-cal-name', calName);
        await page.fill('#new-cal-color', '#e74c3c');
        await page.click('[data-testid="save-calendar"]');

        await expect(page.locator('#cal-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await expect(page.locator('#calendar-list')).toContainText(calName);
        await snap(page, '05-calendar-created');
    });

    test('create multiple calendars', async ({ page }) => {
        await login(page);
        const cal1 = uid('Personal');
        const cal2 = uid('Meetings');

        await createCalendar(page, cal1);
        await createCalendar(page, cal2);

        const calList = page.locator('#calendar-list');
        await expect(calList).toContainText(cal1);
        await expect(calList).toContainText(cal2);
        expect(await page.locator('#calendar-list li').count()).toBeGreaterThanOrEqual(2);
        await snap(page, '06-multiple-calendars');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 3. EVENT CREATION
// ═══════════════════════════════════════════════════════════════════════════

test.describe('3. Event Creation', () => {
    test('create a new event via "+ New Event" button', async ({ page }) => {
        await login(page);
        await createCalendar(page);

        const eventName = uid('Standup');
        await page.click('#btn-new-event');
        await expect(page.locator('#event-modal')).toHaveClass(/open/);
        await snap(page, '07-new-event-modal');

        const today = new Date().toISOString().split('T')[0];
        await page.fill('[data-testid="event-title"]', eventName);
        await page.fill('[data-testid="event-date"]', today);
        await page.fill('[data-testid="event-start-time"]', '10:00');
        await page.fill('[data-testid="event-end-time"]', '10:30');
        await page.fill('[data-testid="event-location"]', 'Room 42');
        await page.fill('[data-testid="event-description"]', 'Daily standup meeting');
        await snap(page, '08-event-form-filled');

        await page.click('[data-testid="save-event"]');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await page.waitForTimeout(500);

        const chip = page.locator('.event-chip', { hasText: eventName }).first();
        await expect(chip).toBeVisible({ timeout: 5_000 });
        await snap(page, '09-event-in-grid');
    });

    test('create event by clicking on day cell', async ({ page }) => {
        await login(page);
        await createCalendar(page);

        const dayCell = page.locator('.day-cell').first();
        await dayCell.click();
        await expect(page.locator('#event-modal')).toHaveClass(/open/);

        const dateVal = await page.inputValue('[data-testid="event-date"]');
        expect(dateVal).toBeTruthy();

        const eventName = uid('QuickMtg');
        await page.fill('[data-testid="event-title"]', eventName);
        await page.click('[data-testid="save-event"]');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
    });

    test('empty title shows validation toast', async ({ page }) => {
        await login(page);
        await createCalendar(page);

        await page.click('#btn-new-event');
        await expect(page.locator('#event-modal')).toHaveClass(/open/);
        await page.click('[data-testid="save-event"]');
        await expect(page.locator('#toast')).toHaveClass(/show/, { timeout: 3_000 });
        await snap(page, '10-validation-error');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 4. EVENT DETAIL VIEW
// ═══════════════════════════════════════════════════════════════════════════

test.describe('4. Event Detail View', () => {
    test('view event details by clicking event chip', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        const eventName = uid('DetailMtg');
        await createEvent(page, eventName, 14);

        const chip = page.locator('.event-chip', { hasText: eventName }).first();
        await expect(chip).toBeVisible({ timeout: 5_000 });
        await chip.click();

        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await expect(page.locator('#detail-modal-title')).toHaveText(eventName);
        await expect(page.locator('#detail-modal .encryption-badge.encrypted').first()).toBeVisible();
        await snap(page, '11-event-detail');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 5. EVENT EDITING
// ═══════════════════════════════════════════════════════════════════════════

test.describe('5. Event Editing', () => {
    test('edit an event from detail view', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        const originalName = uid('OrigTitle');
        const updatedName = uid('UpdTitle');
        await createEvent(page, originalName, 11);

        // Open detail
        const chip = page.locator('.event-chip', { hasText: originalName }).first();
        await expect(chip).toBeVisible({ timeout: 5_000 });
        await chip.click();
        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });

        // Click Edit
        await page.click('#btn-detail-edit');
        await expect(page.locator('#event-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await expect(page.locator('#event-modal-title')).toHaveText('Edit Event');
        expect(await page.inputValue('[data-testid="event-title"]')).toBe(originalName);
        await snap(page, '12-edit-event-modal');

        // Change the title
        await page.fill('[data-testid="event-title"]', updatedName);
        await page.click('[data-testid="save-event"]');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await page.waitForTimeout(500);

        await expect(page.locator('.event-chip', { hasText: updatedName }).first()).toBeVisible({ timeout: 5_000 });
        await snap(page, '13-event-updated');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 6. EVENT DELETION
// ═══════════════════════════════════════════════════════════════════════════

test.describe('6. Event Deletion', () => {
    test('delete event from detail view', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        const eventName = uid('ToDelete');
        await createEvent(page, eventName, 16);

        const chip = page.locator('.event-chip', { hasText: eventName }).first();
        await expect(chip).toBeVisible({ timeout: 5_000 });
        await chip.click();
        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await snap(page, '14-before-delete');

        await page.click('#btn-detail-delete');
        await expect(page.locator('#detail-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await page.waitForTimeout(500);

        await expect(page.locator('.event-chip', { hasText: eventName })).toHaveCount(0, { timeout: 5_000 });
        await snap(page, '15-after-delete');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 7. WEEK NAVIGATION
// ═══════════════════════════════════════════════════════════════════════════

test.describe('7. Week Navigation', () => {
    test('navigate weeks with prev/next/today', async ({ page }) => {
        await login(page);
        await createCalendar(page);

        const initialRange = await page.textContent('#current-range');
        expect(initialRange).toBeTruthy();
        await snap(page, '16-week-current');

        await page.click('#btn-next');
        await page.waitForTimeout(300);
        const nextRange = await page.textContent('#current-range');
        expect(nextRange).not.toBe(initialRange);
        await snap(page, '17-week-next');

        await page.click('#btn-prev');
        await page.waitForTimeout(300);
        await page.click('#btn-prev');
        await page.waitForTimeout(300);
        const prevRange = await page.textContent('#current-range');
        expect(prevRange).not.toBe(nextRange);
        await snap(page, '18-week-prev');

        await page.click('#btn-today');
        await page.waitForTimeout(300);
        const todayRange = await page.textContent('#current-range');
        expect(todayRange).toBe(initialRange);
        await snap(page, '19-week-today');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 8. CALENDAR SWITCHING
// ═══════════════════════════════════════════════════════════════════════════

test.describe('8. Calendar Switching', () => {
    test('switch between calendars in sidebar', async ({ page }) => {
        await login(page);
        const calA = uid('CalA');
        const calB = uid('CalB');
        await createCalendar(page, calA);
        await createCalendar(page, calB);

        // Calendar B should be active (last created)
        await expect(page.locator('#calendar-list li', { hasText: calB })).toHaveClass(/active/);

        // Click Calendar A
        await page.locator('#calendar-list li', { hasText: calA }).click();
        await page.waitForTimeout(500);
        await expect(page.locator('#calendar-list li', { hasText: calA })).toHaveClass(/active/);
        await snap(page, '20-calendar-switched');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 9. LOGOUT FLOW
// ═══════════════════════════════════════════════════════════════════════════

test.describe('9. Logout Flow', () => {
    test('logout returns to login screen', async ({ page }) => {
        await login(page);
        await expect(page.locator('#app-screen')).toBeVisible();
        await page.click('#btn-logout');

        await expect(page.locator('#login-screen')).toBeVisible();
        await expect(page.locator('#app-screen')).toBeHidden();
        expect(await page.inputValue('#username')).toBe('');
        expect(await page.inputValue('#password')).toBe('');
        await snap(page, '21-after-logout');
    });

    test('can login again after logout', async ({ page }) => {
        await login(page);
        await page.click('#btn-logout');
        await expect(page.locator('#login-screen')).toBeVisible();

        await login(page);
        await expect(page.locator('#app-screen')).toBeVisible();
        await expect(page.locator('#user-label')).toHaveText('proton');
        await snap(page, '22-re-login');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 10. FULL END-TO-END FLOW
// ═══════════════════════════════════════════════════════════════════════════

test.describe('10. Full E2E Flow', () => {
    test('complete workflow: login → calendar → event → edit → delete → logout', async ({ page }) => {
        const calName = uid('E2ECal');
        const eventName = uid('E2EEvt');
        const editedName = uid('E2EEdited');

        // Step 1: Login
        await page.goto('/static/calendar.html');
        await snap(page, '30-flow-login');
        await page.fill('#username', 'proton');
        await page.fill('#password', 'proton');
        await page.click('#login-btn');
        await expect(page.locator('#app-screen')).toBeVisible({ timeout: 10_000 });
        await snap(page, '31-flow-logged-in');

        // Step 2: Create calendar
        await createCalendar(page, calName);
        await snap(page, '32-flow-calendar');

        // Step 3: Create event
        await page.click('#btn-new-event');
        const today = new Date().toISOString().split('T')[0];
        await page.fill('[data-testid="event-title"]', eventName);
        await page.fill('[data-testid="event-date"]', today);
        await page.fill('[data-testid="event-start-time"]', '09:00');
        await page.fill('[data-testid="event-end-time"]', '10:00');
        await page.fill('[data-testid="event-location"]', 'Building 5');
        await page.fill('[data-testid="event-description"]', 'Full E2E test');
        await page.click('[data-testid="save-event"]');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await page.waitForTimeout(500);
        await expect(page.locator('.event-chip', { hasText: eventName }).first()).toBeVisible({ timeout: 5_000 });
        await snap(page, '33-flow-event');

        // Step 4: View event detail
        await page.locator('.event-chip', { hasText: eventName }).first().click();
        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await expect(page.locator('#detail-modal-title')).toHaveText(eventName);
        await snap(page, '34-flow-detail');

        // Step 5: Edit event
        await page.click('#btn-detail-edit');
        await expect(page.locator('#event-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await page.fill('[data-testid="event-title"]', editedName);
        await page.click('[data-testid="save-event"]');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await page.waitForTimeout(500);
        await expect(page.locator('.event-chip', { hasText: editedName }).first()).toBeVisible({ timeout: 5_000 });
        await snap(page, '35-flow-edited');

        // Step 6: Delete event
        await page.locator('.event-chip', { hasText: editedName }).first().click();
        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await page.click('#btn-detail-delete');
        await expect(page.locator('#detail-modal')).not.toHaveClass(/open/, { timeout: 5_000 });
        await page.waitForTimeout(500);
        await expect(page.locator('.event-chip', { hasText: editedName })).toHaveCount(0, { timeout: 5_000 });
        await snap(page, '36-flow-deleted');

        // Step 7: Logout
        await page.click('#btn-logout');
        await expect(page.locator('#login-screen')).toBeVisible();
        await snap(page, '37-flow-logout');
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 11. MODAL INTERACTIONS
// ═══════════════════════════════════════════════════════════════════════════

test.describe('11. Modal Interactions', () => {
    test('close event modal with X button', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        await page.click('#btn-new-event');
        await expect(page.locator('#event-modal')).toHaveClass(/open/);
        await page.click('#event-modal-close');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 3_000 });
    });

    test('close event modal with Cancel button', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        await page.click('#btn-new-event');
        await expect(page.locator('#event-modal')).toHaveClass(/open/);
        await page.click('#btn-cancel-event');
        await expect(page.locator('#event-modal')).not.toHaveClass(/open/, { timeout: 3_000 });
    });

    test('close calendar modal with Cancel', async ({ page }) => {
        await login(page);
        await page.click('#btn-new-calendar');
        await expect(page.locator('#cal-modal')).toHaveClass(/open/);
        await page.click('#btn-cancel-cal');
        await expect(page.locator('#cal-modal')).not.toHaveClass(/open/, { timeout: 3_000 });
    });

    test('close detail modal with Close button', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        const eventName = uid('CloseDetail');
        await createEvent(page, eventName, 13);

        const chip = page.locator('.event-chip', { hasText: eventName }).first();
        await expect(chip).toBeVisible({ timeout: 5_000 });
        await chip.click();
        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });
        await page.click('#btn-detail-close');
        await expect(page.locator('#detail-modal')).not.toHaveClass(/open/, { timeout: 3_000 });
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// 12. ENCRYPTION INDICATORS
// ═══════════════════════════════════════════════════════════════════════════

test.describe('12. Encryption Indicators', () => {
    test('event creation shows encryption badge', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        await page.click('#btn-new-event');
        await expect(page.locator('#event-modal')).toHaveClass(/open/);
        await expect(page.locator('#event-encryption-info')).toContainText('End-to-end encrypted');
        await snap(page, '23-encryption-badge');
    });

    test('event detail shows key packet encryption badges', async ({ page }) => {
        await login(page);
        await createCalendar(page);
        const eventName = uid('EncEvt');
        await createEvent(page, eventName, 15);

        const chip = page.locator('.event-chip', { hasText: eventName }).first();
        await expect(chip).toBeVisible({ timeout: 5_000 });
        await chip.click();
        await expect(page.locator('#detail-modal')).toHaveClass(/open/, { timeout: 5_000 });

        const encBadges = page.locator('#detail-modal .encryption-badge.encrypted');
        expect(await encBadges.count()).toBeGreaterThanOrEqual(1);
        await snap(page, '24-encryption-detail');
    });
});
