import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import eslintConfigPrettier from 'eslint-config-prettier'
import prettierPlugin from 'eslint-plugin-prettier'
import tseslint from 'typescript-eslint'

export default [
  {
    ignores: [
      'dist/**',
      'build/**',
      'node_modules/**',
      '**/coverage/**',
      '**/dist/**',
      '**/data/**',
      // Mastra 的构建产物（已 gitignore）：几万个打包文件会让 lint 卡住数分钟
      '**/.mastra/**',
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
    rules: Object.assign({}, tseslint.configs.recommended.rules, {
      '@typescript-eslint/no-empty-object-type': 'off',
    }),
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
    rules: pluginVue.configs.essential.rules,
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
