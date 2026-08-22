import { useState, type ReactNode } from "react";
import {
  IconCalendarTime,
  IconChevronLeft,
  IconChevronRight,
  IconDotsVertical,
  IconEye,
  IconExternalLink,
  IconEyeOff,
  IconFileExport,
  IconFileImport,
  IconFilter,
  IconFilterCog,
  IconInfinity,
  IconGripVertical,
  IconLayoutList,
  IconLayoutColumns,
  IconLayoutNavbar,
  IconLink,
  IconLoader2,
  IconPin,
  IconPinnedOff,
  IconSearch,
  IconStack2,
  IconPrinter,
  IconTable,
  IconTrash,
  IconX,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Permission } from "@/features/auth";
import { DocTemplates, useDocTemplates } from "@/features/docs";
import {
  FilterBar,
  activeFilterCount,
  fieldIcon,
  filterKind,
  type Filters,
} from "@/features/item";
import {
  TableSettings,
  baseSlug,
  collapseLanguages,
  languageGroups,
  localized,
  type Field,
} from "@/features/table";
import type { DataLanguage } from "@/features/workspace";
import type { TranslationKey } from "@/shared/lib/i18n";
import { toast } from "@/shared/lib/toast";
import { Checkbox } from "@/shared/ui/checkbox";
import { Icon } from "@/shared/ui/icon";
import { CommitInput } from "@/shared/ui/commit-input";
import { Input } from "@/shared/ui/input";
import { LanguageInput } from "@/shared/ui/language-input";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { ToolButton } from "@/shared/ui/tool-button";
import { TAB_GROUP_TYPES, tabGroupField } from "../api/tab-group";
import { columnKey, moveBefore } from "../model/columns";
import { hasUrl, type UrlTemplate } from "../model/url-template";
import { IMPLEMENTED_VIEW_TYPES, TAB_VIEW_TYPES, VIEW_TYPES, type View } from "../model/types";
import { CalendarFields, dateFields } from "./CalendarFields";
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
 *   Настройки самой ТАБЛИЦЫ (в старой админке они звались «General»)
 *   лежат отдельной страницей в секции «Данные», за строкой «Таблица»:
 *   это настройки не view, а хранилища, и меняют они поведение всех
 *   view сразу. Строка «Тип» здесь называется типом, а не «Общими»,
 *   чтобы имя не было занято.
 *
 *   Настройки Timeline — этого типа у нас пока нет вовсе. У календаря
 *   настраиваются только поля дат: остальное (шаг сетки, цвет события,
 *   нерабочие дни) бэкенд отдаёт, но не обновляет ни одной ручкой.
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
/**
 * Необязательный обработчик — это отсутствующая строка настроек, а не
 * запрещённая: панель одна и на таблицу, и на вкладку связи в карточке,
 * а у вкладки нет ни типа, ни своих адресов перехода. Показывать
 * настройку, которой некуда уехать, хуже, чем не показывать вовсе.
 */
export type ViewOptionsLabels = { title: TranslationKey; delete: TranslationKey };

const VIEW_LABELS: ViewOptionsLabels = { title: "view.options", delete: "view.delete" };

