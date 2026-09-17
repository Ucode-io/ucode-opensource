import {
  IconBucket,
  IconFolder,
  IconFolderSymlink,
  IconLayoutGrid,
  IconLink,
  IconPencil,
  IconTable,
  IconTemplate,
  IconTrash,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import type { TranslationKey } from "@/shared/lib/i18n";
import { showsTable, type MenuNode } from "./types";

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
  | "link-table"
  | "create-folder"
  | "create-link"
  | "create-files"
  | "create-microfrontend"
  | "edit"
  | "move"
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
  /** Та же иконка, что и у типа пункта в дереве (MenuIcon.byType) — где
   *  действие заводит пункт того же типа. */
  icon: TablerIcon;
};

const ALL: readonly MenuAction[] = [
  { id: "create-table", labelKey: "menuAction.createTable", requires: "write", icon: IconTable },
  /*
   * Пункт на УЖЕ СУЩЕСТВУЮЩУЮ таблицу — старое «Add table»
   * (TableLinkModal.jsx). Не то же, что «создать таблицу»: таблица
   * остаётся одна, а открывать её начинают из двух мест.
   */
  { id: "link-table", labelKey: "menuAction.linkTable", requires: "write", icon: IconTable },
  { id: "create-folder", labelKey: "menuAction.createFolder", requires: "write", icon: IconFolder },
  { id: "create-link", labelKey: "menuAction.createLink", requires: "write", icon: IconLink },
  { id: "create-files", labelKey: "menuAction.createFiles", requires: "write", icon: IconBucket },
  {
    id: "create-microfrontend",
    labelKey: "menuAction.createMicrofrontend",
    requires: "write",
    icon: IconLayoutGrid,
  },
  { id: "edit", labelKey: "menuAction.edit", requires: "update", icon: IconPencil },
  /*
   * «Перенести» — старое «Move table / Move microfrontend»
   * (MenuButtons.jsx:270, 432): выбрать новую папку списком, а не тащить
   * мышью. Тому же и служит: перетаскивание не достаёт до свёрнутой папки
   * на другом конце дерева и требует глобального права `menu_drag`.
   *
   * Право — `update`: смена родителя уходит тем же PUT /v3/menus, что
   * и переименование. Старая админка спрашивала здесь `menu_settings`,
   * который в базе по умолчанию false, — и пункт не показывался никому,
   * кроме DEFAULT ADMIN, которому права не проверяли вовсе.
   */
  { id: "move", labelKey: "menuAction.move", requires: "update", icon: IconFolderSymlink },
  {
    id: "make-template",
    labelKey: "menuAction.makeTemplate",
    requires: "update",
    icon: IconTemplate,
  },
  /*
   * «Настройки пункта» здесь была и ничего не делала: обработчика у неё
   * нет, экрана за ней не написано. Рабочая на вид кнопка без действия
   * хуже отсутствующей — вернём вместе с экраном. Идентификатор
   * и подпись оставлены: возвращать их придётся в этот же список.
   */
  {
    id: "delete",
    labelKey: "menuAction.delete",
    requires: "delete",
    separated: true,
    danger: true,
    icon: IconTrash,
  },
];

/** Создавать что-то внутри можно только у того, что раскрывается. */
const ONLY_GROUPS = new Set<MenuActionId>([
  "create-table",
  "link-table",
  "create-folder",
  "create-link",
  "create-files",
  "create-microfrontend",
]);

/**
 * Действия, у которых пока нет своего экрана. Они остаются в типах
 * и в переводах, но в меню не показываются.
 */
const UNIMPLEMENTED = new Set<MenuActionId>(["settings"]);

export function actionsFor(node: MenuNode, isAdmin: boolean): MenuAction[] {
  return ALL.filter((action) => {
    if (UNIMPLEMENTED.has(action.id)) return false;
    /*
     * Суперадмину права на пункт не проверяются — так же, как права
     * на таблицу и глобальные права роли (features/auth/model/permissions).
     * Строка в menu_permission у него может быть какой угодно: сервер
     * его всё равно не остановит, а меню без единого действия выглядит
     * поломкой.
     *
     * Из-за этого у микрофронтендов всплывашка бывала пустой: старая
     * админка спрашивала там `menu_settings || DEFAULT ADMIN`
     * (MenuButtons.jsx:430), а мы — только `update`, который у этих
     * пунктов часто снят.
     */
    if (!isAdmin && !node.can[action.requires]) return false;
    if (ONLY_GROUPS.has(action.id) && node.kind !== "group") return false;
    // Шаблон делает только администратор — так было и раньше.
    if (action.id === "make-template" && !isAdmin) return false;
    /*
     * Шаблон делают из папки и из таблицы (MenuButtons.jsx:320 —
     * в старом меню TABLE он тоже был). Список таблиц шаблона задают
     * в самой форме, а `menu_id` решает только, какое дерево пунктов
     * уедет вместе с ними (шлюз, template.go:201 — GetMenuTree);
     * у таблицы это дерево из одного пункта, и оно осмысленно.
     *
     * У ссылки, микрофронтенда и папки хранилища таблиц нет вовсе —
     * шаблон из них был бы пустым.
     */
    if (
      action.id === "make-template" &&
      node.kind !== "group" &&
      !showsTable(node)
    ) {
      return false;
    }
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
