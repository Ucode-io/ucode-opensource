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
 * add_filter, automation) в ответе есть, уезжают обратно как пришли,
 * но экрана под них нет.
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

/**
 * Остальные права на экран. Показаны отдельно, а не в матрице: колонок
 * и так одиннадцать, а прав — двадцать пять. Часть из них фронт читает
 * наравне с матрицей (`group`, `tab_group`, `field_filter`,
 * `search_button`), часть не читает вовсе — они здесь только затем,
 * чтобы не пропасть при сохранении роли.
 */
export const OTHER_SCREEN_RIGHTS = [
  "automation",
  "share_modal",
  "pdf_action",
  "language_btn",
  "add_filter",
  "field_filter",
  "group",
  "tab_group",
  "search_button",
] as const;
export type OtherScreenRight = (typeof OTHER_SCREEN_RIGHTS)[number];

/**
 * Глобальные права роли — не про таблицы, а про кнопки приложения:
 * настройки, проекты, окружения, ключи, биллинг.
 *
 * Значения здесь БУЛЕВЫ, в отличие от прав на таблицы: у них своя
 * таблица в базе (`global_permission`) и своя форма. Список взят
 * из живого ответа, а не из старого экрана: там их рисуют семнадцать,
 * и ровно столько же приходит.
 */
export const GLOBAL_RIGHTS = [
  "menu_button",
  "menu_drag",
  "menu_setting_button",
  "settings_button",
  "profile_settings_button",
  "project_settings_button",
  "project_button",
  "projects_button",
  "environments_button",
  "api_keys_button",
  "redirects_button",
  "version_button",
  "sms_button",
  "billing",
  "gitbook_button",
  "chatwoot_button",
  "gpt_button",
] as const;
export type GlobalRight = (typeof GLOBAL_RIGHTS)[number];

/** Право на одно поле таблицы: видеть и править. */
export type FieldPermission = {
  id: string;
  label: string;
  view: boolean;
  edit: boolean;
};

export type TablePermission = {
  id: string;
  slug: string;
  label: string;
  /** Права на строки: чтение, создание, правка, удаление. */
  record: Record<RecordRight, boolean>;
  /** Права на экран таблицы, которые читает наш фронт. */
  screen: Record<ScreenRight, boolean>;
  /** Остальные права на экран — см. OTHER_SCREEN_RIGHTS. */
  other: Record<OtherScreenRight, boolean>;
  /**
   * Права на отдельные поля. Приезжают в том же ответе и лежат
   * при таблице; поля без записи в `field_permission` в ответ
   * не попадают вовсе — их у роли просто нет ограничений.
   */
  fields: FieldPermission[];
};

export type RolePermissions = {
  roleId: string;
  roleName: string;
  tables: TablePermission[];
  /** Права на кнопки приложения. Пусто — ответ пришёл без них. */
  global: Record<GlobalRight, boolean>;
  /**
   * Ответ сервера как есть. Нужен для записи: PUT ждёт обратно весь
   * объект, включая то, чего мы не показываем.
   */
  raw: Record<string, unknown>;
};

type FieldPermissionDto = {
  field_id?: string;
  label?: string;
  view_permission?: boolean;
  edit_permission?: boolean;
};

type TableDto = {
  id?: string;
  slug?: string;
  label?: string;
  record_permissions?: Record<string, unknown>;
  custom_permission?: Record<string, unknown>;
  field_permissions?: FieldPermissionDto[];
};

/** Глобальные права булевы: `false` — запрет, остальное — разрешение. */
function globalAllowed(value: unknown): boolean {
  return value !== false && value !== "No";
}

/**
 * Ответ завёрнут ДВАЖДЫ: общий конверт снимает http-клиент, а под ним
 * лежит `{project_id, data: {...}}`, и права — в этом `data`. Имя и id
 * роли лежат прямо там же, а не в отдельном `role`.
 */
type RolePermissionsDto = {
  data?: RolePermissionsBody;
} & RolePermissionsBody;

type RolePermissionsBody = {
  guid?: string;
  name?: string;
  tables?: TableDto[];
  global_permission?: Record<string, unknown>;
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
  const outer = (body ?? {}) as RolePermissionsDto;
  // Права лежат во вложенном `data`; старый вид ответа — прямо в корне.
  const dto: RolePermissionsBody = outer.data ?? outer;

  return {
    roleId: dto.guid ?? "",
    roleName: dto.name?.trim() ?? "",
    global: Object.fromEntries(
      GLOBAL_RIGHTS.map((right) => [right, globalAllowed(dto.global_permission?.[right])]),
    ) as Record<GlobalRight, boolean>,
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
        other: Object.fromEntries(
          OTHER_SCREEN_RIGHTS.map((right) => [right, allowed(table.custom_permission?.[right])]),
        ) as Record<OtherScreenRight, boolean>,
        fields: (table.field_permissions ?? [])
          .filter((field) => field.field_id)
          .map((field) => ({
            id: field.field_id ?? "",
            // «IT'S RELATION» — то, что бэкенд пишет полям-связям сам;
            // человеку это ничего не говорит, но заменить нечем: имени
            // поля в этом ответе больше нет.
            label: field.label?.trim() || field.field_id || "",
            view: field.view_permission !== false,
            edit: field.edit_permission !== false,
          })),
      }))
      .sort((a, b) => a.label.localeCompare(b.label)),
    // В `raw` — именно внутренний объект: его и ждёт PUT в поле `data`.
    raw: dto as Record<string, unknown>,
  };
}

