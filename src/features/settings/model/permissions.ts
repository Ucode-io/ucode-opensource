/**
 * Права роли на таблицы.
 *
 * Значения — СТРОКИ `'Yes' | 'No'`, а не булевы: так они лежат в базе
 * (record_permission, VARCHAR ... CHECK IN ('Yes','No')) и так же уходят
 * в ответ. Строка `"No"` в булевом контексте истинна — отсюда перевод
 * в обе стороны здесь, один раз, а не по месту использования.
 *
 * Права двух видов, и это не наша выдумка, а две колонки в ответе:
 *
 *   record_permission   что можно делать со СТРОКАМИ: читать, заводить,
 *                       править, удалять;
 *   custom_permission   что видно на экране таблицы: настройки, колонки,
 *                       закрепление, excel, создание view.
 *
 * Всё, чего этот экран не показывает — права на поля, автофильтры,
 * глобальные права, — хранится в `raw` и уезжает обратно нетронутым:
 * PUT перезаписывает права роли целиком.
 */

/** Права на строки. Ключи — колонки record_permission. */
export const RECORD_RIGHTS = ["read", "write", "update", "delete"] as const;
export type RecordRight = (typeof RECORD_RIGHTS)[number];

/**
 * Права на экран таблицы. Показываются те, что у нас действительно
 * что-то решают: их читает features/auth/model/permissions и по ним
 * прячутся кнопки. Остальные (share_modal, pdf_action, language_btn,
 * add_filter, field_filter, search_button, automation) в ответе есть,
 * уезжают обратно как пришли, но экрана под них нет.
 */
export const SCREEN_RIGHTS = [
  "settings",
  "columns",
  "fix_column",
  "excel_menu",
  "view_create",
  "add_field",
] as const;
export type ScreenRight = (typeof SCREEN_RIGHTS)[number];

export type TablePermission = {
  id: string;
  slug: string;
  label: string;
  /** Права на строки: чтение, создание, правка, удаление. */
  record: Record<RecordRight, boolean>;
  /** Права на экран таблицы. */
  screen: Record<ScreenRight, boolean>;
};

export type RolePermissions = {
  roleId: string;
  roleName: string;
  tables: TablePermission[];
  /**
   * Ответ сервера как есть. Нужен для записи: PUT ждёт обратно весь
   * объект, включая то, чего мы не показываем.
   */
  raw: Record<string, unknown>;
};

type TableDto = {
  id?: string;
  slug?: string;
  label?: string;
  record_permissions?: Record<string, unknown>;
  custom_permission?: Record<string, unknown>;
};

type RolePermissionsDto = {
  role?: { guid?: string; name?: string };
  tables?: TableDto[];
};

/** `'No'` — запрет, всё остальное — разрешение. */
export function allowed(value: unknown): boolean {
  return value !== "No" && value !== false;
}

/** Обратно в то, что понимает база. */
export function toYesNo(value: boolean): "Yes" | "No" {
  return value ? "Yes" : "No";
}

export function toRolePermissions(body: unknown): RolePermissions {
  const dto = (body ?? {}) as RolePermissionsDto;

  return {
    roleId: dto.role?.guid ?? "",
    roleName: dto.role?.name?.trim() ?? "",
    tables: (dto.tables ?? [])
      .filter((table) => table.slug)
      .map((table) => ({
        id: table.id ?? "",
        slug: table.slug ?? "",
        // Подпись бывает пустой у таблиц, заведённых из API: слаг тогда
        // и есть имя — показывать пустую строку в списке нечестно.
        label: table.label?.trim() || table.slug || "",
        record: Object.fromEntries(
          RECORD_RIGHTS.map((right) => [right, allowed(table.record_permissions?.[right])]),
        ) as Record<RecordRight, boolean>,
        screen: Object.fromEntries(
          SCREEN_RIGHTS.map((right) => [right, allowed(table.custom_permission?.[right])]),
        ) as Record<ScreenRight, boolean>,
      }))
      .sort((a, b) => a.label.localeCompare(b.label)),
    raw: (body ?? {}) as Record<string, unknown>,
  };
}

/**
 * Право переключено → новый объект прав, готовый к отправке.
 *
 * Правится и `raw`, и разобранный список: первое уезжает на сервер,
 * второе рисуется на экране. Собирать `raw` заново из разобранного
 * нельзя — в нём есть то, чего мы не показываем.
 */
export function toggleRight(
  permissions: RolePermissions,
  tableSlug: string,
  right: RecordRight | ScreenRight,
  value: boolean,
): RolePermissions {
  const isRecord = (RECORD_RIGHTS as readonly string[]).includes(right);
  const bag = isRecord ? "record_permissions" : "custom_permission";

  const rawTables = Array.isArray(permissions.raw["tables"])
    ? (permissions.raw["tables"] as TableDto[])
    : [];

  return {
    ...permissions,
    tables: permissions.tables.map((table) =>
      table.slug === tableSlug
        ? isRecord
          ? { ...table, record: { ...table.record, [right]: value } }
          : { ...table, screen: { ...table.screen, [right]: value } }
        : table,
    ),
    raw: {
      ...permissions.raw,
      tables: rawTables.map((table) =>
        table.slug === tableSlug
          ? { ...table, [bag]: { ...table[bag], [right]: toYesNo(value) } }
          : table,
      ),
    },
  };
}
