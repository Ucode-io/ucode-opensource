import { useEffect, useMemo, useState } from "react";
import {
  IconChevronDown,
  IconChevronLeft,
  IconChevronRight,
  IconColumns,
  IconDotsVertical,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Modal } from "@/shared/ui/modal";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { Select } from "@/shared/ui/input";
import {
  useClientTypes,
  useCreateRole,
  useDeleteRole,
  useMenuPermissions,
  useRolePermissions,
  useRoles,
  useUpdateMenuPermissions,
  useUpdateRolePermissions,
  type MenuPermission,
  type Role,
} from "../api/roles";
import {
  GLOBAL_RIGHTS,
  OTHER_SCREEN_RIGHTS,
  RECORD_RIGHTS,
  SCREEN_RIGHTS,
  toggleColumn,
  toggleFieldRight,
  toggleGlobalRight,
  toggleRight,
  type RolePermissions,
  type TablePermission,
} from "../model/permissions";

/**
 * Роли и их права.
 *
 * Роль слева, её таблицы справа — по строке на таблицу и по флажку
 * на право. Матрица, а не экран настройки у каждой таблицы: права
 * раздают, сравнивая роли между собой, и ради этого их нужно видеть
 * рядом.
 *
 * В матрице — десять прав: четыре на строки и шесть на экран, которые
 * наш фронт действительно читает. Остальные девять прав экрана есть
 * в ответе и сохраняются, но столбцами не рисуются: их правят из меню
 * строки. Двадцать пять колонок на полторы сотни таблиц — это не
 * матрица, а обои.
 *
 * Глобальные права — отдельной вкладкой: они не про таблицы, а про
 * кнопки приложения, и колонки матрицы им не подходят.
 *
 * Правки копятся и уходят по кнопке, а не по флажку: PUT перезаписывает
 * права роли ЦЕЛИКОМ, и запрос на каждый щелчок означал бы гонку, где
 * побеждает последний ответ, а не последний щелчок.
 */
