import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type PointerEvent,
  type ReactNode,
} from "react";
import { IconChevronLeft, IconChevronRight, IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Field, Relation } from "@/features/table";
import { toast } from "@/shared/lib/toast";
import { CHIP_STYLES, CHIP_SURFACE, hexToChipColor, type ChipColor } from "@/shared/ui/chip";
import { Icon } from "@/shared/ui/icon";
import { Tabs } from "@/shared/ui/tabs";
import { Tooltip } from "@/shared/ui/tooltip";
import {
  CALENDAR_PERIODS,
  SLOT_MINUTES,
  addDays,
  calendarEvents,
  dateKind,
  dateValues,
  dayKey,
  daySlots,
  daysBetween,
  eventsByDay,
  movedTo,
  placeDay,
  shiftPeriod,
  startOfDay,
  startOfWeek,
  timeAt,
  weekSegments,
  weeksBetween,
  type CalendarEvent,
  type CalendarPeriod,
} from "../model/calendar";
import { isBlank } from "../model/cell-value";
import type { Item } from "../model/types";
import { Cell } from "./Cell";
import { fieldIcon } from "./field-icon";

/**
 * Календарь: те же строки, разложенные по дням и часам.
 *
 * Своих запросов нет — строки приезжают обычным get-list с отбором
 * по видимому диапазону, а перенос и растягивание события это обычный
 * PUT двух полей. Здесь только показ: раскладка считается в model/calendar,
 * значения рисуются теми же ячейками, что и в таблице.
 *
 * Период и видимый день приходят снаружи и живут в адресе: диапазон
 * задаёт отбор строк, а ссылка на «эту неделю» обязана открывать
 * ту же неделю.
 */

/** Высота часа в сетке DAY и WEEK. Сутки при ней — четыре экрана. */
const SLOT_HEIGHT = 48;

/** Полоса дней недели над сеткой месяца. */
const HEADER_HEIGHT = 28;

/** Сколько заполненных колонок показывает карточка месяца. */
const MAX_CARD_FIELDS = 5;

/** Ближе этого к краю ленты — просим следующие месяцы. */
const EDGE_GAP = 300;

/**
 * Сколько миллисекунд после переноса щелчок считается его хвостом,
 * а не отдельным щелчком. Браузер шлёт click сразу за pointerup.
 */
const CLICK_GAP = 300;

