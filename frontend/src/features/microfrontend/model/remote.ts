import * as React from "react";
import * as JsxRuntime from "react/jsx-runtime";
import * as ReactDom from "react-dom";
import { replaceExpiredTokensInDev } from "./dev-token";
import { writeLegacyMirror } from "./legacy-mirror";

/**
 * Загрузка чужого приложения через Module Federation.
 *
 * Так же его грузит и старая админка
 * (`components/MicrofrontendComponent/index.jsx`): сборка ремоута лежит
 * по адресу `https://<url>/assets/remoteEntry.js`, из неё берётся модуль
 * `./Page`, и это обычный React-компонент — не страница в рамке.
 *
 * Почему не рамка: ремоуту нужны наш токен, язык и окружение, а iframe
 * их не получит. Цена названа в [ADR-0005](../../../../docs/adr/0005-microfrontend-embedding.md):
 * чужой код исполняется в НАШЕМ origin, песочницы у него нет.
 *
 * **React ремоуту даём мы, и это React 18.** ADR-0005 предполагал
 * обратное — «пусть у каждого свой», — и первый же живой ремоут это
 * опроверг двумя ошибками подряд:
 *
 * 1. Своя копия React у ремоута не работает вовсе. Его компонент рисует
 *    НАШ react-dom, а хуки он зовёт у своей копии, и её диспетчер пуст:
 *    `Cannot read properties of null (reading 'useEffect')`. React в
 *    поддереве может быть только один.
 * 2. Отдать ему нашу 19-ю тоже нельзя. JSX ремоута собран его
 *    собственным `react/jsx-runtime` (в общие он не объявлен — не был
 *    и в старой админке), а элементы React 18 наш React 19 отвергает
 *    явной ошибкой #525 «A React Element from an older version».
 *
 * Поэтому в проекте лежит вторая копия React, 18.3.1, — пакетом
 * `vendor/react18` (почему пакетом, а не парой `npm:`-псевдонимов,
 * написано в его README: иначе `react-dom` 18-й достаётся peer'ом
 * наш React 19). Её мы кладём в общую область федерации и ею же
 * рисуем: ремоут живёт в СВОЁМ корне, в отдельном узле DOM внутри
 * нашей страницы — см. `ui/RemoteHost.tsx`.
 *
 * Цена — +45 кБ в куске микрофронтенда и никакого общего контекста
 * с ремоутом. Второго у нас и не было: всё нужное уезжает пропсами.
 *
 * ponytail: плагин федерации в сборку не ставим — для потребителя
 * хватает рантайма и восьми строк общей области.
 */

/** Что мы обязаны передать ремоуту. Это публичный договор — см. ADR-0005. */
export type RemotePageProps = {
  /** Меняется при каждом открытии: ремоут перемонтируется, а не оживает. */
  activationKey: string;
  /** Тот же ключ вторым именем — старые ремоуты читают его так. */
  microfrontendActivationKey: string;
  environment: "production" | "staging";
  /** Инстанс i18next хоста: подписи ремоута идут на языке админки. */
  i18n: unknown;
  /** Ось http хоста — с токеном и обновлением. Раньше это был axios 0.26. */
  sharedHttpRequest: unknown;
  sharedHttpRequestV2: unknown;
  /** `attributes.params` пункта меню. */
  params: Record<string, string>;
};

/**
 * Компонент чужого React. Опаковый нарочно: наш React его не рисует
 * и типы его пропсов проверить не может — договор описан выше словами
 * и держится на `RemotePageProps`, из которого мы собираем объект.
 */
export type RemoteComponent = unknown;

/**
 * Пропсы ремоута, подменяющего ЭКРАН ВХОДА. Договор другой, и это
 * не оплошность: там нет ни токена, ни окружения, ни пункта меню —
 * есть только способ войти.
 *
 * Имя `loginAction` — из старой админки
 * (`layouts/AuthLayout/LoginMicrofrontend.jsx:19`), где в него уезжал
 * redux-thunk. Написанные под неё ремоуты зовут именно его, поэтому
 * имя сохранено, а телом стал обычный вызов входа.
 */
export type LoginRemoteProps = {
  loginAction: (credentials: { username: string; password: string }) => Promise<void>;
  i18n: unknown;
};

/**
 * Полный адрес сборки.
 *
 * В базе лежит голый хост, без схемы: старая админка всюду склеивает
 * `https://${url}/assets/remoteEntry.js`. Схему всё же проверяем —
 * адрес приходит из поля, которое заполняет человек, и уже записанный
 * с `https://` не должен превратиться в `https://https://…`.
 */
