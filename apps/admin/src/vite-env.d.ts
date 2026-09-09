/// <reference types="vite/client" />

/** Переменные окружения объявлены явно: опечатка в имени станет ошибкой типов. */
interface ImportMetaEnv {
  readonly VITE_API_URL: string;
  readonly VITE_AUTH_URL: string;
  readonly VITE_GOOGLE_CLIENT_ID: string;
  readonly VITE_ICON_CDN_URL: string;
  /** Хранилище файлов: ручка загрузки отдаёт путь, а не адрес. */
  readonly VITE_CDN_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
