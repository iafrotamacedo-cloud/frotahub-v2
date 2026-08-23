// rev 2 — configuração da compilação do front
//
// Gera web/dist/, que é a pasta que a automação envia ao HostGator.
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],

  // O site fica na raiz do subdomínio, não numa subpasta.
  base: '/',

  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // Sem mapa de código em produção: triplica o tamanho e só serve para depurar.
    sourcemap: false,
  },

  server: { port: 5173, open: true },
})
