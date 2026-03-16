import { defineConfig } from '@playwright/test';

export default defineConfig({
    testDir: './tests',
    timeout: 60_000,
    retries: 0,
    outputDir: './test-results',
    use: {
        baseURL: 'http://localhost:8080',
        headless: true,
        screenshot: 'on',
        video: 'retain-on-failure',
    },
    projects: [
        {
            name: 'chromium',
            use: { browserName: 'chromium' },
        },
    ],
    webServer: {
        command: 'cd ../backend && go run .',
        port: 8080,
        timeout: 30_000,
        reuseExistingServer: true,
    },
});
