import { session } from "@/shared/api/session";

/**
 * Зеркало сессии для ремоутов, которые ходят в API САМИ.
 *
 * Договор с ними такой, что договора нет: ремоут поколения «старая
 * админка» не берёт ось из пропсов, а поднимает свой axios и читает
 * токен из localStorage. В старой админке это работало по совпадению —
 * она писала туда redux-persist, и store ремоута рехидратировался её
 * свежим токеном. Мы этих ключей не пишем, поэтому ремоут подбирал
 * что придётся: в разборе это был токен, выданный восемью днями раньше
 * и протухший через сутки после выдачи.
 *
 * Здесь мы кладём в те же ключи наше состояние. Это НЕ признание
 * чужого формата за свой: наружу отдаётся снимок, обратно не читается
 * никогда, и живёт он только пока открыт экран микрофронтенда.
 *
 * **Цена названа отдельно** ([ADR-0005](../../../../docs/adr/0005-microfrontend-embedding.md)):
 * на время этого экрана access-токен оказывается в localStorage, хотя
 * во всё остальное время он живёт только в памяти (`shared/api/session`),
 * потому что XSS достаёт хранилище, но не замыкание. Отсюда и
 * ограничение по времени: пишем на входе, стираем на выходе и на старте
 * приложения.
 *
 * Новым ремоутам это не нужно и не будет предложено: у них ось
 * приезжает пропсом, а токен не покидает памяти.
 */

/** Ключи redux-persist старой админки (`store/rootReducer.js:43,73`). */
const AUTH_KEY = "persist:auth";
const COMPANY_KEY = "persist:company";

/**
 * Формат redux-persist 6: значение ключа — JSON, в котором КАЖДОЕ поле
 * само является JSON-строкой (`createPersistoid.js:77,98`). То есть
 * достать токен можно только двумя `JSON.parse` подряд — так его и
 * достаёт сама старая админка (`LayoutSidebar/index.jsx:459-465`).
 *
 * Поле `_persist` обязательно: без него redux-persist считает запись
 * чужой и не рехидратирует её вовсе.
 */
function persisted(fields: Record<string, unknown>): string {
  const encoded: Record<string, string> = {};
  for (const [name, value] of Object.entries(fields)) encoded[name] = JSON.stringify(value);
  encoded._persist = JSON.stringify({ version: -1, rehydrated: true });
  return JSON.stringify(encoded);
}

/**
 * «Своя строка» в таблицах аудитории — в той форме, в какой её отдавал
 * вход старой админки. У нас она разложена в словарь (`getObjectIds`),
 * а ремоут ждёт список.
 */
function tables(): { table_slug: string; object_id: string }[] {
  return Object.entries(session.getObjectIds()).map(([slug, id]) => ({
    table_slug: slug,
    object_id: id,
  }));
}

/** Убрать зеркало. Отдельно от `start`, потому что зовётся ещё и на старте. */
export function clearLegacyMirror() {
  localStorage.removeItem(AUTH_KEY);
  localStorage.removeItem(COMPANY_KEY);
}

/**
 * Имя окружения, с которым зеркало написали в прошлый раз. Нужно,
 * чтобы переписать его при обновлении токена, не спрашивая экран
 * заново: сессия имени окружения не знает, его знает только список
 * окружений.
 */
let lastEnvironmentName = "";

/**
 * Написать зеркало. Зовёт ЗАГРУЗЧИК, синхронно, перед тем как взять
 * у ремоута модуль: чужой store рехидратируется при вычислении модуля,
 * а эффект экрана к этому моменту может и не успеть.
 *
 * И зовёт только для СТАРОГО поколения. У ремоута на SDK зеркала
 * не будет вовсе — значит и скопированный из старого проекта store
 * не найдёт токена и сломается сразу, у автора, а не у клиента.
 */
export function writeLegacyMirror(environmentName: string) {
  lastEnvironmentName = environmentName;

  const access = session.getAccess();

  // Токена нет — значит и зеркалить нечего: сессия кончилась, пока
  // экран был открыт. Пустое зеркало хуже отсутствующего: ремоут
  // отправит запрос с пустым Bearer вместо того, чтобы промолчать.
  if (!access) return clearLegacyMirror();

  const projectId = session.getProjectId() ?? "";
  const environmentId = session.getEnvironmentId() ?? "";
  const profile = session.getProfile();

  localStorage.setItem(
    AUTH_KEY,
    persisted({
      isAuth: true,
      token: access,
      refreshToken: session.getRefresh() ?? "",
      userId: session.getUserId(),
      /*
       * `roleInfo` и `userInfo` читает разобранный живой ремоут
       * (`getAuthData()` в его сборке) — рядом с токеном, из того же
       * ключа. Отсутствующее поле даст у него `Cannot read properties
       * of null`, поэтому кладём то, что знаем: у нас от профиля есть
       * имя и роль, id пользователя — из сессии.
       */
      roleInfo: { name: profile?.role ?? "" },
      userInfo: { id: session.getUserId(), name: profile?.name ?? "" },
      projectId,
      environmentId,
      permissions: session.getPermissions(),
      globalPermissions: session.getGlobalRights() ?? {},
      tables: tables(),
      clientType: { id: session.getClientTypeId() },
      /*
       * У нас этого поля нет вовсе — вся новая админка живёт без
       * `resource-id`, и шлюз берёт ресурс окружения по умолчанию.
       * Пустая строка, а не пропуск: ремоут пишет заголовок из этого
       * поля безусловно, и `undefined` в axios просто исчез бы, а вот
       * `null` уехал бы строкой "null".
       */
      resourceId: "",
    }),
  );

  localStorage.setItem(
    COMPANY_KEY,
    persisted({
      projectId,
      environmentId,
      /*
       * Ремоут смотрит на `environmentItem.name`, чтобы отличить
       * production от staging (`MicrofrontendComponent/index.jsx:52-62`).
       * Остальные поля слайса — списки для экранов старой админки,
       * ремоуту они не нужны, и заполнять их значило бы обещать больше,
       * чем мы отдаём.
       */
      environmentItem: { id: environmentId, name: environmentName },
      companyId: null,
    }),
  );
}

/**
 * Держать зеркало свежим, пока открыт экран. Возвращает функцию
 * выключения.
 *
 * Сама НЕ создаёт: создаёт загрузчик, и только для старого поколения.
 * Поэтому первая же проверка — «а есть ли что обновлять»: у нового
 * ремоута зеркала нет, и появиться оно здесь не должно.
 *
 * Подписка обязательна, а не «на всякий случай»: токен обновляется
 * посреди работы ремоута, и тот, кто читает хранилище на каждый
 * запрос, обязан увидеть новый. Та же подписка стирает зеркало при
 * выходе — `session.clear()` тоже дёргает слушателей.
 */
export function startLegacyMirror(): () => void {
  const refresh = () => {
    if (!localStorage.getItem(AUTH_KEY)) return;
    writeLegacyMirror(lastEnvironmentName);
  };

  const unsubscribe = session.subscribe(refresh);

  /*
   * `pagehide`, а не `beforeunload`: срабатывает и при уходе в bfcache,
   * и на мобильных. Закрывает единственную настоящую дыру этой схемы —
   * вкладку, закрытую прямо на экране микрофронтенда, с токеном
   * в хранилище.
   */
  window.addEventListener("pagehide", clearLegacyMirror);

  return () => {
    unsubscribe();
    window.removeEventListener("pagehide", clearLegacyMirror);
    clearLegacyMirror();
  };
}