export function CalendarView({
  tableSlug,
  columns,
  rows,
  fromField,
  toField,
  statusField,
  relations,
  period,
  cursor,
  rangeFrom,
  rangeTo,
  disabledDays,
  locale,
  language,
  hasMore,
  onPeriod,
  onCursor,
  onOpenRow,
  onMove,
  onCreate,
  onLoadMore,
  onLoadPast,
  onLoadFuture,
}: {
  tableSlug: string;
  /** Колонки view: первой подписано событие, как и на карточке доски. */
  columns: Field[];
  rows: Item[];
  /** Поле начала события. Без него календаря нет вовсе. */
  fromField: Field;
  /** Поле конца. Не задано — событие точкой в дне. */
  toField: Field | undefined;
  /** Поле, вариантами которого красятся события (`status_field_slug`). */
  statusField: Field | undefined;
  relations: Relation[];
  period: CalendarPeriod;
  /** День, вокруг которого построен видимый период. */
  cursor: Date;
  /**
   * Загруженная лента месяца: её же диапазон стоит в отборе строк.
   * В MONTH календарь прокручивается неделями без конца и просит
   * соседние месяцы у вызывающего — он один знает про запрос.
   */
  rangeFrom: Date;
  rangeTo: Date;
  /** Нерабочие дни: ключи «ГГГГ-ММ-ДД». Такие дни не принимают перенос. */
  disabledDays: ReadonlySet<string>;
  locale: string;
  language: string;
  /** Приехали не все строки видимого диапазона. */
  hasMore?: boolean | undefined;
  onPeriod: (period: CalendarPeriod) => void;
  onCursor: (cursor: Date) => void;
  onOpenRow: (guid: string) => void;
  /**
   * Перенос и растягивание. Не задан — календарь только читается: без
   * права на правку рисовать перетаскивание, которое ответит 403, нельзя.
   */
  onMove?: ((guid: string, values: Record<string, unknown>) => void) | undefined;
  /** Новое событие с уже проставленными датами. Не задан — нет права. */
  onCreate?: ((values: Record<string, unknown>) => void) | undefined;
  onLoadMore?: (() => void) | undefined;
  /** Прокрутка дошла до края ленты месяцев. Не задан — лента не растёт. */
  onLoadPast?: (() => void) | undefined;
  onLoadFuture?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  /** Событие в руке. Пусто — ничего не тащат. */
  const [dragging, setDragging] = useState<CalendarEvent | null>(null);
  /**
   * Растягивание за нижний край: событие и его новый конец. Пока тянут,
   * полоса рисуется этим концом — иначе результат виден только после
   * ответа сервера.
   */
  const [resizing, setResizing] = useState<{ event: CalendarEvent; to: Date } | null>(null);
  /**
   * Месяц, который сейчас перед глазами. В ленте недель курсор стоит
   * на месте, а прокрутка уезжает на месяцы вперёд — заголовок, который
   * при этом продолжает показывать месяц курсора, просто врёт.
   */
  const [visible, setVisible] = useState<Date | null>(null);
  /**
   * Куда смотреть в ленте месяцев: день, выравнивание и НОМЕР.
   *
   * Номер обязателен: «сегодня» нажимают, уже стоя в этом месяце, —
   * ни курсор, ни адрес при этом не меняются, и прыгать было бы не по
   * чему. Каждое нажатие увеличивает его, и лента прыгает ровно столько
   * раз, сколько её об этом просили.
   */
  const [focus, setFocus] = useState(() => ({ day: cursor, align: "top" as "top" | "center", id: 0 }));

  /*
   * Курсор сменился снаружи — стрелками или ссылкой: смотрим на его
   * месяц. Условие не для красоты: «сегодня» само ставит день и центр,
   * и без него этот эффект тут же перебивал бы центр верхом.
   */
  useEffect(() => {
    setFocus((current) =>
      dayKey(current.day) === dayKey(cursor)
        ? current
        : { day: cursor, align: "top", id: current.id + 1 },
    );
  }, [cursor]);

  const byId = useMemo(
    () => new Map(relations.map((relation) => [relation.id, relation])),
    [relations],
  );

  const events = useMemo(
    () => calendarEvents(rows, fromField, toField),
    [rows, fromField, toField],
  );
  const byDay = useMemo(() => eventsByDay(events), [events]);

  /** Цвет события — по варианту поля, а не по HEX из данных (см. Chip). */
  const colorOf = (row: Item): ChipColor => {
    const option = statusField?.options.get(String(row[statusField.slug] ?? ""));
    return option?.color ? hexToChipColor(option.color) : "blue";
  };

  /** Записать новые даты — тем же PUT, что и правка ячейки. */
  const write = (guid: string, from: Date, to: Date) => {
    if (!onMove || !dateKind(fromField)) return;

    onMove(guid, dateValues(from, to, fromField, toField));
  };

  /** Бросок на день или на время внутри дня. */
  const drop = (target: Date) => {
    const event = dragging;
    setDragging(null);
    if (!event?.row.guid || disabledDays.has(dayKey(target))) return;

    const moved = movedTo(event, target);
    if (moved.from.getTime() === event.from.getTime()) return;

    write(event.row.guid, moved.from, moved.to);
  };

  /** Полоса месяца: тот же чип, но с обрезанными краями и ручками. */
  const renderBar = (
    event: CalendarEvent,
    segment: { clippedStart: boolean; clippedEnd: boolean },
    handlers: {
      onGrab?: ((ratio: number) => void) | undefined;
      onResize?: ((edge: "start" | "end") => void) | undefined;
      active: boolean;
    },
  ) => (
    <EventChip
      event={event}
      color={colorOf(event.row)}
      columns={columns}
      tableSlug={tableSlug}
      relations={byId}
      locale={locale}
      language={language}
      showTime
      shape="card"
      /* В месяце полосу ведут указателем, а не перетаскиванием браузера:
         только так виден результат до отпускания. */
      draggable={false}
      clippedStart={segment.clippedStart}
      clippedEnd={segment.clippedEnd}
      active={handlers.active}
      {...(handlers.onGrab ? { onGrab: handlers.onGrab } : {})}
      {...(handlers.onResize ? { onResize: handlers.onResize } : {})}
      onDragStart={() => undefined}
      onDragEnd={() => undefined}
      onOpen={() => event.row.guid && onOpenRow(event.row.guid)}
    />
  );

  const chip = (event: CalendarEvent, showTime: boolean) => (
    <EventChip
      event={event}
      color={colorOf(event.row)}
      columns={columns}
      tableSlug={tableSlug}
      relations={byId}
      locale={locale}
      language={language}
      showTime={showTime}
      draggable={Boolean(onMove)}
      /* Обводка на то время, пока событие в руке или его тянут за край:
         иначе непонятно, что именно поедет. */
      active={dragging === event || resizing?.event === event}
      onDragStart={() => setDragging(event)}
      onDragEnd={() => setDragging(null)}
      onOpen={() => event.row.guid && onOpenRow(event.row.guid)}
    />
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
        <div className="flex items-center">
          <button
            type="button"
            onClick={() => onCursor(shiftPeriod(period, cursor, -1))}
            aria-label={t("action.previous")}
            className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconChevronLeft} size={16} />
          </button>
          <button
            type="button"
            onClick={() => onCursor(shiftPeriod(period, cursor, 1))}
            aria-label={t("action.next")}
            className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconChevronRight} size={16} />
          </button>
        </div>

        <button
          type="button"
          /* Сегодня — это и день, и место на экране: лента прокручивается
             так, чтобы текущая неделя оказалась по центру, а не под
             шапкой. */
          onClick={() => {
            const today = new Date();
            onCursor(today);
            setFocus((current) => ({ day: today, align: "center", id: current.id + 1 }));
          }}
          className="h-7 rounded-md border border-border px-2 text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
        >
          {t("calendar.today")}
        </button>

        {/* Подпись периода — капитель: Intl отдаёт месяц строчной буквой
            в русском и узбекском, а это заголовок экрана. */}
        <span className="text-sm font-medium first-letter:uppercase">
          {periodTitle(period, (period === "MONTH" && visible) || cursor, locale)}
        </span>

        {/* Строк в диапазоне больше, чем приехало: сетка показывает
            не всё, и молчать об этом нельзя — на календаре не видно
            ни счётчика, ни номеров страниц. */}
        {hasMore && onLoadMore && (
          <button
            type="button"
            onClick={onLoadMore}
            className="h-7 rounded-md px-2 text-xs text-accent-text transition-colors hover:bg-surface-hover"
          >
            {t("calendar.loadMore")}
          </button>
        )}

        <div className="ml-auto">
          <Tabs
            tabs={CALENDAR_PERIODS.map((item) => ({
              id: item,
              label: t(`calendar.period.${item}` as const),
            }))}
            activeId={period}
            onSelect={(id) => onPeriod(id as CalendarPeriod)}
          />
        </div>
      </div>

      {period === "MONTH" ? (
        <MonthGrid
          cursor={cursor}
          from={rangeFrom}
          to={rangeTo}
          events={events}
          focus={focus}
          disabledDays={disabledDays}
          locale={locale}
          renderEvent={renderBar}
          onVisibleMonth={setVisible}
          /* В месяце времени на экране нет, поэтому перенос меняет
             только день: час события остаётся тем, что был. */
          {...(onMove
            ? {
                onMoveTo: (event: CalendarEvent, start: Date) => {
                  if (!event.row.guid || disabledDays.has(dayKey(start))) return;
                  const target = withTimeOf(start, event);
                  const shifted = movedTo(event, target);
                  write(event.row.guid, shifted.from, shifted.to);
                },
              }
            : {})}
          {...(onLoadPast ? { onLoadPast } : {})}
          {...(onLoadFuture ? { onLoadFuture } : {})}
          {...(onCreate
            ? {
                onCreate: (first: Date, last: Date) =>
                  onCreate(dateValues(first, last, fromField, toField)),
              }
            : {})}
          /*
           * Растягивать нечем, если у view не выбрано поле конца:
           * второй границы у события просто нет. Ручки при этом
           * рисуются всё равно, и попытка потянуть объясняет, чего
           * не хватает, — молчаливо отсутствующая ручка выглядит как
           * поломка, а не как ненастроенный view.
           */
          resizable={Boolean(toField)}
          {...(onMove
            ? {
                onResize: (event: CalendarEvent, edge: "start" | "end", day: Date) => {
                  if (!toField) {
                    toast.error(t("calendar.needEndField"));
                    return;
                  }
                  if (!event.row.guid) return;
                  /* Час не трогаем: в месяце его на экране нет, а
                     растягивание — про дни. */
                  const from = edge === "start" ? withTimeOf(day, event) : event.from;
                  const to = edge === "end" ? withTime(day, event.to) : event.to;

                  if (from.getTime() > to.getTime()) return;
                  write(event.row.guid, from, to);
                },
              }
            : {})}
        />
      ) : (
        <TimeGrid
          days={period === "DAY" ? [startOfDay(cursor)] : weekOf(cursor)}
          byDay={byDay}
          disabledDays={disabledDays}
          locale={locale}
          dragging={Boolean(dragging)}
          resizing={resizing}
          renderEvent={(event) => chip(event, false)}
          onDropAt={drop}
          {...(onCreate
            ? {
                onCreate: (at: Date) =>
                  onCreate(dateValues(at, addMinutes(at, SLOT_MINUTES), fromField, toField)),
              }
            : {})}
          /* Растягивание — только когда есть что растягивать: без поля
             конца у события нет второй границы. */
          {...(onMove && toField
            ? {
                onResize: setResizing,
                onResizeEnd: () => {
                  const current = resizing;
                  setResizing(null);
                  if (current?.event.row.guid) {
                    write(current.event.row.guid, current.event.from, current.to);
                  }
                },
              }
            : {})}
        />
      )}
    </div>
  );
}

