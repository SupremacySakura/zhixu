import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUserStore } from '../user'

describe('useUserStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('should have empty token initially', () => {
    const store = useUserStore()
    expect(store.token).toBe('')
    expect(store.refreshToken).toBe('')
  })

  it('should set token', () => {
    const store = useUserStore()
    store.setToken('test-token-123')
    expect(store.token).toBe('test-token-123')
  })

  it('should set refresh token', () => {
    const store = useUserStore()
    store.setRefreshToken('test-refresh-token-456')
    expect(store.refreshToken).toBe('test-refresh-token-456')
  })

  it('should clear tokens on logout', () => {
    const store = useUserStore()
    store.setToken('test-token-123')
    store.setRefreshToken('test-refresh-token-456')
    store.logout()
    expect(store.token).toBe('')
    expect(store.refreshToken).toBe('')
  })

  it('should set and clear profile', () => {
    const store = useUserStore()
    expect(store.profile).toBeNull()
    store.setProfile({
      id: 1,
      username: 'tester',
      email: 't@example.com',
      avatar_url: '',
      storage_limit: 1024,
      storage_used: 0,
      created_at: '',
    })
    expect(store.profile?.username).toBe('tester')
    store.setProfile(null)
    expect(store.profile).toBeNull()
  })

  it('should clear profile on logout', () => {
    const store = useUserStore()
    store.setToken('test-token-123')
    store.setProfile({
      id: 1,
      username: 'tester',
      email: 't@example.com',
      avatar_url: '',
      storage_limit: 1024,
      storage_used: 0,
      created_at: '',
    })
    store.logout()
    expect(store.profile).toBeNull()
  })

  it('should return isLoggedIn as false when no token', () => {
    const store = useUserStore()
    expect(store.isLoggedIn).toBe(false)
  })

  it('should return isLoggedIn as true when token exists', () => {
    const store = useUserStore()
    store.setToken('valid-token')
    expect(store.isLoggedIn).toBe(true)
  })

  it('should return isLoggedIn as false after logout', () => {
    const store = useUserStore()
    store.setToken('valid-token')
    expect(store.isLoggedIn).toBe(true)
    store.logout()
    expect(store.isLoggedIn).toBe(false)
  })

  it('should update isLoggedIn when token changes', () => {
    const store = useUserStore()
    expect(store.isLoggedIn).toBe(false)
    store.setToken('new-token')
    expect(store.isLoggedIn).toBe(true)
    store.setToken('')
    expect(store.isLoggedIn).toBe(false)
  })
})
