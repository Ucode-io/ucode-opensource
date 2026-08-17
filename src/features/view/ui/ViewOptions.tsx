import { useState, type ReactNode } from "react";
import {
  IconChevronLeft,
  IconChevronRight,
  IconDotsVertical,
  IconEye,
  IconEyeOff,
  IconFileExport,
  IconFileImport,
  IconFilter,
  IconFilterCog,
  IconGripVertical,
  IconLayoutList,
  IconLink,
  IconLoader2,
  IconPin,
  IconPinnedOff,
  IconSearch,
  IconTable,
  IconTag,
  IconTrash,
  IconX,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Permission } from "@/features/auth";
import {
  FilterBar,
  activeFilterCount,
  fieldIcon,
  filterKind,
  type Filters,
} from "@/features/item";
import { localized, type Field } from "@/features/table";
import type { DataLanguage } from "@/features/workspace";
import type { TranslationKey } from "@/shared/lib/i18n";
import { toast } from "@/shared/lib/toast";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { ToolButton } from "@/shared/ui/tool-button";
import { columnKey, moveBefore } from "../model/columns";
import { IMPLEMENTED_VIEW_TYPES, VIEW_TYPES, viewName, type View } from "../model/types";
import { viewIcon } from "./view-icon";

/**
 * Настройки открытого view.
 *
 * Устроено страницами, а не набором отдельных всплывашек: список
 * настроек, а внутри каждой — своя страница с возвратом. Второй слой
 * поверх первого закрывал бы то, что настраивают.
 *
 * Чего здесь нет и почему:
 *
 *   Группировка (`group_by_columns`) и группировка вкладками
 *   (`group_fields`) — таблица не умеет ни того, ни другого; настройки
 *   при этом не теряются, они лежат в `raw` и уходят обратно нетронутыми.
 *   Переключатель, который ничего не меняет, хуже отсутствующего.
 *
 *   Настройки самой ТАБЛИЦЫ (в старой админке они звались «General»:
 *   слаг, таблица входа, кэш, мягкое удаление) — это настройки не view,
 *   а таблицы, и живут они в конструкторе. Строка «Тип» здесь называется
 *   типом, а не «Общими», чтобы имя не было занято.
 *
 *   Настройки Timeline и Calendar — этих типов у нас нет вовсе.
 *
 *   «Строк на странице» — бэкенд не обновляет default_limit этой ручкой
 *   вовсе (view.go, Update: колонки в UPDATE просто нет), а размер
 *   страницы и так живёт в подвале таблицы.
 *
 *   Сортировка — она живёт в адресе, а не в настройках view, и её кнопка
 *   стоит в той же панели инструментов через одну. Два входа в одно и то
 *   же состояние — это не удобство, это вопрос «а эти две одинаковые?».
 *
 * Правки уходят по одной и сразу: панель настроек без кнопки «сохранить»
 * — то же, что переключатель в системных настройках.
 */
export type ViewOptionsHandlers = {
  /** Имя на конкретном языке ДАННЫХ. Язык задаёт вызывающая страница. */
  onRename: (name: string, language: string) => void;
  onType: (type: string) => void;
  onColumns: (columnIds: string[]) => void;
  onQuickFilters: (fields: Field[]) => void;
  /** Закреплённые колонки целиком: список ключей, как в columns. */
  onFixedColumns: (columnIds: string[]) => void;
  /** Отбор, с которым таблица открывается. Пустой — отбора нет. */
  onDefaultFilters: (filters: Filters) => void;
  /** Настроить поле: открывает ту же панель, что и меню колонки. */
  onEditField: (field: Field, anchor: DOMRect) => void;
  /** Удалить поле из ТАБЛИЦЫ, а не из view. Спрашивает подтверждение вызывающий. */
  onDeleteField: (field: Field) => void;
  onImport: () => void;
  onExport: () => void;
  /** Нет обработчика — удалять нельзя (последняя вкладка). */
  onDelete?: () => void;
};

