import { test, expect } from '@playwright/test'
import { loginAs } from './helpers/auth'

test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    // Mock setup API
    await page.route('**/v1/setup/status', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ setup_completed: true, deploy_mode: 'opensource' }),
      })
    })

    // Mock common API endpoints that pages may call on mount
    await page.route('**/v1/**', (route) => {
      if (route.request().url().includes('/setup/status')) return route.fallback()
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], total: 0 }),
      })
    })

    await loginAs(page)
  })

  const coreRoutes = [
    { path: '/dashboard', title: 'Dashboard' },
    { path: '/agents', title: 'Agents' },
    { path: '/models', title: 'Models' },
    { path: '/knowledge', title: 'Knowledge' },
    { path: '/workflows', title: 'Workflows' },
    { path: '/settings', title: 'Settings' },
  ]

  for (const { path, title } of coreRoutes) {
    test(`navigates to ${title} (${path})`, async ({ page }) => {
      await page.goto(path)
      await expect(page).toHaveURL(new RegExp(path))
      // Page should render without crash
      await expect(page.locator('body')).toBeVisible()
    })
  }

  const p8p10Routes = [
    { path: '/observe', title: 'Observe' },
    { path: '/webhooks', title: 'Webhooks' },
    { path: '/developer', title: 'Developer' },
    { path: '/security', title: 'Security' },
    { path: '/goals', title: 'Goals' },
    { path: '/finetune', title: 'FineTune' },
  ]

  for (const { path, title } of p8p10Routes) {
    test(`P8-P10: ${title} page loads (${path})`, async ({ page }) => {
      await page.goto(path)
      await expect(page).toHaveURL(new RegExp(path))
      // Wait for page heading or tab buttons to appear
      await expect(page.locator('h1, h2, h3, [role="tablist"]').first()).toBeVisible({ timeout: 10000 })
    })
  }

  test('404 page for unknown routes', async ({ page }) => {
    await page.goto('/this-does-not-exist')
    await expect(page.locator('body')).toBeVisible()
  })

  test('sidebar navigation links are visible', async ({ page }) => {
    await page.goto('/dashboard')
    // Layout sidebar should have navigation links
    const nav = page.locator('nav, aside, [role="navigation"]').first()
    await expect(nav).toBeVisible({ timeout: 5000 })
  })
})