export type ViewOptionsHandlers = {
  /** Имя на конкретном языке ДАННЫХ. Язык задаёт вызывающая страница. */
  onRename: (name: string, language: string) => void;
  onColumns: (columnIds: string[]) => void;
  onQuickFilters: (fields: Field[]) => void;
  /** Закреплённые колонки целиком: список ключей, как в columns. */
  onFixedColumns: (columnIds: string[]) => void;
  /** Отбор, с которым таблица открывается. Пустой — отбора нет. */
  onDefaultFilters: (filters: Filters) => void;
  /** Смена типа. Нет обработчика — строки «Тип» нет вовсе. */
  onType?: (type: string) => void;
  /** Куда уводит щелчок по строке. Пустой адрес — открывается карточка. */
  onNavigate?: (template: UrlTemplate) => void;
  /** Куда ведёт «новая запись». Пустой адрес — строка заводится в таблице. */
  onObjectUrl?: (template: UrlTemplate) => void;
  /** Адрес PDF записи. Пусто — кнопки в карточке нет. */
  onPdfUrl?: (url: string) => void;
  /**
   * Догружать строки прокруткой вместо номеров страниц. Нет обработчика
   * — нет и переключателя: у вкладки связи свой подвал.
   */
  onInfiniteScroll?: (enabled: boolean) => void;
  /** Поле группировки строк. Пустая строка — без группировки. */
  /** Поля группировки в порядке уровней. Пустой список — снять её. */
  onGroupBy?: (fieldIds: string[]) => void;
  /** Поле раскладки вкладками. Пустая строка — без вкладок. */
  onTabGroup?: (fieldId: string) => void;
  /**
   * Поля дат календаря. Слаги, а не id: так эту настройку хранит база
   * (колонки `calendar_from_slug` и `calendar_to_slug`).
   */
  onDateFrom?: (slug: string) => void;
  onDateTo?: (slug: string) => void;
  /** Настроить поле: открывает ту же панель, что и меню колонки. */
  onEditField?: (field: Field, anchor: DOMRect) => void;
  /** Удалить поле из ТАБЛИЦЫ, а не из view. Спрашивает подтверждение вызывающий. */
  onDeleteField?: (field: Field) => void;
  onImport?: () => void;
  onExport?: () => void;
  /** Нет обработчика — удалять нечем. */
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
  labels = VIEW_LABELS,
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
  /**
   * Как называть настраиваемое. По умолчанию это view; вкладка связи
   * в карточке зовёт ту же панель своими словами — «Настройки вкладки»
   * и «Убрать вкладку».
   */
  labels?: ViewOptionsLabels;
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
          label={t(labels.title)}
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
          labels={labels}
          close={close}
        />
      )}
    </Popover>
  );
}

