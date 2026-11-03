import request from '../request'
import type { ResponseData } from '../types'
import type { LoginRequest, LoginResponse, RegisterRequest, UserProfile } from '../types/auth'

export const register = async (data: RegisterRequest): Promise<ResponseData<null>> => {
  const res = await request.post<ResponseData<null>>('/api/auth/register', JSON.stringify(data))
  return res.data
}

export const login = async (data: LoginRequest): Promise<ResponseData<LoginResponse>> => {
  const res = await request.post<ResponseData<LoginResponse>>(
    '/api/auth/login',
    JSON.stringify(data),
  )
  return res.data
}

export const checkLogin = async (): Promise<ResponseData<null>> => {
  const res = await request.get<ResponseData<null>>('/api/auth/check')
  return res.data
}

export const getProfile = async (signal?: AbortSignal): Promise<ResponseData<UserProfile>> => {
  const res = await request.get<ResponseData<UserProfile>>('/api/auth/profile', { signal })
  return res.data
}
