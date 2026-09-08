import type { MenuDto } from "../api/dto";

/**
 * Все типы пунктов меню, которые отдаёт бэкенд. Список закрытый: пункт
 * незнакомого типа рисуется, но ведёт на страницу «тип не поддерживается»,
 * а не исчезает молча — пропавший пункт меню выглядит как потеря данных.
 */
export const MENU_TYPES = [
  "FOLDER",
  "TABLE",
  "LINK",
  "MICROFRONTEND",
  "MINIO_FOLDER",
  "PIVOT",
  "REST",
  "USER",
  "WEBPAGE",
  "WIKI",
  "WIKI_FOLDER",
] as const;

export type MenuType = (typeof MENU_TYPES)[number];

/**
 * Как пункт ведёт себя в сайдбаре, независимо от типа:
 *   group — раскрывается, сам никуда не ведёт
 *   leaf  — открывает экран внутри приложения
 *   link  — уводит наружу
 */
export type MenuKind = "group" | "leaf" | "link";

/*
 * MINIO_FOLDER сюда не входит намеренно: внутри него лежат ФАЙЛЫ,
 * а не пункты меню, и раскрывать в дереве нечего — щелчок открывает
 * хранилище. Так же ведёт себя и старая админка: пункт уводит
 * на страницу файлов, а «создать внутри» у него означает загрузку.
 */
const GROUPS = new Set<string>(["FOLDER", "WIKI_FOLDER"]);

export function kindOf(type: string): MenuKind {
  if (GROUPS.has(type)) return "group";
  if (type === "LINK") return "link";
  return "leaf";
}

/** Экраны, которые действительно есть. Остальные ведут на объяснение. */
export const IMPLEMENTED_TYPES = new Set<string>(["TABLE"]);

/**
 * Показывает ли пункт таблицу.
 *
 * Не только `type === "TABLE"`. Старая админка умела привязывать
 * к меню УЖЕ СУЩЕСТВУЮЩУЮ таблицу, и делала это пунктом типа `LINK`
 * с заполненным `table_id` (layouts/MainLayout/LinkTableModal.jsx:53).
 * Ссылки наружу у такого пункта нет вовсе, зато есть таблица и её view,
 * — и открываться он должен таблицей, а не объяснением «экрана нет».
 */
export function showsTable(menu: { type: string; tableId: string }): boolean {
  return IMPLEMENTED_TYPES.has(menu.type) || (menu.type === "LINK" && Boolean(menu.tableId));
}

export type MenuNode = {
  id: string;
  /** Подпись на текущем языке ДАННЫХ, иначе базовая. Для показа. */
  label: string;
  /**
   * Подписи по языкам данных целиком. Нужны форме переименования: у неё
   * по полю на язык, и показанной подписи для этого мало.
   */
  labels: Record<string, string>;
  /** Имя иконки от бэкенда. Может быть пустым — тогда берём по типу. */
  icon: string;
  type: string;
  kind: MenuKind;
  /**
   * Таблица, привязанная к пункту. У обычной таблицы стоит вместе
   * с типом TABLE, у пункта-ссылки на существующую таблицу — вместо
   * адреса (см. showsTable).
   */
  tableId: string;
  /**
   * Папка в файловом хранилище (`attributes.path`) — у пунктов
   * `MINIO_FOLDER`. Пункт меню и ЕСТЬ папка: списка папок в ручках нет,
   * соседняя папка — это соседний пункт.
   */
  folder: string;
  /**
   * Страница, встроенная рамкой в экран (`attributes.website_link`).
   * Пусто — встраивать нечего.
   *
   * Не путать с `href`: тот уводит НАРУЖУ, новой вкладкой. Разница
   * не наша выдумка — так эти две настройки различала и старая админка.
   */
  embedUrl: string;
  /**
   * Чужое приложение, которое показывает пункт (`menu.microfrontend_id`).
   * Пусто — показывать нечего.
   *
   * Адреса здесь нет: пункт хранит только идентификатор, а адрес сборки
   * лежит у микрофронтенда (`Function.url`) и дочитывается отдельно —
   * см. features/microfrontend.
   */
  microfrontendId: string;
  /**
   * Настройки запуска микрофронтенда (`attributes.params`): список пар
   * «ключ — значение», который старая админка превращала в query-строку.
   */
  params: Record<string, string>;
  order: number;
  /** Системный пункт: бэкенд запрещает его удалять (STATIC_MENU_IDS). */
  isStatic: boolean;
  parentId: string | null;
  children: MenuNode[];
  /** Права текущей роли на этот пункт. */
  can: MenuPermissions;
  /** Только у kind === "link". */
  href?: string;
  /**
   * Пункт, как его отдал бэкенд. Нужен для записи: PUT /v3/menus
   * перезаписывает строку целиком, а label здесь базовый — не тот
   * локализованный, что показан в сайдбаре.
   */
  raw: MenuDto;
};

/**
 * Права приходят на каждом пункте отдельно (menu_permission по роли).
 * Отсутствие права — не ошибка, а обычное состояние: действие просто
 * не показывается.
 */
export type MenuPermissions = {
  read: boolean;
  /** Право создавать внутри. Колонки "create" в menu_permission нет. */
  write: boolean;
  update: boolean;
  delete: boolean;
  /** Настройки внешнего вида сайдбара. */
  settings: boolean;
};