export function entryUrl(host: string): string {
  const trimmed = host.trim().replace(/\/+$/, "");
  if (!trimmed) return "";

  const withScheme = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
  return `${withScheme}/assets/remoteEntry.js`;
}

/**
 * Имя ремоута в рантайме федерации. Оно становится частью строки
 * `имя/Page`, поэтому ни косой черты, ни точки в нём быть не может —
 * берём идентификатор микрофронтенда и оставляем от него буквы и цифры.
 */
export function remoteName(id: string): string {
  return `mf_${id.replace(/[^a-zA-Z0-9]/g, "")}`;
}

/** Что уже зарегистрировано: имя → адрес сборки. */
const registered = new Map<string, string>();

/**
 * Положить модуль в общую область федерации `@originjs` — плагина,
 * которым собраны все ремоуты ucode.
 *
 * Протокол простой и целиком в глобальной переменной. Потребитель
 * (`@originjs/vite-plugin-federation/dist/index.js:1846`) читает
 * `globalThis.__federation_shared__[scope][имя]`, берёт ПЕРВУЮ версию
 * и зовёт `(await entry.get())()`. Хост-плагин кладёт туда ровно
 * `{get, loaded}` (там же, строка 1580) — то же кладём и мы, только
 * без сборочного плагина: модуль у нас уже в руках, оборачивать его
 * в чанк незачем.
 *
 * Не найдёт — возьмёт копию из своей сборки. Именно это и происходило
 * до появления этой функции.
 *
 * **Область одна на всех**: развести поколения по разным областям
 * нельзя. Опция `shareScope` у плагина влияет только на сторону хоста,
 * а потребитель собирается с вызовом `importShared('имя')` БЕЗ второго
 * аргумента (там же, строки 521, 529, 538) — то есть всегда читает
 * `default`.
 *
 * **Зато версий в ней может быть несколько**, и порядок ключей решает
 * всё. Потребитель, у которого в `shared` не проставлен
 * `requiredVersion`, берёт `Object.keys(versionObj)[0]` — первую;
 * с `requiredVersion` — ищет подходящую перебором
 * (`@originjs 1.4.1/dist/index.js:956`). Все существующие ремоуты
 * пишут `shared: ["react","react-dom"]`, то есть без версии, и первой
 * им обязана достаться 18-я.
 *
 * Поэтому объект версий пересобирается целиком и всегда по возрастанию
 * мажора: 18, потом 19. Иначе порядок зависел бы от того, какой
 * микрофронтенд открыли первым, — и старый ремоут после нового молча
 * получил бы React 19.
 */
type SharedEntry = Record<string, { get: () => () => unknown; loaded: number }>;

/** Что уже отдано ремоутам: имя модуля → версия → сам модуль. */
const shared = new Map<string, Map<string, unknown>>();

export function shareWithRemotes(name: string, module: unknown, version: string) {
  const versions = shared.get(name) ?? new Map<string, unknown>();
  versions.set(version, module);
  shared.set(name, versions);

  const ordered: SharedEntry = {};
  for (const [known, value] of [...versions].sort(([a], [b]) => parseInt(a) - parseInt(b))) {
    ordered[known] = { get: () => () => value, loaded: 1 };
  }

  const host = globalThis as {
    __federation_shared__?: Record<string, Record<string, SharedEntry>>;
  };
  const scopes = (host.__federation_shared__ ??= {});
  const scope = (scopes.default ??= {});
  scope[name] = ordered;
}

/** Пара React 18 грузится один раз на страницу — и корней, и ремоутов бывает несколько. */
let react18: Promise<{
  createElement: (type: unknown, props?: object) => unknown;
  createRoot: (container: Element) => { render(node: unknown): void; unmount(): void };
}> | null = null;

function loadReact18() {
  return (react18 ??= (async () => {
    const [react, reactDom, client] = await Promise.all([
      import("react18/react"),
      import("react18/dom"),
      import("react18/client"),
    ]);

    // Ремоут спрашивает `react` и `react-dom` — ровно те два имени, что
    // стоят у него в `shared` (и стояли у старой админки). Кладём сами
    // CJS-объекты: потребитель разворачивает `.default`, если он есть,
    // а у них его нет, — значит и гадать про интероп не приходится.
    //
    // Версию берём из самого React, а не из константы: расходиться
    // с тем, что реально лежит в сборке, ей незачем.
    shareWithRemotes("react", react.default, react.default.version);
    shareWithRemotes("react-dom", reactDom.default, react.default.version);

    return { createElement: react.default.createElement, createRoot: client.default.createRoot };
  })());
}

