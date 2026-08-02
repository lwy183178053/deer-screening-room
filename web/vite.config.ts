import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue({ template: { compilerOptions: { isCustomElement: tag => tag.startsWith('media-') } } })],
  server: { proxy: { '/api': 'http://localhost:8080' } },
  test: { environment: 'jsdom', globals: true, include: ['src/**/*.test.ts'] },
})
