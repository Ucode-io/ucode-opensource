import type { TranslationKey } from "@/shared/lib/i18n";
import type { MenuNode } from "./types";

/**
 * Реестр действий над пунктом меню.
 *
 * В старом ucode эти же действия были расписаны по типам: «Edit folder»,
 * «Edit table», «Edit Wiki», «Edit Website», «Edit microfrontend» — пять
 * веток одного и того же, и файл вырос до 862 строк. Здесь действие одно,
 * а тип влияет только на подпись.
 */

export type MenuActionId =
  | "create-table"
  | "create-folder"
  | "create-link"
  | "edit"
  | "settings"
  | "make-template"
  | "delete";

export type MenuAction = {
  id: MenuActionId;
  /** Ключ перевода. Подпись зависит от типа пункта: «Изменить папку» / «Изменить таблицу». */
  labelKey: TranslationKey;
  /** Право, без которого действие не показывается. */
  requires: keyof MenuNode["can"];
  /** Разделитель перед пунктом — отделяет опасное от обычного. */
  separated?: boolean;
  danger?: boolean;
};

const ALL: readonly MenuAction[] = [
  { id: "create-table", labelKey: "menuAction.createTable", requires: "write" },
  { id: "create-folder", labelKey: "menuAction.createFolder", requires: "write" },
  { id: "create-link", labelKey: "menuAction.createLink", requires: "write" },
  { id: "edit", labelKey: "menuAction.edit", requires: "update" },
  { id: "settings", labelKey: "menuAction.settings", requires: "settings" },
  { id: "make-template", labelKey: "menuAction.makeTemplate", requires: "update" },
  { id: "delete", labelKey: "menuAction.delete", requires: "delete", separated: true, danger: true },
];

/** Создавать что-то внутри можно только у того, что раскрывается. */
const ONLY_GROUPS = new Set<MenuActionId>([
  "create-table",
  "create-folder",
  "create-link",
  "make-template",
]);

export function actionsFor(node: MenuNode, isAdmin: boolean): MenuAction[] {
  return ALL.filter((action) => {
    if (!node.can[action.requires]) return false;
    if (ONLY_GROUPS.has(action.id) && node.kind !== "group") return false;
    // Шаблон из папки делает только администратор — так было и раньше.
    if (action.id === "make-template" && !isAdmin) return false;
    // Системные пункты бэкенд удалять запрещает (STATIC_MENU_IDS),
    // поэтому кнопки, которая всегда вернёт ошибку, быть не должно.
    if (action.id === "delete" && node.isStatic) return false;
    return true;
  });
}

/**
 * Слово для типа: «папку», «таблицу», «ссылку». Карта явная, а не собранная
 * из строки: тогда отсутствующий перевод — ошибка компиляции, а не пустое
 * место в подписи кнопки.
 */
const TYPE_WORDS: Record<string, TranslationKey> = {
  FOLDER: "menuType.FOLDER",
  WIKI_FOLDER: "menuType.WIKI_FOLDER",
  MINIO_FOLDER: "menuType.MINIO_FOLDER",
  TABLE: "menuType.TABLE",
  PIVOT: "menuType.PIVOT",
  REST: "menuType.REST",
  USER: "menuType.USER",
  WEBPAGE: "menuType.WEBPAGE",
  WIKI: "menuType.WIKI",
  MICROFRONTEND: "menuType.MICROFRONTEND",
  LINK: "menuType.LINK",
};

export function typeWordKey(type: string): TranslationKey {
  return TYPE_WORDS[type] ?? "menuType.UNKNOWN";
}
