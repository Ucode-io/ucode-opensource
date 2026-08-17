import { useEffect, useRef, useState } from "react";
import {
  IconArrowNarrowDown,
  IconArrowNarrowUp,
  IconArrowsDiagonal,
  IconCheck,
  IconDotsVertical,
  IconPlus,
  IconTablePlus,
  IconX,
} from "@tabler/icons-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useTranslation } from "react-i18next";
import { localized, type Field, type Relation } from "@/features/table";
import { toast } from "@/shared/lib/toast";
import { Checkbox } from "@/shared/ui/checkbox";
import { Icon } from "@/shared/ui/icon";
import { editorKind } from "../model/cell-kind";
import type { Sort, SortDirection } from "../model/query";
import { relationDataKey, type Item } from "../model/types";
import { rowErrors, type CellError } from "../model/validate";
import { ActiveCell } from "./CellEditor";
import { Cell } from "./Cell";
import { ColumnMenu, type ColumnActions } from "./ColumnMenu";
import { fieldIcon } from "./field-icon";

/**
 * Таблица строк.
 *
 * Настоящий <table>, а не сетка из div'ов: липкая шапка, фиксированные
 * ширины колонок и доступность с клавиатуры достаются даром.
 *
 * Виртуализация — на строках: в DOM живут только видимые плюс запас,
 * остальное занимают две распорки сверху и снизу. Ширины при этом
 * остаются на <colgroup>, поэтому колонки не разъезжаются, когда
 * в видимой части оказались короткие значения.
 */

/** Ширины до resize'а. Первая колонка шире: в ней обычно имя записи. */
const FIRST_WIDTH = 240;
const WIDTH = 180;
const PIN_WIDTH = 40;

/**
 * Высота строки. Дублирует токен --spacing-row (класс h-row): виртуализатор
 * считает смещения в JS и прочитать CSS-переменную не может. Значения
 * обязаны совпадать — иначе строки поедут относительно прокрутки.
 */
const ROW_HEIGHT = 36;

/*
 * overflow-hidden обязателен: table-fixed задаёт ширину колонки, но
 * длинное значение всё равно вылезает поверх соседей — ссылка на файл
 * растягивала строку на весь экран.
 */
const cellBase = "h-row overflow-hidden border-b border-border text-sm";
const cell = `${cellBase} px-2`;

/* Закреплённые колонки узкие: их содержимое центрируется, а не отступает. */
const pinCell = `${cellBase} p-0`;

/*
 * Закреплённые крайние колонки. Фон непрозрачный: под ними проезжают
 * ячейки, и без него текст накладывается на текст.
 *
 * Граница нарисована тенью, а не border: у sticky-ячейки собственная
 * граница уезжает вместе с прокруткой на пиксель и мерцает.
 */
const pinLeft = "sticky left-0 z-10 bg-surface shadow-[1px_0_0_0_var(--color-border)]";
const pinRight = "sticky right-0 z-10 bg-surface shadow-[-1px_0_0_0_var(--color-border)]";

/** Общий пустой набор: без него у DataGrid на каждый рендер новый Set. */
const EMPTY_PINS: ReadonlySet<string> = new Set<string>();

/** Какая ячейка раскрыта. Одна на таблицу: двух курсоров не бывает. */
type Active = { index: number; slug: string; anchor: DOMRect };

/**
 * Индекс черновика новой строки. Отрицательный, потому что в `rows` его
 * нет: строка ещё не существует, а раскрытая ячейка адресуется индексом.
 */
const DRAFT = -1;

/** Пустой набор ошибок: без него у грида без черновика новая Map на рендер. */
const NO_ERRORS: ReadonlyMap<string, CellError> = new Map();

/** Открытое меню колонки. Тоже одно: оно всплывает поверх таблицы. */
type Menu = { slug: string; anchor: DOMRect };

/**
 * Закреплённые колонки: сдвигаются влево и остаются на месте при
 * прокрутке вбок.
 *
 * Ключевое — «сдвигаются». Липкой можно сделать только колонку, левее
 * которой ничего не прокручивается, поэтому закреплённые собираются
 * в начало ряда, а не подсвечиваются на своих местах. Их взаимный
 * порядок при этом остаётся порядком view: закрепление — не сортировка.
 *
 * Смещения считаются здесь же: sticky нужен точный left в пикселях,
 * а он складывается из ширины колонки с флажками и ширин всех
 * закреплённых слева.
 */