export function ViewOptions({
  view,
  fields,
  language,
  languages,
  defaultFilters,
  can,
  exporting,
  busy,
  handlers,
}: {
  view: View;
  /** ВСЕ поля таблицы: скрытых во view здесь ещё нет, а показать их надо. */
  fields: Field[];
  language: string;
  /** Языки ДАННЫХ проекта: имя view задаётся на каждом. */
  languages: DataLanguage[];
  /** Отбор по умолчанию, уже разобранный из настроек view. */
  defaultFilters: Filters;
  /** Права роли на эту таблицу. Что нельзя — того в панели нет. */
  can: Permission;
  exporting: boolean;
  busy: boolean;
  handlers: ViewOptionsHandlers;
}) {
  const { t } = useTranslation();

  /*
   * Три точки, а не шестерёнка: рядом стоят поиск, отбор и сортировка —
   * все три про таблицу, и все три шестерёнке ровня. Точки читаются
   * как «а тут остальное», и это ровно то, что здесь лежит.
   *
   * Панель может оказаться пустой целиком: роль без единого права
   * не должна видеть кнопку, которая открывает пустоту.
   */
  const anything = can.settings || can.columns || can.fixColumn || can.excelMenu;
  if (!anything) return null;

  return (
    <Popover
      align="end"
      trigger={({ open, toggle }) => (
        <ToolButton
          icon={IconDotsVertical}
          label={t("view.options")}
          open={open}
          onClick={toggle}
        />
      )}
    >
      {(close) => (
        <Panel
          view={view}
          fields={fields}
          language={language}
          languages={languages}
          defaultFilters={defaultFilters}
          can={can}
          exporting={exporting}
          busy={busy}
          handlers={handlers}
          close={close}
        />
      )}
    </Popover>
  );
}

/** Открытая страница панели. null — список настроек. */
type PanelPage =
  | "type"
  | "view"
  | "columns"
  | "defaultFilters"
  | "quickFilters"
  | "fixed"
  | "fields"
  | null;

