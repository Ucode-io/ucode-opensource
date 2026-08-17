/**
 * Хранилище сессии. Access-токен живёт только в памяти — в localStorage
 * его нет, потому что XSS достаёт localStorage, но не замыкание.
 * Refresh-токен в localStorage: без него сессия не переживёт перезагрузку.
 *
 * Environment здесь, а не в URL — см. docs/adr/0001. Project тоже здесь,
 * потому что нужен интерцептору и обновлению токена; в URL он дублируется
 * как читаемый адрес, а не как источник правды.
 */
const REFRESH_KEY = "ucode.refresh";
const ENV_KEY = "ucode.env";
const PROJECT_KEY = "ucode.project";
const PROFILE_KEY = "ucode.profile";
const USER_KEY = "ucode.user";
const PERMISSIONS_KEY = "ucode.permissions";

/** Что показывать в шапке сайдбара. Не секрет — только подписи. */
export type Profile = {
  name: string;
  role: string;
  company: string;
};

/**
 * Права роли на таблицы: слаг → набор флагов. Что значит каждый флаг —
 * знает features/auth; здесь это просто мешок булевых, потому что
 * shared/ не имеет права знать про фичи.
 *
 * Лежит в localStorage, а не только в памяти. Приезжает в ответе логина
 * и обновления токена, то есть после перезагрузки страницы появляется
 * лишь ПОСЛЕ первого 401 → refresh. Без localStorage панель настроек
 * успевала бы моргнуть «всё запрещено» и вернуться. Это не секрет:
 * решение принимает сервер, здесь только то, что рисовать.
 */
export type PermissionMap = Record<string, Record<string, boolean>>;

let accessToken: string | null = null;

/**
 * Слушатели изменений. Сессия живёт вне React, поэтому компоненты обязаны
 * на неё подписываться — иначе после переключения проекта они продолжат
 * рендериться со старым projectId, и ключи запросов не поменяются.
 *
 * version меняется при каждом изменении: это снимок для useSyncExternalStore.
 */
const listeners = new Set<() => void>();
let version = 0;

const notify = () => {
  version += 1;
  listeners.forEach((fn) => fn());
};

export const session = {
  getAccess: () => accessToken,
  getRefresh: () => localStorage.getItem(REFRESH_KEY),
  getEnvironmentId: () => localStorage.getItem(ENV_KEY),
  getProjectId: () => localStorage.getItem(PROJECT_KEY),

  /**
   * Id пользователя. Нужен как параметр запроса (например, список
   * компаний фильтруется по owner_id).
   *
   * Если он не сохранён — достаём из claim'ов access-токена. Подпись
   * при этом не проверяется, и это допустимо: мы читаем собственную
   * личность для параметра запроса, а не принимаем решение о доступе.
   * Проверку делает сервер. Запасной путь нужен, чтобы уже открытые
   * сессии заработали без повторного входа.
   */
  getUserId(): string {
    return localStorage.getItem(USER_KEY) || readClaim(accessToken, "user_id");
  },

  /**
   * Тип клиента текущего входа. Живёт только в токене: в ответе логина
   * его нет, а ручки профиля без него отвечают «client type id is an
   * invalid uuid» (auth_service, user_v2.go:191).
   */
  getClientTypeId(): string {
    return readClaim(accessToken, "client_type_id");
  },

  getProfile(): Profile | null {
    const raw = localStorage.getItem(PROFILE_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as Profile;
    } catch {
      // Испорченное значение — не повод падать: покажем пустую шапку.
      return null;
    }
  },

  setProfile(profile: Profile) {
    localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
    notify();
  },

  getPermissions(): PermissionMap {
    const raw = localStorage.getItem(PERMISSIONS_KEY);
    if (!raw) return {};
    try {
      return JSON.parse(raw) as PermissionMap;
    } catch {
      return {};
    }
  },

  setPermissions(permissions: PermissionMap) {
    localStorage.setItem(PERMISSIONS_KEY, JSON.stringify(permissions));
    notify();
  },

  /** Есть чем восстановиться после перезагрузки. */
  isAuthenticated: () => accessToken !== null || localStorage.getItem(REFRESH_KEY) !== null,

  /** Версия для useSyncExternalStore: меняется при любом изменении сессии. */
  getVersion: () => version,

  /**
   * Всё, что меняется вместе, записывается одним вызовом. Иначе между
   * записью токена и записью проекта случится промежуточный рендер,
   * где окружение уже новое, а проект ещё старый.
   */
  set(tokens: {
    access: string;
    refresh: string;
    environmentId?: string;
    projectId?: string;
    userId?: string;
  }) {
    accessToken = tokens.access;
    localStorage.setItem(REFRESH_KEY, tokens.refresh);
    if (tokens.environmentId) localStorage.setItem(ENV_KEY, tokens.environmentId);
    if (tokens.projectId) localStorage.setItem(PROJECT_KEY, tokens.projectId);
    if (tokens.userId) localStorage.setItem(USER_KEY, tokens.userId);
    notify();
  },

  clear() {
    accessToken = null;
    localStorage.removeItem(REFRESH_KEY);
    localStorage.removeItem(ENV_KEY);
    localStorage.removeItem(PROJECT_KEY);
    localStorage.removeItem(PROFILE_KEY);
    localStorage.removeItem(USER_KEY);
    localStorage.removeItem(PERMISSIONS_KEY);
    notify();
  },

  subscribe(fn: () => void) {
    listeners.add(fn);
    return () => void listeners.delete(fn);
  },
};

/** Читает одно поле из полезной нагрузки JWT. Подпись не проверяется. */
function readClaim(token: string | null, name: string): string {
  if (!token) return "";

  try {
    const payload = token.split(".")[1];
    if (!payload) return "";

    const json = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    const value = (JSON.parse(json) as Record<string, unknown>)[name];
    return typeof value === "string" ? value : "";
  } catch {
    return "";
  }
}