function pinLayout(columns: Field[], pinned: ReadonlySet<string>) {
  if (!pinned.size) return { ordered: columns, lefts: new Map<string, number>(), count: 0 };

  const front = columns.filter((column) => pinned.has(column.id));
  const rest = columns.filter((column) => !pinned.has(column.id));
  const ordered = [...front, ...rest];

  const lefts = new Map<string, number>();
  let left = PIN_WIDTH;

  ordered.slice(0, front.length).forEach((column, index) => {
    lefts.set(column.id, left);
    left += index === 0 ? FIRST_WIDTH : WIDTH;
  });

  return { ordered, lefts, count: front.length };
}

export function DataGrid({
  tableSlug,
  columns,
  pinned,
  rows,
  relations,
  locale,
  language,
  selected,
  onSelect,
  sorts,
  onSort,
  onAddField,
  columnActions,
  onOpenRow,
  onEdit,
  onCreate,
  onAddRow,
  creating,
}: {
  /** Слаг таблицы: ячейка-связь пишет не только в свою строку. */
  tableSlug: string;
  columns: Field[];
  /** id закреплённых колонок. Пусто — обычная таблица. */
  pinned?: ReadonlySet<string>;
  rows: Item[];
  relations: Relation[];
  /** Локаль интерфейса: форматы дат и чисел. */
  locale: string;
  /** Язык данных: подписи полей и вариантов. */
  language: string;
  /** guid отмеченных строк. */
  selected: ReadonlySet<string>;
  onSelect: (next: Set<string>) => void;
  sorts: Sort[];
  /** Без направления — клик по заголовку: вверх, вниз, никак. */
  onSort: (field: string, direction?: SortDirection) => void;
  /** Кнопка «+» в шапке. Панель нового поля всплывает под ней — отсюда якорь. */
  onAddField?: (anchor: DOMRect) => void;
  /** Действия над колонкой. Нет — меню в шапке не появляется. */
  columnActions?: ColumnActions;
  /** Раскрыть строку целиком. Нет — кнопка в строке не появляется. */
  onOpenRow?: (guid: string) => void;
  /**
   * Своё действие вместо строки-черновика: админ мог задать view адрес
   * собственной формы создания (attributes.url_object). Не задан —
   * строка заводится на месте.
   */
  onAddRow?: (() => void) | undefined;
  /** Без обработчика таблица только читается: ячейка раскрывается, но не правится. */
  onEdit?: (guid: string, slug: string, value: unknown) => void;
  /**
   * Создать строку. `done` закрывает черновик — зовётся только после
   * ответа сервера: строка, исчезнувшая с экрана до того, как её приняли,
   * при отказе уносит с собой всё набранное.
   *
   * Нет обработчика — нет и строки «новая запись».
   */
  onCreate?: (values: Item, done: () => void) => void;
  /** Запрос создания в пути: второе нажатие завело бы вторую строку. */
  creating?: boolean;
}) {
  const { t } = useTranslation();

  const scroller = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState<Active | null>(null);
  const [menu, setMenu] = useState<Menu | null>(null);

  /**
   * Черновик новой строки — та же строка таблицы, только её ещё нет
   * в базе. Правится теми же редакторами (ActiveCell), поэтому и хранится
   * в той же форме: у поля один способ правки, где бы его ни открыли.
   *
   * guid придумывает клиент, а не сервер (см. api/relations): редакторы
   * без первичного ключа не открываются, а ждать его от вставки — значит
   * не дать заполнить строку до её создания.
   */
  const [draft, setDraft] = useState<Item | null>(null);
  /**
   * Показывать ли ошибки. Не с первого нажатия: у новой строки пусты
   * все обязательные поля сразу, и красный ряд в ответ на «создать
   * запись» — это выговор за то, чего человек ещё не делал.
   */
  const [showErrors, setShowErrors] = useState(false);

  const errors = draft ? rowErrors(columns, draft) : NO_ERRORS;

  /** Сообщение админа важнее нашего: он писал его про конкретное поле. */
  const errorText = (error: CellError) =>
    error.message || t(error.kind === "required" ? "cell.required" : "cell.invalid");

  const patchDraft = (values: Item) =>
    setDraft((current) => (current ? { ...current, ...values } : current));

  const startDraft = () => {
    setDraft({ guid: crypto.randomUUID() });
    setShowErrors(false);
  };

  const cancelDraft = () => {
    setDraft(null);
    setActive(null);
  };

  /**
   * Отправка черновика. Не прошло проверку — не отправляем и говорим
   * чем именно: незаполненная колонка бывает уехавшей за край экрана,
   * и красной рамки, которой не видно, недостаточно.
   */
  const submitDraft = () => {
    if (!draft || !onCreate || creating) return;

    const [first] = [...errors];
    if (first) {
      const [slug, error] = first;
      const field = columns.find((column) => column.slug === slug);
      const label = field ? localized(field.labels, language, field.label) : slug;

      setShowErrors(true);
      toast.error(`${label}: ${errorText(error)}`);
      return;
    }

    /*
     * Сопровождающая запись связи (`<слаг>_data`) остаётся на экране,
     * но не уезжает: это не колонка таблицы, а то, что бэкенд дописывает
     * к ответу. Вставка по несуществующей колонке — 500.
     */
    const values: Item = { ...draft };
    for (const column of columns) {
      if (editorKind(column) === "relation") delete values[relationDataKey(column.slug)];
    }

    setActive(null);
    onCreate(values, () => setDraft(null));
  };

  // Связи ищутся по id на каждой ячейке-ссылке — держим индексом.
  const byId = new Map(relations.map((relation) => [relation.id, relation]));

  const { ordered, lefts, count: pinnedCount } = pinLayout(columns, pinned ?? EMPTY_PINS);

  const ids = rows.map(rowKey);
  const checked = ids.filter((id) => selected.has(id));
  const allChecked = ids.length > 0 && checked.length === ids.length;

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scroller.current,
    estimateSize: () => ROW_HEIGHT,
    // Запас сверху и снизу: без него при быстрой прокрутке видно пустоту
    // раньше, чем React успевает дорисовать строки.
    overscan: 10,
  });

  const visible = virtualizer.getVirtualItems();
  const first = visible[0];
  const last = visible[visible.length - 1];
  const before = first ? first.start : 0;
  const after = last ? virtualizer.getTotalSize() - last.end : 0;

  /** Столбцов в строке — для распорок, у которых своих ячеек нет. */
  const span = columns.length + 2;

  /**
   * «Выделить всё» — только про загруженную страницу. Отметить строки,
   * которых на экране нет, значит удалить их вслепую.
   */
  const toggleAll = () => {
    const next = new Set(selected);
    if (allChecked) ids.forEach((id) => next.delete(id));
    else ids.forEach((id) => next.add(id));
    onSelect(next);
  };

  const toggleRow = (id: string) => {
    const next = new Set(selected);
    if (!next.delete(id)) next.add(id);
    onSelect(next);
  };

  /**
   * Клик по ячейке. Флажок переключается на месте — ради двух состояний
   * открывать меню незачем; остальное раскрывается поверх таблицы.
   */
  const open = (index: number, field: Field, element: HTMLElement) => {
    const row = index === DRAFT ? draft : rows[index];
    const guid = row?.guid;
    const boolean = editorKind(field) === "boolean";

    if (index === DRAFT) {
      if (boolean) return patchDraft({ [field.slug]: !row?.[field.slug] });
    } else if (row && guid && onEdit && boolean) {
      onEdit(guid, field.slug, !row[field.slug]);
      return;
    }

    setActive({ index, slug: field.slug, anchor: element.getBoundingClientRect() });
  };

  const activeField = active ? columns.find((column) => column.slug === active.slug) : undefined;
  const menuField = menu ? columns.find((column) => column.slug === menu.slug) : undefined;
  const activeDraft = active?.index === DRAFT;
  const activeRow = active ? (activeDraft ? draft : rows[active.index]) : undefined;
  // Черновик правится всегда: его guid для того и придуман заранее.
  const activeGuid = activeDraft || onEdit ? activeRow?.guid : undefined;

  /*
   * Новая страница, другой фильтр, другая сортировка — таблица
   * возвращается наверх. Без этого прокрутка остаётся на прежнем месте,
   * и вторая страница открывается с середины. Обновление тех же строк
   * (после правки ячейки) первую строку не меняет и прокрутку не трогает.
   */
  const topId = ids[0];
  useEffect(() => {
    scroller.current?.scrollTo({ top: 0 });
  }, [topId]);

  /*
   * Enter отправляет черновик, Escape закрывает его — но только когда
   * поверх него ничего нет: у открытого редактора свой Enter («применить»)
   * и свой Escape («отменить правку»), и одно нажатие не должно делать
   * оба действия сразу.
   *
   * Без списка зависимостей намеренно: обработчик держит в себе черновик
   * целиком, и перечислять то, из чего он собран, — способ однажды
   * отправить позавчерашнее значение.
   */
  useEffect(() => {
    if (!draft || active) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Enter") submitDraft();
      if (event.key === "Escape") cancelDraft();
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  });

  return (
    <div ref={scroller} className="min-h-0 flex-1 overflow-auto">
      {/*
        w-full растягивает таблицу на всю ширину, min-w-max не даёт ей
        сжаться уже содержимого. Слабину забирает ПОСЛЕДНЯЯ колонка —
        ей одной не задана ширина, и table-fixed отдаёт ей всё, что
        осталось. Раньше слабину забирала отдельная колонка-распорка,
        и на экране она читалась как лишний пустой столбец в конце
        таблицы — с рамками, но без заголовка и без содержимого.
      */}
      <table className="w-full min-w-max table-fixed border-separate border-spacing-0">
        <colgroup>
          <col style={{ width: PIN_WIDTH }} />
          {ordered.map((column, index) => (
            /*
             * Последней колонке ширина не задаётся: она растягивается
             * на всё свободное место, и таблица из двух полей занимает
             * экран целиком, а не жмётся узкой полосой у левого края.
             *
             * Когда колонок больше, чем влезает, растягивать нечего —
             * min-w-max держит их естественную ширину, и последняя
             * ведёт себя как остальные.
             */
            <col
              key={column.id}
              {...(index === ordered.length - 1
                ? {}
                : { style: { width: index === 0 ? FIRST_WIDTH : WIDTH } })}
            />
          ))}

          <col style={{ width: PIN_WIDTH }} />
        </colgroup>

        {/* Шапка липкая: колонки нужны и на тысячной строке. */}
        <thead className="sticky top-0 z-20 bg-surface">
          <tr>
            <th className={`${pinCell} ${pinLeft} z-30`}>
              <span className="grid h-full place-items-center">
                <Checkbox
                  checked={allChecked}
                  indeterminate={checked.length > 0 && !allChecked}
                  onChange={toggleAll}
                  aria-label={t("table.selectAll")}
                  disabled={!ids.length}
                />
              </span>
            </th>

            {ordered.map((column, index) => (
              <HeaderCell
                key={column.id}
                column={column}
                language={language}
                sorts={sorts}
                left={lefts.get(column.id)}
                lastPinned={index === pinnedCount - 1}
                last={index === ordered.length - 1}
                onSort={onSort}
                onMenu={
                  columnActions
                    ? (element) =>
                        setMenu({ slug: column.slug, anchor: element.getBoundingClientRect() })
                    : undefined
                }
              />
            ))}

            <th className={`${pinCell} ${pinRight} z-30`}>
              <span className="grid h-full place-items-center">
                <button
                  type="button"
                  onClick={(event) => onAddField?.(event.currentTarget.getBoundingClientRect())}
                  disabled={!onAddField}
                  aria-label={t("table.addField")}
                  title={t("table.addField")}
                  className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg disabled:pointer-events-none disabled:opacity-40"
                >
                  <Icon as={IconTablePlus} />
                </button>
              </span>
            </th>
          </tr>
        </thead>

        <tbody>
          {/* Распорки вместо невидимых строк: одна ячейка нужной высоты
              дешевле тысячи <tr> и не ломает ни ширины, ни прокрутку. */}
          {before > 0 && <Spacer height={before} span={span} />}

          {visible.map((virtual) => {
            const index = virtual.index;
            const row = rows[index]!;
            const id = ids[index]!;
            const isSelected = selected.has(id);

            /*
             * Фон отмеченной строки задаётся и на закреплённых ячейках:
             * у них собственный непрозрачный фон, и подсветка строки
             * из-под них не видна.
             */
            const pinBg = isSelected ? "bg-accent-subtle" : "bg-surface";

            return (
              <tr
                key={id}
                className={`group/row ${isSelected ? "bg-accent-subtle" : "hover:bg-surface-hover"}`}
              >
                <td className={`${pinCell} ${pinLeft} ${pinBg}`}>
                  <span className="grid h-full place-items-center">
                    <Checkbox
                      checked={isSelected}
                      onChange={() => toggleRow(id)}
                      aria-label={t("table.selectRow")}
                    />
                  </span>
                </td>

                {ordered.map((column, columnIndex) => {
                  const isActive = active?.index === index && active.slug === column.slug;
                  const left = lefts.get(column.id);

                  return (
                    <td
                      key={column.id}
                      onClick={(event) => open(index, column, event.currentTarget)}
                      /* Раскрытая ячейка подсвечивается: карточка редактора
                         накрывает её не целиком, когда та шире колонки.

                         У закреплённой фон обязателен и здесь: под ней
                         проезжают чужие ячейки, и прозрачная показала бы
                         текст поверх текста. */
                      style={left === undefined ? undefined : { left }}
                      className={`${cell} cursor-default border-r ${
                        isActive ? "bg-accent-subtle" : ""
                      } ${
                        left === undefined
                          ? ""
                          : `sticky z-10 ${isActive ? "" : pinBg} ${
                              columnIndex === pinnedCount - 1
                                ? "shadow-[1px_0_0_0_var(--color-border)]"
                                : ""
                            }`
                      }`}
                    >
                      <span className="flex h-full min-w-0 items-center">
                        <Cell
                          field={column}
                          row={row}
                          tableSlug={tableSlug}
                          relations={byId}
                          locale={locale}
                          language={language}
                        />
                      </span>
                    </td>
                  );
                })}

                {/* Раскрыть строку. Кнопка появляется по наведению:
                    значок в каждой строке — рябь на весь экран. */}
                <td className={`${pinCell} ${pinRight} ${pinBg}`}>
                  {onOpenRow && (
                    <span className="grid h-full place-items-center">
                      <button
                        type="button"
                        onClick={() => onOpenRow(id)}
                        aria-label={t("drawer.open")}
                        title={t("drawer.open")}
                        className="hidden size-6 place-items-center rounded-md text-fg-subtle transition-colors group-hover/row:grid hover:bg-surface-active hover:text-fg"
                      >
                        <Icon as={IconArrowsDiagonal} size={14} />
                      </button>
                    </span>
                  )}
                </td>
              </tr>
            );
          })}

          {after > 0 && <Spacer height={after} span={span} />}

          {/*
            Черновик новой строки — последней, там же, где её создали.
            Он вне виртуализации: строка одна, и уезжать за пределы окна
            прокрутки вместе с данными она не должна.
          */}
          {draft && (
            <tr className="bg-surface">
              <td className={`${pinCell} ${pinLeft}`}>
                <span className="grid h-full place-items-center">
                  <button
                    type="button"
                    onClick={cancelDraft}
                    aria-label={t("action.cancel")}
                    title={t("action.cancel")}
                    className="grid size-6 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
                  >
                    <Icon as={IconX} size={14} />
                  </button>
                </span>
              </td>

              {ordered.map((column, columnIndex) => {
                const isActive = activeDraft && active?.slug === column.slug;
                const left = lefts.get(column.id);
                const problem = showErrors ? errors.get(column.slug) : undefined;

                return (
                  <td
                    key={column.id}
                    onClick={(event) => open(DRAFT, column, event.currentTarget)}
                    /* Полный текст подсказкой: в углу ячейки шириной
                       180 пикселей длинное сообщение админа обрезано. */
                    title={problem ? errorText(problem) : undefined}
                    style={left === undefined ? undefined : { left }}
                    /* Рамка внутрь (-outline-offset), а не border: своя
                       граница у ячейки уже занята сеткой таблицы, а
                       outline рисуется поверх неё и ничего не сдвигает. */
                    className={`${cell} relative cursor-default border-r ${
                      problem ? "outline-1 -outline-offset-1 outline-danger" : ""
                    } ${isActive ? "bg-accent-subtle" : ""} ${
                      left === undefined
                        ? ""
                        : `sticky z-10 ${isActive ? "" : "bg-surface"} ${
                            columnIndex === pinnedCount - 1
                              ? "shadow-[1px_0_0_0_var(--color-border)]"
                              : ""
                          }`
                    }`}
                  >
                    <span className="flex h-full min-w-0 items-center">
                      <Cell
                        field={column}
                        row={draft}
                        tableSlug={tableSlug}
                        relations={byId}
                        locale={locale}
                        language={language}
                      />
                    </span>

                    {/* Что именно не так — в углу самой ячейки: строка
                        высотой 36px не даёт места под подпись снизу, а
                        значение под текстом ошибки прикрыто фоном. */}
                    {problem && (
                      <span className="pointer-events-none absolute right-0 bottom-0 max-w-full truncate bg-surface px-1 text-2xs leading-tight text-danger">
                        {errorText(problem)}
                      </span>
                    )}
                  </td>
                );
              })}

              <td className={`${pinCell} ${pinRight}`}>
                <span className="grid h-full place-items-center">
                  <button
                    type="button"
                    onClick={submitDraft}
                    disabled={creating}
                    aria-label={t("table.saveRow")}
                    title={t("table.saveRow")}
                    className="grid size-6 place-items-center rounded-md text-accent-text transition-colors hover:bg-accent-subtle disabled:opacity-40"
                  >
                    <Icon as={IconCheck} size={14} />
                  </button>
                </span>
              </td>
            </tr>
          )}

          {/* Кнопка новой строки — под последней, а не в шапке: строку
              дописывают в конец списка, туда же и смотрят. */}
          {onCreate && !draft && (
            <tr className="group/add hover:bg-surface-hover">
              <td colSpan={span} className="border-b border-border p-0">
                <button
                  type="button"
                  onClick={onAddRow ?? startDraft}
                  /* Кнопка липнет к левому краю: у таблицы шире экрана
                     она иначе уезжает из виду вместе с первой колонкой. */
                  className="sticky left-0 flex h-row items-center gap-1.5 px-3 text-sm text-fg-subtle transition-colors group-hover/add:text-fg"
                >
                  <Icon as={IconPlus} size={14} />
                  {t("table.addRow")}
                </button>
              </td>
            </tr>
          )}
        </tbody>
      </table>

      {menu && menuField && columnActions && (
        <ColumnMenu
          key={menu.slug}
          field={menuField}
          language={language}
          anchor={menu.anchor}
          actions={columnActions}
          onSort={onSort}
          onClose={() => setMenu(null)}
        />
      )}

      {active && activeField && activeRow && (
        <ActiveCell
          // Ключ по строке и полю: перенос на соседнюю ячейку — это
          // другой редактор с другим черновиком, а не тот же самый.
          key={`${active.index}:${active.slug}`}
          field={activeField}
          row={activeRow}
          guid={activeGuid}
          tableSlug={tableSlug}
          anchor={active.anchor}
          relations={byId}
          locale={locale}
          language={language}
          // «Настроить поле» прямо из раскрытой ячейки: варианты
          // статуса правят, глядя на список, а не на схему таблицы.
          onSettings={columnActions?.settings}
          onEdit={(value) => {
            if (activeDraft) patchDraft({ [active.slug]: value });
            else if (activeGuid) onEdit?.(activeGuid, active.slug, value);
          }}
          /*
           * У черновика связь не уезжает запросом: строки в базе ещё нет.
           * Выбранная запись ложится рядом со ссылкой (`<слаг>_data`) —
           * из неё ячейка и берёт, что показать вместо uuid.
           */
          onLink={
            activeDraft
              ? (item) =>
                  patchDraft({
                    [active.slug]: item?.guid ?? null,
                    [relationDataKey(active.slug)]: item,
                  })
              : undefined
          }
          onClose={() => setActive(null)}
        />
      )}
    </div>
  );
}