function Panel({
  view,
  fields,
  language,
  languages,
  defaultFilters,
  can,
  exporting,
  busy,
  handlers,
  close,
}: {
  view: View;
  fields: Field[];
  language: string;
  languages: DataLanguage[];
  defaultFilters: Filters;
  can: Permission;
  exporting: boolean;
  busy: boolean;
  handlers: ViewOptionsHandlers;
  close: () => void;
}) {
  const { t } = useTranslation();
  const [page, setPage] = useState<PanelPage>(null);
  /*
   * Поиск по полям — один на все страницы со списками. Своего состояния
   * на страницу не заводим: страницы взаимоисключающие, а сбрасывать его
   * при переходе всё равно надо.
   */
  const [query, setQuery] = useState("");

  const shown = shownFields(view, fields);
  const quick = quickFilterFields(view, fields);
  const fixed = fixedFields(view, shown);
  const open = (next: PanelPage) => {
    setQuery("");
    setPage(next);
  };
  const back = () => open(null);
  const types = switchableTypes(view);
  const typeLabel = t(`view.type.${view.type}` as TranslationKey, { defaultValue: view.type });

  if (page === "type") {
    return (
      <Subpage title={t("view.viewType")} busy={busy} onBack={back}>
        {types.map((type) => (
          <PopoverItem
            key={type}
            active={type === view.type}
            icon={<Icon as={viewIcon(type)} size={16} className="shrink-0" />}
            onClick={() => handlers.onType(type)}
          >
            {t(`view.type.${type}` as TranslationKey, { defaultValue: type })}
          </PopoverItem>
        ))}
      </Subpage>
    );
  }

  if (page === "view") {
    /*
     * Имя на каждом языке ДАННЫХ проекта. Отдельная страница, а не одно
     * поле в шапке: в шапке правится только текущий язык, и админ,
     * работающий в русском интерфейсе, годами не видел, что узбекское
     * имя вкладки пустое.
     */
    return (
      <Subpage title={t("view.viewSettings")} busy={busy} onBack={back} hint={t("view.namesHint")}>
        <div className="flex flex-col gap-1 p-1">
          {languages.map((item) => (
            <label key={item.code} className="flex flex-col gap-0.5">
              <span className="px-0.5 text-2xs text-fg-subtle">{item.nativeName}</span>
              <NameInput
                value={view.names[item.code] ?? ""}
                placeholder={typeLabel}
                onCommit={(name) => handlers.onRename(name, item.code)}
              />
            </label>
          ))}
        </div>
      </Subpage>
    );
  }

  if (page === "columns") {
    /*
     * guid не показывается и не скрывается: это служебный ключ строки,
     * колонкой он не бывает. Исключение симметрично — и из «Показать
     * все», и из списка скрытых, и из показанных. Раньше оно стояло
     * только на «Показать все», и кнопка молча снимала guid у view,
     * который его действительно показывал.
     */
    const hideable = fields.filter((field) => field.slug !== "guid");
    const visible = shown.filter((field) => field.slug !== "guid");
    const shownIds = new Set(visible.map((field) => field.id));
    const hidden = matching(
      hideable.filter((field) => !shownIds.has(field.id)),
      query,
      language,
    );

    return (
      <Subpage title={t("view.columns")} busy={busy} onBack={back} hint={t("view.columnsHint")}>
        <div className="flex gap-1 px-1 pb-1">
          <BulkButton
            label={t("view.showAll")}
            onClick={() => handlers.onColumns(hideable.map(columnKey))}
          />
          {/* Совсем без колонок view оставлять нельзя — экран станет пустым
              без единой подсказки, что делать. Первая остаётся. */}
          <BulkButton
            label={t("view.hideAll")}
            onClick={() => handlers.onColumns(visible.slice(0, 1).map(columnKey))}
          />
        </div>

        <FieldSearch value={query} onChange={setQuery} />

        <List>
          <ColumnOrder
            /* Перетаскивание при поиске отключается вместе с фильтрацией:
               порядок отдаётся списком целиком, а бросок внутри выборки
               из трёх строк переставил бы и остальные тридцать. */
            shown={query ? matching(visible, query, language) : visible}
            language={language}
            draggable={!query}
            onReorder={handlers.onColumns}
            onHide={(field) => handlers.onColumns(toggleColumn(view, field, false))}
          />

          {hidden.map((field) => (
            <PopoverItem
              key={field.id}
              icon={<Icon as={IconEyeOff} size={16} className="shrink-0 text-fg-subtle" />}
              onClick={() => handlers.onColumns(toggleColumn(view, field, true))}
            >
              <span className="text-fg-subtle">
                {localized(field.labels, language, field.label)}
              </span>
            </PopoverItem>
          ))}
        </List>
      </Subpage>
    );
  }

  if (page === "defaultFilters") {
    /*
     * Отбор, с которым таблица открывается у всех. Тот же редактор, что
     * и в подшапке: покажи админу другой — он задаст условие, которого
     * в подшапке потом не увидит.
     *
     * Поля — ВСЕ, а не колонки view. Отбор по умолчанию применяется
     * независимо от видимости колонки, и чип по скрытому полю иначе
     * не рисовался бы вовсе: снять заданное условие было бы нечем,
     * а список продолжал бы приезжать урезанным.
     */
    return (
      <Subpage
        title={t("view.defaultFilters")}
        busy={busy}
        onBack={back}
        hint={t("view.defaultFiltersHint")}
        wide
      >
        <DefaultFilters
          fields={fields}
          language={language}
          initial={defaultFilters}
          onChange={handlers.onDefaultFilters}
        />
      </Subpage>
    );
  }

  if (page === "quickFilters") {
    /*
     * Чипы, которые админ предлагает открывающему таблицу. Это подсказка,
     * а не ограничение: в подшапке можно добавить любое поле, и выбор
     * уезжает в адрес. Поэтому правка видна на пустом отборе — когда
     * фильтров в адресе ещё нет.
     */
    const quickIds = new Set(quick.map((field) => field.id));
    const filterable = matching(
      fields.filter((field) => filterKind(field) !== null),
      query,
      language,
    );

    return (
      <Subpage title={t("view.filters")} busy={busy} onBack={back} hint={t("view.filtersHint")}>
        <FieldSearch value={query} onChange={setQuery} />

        <List>
          {filterable.map((field) => (
            <PopoverItem
              key={field.id}
              active={quickIds.has(field.id)}
              icon={<Icon as={fieldIcon(field.type)} size={16} className="shrink-0" />}
              onClick={() =>
                handlers.onQuickFilters(
                  quickIds.has(field.id)
                    ? quick.filter((item) => item.id !== field.id)
                    : [...quick, field],
                )
              }
            >
              {localized(field.labels, language, field.label)}
            </PopoverItem>
          ))}
        </List>
      </Subpage>
    );
  }

  if (page === "fixed") {
    /*
     * Закрепляются только показанные колонки: закрепить скрытую нечего,
     * а её ключ в настройке пережил бы скрытие и всплыл при возврате.
     *
     * Пишется id ПОЛЯ, а не ключ колонки: см. ViewEdit.fixedColumns.
     */
    const fixedIds = new Set(fixed.map((field) => field.id));

    return (
      <Subpage title={t("view.fixColumns")} busy={busy} onBack={back} hint={t("view.fixHint")}>
        <FieldSearch value={query} onChange={setQuery} />

        <List>
          {matching(shown, query, language).map((field) => {
            const pinned = fixedIds.has(field.id);

            return (
              <PopoverItem
                key={field.id}
                active={pinned}
                icon={
                  <Icon
                    as={pinned ? IconPin : IconPinnedOff}
                    size={16}
                    className={`shrink-0 ${pinned ? "" : "text-fg-subtle"}`}
                  />
                }
                onClick={() =>
                  handlers.onFixedColumns(
                    pinned
                      ? fixed.filter((item) => item.id !== field.id).map((item) => item.id)
                      : [...fixed, field].map((item) => item.id),
                  )
                }
              >
                {localized(field.labels, language, field.label)}
              </PopoverItem>
            );
          })}
        </List>
      </Subpage>
    );
  }

  if (page === "fields") {
    /*
     * Поля ТАБЛИЦЫ, а не колонки view: строка ведёт в редактор поля
     * и умеет его удалить. Удаление здесь — правка схемы: поле пропадёт
     * во всех view сразу, вместе со значениями. Поэтому подтверждение
     * спрашивает вызывающий, тем же диалогом, что и меню колонки.
     */
    return (
      <Subpage title={t("view.fields")} busy={busy} onBack={back} hint={t("view.fieldsHint")}>
        <FieldSearch value={query} onChange={setQuery} />

        <List>
          {matching(fields, query, language).map((field) => (
            <div key={field.id} className="flex items-center">
              <PopoverItem
                icon={<Icon as={fieldIcon(field.type)} size={16} className="shrink-0" />}
                trailing={
                  <Icon as={IconChevronRight} size={14} className="shrink-0 text-fg-subtle" />
                }
                onClick={(event) => {
                  handlers.onEditField(field, event.currentTarget.getBoundingClientRect());
                  close();
                }}
              >
                {localized(field.labels, language, field.label)}
              </PopoverItem>

              <button
                type="button"
                onClick={() => {
                  handlers.onDeleteField(field);
                  close();
                }}
                aria-label={t("column.delete")}
                title={t("column.delete")}
                className="grid size-7 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
              >
                <Icon as={IconTrash} size={14} />
              </button>
            </div>
          ))}
        </List>
      </Subpage>
    );
  }

  const title = viewName(view, language);
  const defaultCount = activeFilterCount(defaultFilters);

  return (
    <div className="w-80">
      <Header title={t("view.options")} busy={busy} onClose={close} />

      {can.settings && (
        <>
          <div className="flex items-center gap-1.5 p-1">
            <span className="grid size-8 shrink-0 place-items-center rounded-md border border-border text-fg-muted">
              <Icon as={viewIcon(view.type)} size={16} />
            </span>
            {/* Placeholder — тип, а не пустота: у большинства view имени нет,
                и пустая строка ввода читается как «настройка сломалась». */}
            <NameInput
              value={view.names[language] ?? view.name}
              placeholder={typeLabel}
              onCommit={(name) => handlers.onRename(name, language)}
            />
          </div>

          <Row
            icon={IconLayoutList}
            label={t("view.viewType")}
            value={typeLabel}
            onClick={() => open("type")}
          />
          <Row
            icon={IconTag}
            label={t("view.viewSettings")}
            value={title}
            onClick={() => open("view")}
          />

          <PopoverSeparator />
        </>
      )}

      {can.columns && (
        <Row
          icon={IconEye}
          label={t("view.columns")}
          value={String(shown.length)}
          onClick={() => open("columns")}
        />
      )}
      {can.settings && (
        <>
          <Row
            icon={IconFilterCog}
            label={t("view.defaultFilters")}
            value={defaultCount ? String(defaultCount) : ""}
            onClick={() => open("defaultFilters")}
          />
          <Row
            icon={IconFilter}
            label={t("view.filters")}
            value={quick.length ? String(quick.length) : ""}
            onClick={() => open("quickFilters")}
          />
        </>
      )}
      {can.fixColumn && (
        <Row
          icon={IconPin}
          label={t("view.fixColumns")}
          value={fixed.length ? String(fixed.length) : ""}
          onClick={() => open("fixed")}
        />
      )}

      <PopoverSeparator />

      {can.excelMenu && (
        <>
          <PopoverItem
            icon={<Icon as={IconFileImport} size={16} className="shrink-0 text-fg-muted" />}
            onClick={() => {
              handlers.onImport();
              close();
            }}
          >
            {t("view.import")}
          </PopoverItem>

          <PopoverItem
            icon={
              <Icon
                as={exporting ? IconLoader2 : IconFileExport}
                size={16}
                className={`shrink-0 text-fg-muted ${exporting ? "animate-spin" : ""}`}
              />
            }
            onClick={() => handlers.onExport()}
          >
            {t("view.export")}
          </PopoverItem>
        </>
      )}

      <PopoverItem
        icon={<Icon as={IconLink} size={16} className="shrink-0 text-fg-muted" />}
        onClick={() => {
          // Адрес целиком: в нём и view, и страница, и отбор — тот самый
          // экран, который человек видит, а не просто пункт меню.
          void navigator.clipboard.writeText(window.location.href);
          toast.success(t("view.linkCopied"));
          close();
        }}
      >
        {t("view.copyLink")}
      </PopoverItem>

      <PopoverSeparator />
      <p className="px-2 py-1 text-2xs text-fg-subtle">{t("view.dataSection")}</p>

      {/* Таблица показана, но не выбирается: слаг задаётся при создании
          пункта меню и меняет смысл всего экрана, а не вид одного view. */}
      <div className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-sm text-fg">
        <Icon as={IconTable} size={16} className="shrink-0 text-fg-muted" />
        <span className="flex-1 truncate">{t("view.source")}</span>
        <span className="max-w-[9rem] truncate text-fg-subtle">{view.tableSlug}</span>
      </div>

      {can.settings && (
        <Row
          icon={IconLayoutList}
          label={t("view.fields")}
          value={String(fields.length)}
          onClick={() => open("fields")}
        />
      )}

      {handlers.onDelete && can.settings && (
        <>
          <PopoverSeparator />

          <PopoverItem
            danger
            icon={<Icon as={IconTrash} size={16} className="shrink-0" />}
            onClick={() => {
              handlers.onDelete?.();
              close();
            }}
          >
            {t("view.delete")}
          </PopoverItem>
        </>
      )}
    </div>
  );
}

