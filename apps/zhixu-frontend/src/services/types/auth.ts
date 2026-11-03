export interface RegisterRequest {
  email: string
  username: string
  password: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
}

export interface UserProfile {
  id: number
  username: string
  email: string
  avatar_url: string
  storage_limit: number
  storage_used: number
  created_at: string
}