function Spacer({ height, span }: { height: number; span: number }) {
  return (
    <tr aria-hidden>
      <td colSpan={span} className="p-0" style={{ height }} />
    </tr>
  );
}

/**
 * Скелетон таблицы.
 *
 * Геометрия та же, что у настоящей: те же ширины колонок, та же высота
 * строки, та же липкая шапка. Поэтому в момент, когда приедут данные,
 * ничего не прыгает — плашки просто заменяются текстом.
 *
 * Слово «Загрузка…» посреди пустого экрана не говорит ни сколько ждать,
 * ни что появится; полосы говорят и то, и другое.
 */
export function GridSkeleton({ columns = 5, rows = 14 }: { columns?: number; rows?: number }) {
  // Разная длина плашек: одинаковые читаются как разметка, а не как текст.
  const widths = [70, 45, 60, 85, 55, 75];

  return (
    <div className="min-h-0 flex-1 overflow-hidden" aria-hidden>
      <table className="w-full min-w-max table-fixed border-separate border-spacing-0">
        <colgroup>
          <col style={{ width: PIN_WIDTH }} />
          {Array.from({ length: columns }, (_, index) => (
            <col key={index} style={{ width: index === 0 ? FIRST_WIDTH : WIDTH }} />
          ))}
          <col />
          <col style={{ width: PIN_WIDTH }} />
        </colgroup>

        <thead className="sticky top-0 z-20 bg-surface">
          <tr>
            <th className={`${pinCell} ${pinLeft} z-30`} />
            {Array.from({ length: columns }, (_, index) => (
              <th key={index} className={cell}>
                <SkeletonBar width={widths[index % widths.length]! - 15} />
              </th>
            ))}
            <th className={cell} />
            <th className={`${pinCell} ${pinRight} z-30`} />
          </tr>
        </thead>

        <tbody>
          {Array.from({ length: rows }, (_, row) => (
            <tr key={row}>
              <td className={`${pinCell} ${pinLeft} bg-surface`} />
              {Array.from({ length: columns }, (_, index) => (
                <td key={index} className={cell}>
                  <SkeletonBar width={widths[(row + index) % widths.length]!} />
                </td>
              ))}
              <td className={cell} />
              <td className={`${pinCell} ${pinRight} bg-surface`} />
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SkeletonBar({ width }: { width: number }) {
  return <div className="h-3 rounded-sm bg-surface-active" style={{ width: `${width}%` }} />;
}

function HeaderCell({
  column,
  language,
  sorts,
  left,
  lastPinned,
  last,
  onSort,
  onMenu,
}: {
  column: Field;
  language: string;
  sorts: Sort[];
  /** Отступ слева у закреплённой колонки. undefined — колонка обычная. */
  left?: number | undefined;
  /** Последняя закреплённая: только у неё рисуется граница-тень. */
  lastPinned?: boolean;
  /**
   * Последняя колонка таблицы: ей одной не задана ширина, и она
   * забирает свободное место. Пола нет — при узкой таблице она стала бы
   * шириной по своей подписи, то есть уже соседей.
   */
  last?: boolean;
  onSort: (field: string) => void;
  onMenu?: ((element: HTMLElement) => void) | undefined;
}) {
  const { t } = useTranslation();
  const active = sorts.find((sort) => sort.field === column.slug);

  return (
    <th
      style={left === undefined ? undefined : { left }}
      /* z-30, а не 20: шапка целиком липкая сверху, и закреплённая
         ячейка обязана оказаться выше проезжающих под ней соседей. */
      className={`${cell} group/head border-r text-left font-normal ${
        left === undefined
          ? ""
          : `sticky z-30 bg-surface ${lastPinned ? "shadow-[1px_0_0_0_var(--color-border)]" : ""}`
      }`}
    >
      {/*
        Пол последней колонки задаётся здесь, а не в colgroup: у таблицы
        с table-fixed ширина колонки от содержимого не зависит, но
        min-w-max самой таблицы считает как раз по содержимому — и
        распорка в заголовке до него доходит.
      */}
      <span className={`flex h-full items-center ${last ? "min-w-[164px]" : "min-w-0"}`}>
        <button
          type="button"
          onClick={() => onSort(column.slug)}
          className="flex h-full min-w-0 flex-1 items-center gap-1.5 text-fg-muted transition-colors hover:text-fg"
        >
          <Icon as={fieldIcon(column.type)} size={14} />
          <span className="truncate">{localized(column.labels, language, column.label)}</span>

          {/* Стрелка только у сортированной колонки: значок «можно
              сортировать» на каждом заголовке — это шум в плотной шапке. */}
          {active && (
            <Icon
              as={active.direction === "asc" ? IconArrowNarrowUp : IconArrowNarrowDown}
              size={14}
              className="text-accent-text"
            />
          )}
        </button>

        {/* Кнопка меню появляется по наведению: в шапке из десяти колонок
            десять одинаковых значков — это рябь, а не подсказка. */}
        {onMenu && (
          <button
            type="button"
            onClick={(event) => onMenu(event.currentTarget)}
            aria-label={t("column.menu")}
            className="ml-1 hidden size-6 shrink-0 place-items-center rounded-md text-fg-muted transition-colors group-hover/head:grid hover:bg-surface-active hover:text-fg"
          >
            <Icon as={IconDotsVertical} size={14} />
          </button>
        )}
      </span>
    </th>
  );
}

/** Первичный ключ в ucode — guid. Индекс только на случай его отсутствия. */
function rowKey(row: Item, index = 0): string {
  return typeof row.guid === "string" && row.guid ? row.guid : String(index);
}