/**
 * Показанные колонки в их порядке — с перетаскиванием.
 *
 * Штатный drag-and-drop браузера, без библиотеки: список короткий,
 * вертикальный и без вложенности, а всё, что для него нужно, — атрибут
 * `draggable` и три обработчика. Ближайшая библиотека тянет за собой
 * сенсоры, коллизии и модификаторы, которым здесь нечего решать.
 *
 * ponytail: мышь и только мышь. Порядок колонок правит админ и раз
 * в полгода; клавиатурная перестановка появится, если об неё
 * действительно споткнутся.
 *
 * Порядок отдаётся целиком и сразу, как и остальные правки панели:
 * список колонок — одно значение, а не набор независимых.
 */
function ColumnOrder({
  shown,
  language,
  draggable,
  onReorder,
  onHide,
}: {
  shown: Field[];
  language: string;
  /** Список отфильтрован поиском — перетаскивать нечего: см. страницу колонок. */
  draggable: boolean;
  onReorder: (columnIds: string[]) => void;
  onHide: (field: Field) => void;
}) {
  const { t } = useTranslation();
  const [dragged, setDragged] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);

  const drop = (target: string) => {
    if (dragged && dragged !== target) {
      onReorder(moveBefore(shown.map(columnKey), dragged, target));
    }
    setDragged(null);
    setOver(null);
  };

  return (
    <>
      {shown.map((field) => {
        const key = columnKey(field);

        return (
          <div
            key={field.id}
            draggable={draggable}
            onDragStart={() => setDragged(key)}
            onDragEnd={() => {
              setDragged(null);
              setOver(null);
            }}
            /* preventDefault обязателен: без него браузер запрещает
               бросок, и onDrop не случается вовсе. */
            onDragOver={(event) => {
              event.preventDefault();
              setOver(key);
            }}
            onDrop={(event) => {
              event.preventDefault();
              drop(key);
            }}
            className={`flex h-8 w-full items-center gap-2 rounded-md px-2 text-sm transition-colors ${
              dragged === key
                ? "opacity-40"
                : over === key && dragged
                  ? "bg-accent-subtle"
                  : "hover:bg-surface-hover"
            }`}
          >
            {draggable && (
              <Icon
                as={IconGripVertical}
                size={14}
                className="shrink-0 cursor-grab text-fg-subtle"
              />
            )}
            <Icon as={fieldIcon(field.type)} size={16} className="shrink-0 text-fg-muted" />
            <span className="flex-1 truncate">
              {localized(field.labels, language, field.label)}
            </span>

            <button
              type="button"
              onClick={() => onHide(field)}
              aria-label={t("view.hideColumn")}
              title={t("view.hideColumn")}
              className="grid size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
            >
              <Icon as={IconEye} size={14} />
            </button>
          </div>
        );
      })}
    </>
  );
}