/**
 * React ДЛЯ НОВЫХ РЕМОУТОВ — наш собственный, тот же самый, на котором
 * работает админка. Такой ремоут живёт прямо в нашем дереве: ни второго
 * корня, ни второй копии React, и контекст у нас с ним общий.
 *
 * `react/jsx-runtime` в общую область идёт наравне с `react`, и это
 * обязательное условие, а не запас: JSX ремоута собирается ИМЕННО им,
 * а элемент чужого рантайма наш React 19 отвергает ошибкой #525
 * (`react-dom-client.production.js:3291`). Ровно на этом и сломался
 * первый заход — см. ADR-0005.
 */
function loadReact19Into() {
  shareWithRemotes("react", React, React.version);
  shareWithRemotes("react-dom", ReactDom, React.version);
  shareWithRemotes("react/jsx-runtime", JsxRuntime, React.version);
}

export type RemoteRoot = {
  render(page: RemoteComponent, props: object): void;
  unmount(): void;
};

/**
 * Корень ремоута в отдельном узле DOM: чужое дерево рисует React 18,
 * наше — React 19, и общего у них только этот узел.
 *
 * Узел передаёт вызывающий, и он же его убирает: React 18 должен быть
 * единственным владельцем контейнера, иначе наш React 19 вырвет DOM
 * из-под чужого дерева при перерисовке.
 */
export async function createRemoteRoot(container: Element): Promise<RemoteRoot> {
  const { createElement, createRoot } = await loadReact18();
  const root = createRoot(container);

  return {
    render: (page, props) => root.render(createElement(page, props)),
    unmount: () => root.unmount(),
  };
}

/**
 * Самоописание ремоута — модуль `./ucode`, который экспонируют только
 * новые сборки (см. SDK). По его наличию хост и различает поколения:
 * спросить сборку напрямую больше не о чем, `shared` в remoteEntry
 * снаружи не виден.
 *
 * Имя `./ucode`, а не `./meta`: `meta` — слово общего пользования,
 * и чужой ремоут мог экспонировать его для своего.
 */
export type RemoteMeta = {
  /** Версия договора. Хост знает версии по CONTRACT включительно. */
  contract: number;
  /** На чём собран ремоут. Пока значение одно, но читаем его, а не подразумеваем. */
  react: "19";
};

/** До какой версии договора умеет этот хост. */
export const CONTRACT = 1;

/** Ремоут новее админки. Отдельный тип, потому что и текст на экране отдельный. */
export class RemoteContractError extends Error {
  constructor(readonly required: number) {
    super(`microfrontend requires contract ${required}, host knows ${CONTRACT}`);
    this.name = "RemoteContractError";
  }
}

/**
 * Спросить у ремоута модуль и простить отказ.
 *
 * Отказ здесь — обычный ответ «такого модуля нет», а не сбой: @originjs
 * бросает из `get` (`if(!moduleMap[module]) throw new Error('Can not
 * find remote module …')`, 1.4.1/dist/index.js:1207; в 1.2.x — TypeError
 * на `undefined`), а рантайм эту ошибку пробрасывает наружу.
 *
 * Пробовать безопасно: рантайм кэширует модуль по ИМЕНИ РЕМОУТА, а не
 * по экспоузу (`runtime-core/dist/remote/index.js:322-330`), и отказ
 * `Module.get` нигде не запоминается — следующая попытка идёт начисто.
 * Сети это тоже не стоит: `moduleMap` уже в памяти, бросок синхронный.
 */
async function tryLoad<T>(
  loadRemote: <R>(id: string) => Promise<R | null>,
  id: string,
): Promise<T | null> {
  try {
    return await loadRemote<T>(id);
  } catch (error) {
    /*
     * Причину печатаем целиком, а не глотаем: «модуля нет» и «модуль
     * есть, но упал при вычислении» выглядят здесь одинаково, а чинятся
     * по-разному. Уровень debug — потому что для `./ucode` и для `./App`
     * отказ это норма.
     */
    console.debug(`microfrontend: ${id} не загрузился, пробуем дальше`, error);
    return null;
  }
}

/** Разобрать `./ucode`. `null` — ремоут старого поколения. */
function readContract(loaded: unknown): RemoteMeta | null {
  if (!loaded || typeof loaded !== "object") return null;

  // @originjs отдаёт либо сам модуль, либо его `default` — смотря какие
  // в нём экспорты. Договор описан именованным `meta`.
  const module = loaded as { meta?: unknown; contract?: unknown };
  const meta = (typeof module.contract === "number" ? module : module.meta) as RemoteMeta | undefined;

  return meta && typeof meta.contract === "number" ? meta : null;
}

