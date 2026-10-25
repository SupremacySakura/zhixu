import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/home',
  },
  {
    path: '/home',
    component: () => import('../pages/HomePage.vue'),
    redirect: '/home/dashboard',
    meta: { requiresAuth: true },
    children: [
      {
        path: '/home/dashboard',
        component: () => import('../pages/DashboardPage.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/files',
        component: () => import('../pages/FileManagement.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/knowledge',
        component: () => import('../pages/KnowledgeLibrary.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/knowledge/:id',
        component: () => import('../pages/KnowledgeDetail.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/knowledge/:id/chunks',
        component: () => import('../pages/KnowledgeChunks.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/pickup-codes',
        component: () => import('../pages/PickupCodes.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/settings',
        component: () => import('../pages/ModelSettings.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/chat',
        component: () => import('../pages/ChatPage.vue'),
        meta: { requiresAuth: true },
      },
      {
        path: '/home/upload',
        component: () => import('../pages/UploadFile.vue'),
        meta: { requiresAuth: true },
      },
    ],
  },
  {
    path: '/login',
    component: () => import('../pages/LoginPage.vue'),
  },
  {
    path: '/register',
    component: () => import('../pages/RegisterPage.vue'),
  },
  {
    path: '/require-login',
    component: () => import('../pages/RequireLogin.vue'),
  },
  {
    path: '/pickup',
    component: () => import('../pages/FilePickup.vue'),
  },
]
export { routes }