export function RoleSettings() {
  const { t } = useTranslation();
  const { roles, isLoading } = useRoles();

  const [roleId, setRoleId] = useState("");
  const active = roleId || roles[0]?.id || "";

  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Role | null>(null);
  const remove = useDeleteRole();

  const { permissions, isLoading: loadingPermissions } = useRolePermissions(active);
  const update = useUpdateRolePermissions(active);

  /** Черновик: правки видно сразу, а уезжают они по кнопке. */
  const [draft, setDraft] = useState<RolePermissions | null>(null);
  useEffect(() => setDraft(permissions ?? null), [permissions]);

  const [tab, setTab] = useState<"tables" | "global" | "menu">("tables");
  /** Таблиц в живом проекте полторы сотни — без поиска это стена. */
  const [query, setQuery] = useState("");

  const dirty = Boolean(draft && permissions && draft.raw !== permissions.raw);

  const shown = useMemo(() => matching(draft?.tables ?? [], query), [draft?.tables, query]);

  if (isLoading) return <p className="p-4 text-sm text-fg-muted">{t("common.loading")}</p>;
  if (!roles.length) return <p className="p-4 text-sm text-fg-muted">{t("roles.empty")}</p>;

  return (
    <div className="flex min-h-0 min-w-0 flex-1">
      {/* Роли — вторым столбцом слева: их единицы, и переключаются они
          чаще, чем что-либо ещё на этом экране. */}
      <nav className="flex w-52 shrink-0 flex-col border-r border-border">
        {/* «Новая роль» вверху и всегда на виду: ролей в живом проекте
            два десятка, и внизу списка кнопка оказывалась за краем. */}
        <button
          type="button"
          onClick={() => setCreating(true)}
          className="flex h-9 shrink-0 items-center gap-1.5 border-b border-border px-3 text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconPlus} size={14} />
          {t("roles.create")}
        </button>

        <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto p-2">
        {roles.map((role) => (
          <div
            key={role.id}
            className={`group/role flex h-8 shrink-0 items-center rounded-md pr-1 transition-colors ${
              role.id === active ? "bg-surface-active" : "hover:bg-surface-hover"
            }`}
          >
            <button
              type="button"
              onClick={() => setRoleId(role.id)}
              className={`flex h-full min-w-0 flex-1 items-center px-2 text-left text-sm ${
                role.id === active ? "text-fg" : "text-fg-muted group-hover/role:text-fg"
              }`}
            >
              <span className="truncate">{role.name}</span>
            </button>

            {/* Системную роль бэкенд удалить не даст — не предлагаем. */}
            {!role.isSystem && (
              <button
                type="button"
                onClick={() => setDeleting(role)}
                aria-label={t("roles.delete")}
                title={t("roles.delete")}
                className="hidden size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors group-hover/role:grid hover:bg-danger-subtle hover:text-danger"
              >
                <Icon as={IconTrash} size={14} />
              </button>
            )}
          </div>
        ))}

        </div>
      </nav>

      {creating && (
        <RoleCreateDialog
          onClose={() => setCreating(false)}
          onCreated={() => setCreating(false)}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title={t("roles.deleteTitle", { name: deleting.name })}
          description={t("roles.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() =>
            remove.mutate(deleting.id, {
              onSuccess: () => {
                // Открытая роль исчезла — возвращаемся к первой.
                if (deleting.id === active) setRoleId("");
                setDeleting(null);
              },
            })
          }
        />
      )}

      {/*
        min-w-0 обязателен: без него у flex-элемента ширина не может
        стать меньше содержимого, и широкая матрица распирала бы всё
        окно наружу вместо того, чтобы прокручиваться внутри себя.
      */}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        {loadingPermissions || !draft ? (
          <p className="p-4 text-sm text-fg-muted">{t("common.loading")}</p>
        ) : (
          <>
            <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
              <Tab active={tab === "tables"} onClick={() => setTab("tables")}>
                {t("roles.tables")}
              </Tab>
              <Tab active={tab === "menu"} onClick={() => setTab("menu")}>
                {t("roles.menu")}
              </Tab>
              <Tab active={tab === "global"} onClick={() => setTab("global")}>
                {t("roles.global")}
              </Tab>

              {tab === "tables" && (
                <div className="ml-auto w-56">
                  <Input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    placeholder={t("roles.searchTable")}
                    aria-label={t("roles.searchTable")}
                  />
                </div>
              )}
            </div>

            {tab === "tables" && <TableMatrix tables={shown} draft={draft} onChange={setDraft} />}
            {tab === "global" && <GlobalRights draft={draft} onChange={setDraft} />}
            {/* Права на меню — своя пара ручек и своё сохранение:
                дерево грузится по уровню, и класть его в общий черновик
                прав на таблицы нечем. */}
            {tab === "menu" && <MenuRights roleId={active} />}

            {tab !== "menu" && (
              <div className="flex h-12 shrink-0 items-center justify-between gap-3 border-t border-border px-3">
                <p className="text-xs text-fg-subtle">{t("roles.hint")}</p>

                <Button
                  size="sm"
                  disabled={!dirty || update.isPending}
                  onClick={() => update.mutate(draft)}
                >
                  {update.isPending ? t("common.saving") : t("action.save")}
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

/**
 * Матрица «таблица × право».
 *
 * Флажок в шапке колонки раздаёт право всем ПОКАЗАННЫМ таблицам: сузил
 * поиском — значит «этим». Он же и снимает: у половины ролей право
 * нужно ровно двум таблицам из ста пятидесяти.
 */
function TableMatrix({
  tables,
  draft,
  onChange,
}: {
  tables: TablePermission[];
  draft: RolePermissions;
  onChange: (next: RolePermissions) => void;
}) {
  const { t } = useTranslation();
  const rights = [...RECORD_RIGHTS, ...SCREEN_RIGHTS] as const;
  const slugs = tables.map((table) => table.slug);

  const valueOf = (table: TablePermission, right: (typeof rights)[number]) =>
    (RECORD_RIGHTS as readonly string[]).includes(right)
      ? table.record[right as (typeof RECORD_RIGHTS)[number]]
      : table.screen[right as (typeof SCREEN_RIGHTS)[number]];

  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <table className="w-full min-w-max border-separate border-spacing-0 text-sm">
        <thead className="sticky top-0 z-10 bg-surface">
          <tr>
            {/* Имя таблицы липкое: без него прокрутка вправо оставляет
                ряд флажков без ответа на вопрос «чьи это права». */}
            <th className="sticky left-0 z-20 h-14 border-b border-border bg-surface px-3 text-left align-middle font-normal text-fg-muted">
              {t("roles.table")}
            </th>

            {rights.map((right, index) => {
              const all = tables.length > 0 && tables.every((table) => valueOf(table, right));

              return (
                <th
                  key={right}
                  /* Тонкая граница отделяет права на строки от прав
                     на экран: колонок десять, и без неё они читаются
                     одним рядом. */
                  /* Шапка выше строк: в ней две вещи — подпись и флажок
                     «всем», — и на высоте строки флажок ложился прямо
                     на нижнюю границу. */
                  className={`h-14 w-24 border-b border-border px-2 py-2 text-center text-xs font-normal text-fg-muted ${
                    index === RECORD_RIGHTS.length ? "border-l" : ""
                  }`}
                >
                  <span className="flex h-full flex-col items-center justify-between gap-1.5">
                    {t(`roles.right.${right}` as TranslationKey)}
                    <Checkbox
                      checked={all}
                      indeterminate={!all && tables.some((table) => valueOf(table, right))}
                      aria-label={t("roles.allTables")}
                      onChange={(event) =>
                        onChange(toggleColumn(draft, slugs, right, event.target.checked))
                      }
                    />
                  </span>
                </th>
              );
            })}

            <th className="h-14 w-10 border-b border-border" />
          </tr>
        </thead>

        <tbody>
          {tables.map((table) => (
            <tr key={table.slug} className="hover:bg-surface-hover">
              <td className="sticky left-0 z-10 h-9 max-w-64 truncate border-b border-r border-border bg-surface px-3">
                <span className="text-fg">{table.label}</span>
                <span className="ml-2 font-mono text-2xs text-fg-subtle">{table.slug}</span>
              </td>

              {rights.map((right, index) => (
                <td
                  key={right}
                  className={`h-9 border-b border-border text-center ${
                    index === RECORD_RIGHTS.length ? "border-l" : ""
                  }`}
                >
                  <Checkbox
                    checked={valueOf(table, right)}
                    aria-label={`${table.label}: ${t(`roles.right.${right}` as TranslationKey)}`}
                    onChange={(event) =>
                      onChange(toggleRight(draft, table.slug, right, event.target.checked))
                    }
                  />
                </td>
              ))}

              <td className="h-9 border-b border-border text-center">
                <OtherRights table={table} draft={draft} onChange={onChange} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Меню строки: остальные права экрана и права на поля.
 *
 * Двумя страницами в одном всплывающем окне, а не двумя кнопками
 * в строке: у таблицы в матрице и так одиннадцать флажков, и ещё две
 * иконки в каждой строке — это рябь.
 *
 * Права на поля приезжают тем же ответом и лежат при таблице. Поля,
 * у которых записи о правах нет, в ответ не попадают вовсе — у роли
 * на них просто нет ограничений, и показывать их не из чего.
 */
function OtherRights({
  table,
  draft,
  onChange,
}: {
  table: TablePermission;
  draft: RolePermissions;
  onChange: (next: RolePermissions) => void;
}) {
  const { t } = useTranslation();

  return (
    <Popover
      align="end"
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={open}
          aria-label={t("roles.otherRights")}
          title={t("roles.otherRights")}
          className="grid size-6 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
        >
          <Icon as={IconDotsVertical} size={14} />
        </button>
      )}
    >
      {() => <RowMenu table={table} draft={draft} onChange={onChange} />}
    </Popover>
  );
}

function RowMenu({
  table,
  draft,
  onChange,
}: {
  table: TablePermission;
  draft: RolePermissions;
  onChange: (next: RolePermissions) => void;
}) {
  const { t } = useTranslation();
  const [page, setPage] = useState<"rights" | "fields">("rights");

  if (page === "fields") {
    return (
      <div className="w-72">
        <button
          type="button"
          onClick={() => setPage("rights")}
          className="flex h-8 w-full items-center gap-1.5 rounded-md px-2 text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconChevronLeft} size={14} />
          {t("action.back")}
        </button>

        <div className="grid grid-cols-[1fr_auto_auto] items-center gap-x-2 px-2 py-1 text-2xs text-fg-subtle">
          <span>{t("roles.field")}</span>
          <span>{t("roles.fieldView")}</span>
          <span>{t("roles.fieldEdit")}</span>
        </div>

        <div className="max-h-72 overflow-y-auto">
          {table.fields.map((field) => (
            <div
              key={field.id}
              className="grid grid-cols-[1fr_auto_auto] items-center gap-x-2 rounded-md px-2 py-1 text-sm hover:bg-surface-hover"
            >
              <span className="truncate">{field.label}</span>

              <Checkbox
                checked={field.view}
                aria-label={`${field.label}: ${t("roles.fieldView")}`}
                onChange={(event) =>
                  onChange(
                    toggleFieldRight(draft, table.slug, field.id, "view", event.target.checked),
                  )
                }
              />
              <Checkbox
                checked={field.edit}
                aria-label={`${field.label}: ${t("roles.fieldEdit")}`}
                onChange={(event) =>
                  onChange(
                    toggleFieldRight(draft, table.slug, field.id, "edit", event.target.checked),
                  )
                }
              />
            </div>
          ))}

          {!table.fields.length && (
            <p className="px-2 py-2 text-xs text-fg-subtle">{t("roles.noFieldRights")}</p>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="w-56">
      <p className="px-2 py-1 text-2xs text-fg-subtle">{t("roles.otherRightsHint")}</p>

      {OTHER_SCREEN_RIGHTS.map((right) => (
        <label
          key={right}
          className="flex h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-sm transition-colors hover:bg-surface-hover"
        >
          <Checkbox
            checked={table.other[right]}
            onChange={(event) =>
              onChange(toggleRight(draft, table.slug, right, event.target.checked))
            }
          />
          <span className="truncate">{t(`roles.right.${right}` as TranslationKey)}</span>
        </label>
      ))}

      <PopoverSeparator />

      <PopoverItem
        icon={<Icon as={IconColumns} size={16} className="shrink-0 text-fg-muted" />}
        onClick={() => setPage("fields")}
      >
        {t("roles.fields", { count: table.fields.length })}
      </PopoverItem>
    </div>
  );
}

/** Глобальные права: кнопки приложения, а не таблицы. */
function GlobalRights({
  draft,
  onChange,
}: {
  draft: RolePermissions;
  onChange: (next: RolePermissions) => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="min-h-0 flex-1 overflow-y-auto p-4">
      <p className="mb-3 text-xs text-fg-subtle">{t("roles.globalHint")}</p>

      <div className="grid max-w-3xl grid-cols-2 gap-1">
        {GLOBAL_RIGHTS.map((right) => (
          <label
            key={right}
            className="flex h-9 cursor-pointer items-center gap-2 rounded-md px-2 text-sm transition-colors hover:bg-surface-hover"
          >
            <Checkbox
              checked={draft.global[right]}
              onChange={(event) => onChange(toggleGlobalRight(draft, right, event.target.checked))}
            />
            <span className="truncate">{t(`roles.global.${right}` as TranslationKey)}</span>
          </label>
        ))}
      </div>
    </div>
  );
}

function Tab({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`h-7 shrink-0 rounded-md px-2 text-sm transition-colors ${
        active
          ? "bg-accent-subtle text-accent-text"
          : "text-fg-muted hover:bg-surface-hover hover:text-fg"
      }`}
    >
      {children}
    </button>
  );
}

/** Таблицы, подходящие под поиск: по имени и по слагу. */
function matching(tables: TablePermission[], query: string): TablePermission[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return tables;

  return tables.filter(
    (table) =>
      table.label.toLowerCase().includes(needle) || table.slug.toLowerCase().includes(needle),
  );
}

/**
 * Права роли на пункты меню.
 *
 * Дерево грузится по уровню — так его отдаёт и бэкенд, и сам сайдбар:
 * запроса «всё дерево разом» нет. Раскрытая папка тянет свой уровень
 * и хранит правки отдельно от прав на таблицы: у них своя ручка
 * и своё сохранение.
 *
 * Уезжают только изменённые пункты: того, что человек не раскрывал,
 * у нас на руках нет вовсе, и слать за них «как было» значило бы
 * выдумывать.
 */
function MenuRights({ roleId }: { roleId: string }) {
  const { t } = useTranslation();
  const update = useUpdateMenuPermissions(roleId);

  /** Правки по id пункта: они и уезжают на сервер. */
  const [changed, setChanged] = useState<Record<string, MenuPermission>>({});

  const apply = (menu: MenuPermission) =>
    setChanged((current) => ({ ...current, [menu.id]: menu }));

  return (
    <>
      <div className="min-h-0 flex-1 overflow-auto">
        {/* Подписи прав — один раз в шапке, а не у каждого флажка:
            в дереве из полусотни пунктов это двести повторов одного
            и того же слова. */}
        <div className="sticky top-0 z-10 flex h-9 items-center gap-2 border-b border-border bg-surface pr-3 pl-2 text-xs text-fg-muted">
          <span className="min-w-0 flex-1">{t("roles.menuItem")}</span>
          {MENU_RIGHTS.map((right) => (
            <span key={right} className="w-20 shrink-0 text-center">
              {t(`roles.right.${right}` as TranslationKey)}
            </span>
          ))}
        </div>

        <div className="p-2">
          <MenuLevel roleId={roleId} parentId={ROOT_MENU} changed={changed} onChange={apply} />
        </div>
      </div>

      <div className="flex h-12 shrink-0 items-center justify-between gap-3 border-t border-border px-3">
        <p className="text-xs text-fg-subtle">{t("roles.menuHint")}</p>

        <Button
          size="sm"
          disabled={!Object.keys(changed).length || update.isPending}
          onClick={() =>
            update.mutate(Object.values(changed), { onSuccess: () => setChanged({}) })
          }
        >
          {update.isPending ? t("common.saving") : t("action.save")}
        </Button>
      </div>
    </>
  );
}

/**
 * Один уровень дерева меню. Папка раскрывается — рисуется следующий,
 * своим запросом: бэкенд отдаёт меню по одному родителю.
 */
function MenuLevel({
  roleId,
  parentId,
  changed,
  onChange,
  depth = 0,
}: {
  roleId: string;
  parentId: string;
  changed: Record<string, MenuPermission>;
  onChange: (menu: MenuPermission) => void;
  depth?: number;
}) {
  const { t } = useTranslation();
  const { menus, isLoading } = useMenuPermissions(roleId, parentId);
  const [open, setOpen] = useState<Set<string>>(() => new Set());

  if (isLoading) return <p className="px-2 py-1 text-xs text-fg-subtle">{t("common.loading")}</p>;

  return (
    <div className="flex flex-col">
      {menus.map((menu) => {
        const value = changed[menu.id] ?? menu;
        const expanded = open.has(menu.id);

        return (
          <div key={menu.id} className="flex flex-col">
            <div
              className="flex h-9 items-center gap-2 rounded-md pr-2 hover:bg-surface-hover"
              style={{ paddingLeft: depth * 16 }}
            >
              <button
                type="button"
                disabled={menu.type !== "FOLDER"}
                onClick={() =>
                  setOpen((current) => {
                    const next = new Set(current);
                    if (!next.delete(menu.id)) next.add(menu.id);
                    return next;
                  })
                }
                className="grid size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:text-fg disabled:opacity-0"
              >
                <Icon as={expanded ? IconChevronDown : IconChevronRight} size={14} />
              </button>

              <span className="min-w-0 flex-1 truncate text-sm">{menu.label}</span>

              {MENU_RIGHTS.map((right) => (
                <span key={right} className="flex w-20 shrink-0 justify-center">
                  <Checkbox
                    checked={value[right]}
                    aria-label={`${menu.label}: ${t(`roles.right.${right}` as TranslationKey)}`}
                    onChange={(event) => onChange({ ...value, [right]: event.target.checked })}
                  />
                </span>
              ))}
            </div>

            {expanded && (
              <MenuLevel
                roleId={roleId}
                parentId={menu.id}
                changed={changed}
                onChange={onChange}
                depth={depth + 1}
              />
            )}
          </div>
        );
      })}

      {!menus.length && <p className="px-2 py-1 text-xs text-fg-subtle">{t("roles.menuEmpty")}</p>}
    </div>
  );
}

/** Новая роль: имя и тип клиента — без него войти под ролью нельзя. */
function RoleCreateDialog({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const { t } = useTranslation();
  const { clientTypes } = useClientTypes();
  const create = useCreateRole();

  const [name, setName] = useState("");
  const [clientTypeId, setClientTypeId] = useState("");

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate({ name, clientTypeId }, { onSuccess: onCreated });
        }}
      >
        <h2 className="text-base font-semibold">{t("roles.create")}</h2>

        <Field label={t("roles.name")}>
          <Input autoFocus required value={name} onChange={(event) => setName(event.target.value)} />
        </Field>

        <Field label={t("roles.clientType")} hint={t("roles.clientTypeHint")}>
          <Select
            required
            value={clientTypeId}
            onChange={(event) => setClientTypeId(event.target.value)}
          >
            <option value="">—</option>
            {clientTypes.map((type) => (
              <option key={type.id} value={type.id}>
                {type.name}
              </option>
            ))}
          </Select>
        </Field>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!name.trim() || !clientTypeId || create.isPending}>
            {create.isPending ? t("common.saving") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/** Права на пункт меню — те же четыре, что и на строки таблицы. */
const MENU_RIGHTS = ["read", "write", "update", "delete"] as const;

/**
 * Корень дерева меню. Тот же, что у сайдбара: uuid захардкожен в SQL
 * бэкенда и одинаков для всех проектов.
 */
const ROOT_MENU = "c57eedc3-a954-4262-a0af-376c65b5a284";