/**
 * Взять у микрофронтенда его страницу и, если он новый, — договор.
 *
 * Регистрация повторяется только при СМЕНЕ адреса — например, когда
 * версию ремоута продвинули. Иначе каждый заход перезаписывал бы запись
 * рантайма и сбрасывал уже загруженный модуль.
 */
export async function loadRemotePage(
  id: string,
  host: string,
  environmentName = "",
): Promise<{ page: RemoteComponent; meta: RemoteMeta | null }> {
  const name = remoteName(id);
  const entry = entryUrl(host);

  if (!entry) throw new Error("microfrontend has no url");

  /*
   * Рантайм федерации грузится отдельным куском и только здесь: это
   * 70 кБ, которые нужны одному типу пункта меню, а в проекте без
   * микрофронтендов не нужны вовсе.
   */
  const { init, loadRemote, registerRemotes } = await import("@module-federation/runtime");

  /*
   * Без `init` рантайм 2.x падает с RUNTIME-009 «Please call createInstance
   * first»: `registerRemotes` и `loadRemote` верхнего уровня — обёртки над
   * инстансом, который создаёт только `init`. Хост собран без плагина
   * федерации, поэтому создать инстанс некому, кроме нас.
   *
   * Повторный вызов безопасен: `init` находит уже созданный инстанс по имени
   * и лишь дописывает опции.
   */
  init({ name: "ucode-admin", remotes: [] });

  if (registered.get(name) !== entry) {
    // type: "module" — remoteEntry.js собран Vite'ом как ES-модуль
    // (`format: "esm"` у плагина федерации), и рантайм грузит его import'ом.
    registerRemotes([{ name, entry, type: "module" }], { force: true });
    registered.set(name, entry);
  }

  /*
   * Порядок здесь и есть всё решение.
   *
   * Сперва спрашиваем поколение: `./ucode` не импортирует ничего, общей
   * области не касается и `moduleCache` ремоута не заполняет — тот
   * наполняется только вызовами `importShared`, которых в модуле из
   * двух констант нет.
   *
   * Потом кладём в область React. И только потом грузим страницу: её
   * первый `importShared('react')` и замораживает выбор в собственном
   * `moduleCache` ремоута до конца жизни его бандла.
   */
  const meta = readContract(await tryLoad(loadRemote, `${name}/ucode`));

  if (meta && meta.contract > CONTRACT) throw new RemoteContractError(meta.contract);

  if (meta) {
    loadReact19Into();
  } else {
    await loadReact18();
    // Ремоут прежнего поколения ходит в API сам и читает состояние
    // из localStorage. Зеркало пишется ЗДЕСЬ, а не в эффекте экрана:
    // чужой store рехидратируется при вычислении модуля, то есть
    // строкой ниже. Эффект React к этому моменту может и не успеть.
    writeLegacyMirror(environmentName);
    // А на localhost такой ремоут в хранилище и не смотрит — берёт
    // зашитый в сборку токен. Только для разработки, см. dev-token.
    replaceExpiredTokensInDev();
  }

  /*
   * Имя модуля страницы у ремоутов разное, и это не небрежность,
   * а две волны шаблонов: `./Page` в старом (`lodify_test-master`),
   * `./App` в нынешнем (`ucode_template_react` и всё, что от него
   * форкнуто бэкендом). Спрашиваем оба — иначе три четверти
   * существующих микрофронтендов не открываются вовсе.
   *
   * Новым это не нужно: SDK экспонирует ровно `./Page`.
   */
  // Без "./" — рантайм добавляет его сам, разбирая "имя/модуль".
  const exposed = meta ? ["Page"] : ["Page", "App"];

  let loaded: RemoteComponent | { default: RemoteComponent } | null = null;
  for (const expose of exposed) {
    loaded = await tryLoad<RemoteComponent | { default: RemoteComponent }>(
      loadRemote,
      `${name}/${expose}`,
    );
    if (loaded) break;
  }

  if (!loaded) {
    throw new Error(`microfrontend ${name} exposes none of ${exposed.map((e) => `./${e}`).join(", ")}`);
  }

  // Модуль отдают и объектом с default, и самим компонентом.
  const page =
    typeof loaded === "function" ? loaded : (loaded as { default: RemoteComponent }).default;

  return { page, meta };
}
