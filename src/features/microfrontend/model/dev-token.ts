import { session } from "@/shared/api/session";

/**
 * Костыль для РАЗРАБОТКИ: подменить протухший чужой токен живым.
 *
 * Зачем он есть. Ремоуты ucode прежнего поколения зашивают в сборку
 * токен «для разработки» и берут именно его, когда админка открыта на
 * localhost. Разобранный живой микрофронтенд делает это дословно:
 *
 *     const isDev = hostname === "localhost" || hostname === "127.0.0.1";
 *     if (isDev) return devToken;                   // зашит в бандл
 *     else       return из localStorage persist:auth;
 *
 * Токен там был выдан 17 августа и протух через сутки, поэтому на
 * localhost такой ремоут отвечает «Failed to load data» и в хранилище
 * не заглядывает вовсе — ни наше зеркало, ни что-либо ещё ему не
 * помогает. На боевом домене он идёт по второй ветке и работает.
 *
 * Проверить микрофронтенд локально без этой подмены нельзя: сменить
 * хост нельзя тоже, потому что другой хост — другой origin, а сессия
 * живёт в localStorage адреса `localhost:7777`.
 *
 * **В сборку это не попадает.** `import.meta.env.DEV` — константа
 * времени сборки, и в production всё тело функции выкидывается
 * минификатором. Настоящая починка — на стороне ремоута: убрать
 * зашитый токен.
 */

/** Патч ставится один раз на страницу. */
let patched = false;

export function replaceExpiredTokensInDev() {
  // XMLHttpRequest нет в node — загрузчик проверяют тестами, и там
  // патчить нечего.
  if (!import.meta.env.DEV || patched || typeof XMLHttpRequest === "undefined") return;
  patched = true;

  /*
   * Правим на уровне XMLHttpRequest, а не axios: у чужого ремоута
   * своя копия axios внутри его бандла, дотянуться до неё нечем.
   * Оси нашего приложения сюда тоже попадают — и это безвредно:
   * протухший токен они не шлют, его обновляет `refreshOnce`.
   */
  const setHeader = XMLHttpRequest.prototype.setRequestHeader;

  XMLHttpRequest.prototype.setRequestHeader = function (name: string, value: string) {
    const fixed =
      name.toLowerCase() === "authorization" ? withLiveToken(value, session.getAccess()) : value;

    return setHeader.call(this, name, fixed);
  };
}

/**
 * Живой токен вместо протухшего. Чистая — на ней и держится проверка.
 *
 * Трогаем ТОЛЬКО протухшее: чужой ключ, не-JWT или живой токен другого
 * сервиса проходят как есть. Иначе подмена начала бы чинить то, что
 * не сломано, и прятала бы настоящие ошибки доступа.
 */
export function withLiveToken(header: string, access: string | null): string {
  if (!access) return header;

  const token = header.replace(/^Bearer\s+/i, "");
  if (token === access || !isExpired(token)) return header;

  return `Bearer ${access}`;
}

function isExpired(token: string): boolean {
  try {
    const payload = token.split(".")[1];
    if (!payload) return false;

    const { exp } = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/"))) as {
      exp?: unknown;
    };
    return typeof exp === "number" && exp * 1000 < Date.now();
  } catch {
    // Не JWT — не наше дело.
    return false;
  }
}
