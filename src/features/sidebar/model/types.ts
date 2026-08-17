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

const GROUPS = new Set<string>(["FOLDER", "WIKI_FOLDER", "MINIO_FOLDER"]);

export function kindOf(type: string): MenuKind {
  if (GROUPS.has(type)) return "group";
  if (type === "LINK") return "link";
  return "leaf";
}

/** Экраны, которые в v1 действительно есть. Остальные ведут на заглушку. */
export const IMPLEMENTED_TYPES = new Set<string>(["TABLE"]);

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
