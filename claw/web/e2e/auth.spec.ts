import { test, expect } from '@playwright/test'
import { loginAs } from './helpers/auth'

test.describe('Authentication', () => {
  test('redirects to /setup or /login when not authenticated', async ({ page }) => {
    // Without mocking setup status, it may go to /setup or /login
    await page.route('**/v1/setup/status', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ setup_completed: true, deploy_mode: 'opensource' }),
      })
    })
    await page.goto('/')
    await expect(page).toHaveURL(/\/(login|setup)/)
  })

  test('login page renders correctly', async ({ page }) => {
    await page.route('**/v1/setup/status', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ setup_completed: true, deploy_mode: 'opensource' }),
      })
    })
    await page.goto('/login')
    await expect(page.locator('body')).toBeVisible()
    const hasInput = await page.locator('input').count()
    expect(hasInput).toBeGreaterThan(0)
  })

  test('authenticated user can access dashboard', async ({ page }) => {
    await page.route('**/v1/setup/status', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ setup_completed: true, deploy_mode: 'opensource' }),
      })
    })
    // Mock dashboard API calls
    await page.route('**/v1/**', (route) => {
      if (route.request().url().includes('/setup/status')) return route.fallback()
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], total: 0 }),
      })
    })

    await loginAs(page)
    await page.goto('/')
    await expect(page).toHaveURL(/\/dashboard/)
  })
})