/**
 * Редактор отбора по умолчанию.
 *
 * Своё состояние, хотя всё остальное в панели работает от серверного:
 * условие без значения на сервер не уезжает вовсе (toConditions его
 * отбрасывает), и чип, нарисованный от серверного состояния, исчезал
 * бы ровно в тот момент, когда его добавили. Задать отбор по умолчанию
 * было нельзя ни одного разу.
 *
 * Начальное значение берётся при открытии страницы: пока она открыта,
 * правит его только человек.
 */
function DefaultFilters({
  fields,
  language,
  initial,
  onChange,
}: {
  fields: Field[];
  language: string;
  initial: Filters;
  onChange: (filters: Filters) => void;
}) {
  const [filters, setFilters] = useState(initial);

  return (
    <FilterBar
      columns={fields}
      language={language}
      filters={filters}
      sorts={[]}
      onFilters={(next) => {
        setFilters(next);
        onChange(next);
      }}
      onSorts={() => {}}
    />
  );
}

/** Строка списка настроек: значок, название, текущее значение, стрелка. */
function Row({
  icon,
  label,
  value,
  onClick,
}: {
  icon: TablerIcon;
  label: string;
  value: string;
  onClick: () => void;
}) {
  return (
    <PopoverItem
      icon={<Icon as={icon} size={16} className="shrink-0 text-fg-muted" />}
      onClick={onClick}
      trailing={
        <span className="flex min-w-0 shrink-0 items-center gap-1 text-fg-subtle">
          <span className="max-w-[7rem] truncate">{value}</span>
          <Icon as={IconChevronRight} size={14} />
        </span>
      }
    >
      {label}
    </PopoverItem>
  );
}

