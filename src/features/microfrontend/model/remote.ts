import type { ComponentType } from "react";

/**
 * Загрузка чужого приложения через Module Federation.
 *
 * Так же его грузит и старая админка
 * (`components/MicrofrontendComponent/index.jsx`): сборка ремоута лежит
 * по адресу `https://<url>/assets/remoteEntry.js`, из неё берётся модуль
 * `./Page`, и это обычный React-компонент внутри нашего дерева —
 * не страница в рамке.
 *
 * Почему не рамка: ремоуту нужны наш токен, язык и окружение, а iframe
 * их не получит. Цена названа в [ADR-0005](../../../../docs/adr/0005-microfrontend-embedding.md):
 * чужой код исполняется в НАШЕМ origin, песочницы у него нет.
 *
 * Отличие от старой админки одно, и осознанное: мы НЕ объявляем общими
 * `react` и `react-dom`. Ремоуты собраны против React 18, у нас 19,
 * и общий синглтон отдал бы им нашу версию. Пусть каждый работает
 * на своей: два React на странице — штатный запасной путь федерации,
 * а взаимного контекста у нас с ремоутом и нет, всё нужное уезжает
 * пропсами.
 *
 * ponytail: плагин федерации в сборку не ставим — для потребителя
 * хватает рантайма. Понадобится общий React (ремоут захочет наш
 * контекст) — тогда `@module-federation/vite` и `shared`.
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

export type RemotePage = ComponentType<RemotePageProps>;

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

export type LoginRemotePage = ComponentType<LoginRemoteProps>;

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
 * Взять компонент `./Page` у микрофронтенда.
 *
 * Регистрация повторяется только при СМЕНЕ адреса — например, когда
 * версию ремоута продвинули. Иначе каждый заход перезаписывал бы запись
 * рантайма и сбрасывал уже загруженный модуль.
 */
export async function loadRemotePage<P = RemotePageProps>(
  id: string,
  host: string,
): Promise<ComponentType<P>> {
  const name = remoteName(id);
  const entry = entryUrl(host);

  if (!entry) throw new Error("microfrontend has no url");

  /*
   * Рантайм федерации грузится отдельным куском и только здесь: это
   * 70 кБ, которые нужны одному типу пункта меню, а в проекте без
   * микрофронтендов не нужны вовсе.
   */
  const { loadRemote, registerRemotes } = await import("@module-federation/runtime");

  if (registered.get(name) !== entry) {
    // type: "module" — remoteEntry.js собран Vite'ом как ES-модуль
    // (`format: "esm"` у плагина федерации), и рантайм грузит его import'ом.
    registerRemotes([{ name, entry, type: "module" }], { force: true });
    registered.set(name, entry);
  }

  type Page = ComponentType<P>;
  const loaded = await loadRemote<Page | { default: Page }>(`${name}/Page`);
  if (!loaded) throw new Error(`microfrontend ${name} has no ./Page`);

  // Модуль отдают и объектом с default, и самим компонентом.
  return typeof loaded === "function" ? loaded : loaded.default;
}
