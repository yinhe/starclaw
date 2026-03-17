import { Page } from '@playwright/test'

/** Inject auth tokens into localStorage so PrivateRoute allows access. */
export async function loginAs(page: Page, user = {
  id: 'test-user-001',
  email: 'test@starclaw.me',
  username: 'e2e-tester',
  avatar: '',
}) {
  await page.addInitScript((u) => {
    localStorage.setItem('starclaw_token', 'e2e-fake-jwt-token')
    localStorage.setItem('starclaw_user', JSON.stringify(u))
  }, user)
}