function Subpage({
  title,
  busy,
  hint,
  wide,
  onBack,
  children,
}: {
  title: string;
  busy: boolean;
  hint?: string;
  /** Редактору фильтров 320 пикселей мало: чипы складываются в столбик. */
  wide?: boolean;
  onBack: () => void;
  children: ReactNode;
}) {
  return (
    <div className={wide ? "w-[26rem]" : "w-80"}>
      <Header title={title} busy={busy} onBack={onBack} />
      {hint && <p className="px-2 pb-1 text-2xs text-fg-subtle">{hint}</p>}
      {children}
    </div>
  );
}

function Header({
  title,
  busy,
  onBack,
  onClose,
}: {
  title: string;
  busy: boolean;
  onBack?: () => void;
  onClose?: () => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="flex h-8 items-center gap-1 px-1">
      {onBack && <IconTool icon={IconChevronLeft} label={t("action.back")} onClick={onBack} />}

      <span className="flex-1 truncate px-1 text-xs font-medium text-fg-muted">{title}</span>

      {/* Признак работы: правка уезжает на сервер и возвращается оттуда же,
          иначе щелчок выглядит несработавшим. */}
      {busy && <Icon as={IconLoader2} size={12} className="shrink-0 animate-spin text-fg-subtle" />}
      {onClose && <IconTool icon={IconX} label={t("action.close")} onClick={onClose} />}
    </div>
  );
}

function IconTool({
  icon,
  label,
  onClick,
}: {
  icon: TablerIcon;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="grid size-6 shrink-0 place-items-center rounded text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={icon} size={16} />
    </button>
  );
}

function BulkButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="h-7 flex-1 rounded-md border border-border-strong text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      {label}
    </button>
  );
}

function List({ children }: { children: ReactNode }) {
  return <div className="max-h-72 overflow-y-auto">{children}</div>;
}

/**
 * Поиск по полям. Стоит на каждой странице со списком: у таблицы
 * в сорок полей прокрутка на 72 пикселя высоты — это не список,
 * а щель, и нужное поле в ней ищут глазами по десять секунд.
 *
 * Без debounce: фильтруется массив в памяти, запроса здесь нет.
 */