/**
 * Сетка месяца: бесконечная лента недель.
 *
 * Не «один месяц со стрелками», а лента: она прокручивается без конца
 * и просит соседние месяцы у вызывающего, как только прокрутка подошла
 * к краю. Так же устроен и старый экран
 * (useCalendarTemplateProps.jsx: addMorePast, addMoreFuture). Стрелки
 * при этом остаются — они прыгают лентой на месяц, а не листают её.
 *
 * События рисуются ПОЛОСАМИ через клетки, а не чипом в каждом дне:
 * командировка с 3-го по 7-е — одно событие, и растянуть его мышью
 * можно только тогда, когда у полосы есть видимый край.
 *
 * Два слоя. Нижний — клетки: фон, число, «+» на наведении, приём
 * броска. Верхний — полосы, и он `pointer-events-none`, чтобы клетки
 * под ним оставались живыми везде, кроме самих полос. Верхний слой
 * при этом лежит в потоке и задаёт высоту строки: недели с пятью
 * событиями выше пустых.
 */
function MonthGrid({
  cursor,
  from,
  to,
  events,
  focus,
  disabledDays,
  locale,
  renderEvent,
  resizable,
  onMoveTo,
  onVisibleMonth,
  onCreate,
  onResize,
  onLoadPast,
  onLoadFuture,
}: {
  cursor: Date;
  /** Границы загруженной ленты. */
  from: Date;
  to: Date;
  events: CalendarEvent[];
  /** Куда смотреть: день, выравнивание и номер просьбы. */
  focus: { day: Date; align: "top" | "center"; id: number };
  disabledDays: ReadonlySet<string>;
  locale: string;
  renderEvent: (
    event: CalendarEvent,
    segment: { clippedStart: boolean; clippedEnd: boolean },
    handlers: {
      /** Взяли полосу за середину: наружу — доля её ширины. */
      onGrab?: ((ratio: number) => void) | undefined;
      onResize?: ((edge: "start" | "end") => void) | undefined;
      /** Эту полосу сейчас тащат: её видно по обводке. */
      active: boolean;
    },
  ) => ReactNode;
  /** Есть ли поле конца. Нет — ручки видно, но тянуть они не дают. */
  resizable: boolean;
  /** Перенос: событие и его новый первый день. Не задан — нет прав. */
  onMoveTo?: ((event: CalendarEvent, start: Date) => void) | undefined;
  /** Месяц, который сейчас перед глазами: им подписан заголовок. */
  onVisibleMonth: (month: Date) => void;
  /** Новая запись на выделенных днях. Не задан — нет права на запись. */
  onCreate?: ((first: Date, last: Date) => void) | undefined;
  /** Новый край события. Не задан — растягивать нечем. */
  onResize?: ((event: CalendarEvent, edge: "start" | "end", day: Date) => void) | undefined;
  onLoadPast?: (() => void) | undefined;
  onLoadFuture?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  const weeks = useMemo(() => weeksBetween(from, to), [from, to]);
  const today = dayKey(new Date());

  /**
   * Что тянут мышью: выделение новых дней или край события.
   *
   * Одно состояние на оба, потому что ведут они себя одинаково: жмут
   * в одной клетке, ведут до другой, отпускают. Разница только в том,
   * что происходит на отпускании.
   */
  const [drag, setDrag] = useState<
    | { kind: "create"; anchor: Date; day: Date }
    | { kind: "resize"; event: CalendarEvent; edge: "start" | "end"; day: Date }
    | { kind: "move"; event: CalendarEvent; anchor: Date; day: Date }
    | null
  >(null);

  /**
   * Когда полосу в последний раз действительно сдвинули. Нужен, чтобы
   * отличить перенос от щелчка: браузер шлёт click после pointerup,
   * а полоса под курсором к этому моменту уже переехала — без развилки
   * каждый перенос заканчивался бы открытой карточкой.
   *
   * Время, а не флаг: click приходит следом или не приходит вовсе
   * (отпустили мимо полосы), и невостребованный флаг съел бы следующий
   * настоящий щелчок.
   */
  const moved = useRef(0);

  /*
   * Отпускание ловится на окне, а не на сетке: кнопку отпускают и за её
   * пределами — на панели, на соседнем экране, — и без этого выделение
   * оставалось бы висеть до следующего щелчка.
   */
  useEffect(() => {
    if (!drag) return;

    const finish = () => {
      setDrag(null);

      if (drag.kind === "resize") {
        const edge = drag.edge === "start" ? drag.event.from : drag.event.to;
        // Край не сдвинули — запрос ничего не изменит.
        if (dayKey(edge) !== dayKey(drag.day)) {
          moved.current = performance.now();
          onResize?.(drag.event, drag.edge, drag.day);
        }
        return;
      }

      if (drag.kind === "move") {
        const shift = daysBetween(drag.anchor, drag.day);
        // Не сдвинули — это был щелчок, и его дело открыть карточку.
        if (shift === 0) return;

        moved.current = performance.now();
        onMoveTo?.(drag.event, addDays(startOfDay(drag.event.from), shift));
        return;
      }

      /*
       * Щелчок по клетке — не событие в один день: для одного дня
       * есть «+», который видно на наведении. Иначе промах мимо
       * полосы каждый раз заводил бы запись.
       */
      if (dayKey(drag.anchor) === dayKey(drag.day)) return;

      const first = drag.anchor <= drag.day ? drag.anchor : drag.day;
      const last = drag.anchor <= drag.day ? drag.day : drag.anchor;
      onCreate?.(first, last);
    };

    window.addEventListener("pointerup", finish);
    return () => window.removeEventListener("pointerup", finish);
  }, [drag, onCreate, onResize, onMoveTo]);

  /*
   * Растягивание видно до ответа сервера: событие подменяется копией
   * с новым краем, и полосы пересчитываются из неё. Отдельного
   * «предпросмотра» поверх сетки нет — это была бы вторая раскладка
   * с теми же правилами.
   */
  const shown = useMemo(() => {
    if (drag?.kind !== "resize" && drag?.kind !== "move") return events;

    return events.map((event) => {
      if (event !== drag.event) return event;

      if (drag.kind === "move") {
        const shift = daysBetween(drag.anchor, drag.day);
        return { ...event, from: addDays(event.from, shift), to: addDays(event.to, shift) };
      }

      const from = drag.edge === "start" ? withTime(drag.day, event.from) : event.from;
      const to = drag.edge === "end" ? withTime(drag.day, event.to) : event.to;
      return from.getTime() <= to.getTime() ? { ...event, from, to } : event;
    });
  }, [events, drag]);

  /** Клетки выделения: между тем, где нажали, и тем, где сейчас курсор. */
  const selected = (day: Date) => {
    if (drag?.kind !== "create") return false;
    const value = startOfDay(day).getTime();
    const anchor = startOfDay(drag.anchor).getTime();
    const current = startOfDay(drag.day).getTime();

    return value >= Math.min(anchor, current) && value <= Math.max(anchor, current);
  };

  const box = useRef<HTMLDivElement>(null);
  /** Строки недель: по ним и прыгают к месяцу, и узнают видимый. */
  const rows = useRef(new Map<string, HTMLDivElement>());
  const firstKey = weeks[0]?.[0] ? dayKey(weeks[0][0]) : "";

  /** Номер последней исполненной просьбы прыгнуть. */
  const jumped = useRef(-1);
  /** Последний доложенный месяц: событий прокрутки десятки в секунду. */
  const reported = useRef("");
  const anchored = useRef({ key: "", height: 0 });

  useLayoutEffect(() => {
    const area = box.current;
    if (!area) return;

    if (jumped.current !== focus.id) {
      /*
       * Прыжок: стрелки и «сегодня» двигают не ленту, а взгляд по ней.
       * Верхом — к началу месяца (минус шапка, она липкая и накрыла бы
       * первую неделю); центром — к сегодняшней неделе.
       *
       * Не нашли строку — не отмечаем просьбу исполненной: нужная
       * неделя приедет со следующей порцией ленты, и прыжок случится
       * тогда.
       */
      const week = focus.align === "center"
        ? startOfWeek(focus.day)
        : startOfWeek(new Date(focus.day.getFullYear(), focus.day.getMonth(), 1));
      const target = rows.current.get(dayKey(week));

      if (target) {
        area.scrollTop =
          focus.align === "center"
            ? target.offsetTop - (area.clientHeight - target.clientHeight) / 2
            : target.offsetTop - HEADER_HEIGHT;
        jumped.current = focus.id;
      }
    } else if (anchored.current.key && anchored.current.key !== firstKey) {
      /*
       * Ленту нарастили сверху: без поправки на выросшую высоту недели
       * уезжают из-под курсора вниз, и прокрутка «прыгает» ровно
       * на добавленные месяцы.
       */
      area.scrollTop += area.scrollHeight - anchored.current.height;
    }

    anchored.current = { key: firstKey, height: area.scrollHeight };
  }, [focus, firstKey, weeks]);

  const onScroll = () => {
    const area = box.current;
    if (!area) return;

    if (area.scrollTop < EDGE_GAP) onLoadPast?.();
    if (area.scrollHeight - area.scrollTop - area.clientHeight < EDGE_GAP) onLoadFuture?.();

    /* Видимый месяц — по первой неделе, которая ещё не уехала под
       шапку. Середина недели, а не её начало: неделя на стыке месяцев
       принадлежит тому, которого в ней больше. */
    const top = area.getBoundingClientRect().top + HEADER_HEIGHT;
    for (const week of weeks) {
      const row = week[0] && rows.current.get(dayKey(week[0]));
      if (row && row.getBoundingClientRect().bottom > top) {
        const middle = week[3];
        const key = middle && `${middle.getFullYear()}-${middle.getMonth()}`;
        if (middle && key && reported.current !== key) {
          reported.current = key;
          onVisibleMonth(middle);
        }
        return;
      }
    }
  };

  return (
    <div
      ref={box}
      onScroll={onScroll}
      className="relative min-h-0 flex-1 overflow-y-auto bg-bg select-none"
    >
      <div className="sticky top-0 z-20 grid grid-cols-7 border-b border-border bg-bg">
        {(weeks[0] ?? []).map((day) => (
          <span key={day.getDay()} className="px-2 py-1 text-2xs text-fg-muted capitalize">
            {weekdayName(day, locale)}
          </span>
        ))}
      </div>

      {weeks.map((week) => {
        const key = dayKey(week[0] ?? cursor);
        const segments = weekSegments(shown, week);

        return (
          <div
            key={key}
            ref={(element) => {
              if (element) rows.current.set(key, element);
              else rows.current.delete(key);
            }}
            className="relative min-h-24"
          >
            {/* Клетки: фон, число, «+» и приём броска. */}
            <div className="absolute inset-0 grid grid-cols-7">
              {week.map((day) => {
                const dayId = dayKey(day);
                /* Суббота и воскресенье — другим фоном: месяц читается
                   неделями, и без этой полоски глаз ищет границу недели
                   по числам. */
                const weekend = day.getDay() === 0 || day.getDay() === 6;
                const disabled = disabledDays.has(dayId);

                return (
                  <div
                    key={dayId}
                    onPointerDown={(event) => {
                      // Только левой кнопкой: правой вызывают меню браузера.
                      if (event.button !== 0 || disabled || !onCreate) return;
                      setDrag({ kind: "create", anchor: day, day });
                    }}
                    onPointerEnter={() => {
                      if (!drag || disabled) return;
                      setDrag({ ...drag, day });
                    }}
                    className={`group/day relative border-r border-b border-border ${
                      selected(day)
                        ? "bg-accent-subtle"
                        : disabled
                          ? "bg-surface-active"
                          : weekend
                            ? "bg-bg"
                            : "bg-surface"
                    }`}
                  >
                    {/* «+» появляется на наведении: он нужен раз в день,
                        а рябил бы в каждой клетке постоянно. С клавиатуры
                        доступен по-прежнему — фокус его показывает. */}
                    {onCreate && !disabled && (
                      <button
                        type="button"
                        onPointerDown={(event) => event.stopPropagation()}
                        onClick={() => onCreate(day, day)}
                        aria-label={t("table.addRow")}
                        title={t("table.addRow")}
                        className="absolute top-1 left-1 grid size-6 place-items-center rounded-md border border-border bg-surface text-fg-subtle opacity-0 transition hover:bg-surface-hover hover:text-fg focus-visible:opacity-100 group-hover/day:opacity-100"
                      >
                        <Icon as={IconPlus} size={14} />
                      </button>
                    )}

                    {/* Число справа: слева его закрывал бы «+». */}
                    <span
                      className={`absolute top-1 right-1.5 rounded px-1 text-xs tabular-nums ${
                        dayId === today
                          ? "bg-accent-solid text-accent-fg"
                          : day.getDate() === 1
                            ? "text-fg"
                            : "text-fg-muted"
                      }`}
                    >
                      {day.getDate() === 1 ? monthDay(day, locale) : day.getDate()}
                    </span>
                  </div>
                );
              })}
            </div>

            {/* Полосы. Слой не ловит указатель — ловят сами полосы,
                иначе выделение мышью не начиналось бы в клетке под ними. */}
            {/* Строки сетки задаются содержимым: карточка с четырьмя
                полями выше карточки с одним, и неделя вырастает вместе
                с ними — так же, как в референсе. Ничего не считаем
                руками: это и есть обычная сетка. */}
            <div className="pointer-events-none relative grid grid-cols-7 gap-y-1 px-1 pt-8 pb-1.5">
              {segments.map((segment) => (
                <div
                  key={segment.event.row.guid ?? String(segment.event.from)}
                  style={{
                    gridColumn: `${segment.start + 1} / span ${segment.span}`,
                    gridRow: segment.lane + 1,
                  }}
                  /* Щелчок после переноса гасится здесь, на перехвате:
                     до самой полосы он тогда не доходит и карточку
                     не открывает. */
                  onClickCapture={(event) => {
                    if (performance.now() - moved.current > CLICK_GAP) return;
                    moved.current = 0;
                    event.stopPropagation();
                  }}
                  className="pointer-events-auto min-w-0 px-0.5"
                >
                  {renderEvent(segment.event, segment, {
                    ...(onMoveTo
                      ? {
                          onGrab: (ratio: number) => {
                            /* Клетка, за которую взялись: доля ширины
                               полосы, разложенная на её же дни. */
                            const column = Math.min(
                              segment.span - 1,
                              Math.max(0, Math.floor(ratio * segment.span)),
                            );
                            const day = week[segment.start + column];
                            if (day) setDrag({ kind: "move", event: segment.event, anchor: day, day });
                          },
                        }
                      : {}),
                    ...(onResize
                      ? {
                          onResize: (edge: "start" | "end") => {
                            const day =
                              edge === "start" ? segment.event.from : segment.event.to;

                            /* Тянуть нечего — сразу отвечаем тем же днём:
                               вызывающий объяснит, чего не хватает,
                               а ложного предпросмотра не будет. */
                            if (!resizable) {
                              onResize(segment.event, edge, day);
                              return;
                            }

                            setDrag({ kind: "resize", event: segment.event, edge, day });
                          },
                        }
                      : {}),
                    /* По строке, а не по событию: пока тянут, в сетке
                       лежит КОПИЯ события с новыми краями, и обводка
                       по самому событию не нашлась бы никогда. */
                    active:
                      (drag?.kind === "move" || drag?.kind === "resize") &&
                      drag.event.row === segment.event.row,
                  })}
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/**
 * Сетка дня и недели: часы строками, дни колонками.
 *
 * События без времени (поле хранит только дату) на часовую сетку
 * не попадают — их место неизвестно, и посадить их на полночь значило бы
 * соврать. Для них полоса под шапкой, как в любом календаре.
 */
function TimeGrid({
  days,
  byDay,
  disabledDays,
  locale,
  dragging,
  resizing,
  renderEvent,
  onDropAt,
  onCreate,
  onResize,
  onResizeEnd,
}: {
  days: Date[];
  byDay: Map<string, CalendarEvent[]>;
  disabledDays: ReadonlySet<string>;
  locale: string;
  dragging: boolean;
  resizing: { event: CalendarEvent; to: Date } | null;
  renderEvent: (event: CalendarEvent) => ReactNode;
  onDropAt: (at: Date) => void;
  onCreate?: ((at: Date) => void) | undefined;
  onResize?: ((state: { event: CalendarEvent; to: Date }) => void) | undefined;
  onResizeEnd?: (() => void) | undefined;
}) {
  const slots = useMemo(() => daySlots(), []);
  const today = dayKey(new Date());
  /** Колонка, в которой тянут край: по ней считается новое время. */
  const column = useRef<HTMLElement | null>(null);
  /** Когда край в последний раз отпустили — см. CLICK_GAP. */
  const resized = useRef(0);

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-auto bg-surface">
      {/* Шапка липнет к верху: после первого экрана прокрутки иначе
          непонятно, в каком дне полоса. */}
      <div className="sticky top-0 z-20 flex shrink-0 border-b border-border bg-surface">
        <div className="w-14 shrink-0" />
        {days.map((day) => {
          const key = dayKey(day);

          return (
            <div key={key} className="flex-1 border-l border-border px-2 py-1">
              <span className="text-2xs text-fg-muted capitalize">{weekdayName(day, locale)}</span>
              <span
                className={`ml-1 rounded px-1 text-sm tabular-nums ${
                  key === today ? "bg-accent-solid text-accent-fg" : "text-fg"
                }`}
              >
                {day.getDate()}
              </span>

              {/* Полоса событий без времени. Пусто — строки нет вовсе:
                  пустая полоса съедала бы высоту у сетки часов. */}
              <div className="mt-1 flex flex-col gap-0.5">
                {(byDay.get(key) ?? [])
                  .filter((event) => event.allDay)
                  .map((event) => (
                    <div key={event.row.guid ?? String(event.from)}>{renderEvent(event)}</div>
                  ))}
              </div>
            </div>
          );
        })}
      </div>

      <div className="flex flex-1">
        {/* Подписи часов. Сдвинуты вверх на половину строки, чтобы стоять
            у линии, а не между линиями. */}
        <div className="w-14 shrink-0">
          {slots.map((slot) => (
            <div
              key={slot.minutes}
              style={{ height: SLOT_HEIGHT }}
              className="relative border-b border-transparent"
            >
              <span className="absolute -top-1.5 right-1 text-2xs text-fg-subtle tabular-nums">
                {slot.minutes ? slot.label : ""}
              </span>
            </div>
          ))}
        </div>

        {days.map((day) => {
          const key = dayKey(day);
          const disabled = disabledDays.has(key);
          const placed = placeDay(
            (byDay.get(key) ?? []).filter((event) => !event.allDay),
            day,
          );

          /** Доля высоты колонки под курсором. */
          const ratioOf = (element: HTMLElement, clientY: number) => {
            const area = element.getBoundingClientRect();
            return (clientY - area.top) / area.height;
          };

          return (
            <section
              key={key}
              /* Выходные — другим фоном, как и в месяце: неделя читается
                 колонками, и край рабочей недели должен быть виден. */
              className={`relative flex-1 border-l border-border ${
                disabled
                  ? "bg-surface-active"
                  : day.getDay() === 0 || day.getDay() === 6
                    ? "bg-bg"
                    : ""
              }`}
              style={{ height: slots.length * SLOT_HEIGHT }}
              onDragOver={(event: DragEvent<HTMLElement>) => {
                if (!dragging || disabled) return;
                event.preventDefault();
              }}
              onDrop={(event: DragEvent<HTMLElement>) => {
                event.preventDefault();
                onDropAt(timeAt(day, ratioOf(event.currentTarget, event.clientY)));
              }}
              onClick={(event) => {
                /*
                 * Отпущенный край — не щелчок по пустому месту. Браузер
                 * шлёт click после pointerup, и без этой развилки каждое
                 * растягивание заканчивалось бы открытым черновиком
                 * новой записи.
                 */
                if (performance.now() - resized.current < CLICK_GAP) return;
                if (disabled) return;
                onCreate?.(timeAt(day, ratioOf(event.currentTarget, event.clientY)));
              }}
              onPointerMove={(event: PointerEvent<HTMLElement>) => {
                if (!resizing || column.current !== event.currentTarget) return;
                const at = timeAt(day, ratioOf(event.currentTarget, event.clientY));
                // Конец не раньше начала: полоса отрицательной длины
                // нарисовалась бы вверх ногами.
                if (at.getTime() > resizing.event.from.getTime() && at.getTime() !== resizing.to.getTime()) {
                  onResize?.({ event: resizing.event, to: at });
                }
              }}
              onPointerUp={() => {
                if (!resizing) return;
                resized.current = performance.now();
                column.current = null;
                onResizeEnd?.();
              }}
            >
              {/* Линии часов. Отдельным слоем, а не рамками у событий:
                  события лежат поверх и рамки бы рвали. */}
              {slots.map((slot) => (
                <div
                  key={slot.minutes}
                  style={{ top: (slot.minutes / (24 * 60)) * 100 + "%" }}
                  className="pointer-events-none absolute inset-x-0 border-t border-border"
                />
              ))}

              {placed.map((item) => {
                const guid = item.event.row.guid;
                const preview =
                  resizing && guid && resizing.event.row.guid === guid ? resizing.to : null;
                const height = preview
                  ? (preview.getTime() - item.event.from.getTime()) / (24 * 60 * 60_000)
                  : item.height;

                return (
                  <div
                    key={item.event.row.guid ?? String(item.event.from)}
                    style={{
                      top: `${item.top * 100}%`,
                      height: `${Math.max(height, 0.01) * 100}%`,
                      left: `${(item.column / item.columns) * 100}%`,
                      width: `${100 / item.columns}%`,
                    }}
                    className="absolute p-px"
                  >
                    <div className="group/slot relative h-full">
                      {renderEvent(item.event)}

                      {onResize && (
                        /* Полоска в три пикселя по нижнему краю: тянут
                           за неё, а всё остальное поле карточки остаётся
                           под перетаскивание целиком. */
                        <div
                          onPointerDown={(event) => {
                            event.stopPropagation();
                            column.current = event.currentTarget.closest("section");
                            (event.currentTarget as HTMLElement).setPointerCapture?.(
                              event.pointerId,
                            );
                            onResize({ event: item.event, to: item.event.to });
                          }}
                          /* Отпускание ловит колонка: события с захватом
                             указателя всплывают до неё, а два обработчика
                             на одно pointerup — это два одинаковых PUT. */
                          className="absolute inset-x-0 -bottom-0.5 grid h-2 cursor-ns-resize place-items-center opacity-0 transition-opacity group-hover/slot:opacity-100"
                        >
                          {/* Видимая полоска: без неё нижний край
                              не отличить от края карточки. */}
                          <span className="h-1 w-6 rounded-full bg-fg-muted opacity-70" />
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </section>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Событие: цветная плашка со временем и первой колонкой view.
 *
 * Первая колонка — заголовком, как на карточке доски: у задачи это её
 * название, и ради него на календарь и смотрят. Время слева и только
 * там, где его не видно из сетки: в месяце у клетки часов нет.
 *
 * Экспортируется ради таймлайна: полоса на оси дней — это то же самое
 * событие с теми же ручками, обрезанными краями и обводкой. Второй
 * такой же компонент разошёлся бы с этим на первой же правке.
 */
export function EventChip({
  event,
  color,
  columns,
  tableSlug,
  relations,
  locale,
  language,
  showTime,
  shape = "row",
  draggable,
  clippedStart = false,
  clippedEnd = false,
  active = false,
  onGrab,
  onResize,
  onDragStart,
  onDragEnd,
  onOpen,
}: {
  event: CalendarEvent;
  color: ChipColor;
  columns: Field[];
  tableSlug: string;
  relations: Map<string, Relation>;
  locale: string;
  language: string;
  showTime: boolean;
  /**
   * Во что событие превращается на экране:
   *
   *   card — карточка в столбик: в клетке месяца есть высота, и поля
   *          читаются одно под другим — заголовок, статус, срок;
   *   row  — строка: в сетке часов высота означает длительность,
   *          и растить её под данные нельзя;
   *   bar  — полоса таймлайна: высота строки постоянна, значения идут
   *          по горизонтали, а текст стоит по центру полосы.
   */
  shape?: "card" | "row" | "bar";
  draggable: boolean;
  /** Полоса уходит за край недели: там нет ни скругления, ни ручки. */
  clippedStart?: boolean;
  clippedEnd?: boolean;
  /** Эту полосу сейчас тащат или тянут за край. */
  active?: boolean;
  /**
   * Взяли полосу за середину. Наружу уходит доля её ширины: у полосы
   * в пять клеток важно, за какую именно потянули, иначе событие
   * прыгнет началом под курсор, — а какие это клетки, знает сетка.
   *
   * Не задан — переносить нечем (нет прав), и курсор остаётся обычным.
   */
  onGrab?: ((ratio: number) => void) | undefined;
  /** Взяли за край. Не задан — растягивать нечем (нет поля конца или прав). */
  onResize?: ((edge: "start" | "end") => void) | undefined;
  onDragStart: () => void;
  onDragEnd: () => void;
  onOpen: () => void;
}) {
  const { t } = useTranslation();
  /*
   * Заголовок — первая ЗАПОЛНЕННАЯ колонка view, а не просто первая:
   * запись, заведённую протяжкой по дням, ещё не назвали, и полоса
   * с прочерком не сообщает ничего. Следом — остальные заполненные
   * колонки, через точку: статус и исполнитель на полосе читаются
   * так же, как на карточке доски.
   */
  const card = shape === "card";
  const filled = columns.filter((field) => !isBlank(event.row[field.slug]));
  /* ponytail: потолок на строки. Во view бывает тридцать колонок,
     и карточка из тридцати строк — это месяц из одной недели. Нужно
     больше — их показывает карточка записи. */
  const [title, ...rest] = filled.slice(0, card ? MAX_CARD_FIELDS : filled.length);
  /**
   * Держат за край. Пока держат, перетаскивание браузером выключено:
   * иначе полоса уезжает «картинкой под курсором» вместо того, чтобы
   * растягиваться, — а событие dragstart приходит на саму полосу,
   * и погасить его из ручки нельзя.
   */
  const [holding, setHolding] = useState(false);

  useEffect(() => {
    if (!holding) return;

    const release = () => setHolding(false);
    window.addEventListener("pointerup", release);
    return () => window.removeEventListener("pointerup", release);
  }, [holding]);

  return (
    <article
      draggable={draggable && !holding}
      onDragStart={(dragEvent: DragEvent<HTMLElement>) => {
        // Firefox не начинает перетаскивание без данных в буфере,
        // даже если они никому не нужны.
        dragEvent.dataTransfer.setData("text/plain", event.row.guid ?? "");
        dragEvent.dataTransfer.effectAllowed = "move";
        onDragStart();
      }}
      onDragEnd={onDragEnd}
      onClick={(clickEvent) => {
        // Иначе сработает клетка под событием и рядом откроется
        // черновик новой записи.
        clickEvent.stopPropagation();
        onOpen();
      }}
      title={eventHint(event, locale)}
      {...(onGrab
        ? {
            onPointerDown: (pointer: PointerEvent<HTMLElement>) => {
              if (pointer.button !== 0) return;
              /*
               * Наружу уезжает доля ширины, а не день: сколько клеток
               * в этой полосе и какие именно, знает сетка — полоса может
               * быть обрезана краем недели.
               */
              const area = pointer.currentTarget.getBoundingClientRect();
              onGrab(area.width ? (pointer.clientX - area.left) / area.width : 0);
            },
          }
        : {})}
      /* select-none: без него перетаскивание начинается с выделения
         текста, и в руке оказывается кусок текста вместо события.

         Курсор — «взять» и «тащу»: полоса, которую можно двигать,
         обязана об этом сообщать до того, как её потянули. */
      className={`group/bar relative flex h-full w-full overflow-hidden text-xs transition-shadow select-none ${
        card
          ? /* Карточка: поля в столбик, подложка мягкая — на ней лежат
               чипы значений, и полный цвет спорил бы с ними. */
            `min-h-8 flex-col items-start gap-0.5 border border-border p-1 ${CHIP_SURFACE[color]} ${
              clippedStart ? "" : "rounded-l-md"
            } ${clippedEnd ? "" : "rounded-r-md"}`
          : shape === "bar"
            ? /* Полоса: высота задана строкой оси, поэтому текст стоит
                 по центру, а не липнет к верхнему краю. */
              `min-h-6 items-center gap-1.5 px-2 ${CHIP_STYLES[color]} ${
                clippedStart ? "" : "rounded-l-sm"
              } ${clippedEnd ? "" : "rounded-r-sm"}`
            : `min-h-5 items-start gap-1 px-1.5 py-px ${CHIP_STYLES[color]} ${
                clippedStart ? "" : "rounded-l-sm"
              } ${clippedEnd ? "" : "rounded-r-sm"}`
      } ${onGrab || draggable ? "cursor-grab active:cursor-grabbing" : "cursor-pointer"} ${
        active ? "shadow-raised ring-2 ring-accent" : ""
      }`}
    >
      {/* Ручки по краям: за них полосу растягивают на соседние дни.
          У обрезанного края ручки нет — настоящий край события лежит
          в другой неделе, и тянуть надо там. */}
      {onResize && !clippedStart && (
        <Handle edge="start" onResize={onResize} onHold={() => setHolding(true)} />
      )}
      {onResize && !clippedEnd && (
        <Handle edge="end" onResize={onResize} onHold={() => setHolding(true)} />
      )}

      {showTime && !event.allDay && !card && (
        <span className="shrink-0 tabular-nums opacity-70">{hhmm(event.from)}</span>
      )}

      {title ? (
        <div className={`flex min-w-0 flex-1 ${card ? "flex-col gap-0.5" : "items-center gap-1.5"}`}>
          <div className="flex min-w-0 items-center gap-1 font-medium">
            {/* Значок типа поля — как в референсе: по нему видно, что
                это заголовок записи, а не ещё одно значение. */}
            {card && (
              <Icon as={fieldIcon(title.type)} size={13} className="shrink-0 opacity-70" />
            )}
            {card && showTime && !event.allDay && (
              <span className="shrink-0 tabular-nums opacity-70">{hhmm(event.from)}</span>
            )}
            <Tooltip label={title.slug}>
              <Cell
                field={title}
                row={event.row}
                tableSlug={tableSlug}
                relations={relations}
                locale={locale}
                language={language}
              />
            </Tooltip>
          </div>

          {/* Остальные заполненные колонки: в столбик — одна под другой,
              в строку — по мере ширины. */}
          {/* Значения мельче заголовка: на карточку смотрят ради
              названия, остальное читают вторым взглядом. */}
          {rest.map((field) => (
            <div
              key={field.id}
              className={`flex min-w-0 items-center opacity-80 ${card ? "text-2xs" : ""}`}
            >
              {/* Чьё это значение — видно по слагу: подписи полей
                  на плашке нет, а место у неё одно. */}
              <Tooltip label={field.slug}>
                <Cell
                  field={field}
                  row={event.row}
                  tableSlug={tableSlug}
                  relations={relations}
                  locale={locale}
                  language={language}
                />
              </Tooltip>
            </div>
          ))}
        </div>
      ) : (
        /* Ни одно поле не заполнено — так выглядит запись, заведённая
           протяжкой: даты есть, остального ещё нет. */
        <span className="truncate opacity-70">{t("board.untitled")}</span>
      )}
    </article>
  );
}

/**
 * Ручка края полосы.
 *
 * Указателем, а не перетаскиванием браузера: у HTML5-перетаскивания
 * своя «картинка под курсором» и своё событие броска, а край тянут
 * по клеткам — и результат должен быть виден до того, как кнопку
 * отпустили. Поэтому же обычное перетаскивание полосы здесь гасится.
 */
function Handle({
  edge,
  onResize,
  onHold,
}: {
  edge: "start" | "end";
  onResize: (edge: "start" | "end") => void;
  onHold: () => void;
}) {
  return (
    <span
      onPointerDown={(event) => {
        event.stopPropagation();
        onHold();
        onResize(edge);
      }}
      onDragStart={(event) => event.preventDefault()}
      onClick={(event) => event.stopPropagation()}
      /* Видна на наведении: постоянные ручки у каждой полосы рябят,
         а курсор всё равно приходит к карточке раньше, чем к её краю. */
      className={`absolute inset-y-0 z-10 grid w-3 cursor-ew-resize place-items-center opacity-0 transition-opacity group-hover/bar:opacity-100 ${
        edge === "start" ? "left-0" : "right-0"
      }`}
    >
      <span className="h-3 w-1 rounded-full bg-current" />
    </span>
  );
}

/** Тот же день, но с часами события: перенос в месяце времени не трогает. */
function withTimeOf(day: Date, event: CalendarEvent | null): Date {
  return event ? withTime(day, event.from) : day;
}

/** Тот же день, но с часами и минутами другого момента. */
function withTime(day: Date, source: Date): Date {
  return new Date(
    day.getFullYear(),
    day.getMonth(),
    day.getDate(),
    source.getHours(),
    source.getMinutes(),
  );
}

/** Первое число — с месяцем: в ленте недель без этого теряешь, где ты. */
function monthDay(day: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" }).format(day);
}

function addMinutes(date: Date, minutes: number): Date {
  return new Date(date.getTime() + minutes * 60_000);
}

/** Неделя, в которую попал день. Понедельник первым — как в model. */
function weekOf(cursor: Date): Date[] {
  const start = startOfWeek(cursor);
  return Array.from({ length: 7 }, (_, index) => addDays(start, index));
}

/**
 * Подсказка на событии: когда оно идёт.
 *
 * Даты, а не только часы: на оси таймлайна и в клетке месяца день полосы
 * приходится считать глазами по шапке, а у события без времени часов нет
 * вовсе. Одна дата вместо двух, когда событие в один день.
 */
function eventHint(event: CalendarEvent, locale: string): string {
  const day = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" });
  const stamp = (date: Date) =>
    event.allDay ? day.format(date) : `${day.format(date)}, ${hhmm(date)}`;

  const from = stamp(event.from);
  const to = stamp(event.to);

  return from === to ? from : `${from} – ${to}`;
}

function hhmm(date: Date): string {
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function weekdayName(day: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, { weekday: "short" }).format(day);
}

/**
 * Подпись периода: «январь 2026», «6 – 12 января 2026», «6 января 2026».
 *
 * Через Intl, а не своим списком месяцев: три языка интерфейса, и
 * склонение месяца в русском у Intl уже правильное.
 */
function periodTitle(period: CalendarPeriod, cursor: Date, locale: string): string {
  if (period === "MONTH") {
    return new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" }).format(cursor);
  }

  if (period === "DAY") {
    return new Intl.DateTimeFormat(locale, { day: "numeric", month: "long", year: "numeric" }).format(
      cursor,
    );
  }

  const days = weekOf(cursor);
  const first = days[0] ?? cursor;
  const last = days[6] ?? cursor;
  const short = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" });
  const full = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", year: "numeric" });

  return `${short.format(first)} – ${full.format(last)}`;
}

