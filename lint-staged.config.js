/**
 * lint-staged 配置文件
 * 在 pre-commit hook 中自动运行
 */

export default {
  // 前端 TS/TSX/Vue 文件：运行 ESLint 自动修复（含 world 世界守卫）
  'apps/zhixu-frontend/**/*.{ts,tsx,vue}': [
    'eslint --fix',
  ],
  // AI 应用与后端脚本：根 eslint 覆盖 apps/**，此前被 lint-staged 漏掉，
  // 导致 zhixu-ai 的格式错误能一路活到合并（实证：evolve.ts 的 prettier
  // 错误从 eeb0ab2 起活了 19 个提交）。
  'apps/zhixu-ai/**/*.{ts,tsx,js,jsx}': [
    'eslint --fix',
  ],
};
