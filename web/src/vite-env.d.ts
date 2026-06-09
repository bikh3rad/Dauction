/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE?: string;
  readonly VITE_USE_MOCK?: string;
  readonly VITE_DEV_ACCOUNT_ID?: string;
  readonly VITE_GATEWAY_PROXY?: string;
}
interface ImportMeta {
  readonly env: ImportMetaEnv;
}