/**
 * Право переключено → новый объект прав, готовый к отправке.
 *
 * Правится и `raw`, и разобранный список: первое уезжает на сервер,
 * второе рисуется на экране. Собирать `raw` заново из разобранного
 * нельзя — в нём есть то, чего мы не показываем: права на поля,
 * автофильтры, права на view и действия.
 */
export function toggleRight(
  permissions: RolePermissions,
  tableSlug: string,
  right: RecordRight | ScreenRight | OtherScreenRight,
  value: boolean,
): RolePermissions {
  return setRights(permissions, (table) => (table.slug === tableSlug ? { [right]: value } : null));
}

/**
 * То же право сразу у ВСЕХ показанных таблиц.
 *
 * Без этого право раздают по одной галочке на таблицу, а таблиц
 * в живом проекте полторы сотни. Отмечаются именно показанные: если
 * человек сузил список поиском, «всем» означает «этим».
 */
export function toggleColumn(
  permissions: RolePermissions,
  slugs: readonly string[],
  right: RecordRight | ScreenRight | OtherScreenRight,
  value: boolean,
): RolePermissions {
  const targets = new Set(slugs);
  return setRights(permissions, (table) => (targets.has(table.slug) ? { [right]: value } : null));
}

/** Глобальное право: своя таблица в базе и булевы значения. */
export function toggleGlobalRight(
  permissions: RolePermissions,
  right: GlobalRight,
  value: boolean,
): RolePermissions {
  const global = (permissions.raw["global_permission"] ?? {}) as Record<string, unknown>;

  return {
    ...permissions,
    global: { ...permissions.global, [right]: value },
    raw: { ...permissions.raw, global_permission: { ...global, [right]: value } },
  };
}

/**
 * Общий двигатель правок: обходит таблицы один раз и для каждой
 * спрашивает, что в ней поменять. Одна реализация на «переключить
 * у одной» и «переключить у всех» — иначе два места, где легко
 * разойтись в том, куда пишется право.
 */
function setRights(
  permissions: RolePermissions,
  patchOf: (table: TablePermission) => Record<string, boolean> | null,
): RolePermissions {
  const rawTables = Array.isArray(permissions.raw["tables"])
    ? (permissions.raw["tables"] as TableDto[])
    : [];

  const patches = new Map<string, Record<string, boolean>>();

  const tables = permissions.tables.map((table) => {
    const patch = patchOf(table);
    if (!patch) return table;

    patches.set(table.slug, patch);
    return {
      ...table,
      record: { ...table.record, ...pick(patch, RECORD_RIGHTS) },
      screen: { ...table.screen, ...pick(patch, SCREEN_RIGHTS) },
      other: { ...table.other, ...pick(patch, OTHER_SCREEN_RIGHTS) },
    };
  });

  return {
    ...permissions,
    tables,
    raw: {
      ...permissions.raw,
      tables: rawTables.map((table) => {
        const patch = table.slug ? patches.get(table.slug) : undefined;
        if (!patch) return table;

        const record = { ...table.record_permissions };
        const custom = { ...table.custom_permission };

        for (const [right, value] of Object.entries(patch)) {
          const bag = (RECORD_RIGHTS as readonly string[]).includes(right) ? record : custom;
          bag[right] = toYesNo(value);
        }

        return { ...table, record_permissions: record, custom_permission: custom };
      }),
    },
  };
}

/** Из правки — только те ключи, что относятся к этой группе прав. */
function pick<T extends string>(
  patch: Record<string, boolean>,
  rights: readonly T[],
): Partial<Record<T, boolean>> {
  const out: Partial<Record<T, boolean>> = {};
  for (const right of rights) {
    if (right in patch) out[right] = patch[right];
  }
  return out;
}

/**
 * Право на поле: видеть или править.
 *
 * Правится и разобранный список, и `raw`: PUT перезаписывает права роли
 * целиком, и поле, которого не оказалось в теле, потеряет своё
 * ограничение.
 */
export function toggleFieldRight(
  permissions: RolePermissions,
  tableSlug: string,
  fieldId: string,
  right: "view" | "edit",
  value: boolean,
): RolePermissions {
  const key = right === "view" ? "view_permission" : "edit_permission";

  const rawTables = Array.isArray(permissions.raw["tables"])
    ? (permissions.raw["tables"] as TableDto[])
    : [];

  return {
    ...permissions,
    tables: permissions.tables.map((table) =>
      table.slug === tableSlug
        ? {
            ...table,
            fields: table.fields.map((field) =>
              field.id === fieldId ? { ...field, [right]: value } : field,
            ),
          }
        : table,
    ),
    raw: {
      ...permissions.raw,
      tables: rawTables.map((table) =>
        table.slug === tableSlug
          ? {
              ...table,
              field_permissions: (table.field_permissions ?? []).map((field) =>
                field.field_id === fieldId ? { ...field, [key]: value } : field,
              ),
            }
          : table,
      ),
    },
  };
}
