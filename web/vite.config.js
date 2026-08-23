// rev 1 — configuração da compilação do front
//
// Por enquanto o mínimo: pegar o que está em web/ e gerar web/dist/, que é a pasta
// que a automação envia para o HostGator. Quando o React entrar, é aqui que o plugin
// dele é acrescentado.
import { defineConfig } from 'vite'

export default defineConfig({
  // O site fica na raiz do subdomínio (novo.frotamacedo.com.br), não numa subpasta.
  base: '/',

  build: {
    outDir: 'dist',
    emptyOutDir: true,
    // Sem mapa de código em produção: ele triplica o tamanho e só serve para depurar.
    sourcemap: false,
  },

  server: {
    port: 5173,
    open: true,
  },
})
