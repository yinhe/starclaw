import { test, expect } from '@playwright/test'
import { loginAs } from './helpers/auth'

test.describe('Page Interactions', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/v1/setup/status', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ setup_completed: true, deploy_mode: 'opensource' }),
      })
    })

    // Mock all API calls with sensible defaults
    await page.route('**/v1/**', (route) => {
      if (route.request().url().includes('/setup/status')) return route.fallback()
      const url = route.request().url()

      // Dashboard stats
      if (url.includes('/stats') || url.includes('/overview')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            total_agents: 5, total_chats: 120, total_workflows: 8,
            active_users: 3, data: [], total: 0,
          }),
        })
      }

      // Lists / paginated endpoints
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [], items: [], total: 0, traces: [], logs: [], rules: [], goals: [], adapters: [] }),
      })
    })

    await loginAs(page)
  })

  test('Observe page has tabs', async ({ page }) => {
    await page.goto('/observe')
    const tabs = page.locator('button')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(2)
  })

  test('Webhooks page has tabs', async ({ page }) => {
    await page.goto('/webhooks')
    const tabs = page.locator('button')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(1)
  })

  test('Security page has tabs', async ({ page }) => {
    await page.goto('/security')
    const tabs = page.locator('button')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(2)
  })

  test('Developer page has tabs', async ({ page }) => {
    await page.goto('/developer')
    const tabs = page.locator('button')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(1)
  })

  test('Goals page has tabs', async ({ page }) => {
    await page.goto('/goals')
    const tabs = page.locator('button')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(1)
  })

  test('FineTune page has tabs', async ({ page }) => {
    await page.goto('/finetune')
    await expect(page.locator('h1').first()).toBeVisible({ timeout: 10000 })
    const tabs = page.locator('button')
    const tabCount = await tabs.count()
    expect(tabCount).toBeGreaterThan(1)
  })

  test('Chat page renders input area', async ({ page }) => {
    await page.goto('/chat')
    await expect(page.locator('textarea, input[type="text"]').first()).toBeVisible({ timeout: 5000 })
  })

  test('Settings page renders', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.locator('body')).toBeVisible()
  })
})