function FieldSearch({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { t } = useTranslation();

  return (
    <div className="relative px-1 pb-1">
      <Icon
        as={IconSearch}
        size={14}
        className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-fg-subtle"
      />
      <Input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t("table.searchField")}
        aria-label={t("table.searchField")}
        className="pl-7"
      />
    </div>
  );
}

/**
 * Поля, подходящие под строку поиска. Ищем и по подписи на языке данных,
 * и по слагу: админ помнит колонку то так, то так, а в списке она
 * подписана только первым.
 */
function matching(fields: Field[], query: string, language: string): Field[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return fields;

  return fields.filter(
    (field) =>
      localized(field.labels, language, field.label).toLowerCase().includes(needle) ||
      field.slug.toLowerCase().includes(needle),
  );
}

/**
 * Имя view. Применяется по Enter и по уходу фокуса — как правка ячейки:
 * запрос на каждую букву превратил бы переименование в десяток PUT'ов.
 */
function NameInput({
  value,
  placeholder,
  onCommit,
}: {
  value: string;
  placeholder: string;
  onCommit: (name: string) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState(value);

  const commit = () => {
    const next = name.trim();
    if (next && next !== value) onCommit(next);
  };

  return (
    <Input
      value={name}
      onChange={(event) => setName(event.target.value)}
      onBlur={commit}
      onKeyDown={(event) => {
        if (event.key === "Enter") {
          event.preventDefault();
          commit();
          event.currentTarget.blur();
        }
      }}
      placeholder={placeholder}
      aria-label={t("view.name")}
    />
  );
}

/**
 * Раскладки, на которые можно переключить view.
 *
 * Только те, которые мы действительно рисуем, — предлагать доску,
 * календарь и сводную, за которыми стоит «экран не готов», значит
 * запирать view: панель, из которой тип меняют, исчезает вместе
 * с таблицей. Плюс текущий тип: view, созданный доской в старой
 * админке, иначе некуда вернуть.
 */
function switchableTypes(view: View): string[] {
  return VIEW_TYPES.filter((type) => IMPLEMENTED_VIEW_TYPES.has(type) || type === view.type);
}

/** Поля, показанные во view, в порядке view. Та же логика, что в resolveColumns. */
function shownFields(view: View, fields: Field[]): Field[] {
  const index = new Map<string, Field>();
  for (const field of fields) {
    index.set(field.id, field);
    if (field.relationId) index.set(field.relationId, field);
  }

  const seen = new Set<string>();
  const shown: Field[] = [];

  for (const id of view.columnIds) {
    const field = index.get(id);
    if (!field || seen.has(field.id)) continue;

    seen.add(field.id);
    shown.push(field);
  }

  return shown;
}

/** Поля, предложенные чипами в подшапке. Ключ тот же, что и у колонок. */
function quickFilterFields(view: View, fields: Field[]): Field[] {
  const wanted = new Set(view.quickFilterIds);

  return fields.filter((field) => wanted.has(field.relationId ?? field.id) || wanted.has(field.id));
}

/**
 * Закреплённые колонки — в порядке показа, а не в порядке закрепления.
 * Слева они рисуются подряд, и «второй закреплённой» на экране всегда
 * оказывается та, что правее в columns, а не та, что нажали второй.
 */
function fixedFields(view: View, shown: Field[]): Field[] {
  const wanted = new Set(view.fixedColumnIds);

  return shown.filter((field) => wanted.has(field.relationId ?? field.id) || wanted.has(field.id));
}

/**
 * Новый список колонок после переключения одной.
 *
 * При скрытии убираются ОБА ключа поля-связи — и id поля, и id связи.
 * Бэкенд при создании view кладёт в columns оба (view.go, INSERT), и
 * колонка, снятая по одному ключу, продолжает находиться по второму:
 * в старой админке галочка снималась, а колонка оставалась.
 *
 * Новая колонка встаёт в конец: у view нет «правильного места» для неё,
 * а вставка в середину переставила бы соседние без спроса.
 */
function toggleColumn(view: View, field: Field, visible: boolean): string[] {
  const keys = new Set([field.id, ...(field.relationId ? [field.relationId] : [])]);
  const rest = view.columnIds.filter((id) => !keys.has(id));

  return visible ? [...rest, columnKey(field)] : rest;
}
