/// <reference types="vite/client" />

// O endereço do motor não é escrito no código: ele entra na compilação, vindo de
// uma variável do repositório (`VITE_MOTOR_URL`). Assim o mesmo código serve o
// site de teste e o definitivo sem ninguém editar arquivo.
interface ImportMetaEnv {
  readonly VITE_MOTOR_URL?: string
}
interface ImportMeta {
  readonly env: ImportMetaEnv
}
