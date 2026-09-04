// rev 4 — o ponto de partida do front
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './estilos/tokens.css'
import './estilos/base.css'
import './estilos/telas.css'
import './estilos/trilogo.css'
import './estilos/escuro.css'
import './estilos/orcamentos.css'
// Estatísticas entra por último: tudo nela mora dentro de `.est`, então a ordem
// não muda nada — mas se um dia alguém tirar o invólucro, é aqui que se vê que
// ela passou a poder pisar nas outras.
import './estilos/estatisticas.css'
import './estilos/servicos.css'
import App from './App'

const raiz = document.getElementById('raiz')
if (!raiz) throw new Error('Elemento #raiz não encontrado no index.html')

createRoot(raiz).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
