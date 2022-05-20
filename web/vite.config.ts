
import { resolve } from 'path';
import { defineConfig } from 'vite';

// https://vitejs.dev/config/
export default defineConfig({
  build: {
    lib: {
      entry: 'src/my-element.ts',
      formats: ['es']
    },
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
      },
      external: /^lit/
    }
  }
})
