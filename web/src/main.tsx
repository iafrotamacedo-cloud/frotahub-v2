// rev 2 — o ponto de partida do front
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './estilos/tokens.css'
import './estilos/base.css'
import './estilos/telas.css'
import App from './App'

const raiz = document.getElementById('raiz')
if (!raiz) throw new Error('Elemento #raiz não encontrado no index.html')

createRoot(raiz).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
