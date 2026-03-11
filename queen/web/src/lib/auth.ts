import type { UserInfo } from './api';

const TOKEN_KEY = 'sc_token';
const USER_KEY = 'sc_user';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function getUser(): UserInfo | null {
  try {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export function setAuth(token: string, user: UserInfo) {
  localStorage.setItem(TOKEN_KEY, token);
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

export function isLoggedIn(): boolean {
  return !!getToken();
}

export function getUserDisplayName(): string {
  const u = getUser();
  if (!u) return 'User';
  return u.nickname || u.email || u.phone || 'User';
}

export function getUserInitial(): string {
  return getUserDisplayName().charAt(0).toUpperCase();
}
