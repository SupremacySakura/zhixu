import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { UserProfile } from '../services/types/auth'

export const useUserStore = defineStore(
  'user',
  () => {
    const token = ref('')
    const refreshToken = ref('')
    const profile = ref<UserProfile | null>(null)
    const setToken = (newToken: string) => {
      token.value = newToken
    }
    const setRefreshToken = (newRefreshToken: string) => {
      refreshToken.value = newRefreshToken
    }
    const setProfile = (newProfile: UserProfile | null) => {
      profile.value = newProfile
    }

    const logout = () => {
      token.value = ''
      refreshToken.value = ''
      profile.value = null
    }

    const isLoggedIn = computed(() => !!token.value)

    return {
      token,
      refreshToken,
      profile,
      isLoggedIn,
      setToken,
      setRefreshToken,
      setProfile,
      logout,
    }
  },
  {
    persist: true,
  },
)