/** Открытая страница панели. null — список настроек. */
type PanelPage =
  | "type"
  | "columns"
  | "defaultFilters"
  | "quickFilters"
  | "fixed"
  | "group"
  | "tabGroup"
  | "calendar"
  | "fields"
  | "table"
  | "docs"
  | "navigation"
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
  labels,
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
  labels: ViewOptionsLabels;
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

  /* Сколько печатных форм у таблицы — числом в строке, как у полей.
     Тот же запрос, что и у кнопки печати в карточке: ключ общий. */
  const { templates: docTemplates } = useDocTemplates(view.tableSlug);

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
  /*
   * Доска раскладывает те же строки по колонкам, поэтому настройка
   * `group_fields` у неё называется своими словами: у таблицы это
   * вкладки, у доски — колонки. Настройка одна, экран разный.
   */
  const isBoard = view.type === "BOARD";
  /**
   * У календаря вместо колонок — дни, у таймлайна — ось дней, и настройка
   * у обоих одна и та же: поля дат. Закреплённых колонок и группировки
   * строк нет ни у того, ни у другого.
   */
  const isCalendar = view.type === "CALENDAR" || view.type === "TIMELINE";

  if (page === "type") {
    return (
      <Subpage title={t("view.viewType")} busy={busy} onBack={back}>
        {types.map((type) => (
          <PopoverItem
            key={type}
            active={type === view.type}
            icon={<Icon as={viewIcon(type)} size={16} className="shrink-0" />}
            onClick={() => handlers.onType?.(type)}
          >
            {t(`view.type.${type}` as TranslationKey, { defaultValue: type })}
          </PopoverItem>
        ))}
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
    /*
     * Мультиязычное поле — одна строка в списке, а не по строке на язык:
     * в таблице оно одна колонка (см. collapseLanguages), и показывать
     * его тремя одинаковыми подписями значит предложить скрыть половину
     * колонки. Переключается вся языковая группа сразу — иначе колонка
     * теряет язык и подписывается «Название (cyr)».
     */
    const codes = languages.map((item) => item.code);
    const visible = collapseLanguages(
      shown.filter((field) => field.slug !== "guid"),
      codes,
      language,
    );
    const shownSlugs = new Set(shown.map((field) => field.slug));
    const hidden = matching(
      collapseLanguages(hideable, codes, language).filter(
        (field) => !shownSlugs.has(field.slug),
      ),
      query,
      language,
    );
    const groups = languageGroups(fields, codes);
    /** Все языковые варианты поля. Обычное поле — оно само. */
    const groupOf = (field: Field): Field[] => {
      const base = baseSlug(field, codes);
      return (base === null ? undefined : groups.get(base)) ?? [field];
    };

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
            onHide={(field) => handlers.onColumns(toggleColumn(view, groupOf(field), false))}
          />

          {hidden.map((field) => (
            <PopoverItem
              key={field.id}
              icon={<Icon as={IconEyeOff} size={16} className="shrink-0 text-fg-subtle" />}
              onClick={() => handlers.onColumns(toggleColumn(view, groupOf(field), true))}
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

  if (page === "group") {
    /*
     * Поле группировки — одно из ПОКАЗАННЫХ: заголовок группы рисует
     * та же ячейка, что и колонку, а группировка по скрытому полю
     * показывала бы группы без колонки, из которой они растут.
     *
     * Языковые варианты сведены до одного, как на странице колонок:
     * группа по «Название (en)» и группа по «Название (cyr)» — это
     * одна настройка, а не три.
     */
    const codes = languages.map((item) => item.code);
    const groupable = matching(collapseLanguages(shown, codes, language), query, language);

    return (
      <Subpage title={t("view.groupBy")} busy={busy} onBack={back} hint={t("view.groupByHint")}>
        <FieldSearch value={query} onChange={setQuery} />

        <List>
          <PopoverItem
            active={!view.groupByIds.length}
            icon={<Icon as={IconX} size={16} className="shrink-0 text-fg-subtle" />}
            onClick={() => handlers.onGroupBy?.([])}
          >
            {t("view.groupNone")}
          </PopoverItem>

          {/*
            Выбранные — сверху и по порядку уровней: список галочек
            без видимого порядка не сообщает, что вложено во что.
            Щелчок по выбранному снимает его, по новому — добавляет
            уровнем ниже.
          */}
          {order(groupable, view.groupByIds).map((field) => {
            const key = columnKey(field);
            const level = view.groupByIds.indexOf(key);

            return (
              <PopoverItem
                key={field.id}
                active={level >= 0}
                icon={<Icon as={fieldIcon(field.type)} size={16} className="shrink-0" />}
                onClick={() =>
                  handlers.onGroupBy?.(
                    level >= 0
                      ? view.groupByIds.filter((id) => id !== key)
                      : [...view.groupByIds, key],
                  )
                }
              >
                <span className="flex min-w-0 flex-1 items-center gap-1.5">
                  <span className="truncate">{localized(field.labels, language, field.label)}</span>
                  {/* Номер уровня: по нему видно, что группируется первым. */}
                  {level >= 0 && (
                    <span className="text-2xs ml-auto shrink-0 text-fg-subtle tabular-nums">
                      {level + 1}
                    </span>
                  )}
                </span>
              </PopoverItem>
            );
          })}
        </List>
      </Subpage>
    );
  }

  if (page === "tabGroup") {
    /*
     * Раскладка вкладками — не группировка: список сужается до одного
     * значения, и остальные строки на экран не приезжают. Поэтому
     * поля здесь ВСЕ, а не показанные: раскладывать по скрытой колонке
     * — обычное дело («только заказы этого склада»), а колонка со складом
     * в таблице при этом лишняя.
     *
     * Годятся только поля с конечным набором значений — те же пять типов,
     * что и в старой админке. По строке вкладок не сделать: их было бы
     * столько же, сколько записей.
     */
    const groupable = matching(
      fields.filter((field) => TAB_GROUP_TYPES.has(field.type)),
      query,
      language,
    );

    return (
      <Subpage
        title={t(isBoard ? "view.boardGroup" : "view.tabGroup")}
        busy={busy}
        onBack={back}
        hint={t(isBoard ? "view.boardGroupHint" : "view.tabGroupHint")}
      >
        <FieldSearch value={query} onChange={setQuery} />

        <List>
          {/* У доски «без раскладки» не бывает: без поля у неё нет
              и колонок, а значит и самой доски. */}
          {!isBoard && (
            <PopoverItem
              active={!view.tabGroupId}
              icon={<Icon as={IconX} size={16} className="shrink-0 text-fg-subtle" />}
              onClick={() => handlers.onTabGroup?.("")}
            >
              {t("view.tabGroupNone")}
            </PopoverItem>
          )}

          {groupable.map((field) => (
            <PopoverItem
              key={field.id}
              active={view.tabGroupId === field.id || view.tabGroupId === field.relationId}
              icon={<Icon as={fieldIcon(field.type)} size={16} className="shrink-0" />}
              /* Ключ тот же, что и у колонок: у связи это id связи —
                 так эту настройку пишет и читает старая админка. */
              onClick={() => handlers.onTabGroup?.(columnKey(field))}
            >
              {localized(field.labels, language, field.label)}
            </PopoverItem>
          ))}

          {/* Список пуст не «потому что не нашлось», а потому что таблице
              нечем: без подходящих полей человек искал бы опечатку в поиске. */}
          {!groupable.length && !query && (
            <p className="px-2 py-1.5 text-2xs text-fg-subtle">{t("view.tabGroupEmpty")}</p>
          )}
        </List>
      </Subpage>
    );
  }

  if (page === "calendar") {
    /*
     * Поля дат — из ВСЕХ полей таблицы, а не из колонок view: срок
     * задачи бывает и не показан в списке колонок, а событию он всё
     * равно нужен.
     *
     * Настраиваются только эти две; шаг сетки, цвет события и нерабочие
     * дни календарь читает, но задать их нечем — ни одна ручка эти
     * колонки не обновляет (view.go, Update: их нет в теле запроса).
     * Поле ввода, которое молча ничего не сохраняет, — ровно то, за что
     * переписан старый конструктор. См. docs/backend-notes.md.
     */
    const dates = matching(dateFields(fields), query, language);

    return (
      <Subpage
        title={t("view.calendarFields")}
        busy={busy}
        onBack={back}
        hint={t("view.calendarFieldsHint")}
      >
        <FieldSearch value={query} onChange={setQuery} />

        <List>
          <CalendarFields
            from={view.dateFromSlug}
            to={view.dateToSlug}
            dates={dates}
            language={language}
            onDateFrom={(slug) => handlers.onDateFrom?.(slug)}
            onDateTo={(slug) => handlers.onDateTo?.(slug)}
          />

          {/* Список пуст не «потому что не нашлось»: без полей с датой
              календарю нечего показывать вовсе. */}
          {!dates.length && !query && (
            <p className="px-2 py-1.5 text-2xs text-fg-subtle">{t("view.calendarEmpty")}</p>
          )}
        </List>
      </Subpage>
    );
  }

  if (page === "table") {
    /*
     * Настройки хранилища, а не показа: имя таблицы, кэш, мягкое
     * удаление, вход. Действуют во всех view сразу — об этом говорит
     * и подсказка, и то, что страница лежит в секции «Данные».
     */
    return (
      <Subpage title={t("tableSettings.title")} busy={busy} onBack={back} hint={t("tableSettings.hint")}>
        <TableSettings tableSlug={view.tableSlug} languages={languages} />
      </Subpage>
    );
  }

  if (page === "docs") {
    /*
     * Печатные формы таблицы. Здесь же, где остальные настройки таблицы,
     * а не отдельным экраном: в старой админке «Docs» уводит со своих
     * данных, и запись для печати приходится выбирать заново.
     */
    return (
      <Subpage title={t("docs.title")} busy={busy} onBack={back} hint={t("docs.hint")}>
        <DocTemplates tableSlug={view.tableSlug} fields={fields} />
      </Subpage>
    );
  }

  if (page === "navigation") {
    /*
     * Адреса, которыми экран подменяется чужим: щелчок по строке уводит
     * на страницу проекта, «новая запись» — на его же форму, а PDF
     * открывается кнопкой в карточке.
     *
     * `{{$слаг}}` в адресе подставляется значением поля строки — та же
     * запись, что и в старой админке, чтобы уже настроенные адреса
     * работали как работали.
     */
    return (
      <Subpage title={t("view.navigation")} busy={busy} onBack={back} hint={t("view.navigationHint")}>
        <div className="flex max-h-[70vh] flex-col gap-3 overflow-y-auto p-1">
          <UrlSetting
            label={t("view.navigateUrl")}
            hint={t("view.navigateUrlHint")}
            template={view.navigate}
            onChange={(template) => handlers.onNavigate?.(template)}
          />

          <UrlSetting
            label={t("view.objectUrl")}
            hint={t("view.objectUrlHint")}
            template={view.objectUrl}
            onChange={(template) => handlers.onObjectUrl?.(template)}
          />

          <label className="flex flex-col gap-0.5">
            <span className="px-0.5 text-2xs text-fg-muted">{t("view.pdfUrl")}</span>
            <CommitInput
              value={view.pdfUrl}
              placeholder={URL_PLACEHOLDER}
              label={t("view.pdfUrl")}
              allowEmpty
              onCommit={(url) => handlers.onPdfUrl?.(url)}
            />
            <span className="px-0.5 text-2xs text-fg-subtle">{t("view.pdfUrlHint")}</span>
          </label>
        </div>
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
                  handlers.onEditField?.(field, event.currentTarget.getBoundingClientRect());
                  close();
                }}
              >
                {localized(field.labels, language, field.label)}
              </PopoverItem>

              <button
                type="button"
                onClick={() => {
                  handlers.onDeleteField?.(field);
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

  const defaultCount = activeFilterCount(defaultFilters);
  /*
   * Настройки, которых у типа view нет, не показываются. У дерева нет
   * ни страниц, ни фильтров, ни группировки, ни перехода по клику:
   * его ручка (/v2/items/{slug}/tree) читает только поля и родителя.
   * Переключатель, который ничего не меняет, хуже отсутствующего.
   */
  const isTree = view.type === "TREE";
  /*
   * Настройки таблицы, которых у доски нет: закреплённых колонок
   * (колонок нет вовсе), номеров страниц (доска листается прокруткой)
   * и группировки строк — на доске за неё отвечают сами колонки.
   */
  const isGrid = !isTree && !isBoard && !isCalendar;
  /*
   * Где строки собираются в группы: таблица и таймлайн. У доски за это
   * отвечают её колонки, у календаря — клетки дней, а дерево строит
   * порядок само.
   */
  const canGroup = isGrid || view.type === "TIMELINE";
  /** Поле начала события — подписью в строке настроек. Здесь это слаг. */
  const dateFrom = fields.find((field) => field.slug === view.dateFromSlug);
  /**
   * Поля группировки — подписью в строке настроек: первое словом,
   * остальные счётом. Ключи те же, что у колонок.
   */
  const grouped = view.groupByIds
    .map((id) => fields.find((field) => field.id === id || field.relationId === id))
    .filter((field): field is Field => Boolean(field));
  /** Поле раскладки вкладками — подписью в строке настроек. */
  const tabGrouped = tabGroupField(view, fields);
  /* Сколько адресов задано: строка настроек молчит, пока их нет. */
  const navigationCount = [hasUrl(view.navigate), hasUrl(view.objectUrl), Boolean(view.pdfUrl)]
    .filter(Boolean).length;

  return (
    <div className="w-80">
      <Header title={t(labels.title)} busy={busy} onClose={close} />

      {can.settings && (
        <>
          <div className="flex items-center gap-1.5 p-1">
            <span className="grid size-8 shrink-0 place-items-center rounded-md border border-border text-fg-muted">
              <Icon as={viewIcon(view.type)} size={16} />
            </span>
            {/*
              Имя на каждом языке ДАННЫХ — одним полем с переключателем
              внутри, как в старой админке (TextFieldWithMultiLanguage).
              Пока правился только текущий язык, админ, работающий
              в русском интерфейсе, годами не видел, что узбекское имя
              вкладки пустое.

              Placeholder — тип, а не пустота: у большинства view имени
              нет, и пустая строка ввода читается как «настройка
              сломалась».
            */}
            <LanguageInput
              languages={languages}
              values={view.names}
              placeholder={typeLabel}
              label={t("view.name")}
              onCommit={(code, name) => handlers.onRename(name, code)}
            />
          </div>

          {handlers.onType && (
            <Row
              icon={IconLayoutList}
              label={t("view.viewType")}
              value={typeLabel}
              onClick={() => open("type")}
            />
          )}
          {handlers.onNavigate && !isTree && (
            <Row
              icon={IconExternalLink}
              label={t("view.navigation")}
              value={navigationCount ? String(navigationCount) : ""}
              onClick={() => open("navigation")}
            />
          )}

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
      {can.settings && !isTree && (
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
      {/* Даты события — первая настройка календаря: без поля начала
          он вообще ничего не рисует. */}
      {can.settings && isCalendar && handlers.onDateFrom && (
        <Row
          icon={IconCalendarTime}
          label={t("view.calendarFields")}
          value={dateFrom ? localized(dateFrom.labels, language, dateFrom.label) : ""}
          onClick={() => open("calendar")}
        />
      )}
      {can.fixColumn && !isBoard && !isCalendar && (
        <Row
          icon={IconPin}
          label={t("view.fixColumns")}
          value={fixed.length ? String(fixed.length) : ""}
          onClick={() => open("fixed")}
        />
      )}
      {can.settings && handlers.onGroupBy && canGroup && (
        <Row
          icon={IconStack2}
          label={t("view.groupBy")}
          value={
            grouped[0]
              ? localized(grouped[0].labels, language, grouped[0].label) +
                (grouped.length > 1 ? ` +${grouped.length - 1}` : "")
              : ""
          }
          onClick={() => open("group")}
        />
      )}
      {/* Раскладка вкладками — своё право роли (`tab_group`), отдельное
          от настройки view: так их и выдаёт бэкенд. */}
      {can.settings && can.tabGroup && handlers.onTabGroup && !isTree && (
        <Row
          icon={isBoard ? IconLayoutColumns : IconLayoutNavbar}
          label={t(isBoard ? "view.boardGroup" : "view.tabGroup")}
          value={tabGrouped ? localized(tabGrouped.labels, language, tabGrouped.label) : ""}
          onClick={() => open("tabGroup")}
        />
      )}

      {/* Переключатель, а не страница: у настройки два состояния,
          и ради них открывать экран незачем. */}
      {can.settings && handlers.onInfiniteScroll && isGrid && (
        <label className="flex h-8 w-full cursor-pointer items-center gap-2 rounded-md px-2 text-sm text-fg transition-colors hover:bg-surface-hover">
          <Icon as={IconInfinity} size={16} className="shrink-0 text-fg-muted" />
          <span className="flex-1 truncate">{t("view.infiniteScroll")}</span>
          <Checkbox
            checked={view.infiniteScroll}
            onChange={(event) => handlers.onInfiniteScroll?.(event.target.checked)}
          />
        </label>
      )}

      <PopoverSeparator />

      {can.excelMenu && handlers.onImport && handlers.onExport && (
        <>
          <PopoverItem
            icon={<Icon as={IconFileImport} size={16} className="shrink-0 text-fg-muted" />}
            onClick={() => {
              handlers.onImport?.();
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
            onClick={() => handlers.onExport?.()}
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

      {/* Таблица не ВЫБИРАЕТСЯ — слаг задаётся при создании пункта меню
          и меняет смысл всего экрана, — но настраивается: строка ведёт
          в настройки самой таблицы. */}
      {can.settings ? (
        <Row
          icon={IconTable}
          label={t("view.source")}
          value={view.tableSlug}
          onClick={() => open("table")}
        />
      ) : (
        <div className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-sm text-fg">
          <Icon as={IconTable} size={16} className="shrink-0 text-fg-muted" />
          <span className="flex-1 truncate">{t("view.source")}</span>
          <span className="max-w-[9rem] truncate text-fg-subtle">{view.tableSlug}</span>
        </div>
      )}

      {/* Печатные формы: шаблон .docx, из которого собирается PDF записи.
          Рядом с настройками таблицы, потому что шаблоны у таблицы общие
          — во всех её view одни и те же. */}
      {can.settings && (
        <Row
          icon={IconPrinter}
          label={t("docs.title")}
          value={String(docTemplates.length)}
          onClick={() => open("docs")}
        />
      )}

      {/* Поля правятся там, где для них есть редактор: у вкладки связи
          это поля ЧУЖОЙ таблицы, и открывать их из карточки записи
          означало бы редактор поверх редактора. */}
      {can.settings && handlers.onEditField && (
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
            {t(labels.delete)}
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
 * С клавиатуры переставляют теми же стрелками, что и мышью — тянуть:
 * ручка списка это кнопка, и на ней ↑/↓ двигают колонку на позицию.
 * Перетаскивание мышью недоступно тому, кто ей не пользуется, а список
 * колонок — единственное место в панели, где порядок вообще задаётся.
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

  /** Колонку на позицию вверх или вниз. Порядок отдаётся целиком. */
  const move = (key: string, delta: number) => {
    const keys = shown.map(columnKey);
    const from = keys.indexOf(key);
    const to = from + delta;
    if (from === -1 || to < 0 || to >= keys.length) return;

    const next = [...keys];
    const [moved] = next.splice(from, 1);
    if (moved) next.splice(to, 0, moved);
    onReorder(next);
  };

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
              <button
                type="button"
                aria-label={t("view.moveColumn")}
                title={t("view.moveColumn")}
                onKeyDown={(event) => {
                  if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
                  // Иначе стрелка прокрутит список под панелью.
                  event.preventDefault();
                  move(key, event.key === "ArrowUp" ? -1 : 1);
                }}
                className="grid size-5 shrink-0 cursor-grab place-items-center rounded text-fg-subtle transition-colors hover:text-fg"
              >
                <Icon as={IconGripVertical} size={14} />
              </button>
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
 * Раскладки, на которые можно переключить view.
 *
 * Только те, которые мы действительно рисуем, — предлагать доску,
 * календарь и сводную, за которыми стоит «экран не готов», значит
 * запирать view: панель, из которой тип меняют, исчезает вместе
 * с таблицей. Плюс текущий тип: view, созданный доской в старой
 * админке, иначе некуда вернуть.
 */
function switchableTypes(view: View): string[] {
  /* У вкладки связи набор свой и короче: дерево отбор по связи не понимает
     (см. TAB_VIEW_TYPES) и показало бы всю чужую таблицу. */
  const allowed = view.isRelationView ? new Set(TAB_VIEW_TYPES) : IMPLEMENTED_VIEW_TYPES;

  return VIEW_TYPES.filter((type) => allowed.has(type) || type === view.type);
}

/**
 * Выбранные поля вперёд и в порядке уровней, остальные — как были.
 * Список, в котором галочки разбросаны, не показывает вложенность.
 */
function order(fields: Field[], selected: string[]): Field[] {
  const chosen = selected
    .map((id) => fields.find((field) => columnKey(field) === id))
    .filter((field): field is Field => Boolean(field));

  const rest = fields.filter((field) => !selected.includes(columnKey(field)));
  return [...chosen, ...rest];
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
 * Переключается СПИСОК полей, а не одно: у мультиязычного поля это все
 * языковые варианты сразу. Показать один вариант из трёх — значит
 * оставить колонку без языковой группы, и в таблице она подпишется
 * «Название (cyr)» вместо «Название».
 *
 * При скрытии убираются ОБА ключа поля-связи — и id поля, и id связи.
 * Бэкенд при создании view кладёт в columns оба (view.go, INSERT), и
 * колонка, снятая по одному ключу, продолжает находиться по второму:
 * в старой админке галочка снималась, а колонка оставалась.
 *
 * Новая колонка встаёт в конец: у view нет «правильного места» для неё,
 * а вставка в середину переставила бы соседние без спроса.
 */
function toggleColumn(view: View, group: Field[], visible: boolean): string[] {
  const keys = new Set(
    group.flatMap((field) => [field.id, ...(field.relationId ? [field.relationId] : [])]),
  );
  const rest = view.columnIds.filter((id) => !keys.has(id));

  return visible ? [...rest, ...group.map(columnKey)] : rest;
}

/** Плейсхолдер один на все три адреса: запись подстановки у них общая. */
const URL_PLACEHOLDER = "/order/{{$guid}}";

/**
 * Адрес и его параметры запроса.
 *
 * Параметры — отдельным списком, а не строкой после «?», потому что
 * так их хранит бэкенд и так их правит старая админка: ключ и значение
 * порознь, оба — шаблоны.
 *
 * Правка уходит целиком: адрес и параметры — одно значение в attributes,
 * и отправлять их по одному значило бы дважды перезаписывать соседа.
 */
function UrlSetting({
  label,
  hint,
  template,
  onChange,
}: {
  label: string;
  hint: string;
  template: UrlTemplate;
  onChange: (template: UrlTemplate) => void;
}) {
  const { t } = useTranslation();

  const setParam = (index: number, patch: Partial<{ key: string; value: string }>) =>
    onChange({
      ...template,
      params: template.params.map((param, at) => (at === index ? { ...param, ...patch } : param)),
    });

  return (
    <div className="flex flex-col gap-1">
      <span className="px-0.5 text-2xs text-fg-muted">{label}</span>

      {/* Пустой адрес — это значение: «открывать карточку, как обычно». */}
      <CommitInput
        value={template.url}
        placeholder={URL_PLACEHOLDER}
        label={label}
        allowEmpty
        onCommit={(url) => onChange({ ...template, url })}
      />

      <span className="px-0.5 text-2xs text-fg-subtle">{hint}</span>

      {template.params.map((param, index) => (
        <div key={index} className="flex items-center gap-1">
          <CommitInput
            value={param.key}
            placeholder={t("view.paramKey")}
            label={t("view.paramKey")}
            onCommit={(key) => setParam(index, { key })}
          />
          <CommitInput
            value={param.value}
            placeholder={t("view.paramValue")}
            label={t("view.paramValue")}
            allowEmpty
            onCommit={(value) => setParam(index, { value })}
          />

          <button
            type="button"
            onClick={() =>
              onChange({ ...template, params: template.params.filter((_, at) => at !== index) })
            }
            aria-label={t("action.delete")}
            title={t("action.delete")}
            className="grid size-7 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
          >
            <Icon as={IconTrash} size={14} />
          </button>
        </div>
      ))}

      {/* Пустая строка добавляется на месте и уезжает только заполненной:
          параметр без ключа отбрасывается при записи. */}
      <BulkButton
        label={t("view.addParam")}
        onClick={() => onChange({ ...template, params: [...template.params, { key: "", value: "" }] })}
      />
    </div>
  );
}
