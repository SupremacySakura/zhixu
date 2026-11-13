import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import eslintConfigPrettier from 'eslint-config-prettier'
import prettierPlugin from 'eslint-plugin-prettier'
import tseslint from 'typescript-eslint'
import worldDrift from './eslint-rules/no-world-drift.js'

export default [
  {
    ignores: [
      'dist/**',
      'build/**',
      'node_modules/**',
      '**/coverage/**',
      '**/dist/**',
      '**/data/**',
    ],
  },
  {
    files: ['**/*.js'],
    ...js.configs.recommended,
  },
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tseslint.parser,
    },
    plugins: {
      '@typescript-eslint': tseslint.plugin,
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      '@typescript-eslint/no-empty-object-type': 'off',
    },
  },
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: tseslint.parser,
        extraFileExtensions: ['.vue'],
        sourceType: 'module',
      },
    },
    plugins: {
      vue: pluginVue,
    },
    rules: {
      ...pluginVue.configs.essential.rules,
      'vue/multi-word-component-names': 'off',
    },
  },
  {
    // 世界防通胀守卫（#38 批 2）。级别先 warn 跑一轮把存量记明白，
    // 存量清零后升 error —— 票面要求的两段式。存量已是 0，所以直接 error；
    // 若日后发现挂账项，用 no-world-drift 的 allow 选项按「文件名:token」登记。
    files: ['**/*.vue'],
    plugins: {
      world: worldDrift,
    },
    rules: {
      'world/no-world-drift': 'error',
    },
  },
  {
    // 守卫的自测样本故意写成违规，跑 eslint 时跳过；
    // 它由 `node scripts/check-world-drift.mjs` 单独断言（见那个脚本的注释）。
    files: ['**/__tests__/fixtures/**'],
    rules: {
      'world/no-world-drift': 'off',
    },
  },
  {
    files: ['**/*.{js,ts,vue}'],
    plugins: {
      prettier: prettierPlugin,
    },
    rules: {
      'prettier/prettier': 'error',
    },
  },
  eslintConfigPrettier,
]
