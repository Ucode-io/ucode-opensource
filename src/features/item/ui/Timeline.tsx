import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent,
  type ReactNode,
} from "react";
import {
  IconArrowLeft,
  IconArrowRight,
  IconChevronDown,
  IconChevronLeft,
  IconChevronRight,
  IconChevronsLeft,
  IconChevronsRight,
  IconPlus,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Field, Relation } from "@/features/table";
import { toast } from "@/shared/lib/toast";
import { hexToChipColor, type ChipColor } from "@/shared/ui/chip";
import { Icon } from "@/shared/ui/icon";
import { Tabs } from "@/shared/ui/tabs";
import { Tooltip } from "@/shared/ui/tooltip";
import {
  addDays,
  calendarEvents,
  dateKind,
  dateValues,
  dayKey,
  daysBetween,
  shiftPeriod,
  startOfDay,
  type CalendarEvent,
} from "../model/calendar";
import { isBlank } from "../model/cell-value";
import { groupEntries, visibleEntries } from "../model/group";
import {
  COLUMN_WIDTH,
  TIMELINE_SCALES,
  dayAt,
  monthGroups,
  timelineBar,
  timelineDays,
  weekGroups,
  type TimelineScale,
} from "../model/timeline";
import type { Item } from "../model/types";
import { EventChip } from "./Calendar";
import { Cell } from "./Cell";
import { fieldIcon } from "./field-icon";

/**
 * Таймлайн: у каждой записи своя строка, дни — колонками общей оси.
 *
 * Отличие от календаря только в раскладке. Строки те же и приезжают тем
 * же get-list с отбором по видимому диапазону, полоса — та же
 * (EventChip), перенос и растягивание — тот же PUT двух полей дат.
 * Поэтому здесь нет ни своих запросов, ни своей геометрии события:
 * считается одно — с какой колонки полоса начинается (model/timeline).
 *
 * Ось — лента без конца, как месяцы календаря: прокрутка у края просит
 * соседние месяцы, и вместе с ними растёт отбор строк.
 *
 * Экран собран из двух половин, как в референсах таймлайнов: слева
 * список записей, прилипший к краю, справа — ось. Половину со списком
 * сворачивают: во вкладке связи панель узкая, и 240 пикселей подписей
 * там дороже, чем сами полосы.
 */

/** Ширина списка записей слева. */
const LEFT_WIDTH = 240;

/** Она же в свёрнутом виде: только кнопка, которой его вернуть. */
const RAIL_WIDTH = 28;

/** Высота строки записи. */
const ROW_HEIGHT = 40;

/** Ближе этого к краю ленты — просим следующие месяцы. */
const EDGE_GAP = 300;

/** Ниже этой ширины колонки число в неё не помещается. */
const NUMBER_WIDTH = 22;

/** Ниже этой — не помещается и день недели. */
const WEEKDAY_WIDTH = 44;

/**
 * Сколько миллисекунд после переноса щелчок считается его хвостом,
 * а не отдельным щелчком. Браузер шлёт click сразу за pointerup.
 */
const CLICK_GAP = 300;

export function Timeline({
  tableSlug,
  columns,
  rows,
  undated,
  fromField,
  toField,
  statusField,
  groups,
  relations,
  scale,
  cursor,
  rangeFrom,
  rangeTo,
  locale,
  language,
  hasMore,
  onScale,
  onCursor,
  onOpenRow,
  onMove,
  onCreate,
  onLoadMore,
  onLoadPast,
  onLoadFuture,
}: {
  tableSlug: string;
  /** Колонки view: первой подписана строка — и слева, и на самой полосе. */
  columns: Field[];
  rows: Item[];
  /**
   * Записи без дат. На оси их нет, но и потерять их нельзя: задачу без
   * срока ставят на таймлайн перетаскиванием — см. api/timeline.
   */
  undated: Item[];
  /** Поле начала полосы. Без него таймлайна нет вовсе. */
  fromField: Field;
  /** Поле конца. Не задано — полоса в один день. */
  toField: Field | undefined;
  /** Поле, вариантами которого красятся полосы (`status_field_slug`). */
  statusField: Field | undefined;
  /**
   * Поля группировки (`attributes.group_by_columns`) в порядке уровней.
   * Записи собираются в свёртываемые группы — так же, как в таблице,
   * и той же разбивкой (model/group). Пусто — плоский список.
   */
  groups: Field[];
  relations: Relation[];
  scale: TimelineScale;
  /** День, вокруг которого построена лента. */
  cursor: Date;
  /** Загруженная лента: её же диапазон стоит в отборе строк. */
  rangeFrom: Date;
  rangeTo: Date;
  locale: string;
  language: string;
  /** Приехали не все строки видимого диапазона. */
  hasMore?: boolean | undefined;
  onScale: (scale: TimelineScale) => void;
  onCursor: (cursor: Date) => void;
  onOpenRow: (guid: string) => void;
  /**
   * Перенос, растягивание и постановка записи на ось. Не задан —
   * таймлайн только читается: без права на правку рисовать
   * перетаскивание, которое ответит 403, нельзя.
   */
  onMove?: ((guid: string, values: Record<string, unknown>) => void) | undefined;
  /** Новая запись с уже проставленными датами. Не задан — нет права. */
  onCreate?: ((values: Record<string, unknown>) => void) | undefined;
  onLoadMore?: (() => void) | undefined;
  onLoadPast?: (() => void) | undefined;
  onLoadFuture?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  const width = COLUMN_WIDTH[scale];
  const days = useMemo(() => timelineDays(rangeFrom, rangeTo), [rangeFrom, rangeTo]);
  /*
   * Верхняя полоса шапки. В масштабе недель она подписывает недели,
   * иначе — месяцы: колонки при этом остаются днями в обоих случаях.
   * Так же устроен и старый экран.
   */
  const captions = useMemo(
    () => (scale === "WEEK" ? weekGroups(days) : monthGroups(days)),
    [days, scale],
  );
  const axisWidth = days.length * width;
  const today = dayKey(new Date());
  const todayIndex = days.findIndex((day) => dayKey(day) === today);

  /** Список записей свёрнут: остаётся одна ось. */
  const [collapsed, setCollapsed] = useState(false);
  const leftWidth = collapsed ? RAIL_WIDTH : LEFT_WIDTH;

  /** Свёрнутые группы — по ключу значения, как в таблице. */
  const [folded, setFolded] = useState<Set<string>>(() => new Set());
  const toggleGroup = (key: string) =>
    setFolded((prev) => {
      const next = new Set(prev);
      if (!next.delete(key)) next.add(key);
      return next;
    });

  const byId = useMemo(
    () => new Map(relations.map((relation) => [relation.id, relation])),
    [relations],
  );

  const events = useMemo(
    () => calendarEvents(rows, fromField, toField),
    [rows, fromField, toField],
  );

  /**
   * Что тянут указателем: полосу, её край или пустую строку.
   *
   * Одно состояние на три, потому что ведут они себя одинаково: жмут
   * в одном дне, ведут до другого, отпускают. Разница только в том,
   * что происходит на отпускании.
   */
  const [drag, setDrag] = useState<
    | { kind: "move"; event: CalendarEvent; anchor: Date; day: Date }
    | { kind: "resize"; event: CalendarEvent; edge: "start" | "end"; day: Date }
    | { kind: "create"; row: Item | null; anchor: Date; day: Date }
    | null
  >(null);

  /**
   * День под курсором в пустой строке. Из него рисуется призрак будущей
   * полосы: протяжку по пустому месту иначе никак не угадать — курсор
   * ничего не обещает, пока по нему не нажали.
   */
  const [hover, setHover] = useState<{ row: Item | null; day: Date } | null>(null);

  /**
   * Полоса под курсором. Её дни подсвечиваются в шапке: на оси в три
   * месяца по самой полосе не прочитать, на какие числа она попала, —
   * так же это показывала и старая админка (setFocusedDays).
   */
  const [hoveredBar, setHoveredBar] = useState<{ from: Date; to: Date } | null>(null);

  /**
   * Месяц, который сейчас перед глазами. Ось уезжает прокруткой
   * на месяцы вперёд, и заголовок, продолжающий показывать месяц
   * курсора, просто врёт.
   */
  const [visible, setVisible] = useState<Date | null>(null);
  /**
   * Куда смотреть: день, выравнивание и НОМЕР просьбы. Номер обязателен
   * — «сегодня» нажимают, уже стоя в этом месяце, и без него прыгать
   * было бы не по чему.
   */
  const [focus, setFocus] = useState(() => ({ day: cursor, align: "start" as "start" | "center", id: 0 }));

  /* Курсор сменился снаружи — стрелками или ссылкой: смотрим на него. */
  useEffect(() => {
    setFocus((current) =>
      dayKey(current.day) === dayKey(cursor)
        ? current
        : { day: cursor, align: "start", id: current.id + 1 },
    );
  }, [cursor]);

  /** Цвет полосы — по варианту поля, а не по HEX из данных (см. Chip). */
  const colorOf = (row: Item): ChipColor => {
    const option = statusField?.options.get(String(row[statusField.slug] ?? ""));
    return option?.color ? hexToChipColor(option.color) : "blue";
  };

  /** Записать новые даты — тем же PUT, что и правка ячейки. */
  const write = (guid: string, from: Date, to: Date) => {
    if (!onMove || !dateKind(fromField)) return;

    onMove(guid, dateValues(from, to, fromField, toField));
  };

  const box = useRef<HTMLDivElement>(null);
  /** Слой колонок: по его левому краю считается день под указателем. */
  const axis = useRef<HTMLDivElement>(null);
  /** Когда полосу в последний раз сдвинули — см. CLICK_GAP. */
  const moved = useRef(0);

  /** День под указателем. Ось одна на все строки, поэтому и мерка одна. */
  const dayUnder = (clientX: number): Date | null => {
    const area = axis.current?.getBoundingClientRect();
    return area ? dayAt(days, clientX - area.left, width) : null;
  };

  /*
   * Отпускание ловится на окне, а не на строке: кнопку отпускают и за
   * её пределами — на панели, на соседнем экране, — и без этого
   * перетаскивание оставалось бы висеть до следующего щелчка.
   */
  useEffect(() => {
    if (!drag) return;

    const finish = () => {
      setDrag(null);

      if (drag.kind === "move") {
        const shift = daysBetween(drag.anchor, drag.day);
        // Не сдвинули — это был щелчок, и его дело открыть карточку.
        if (shift === 0 || !drag.event.row.guid) return;

        moved.current = performance.now();
        write(
          drag.event.row.guid,
          addDays(drag.event.from, shift),
          addDays(drag.event.to, shift),
        );
        return;
      }

      if (drag.kind === "resize") {
        const edge = drag.edge === "start" ? drag.event.from : drag.event.to;
        if (dayKey(edge) === dayKey(drag.day) || !drag.event.row.guid) return;

        /* Час не трогаем: на оси его нет, а растягивание — про дни. */
        const from = drag.edge === "start" ? withTime(drag.day, drag.event.from) : drag.event.from;
        const to = drag.edge === "end" ? withTime(drag.day, drag.event.to) : drag.event.to;
        if (from.getTime() > to.getTime()) return;

        moved.current = performance.now();
        write(drag.event.row.guid, from, to);
        return;
      }

      const first = drag.anchor <= drag.day ? drag.anchor : drag.day;
      const last = drag.anchor <= drag.day ? drag.day : drag.anchor;

      /*
       * Строку без дат ставят на ось и одним днём: у неё в этой строке
       * нет ничего, мимо чего можно промахнуться, а событие без конца —
       * законный случай. Новая же запись заводится только протяжкой:
       * иначе промах по пустой строке каждый раз открывал бы черновик.
       */
      if (drag.row?.guid) {
        write(drag.row.guid, first, last);
        return;
      }

      if (dayKey(first) === dayKey(last)) return;
      onCreate?.(dateValues(first, last, fromField, toField));
    };

    window.addEventListener("pointerup", finish);
    return () => window.removeEventListener("pointerup", finish);
    /* Без списка зависимостей намеренно: обработчик пишет строку и
       обязан видеть последние onMove и onCreate, а не те, что были
       на начало перетаскивания. Подписка живёт только пока тянут. */
  });

  /*
   * Перенос и растягивание видно до ответа сервера: событие подменяется
   * копией с новыми краями, и полосы считаются из неё той же раскладкой.
   */
  const shown = useMemo(() => {
    if (drag?.kind !== "move" && drag?.kind !== "resize") return events;

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

  /**
   * Строки вперемешку с заголовками групп. Разбивка та же, что в таблице
   * (model/group): сервер отдал список, отсортированный полем группы,
   * клиент вставляет заголовок на каждой смене значения. Без группировки
   * — null, и строки идут подряд.
   */
  const groupSlugs = groups.map((field) => field.slug).join("|");
  const entries = useMemo(() => {
    if (!groupSlugs) return null;

    const rowsOfEvents = shown.map((event) => event.row);
    return visibleEntries(groupEntries(rowsOfEvents, groupSlugs.split("|")), folded);
  }, [groupSlugs, shown, folded]);

  /**
   * Дни, подсвеченные в шапке: пока полосу тащат — её будущие края,
   * иначе — края той, на которую навели.
   */
  const focused = useMemo(() => {
    if (drag?.kind !== "move" && drag?.kind !== "resize") return hoveredBar;

    const preview = shown.find((event) => event.row === drag.event.row);
    return preview ? { from: preview.from, to: preview.to } : null;
  }, [drag, shown, hoveredBar]);

  /** Прокрутка к нужному дню и удержание ленты при её росте влево. */
  const firstKey = days[0] ? dayKey(days[0]) : "";
  const jumped = useRef(-1);
  const anchored = useRef({ key: "", width: 0 });
  const reported = useRef("");
  /** День у левого края. По нему ось возвращается на место при смене масштаба. */
  const leftDay = useRef<Date | null>(null);

  /**
   * Какие колонки сейчас на экране. Нужно строкам, чья полоса уехала
   * за край: у них вместо полосы стрелка, которая к ней возвращает.
   */
  const [seen, setSeen] = useState({ first: 0, last: 0 });

  /** Пересчитать видимый кусок оси. Одна мерка на прокрутку и на прыжок. */
  const measure = () => {
    const area = box.current;
    if (!area) return;

    const first = Math.floor(area.scrollLeft / width);
    const last = Math.ceil((area.scrollLeft + area.clientWidth - leftWidth) / width) - 1;

    setSeen((current) =>
      current.first === first && current.last === last ? current : { first, last },
    );
  };

  /*
   * Масштаб сменили: прокрутка в ПИКСЕЛЯХ осталась прежней, а колонка
   * стала другой ширины — без этого «дни → месяцы» уносит на полгода
   * вперёд от того места, куда человек смотрел. Старая админка на смене
   * масштаба просто прыгала на сегодня; день под левым краем — то же
   * лекарство, но без потери места.
   */
  useEffect(() => {
    const day = leftDay.current;
    if (day) setFocus((current) => ({ day, align: "start", id: current.id + 1 }));
  }, [width]);

  useLayoutEffect(() => {
    const area = box.current;
    if (!area || !days[0]) return;

    if (jumped.current !== focus.id) {
      /* Клампим: после смены масштаба лента могла ужаться, и день,
         к которому мы возвращаемся, оказаться за её краем. */
      const index = Math.min(Math.max(daysBetween(days[0], focus.day), 0), days.length - 1);
      const offset = index * width;
      area.scrollLeft =
        focus.align === "center" ? offset - (area.clientWidth - leftWidth - width) / 2 : offset;
      jumped.current = focus.id;
    } else if (anchored.current.key && anchored.current.key !== firstKey) {
      /* Ленту нарастили слева: без поправки на выросшую ширину дни
         уезжают из-под курсора вправо. */
      area.scrollLeft += area.scrollWidth - anchored.current.width;
    }

    anchored.current = { key: firstKey, width: area.scrollWidth };
    measure();
  }, [focus, firstKey, days, width, leftWidth]);

  const onScroll = () => {
    const area = box.current;
    if (!area) return;

    if (area.scrollLeft < EDGE_GAP) onLoadPast?.();
    if (area.scrollWidth - area.scrollLeft - area.clientWidth < EDGE_GAP) onLoadFuture?.();

    /* Видимый месяц — по первой колонке, которая не ушла под список
       записей: им подписан заголовок. */
    measure();

    const day = days[Math.floor(area.scrollLeft / width)];
    leftDay.current = day ?? null;

    const key = day && `${day.getFullYear()}-${day.getMonth()}`;
    if (day && key && reported.current !== key) {
      reported.current = key;
      setVisible(day);
    }
  };

  /** Показать полосу: подвести её к середине видимой части оси. */
  const scrollToBar = (bar: { start: number; span: number }) => {
    const area = box.current;
    if (!area) return;

    const room = area.clientWidth - leftWidth;
    area.scrollTo({
      left: bar.start * width - Math.max(room - bar.span * width, 0) / 2,
      behavior: "smooth",
    });
  };

  /**
   * Полоса записи на её строке — или стрелка к ней, если она уехала
   * за край экрана.
   *
   * Без стрелки строка выглядит пустой: у записи есть даты, но их
   * не видно, и понять, в какую сторону крутить, неоткуда.
   */
  const renderBar = (event: CalendarEvent) => {
    const bar = days[0] ? timelineBar(event, days[0], days.length) : null;
    if (!bar) return null;

    const behind = bar.start + bar.span - 1 < seen.first;
    const ahead = bar.start > seen.last;

    if (behind || ahead) {
      return (
        <button
          type="button"
          onClick={() => scrollToBar(bar)}
          style={behind ? { left: leftWidth + 4 } : undefined}
          /* Стрелка вперёд собирается задом наперёд: подпись должна
             оказаться со стороны экрана, а не за его краем. */
          className={`group/jump sticky z-10 flex items-center gap-1.5 rounded-md border border-border bg-surface px-1 py-0.5 text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg ${
            behind ? "mr-auto" : "right-1 ml-auto flex-row-reverse"
          }`}
        >
          <Icon as={behind ? IconArrowLeft : IconArrowRight} size={14} className="shrink-0" />
          {/* Дата видна на наведении: в ряду одинаковых стрелок только
              она и отличает одну строку от другой. */}
          <span className="text-2xs hidden whitespace-nowrap group-hover/jump:inline">
            {dayRange(event.from, event.to, locale)}
          </span>
        </button>
      );
    }

    return (
      <div
        style={{ left: bar.start * width, width: bar.span * width }}
        /* Щелчок после переноса гасится на перехвате: до полосы он
           тогда не доходит и карточку не открывает. */
        onClickCapture={(click) => {
          if (performance.now() - moved.current > CLICK_GAP) return;
          moved.current = 0;
          click.stopPropagation();
        }}
        onPointerEnter={() => setHoveredBar({ from: event.from, to: event.to })}
        onPointerLeave={() => setHoveredBar(null)}
        className="absolute inset-y-1.5 px-px"
      >
        <EventChip
          event={event}
          color={colorOf(event.row)}
          columns={columns}
          tableSlug={tableSlug}
          relations={byId}
          locale={locale}
          language={language}
          showTime={false}
          shape="bar"
          /* Полосу ведут указателем, а не перетаскиванием браузера:
             только так виден результат до отпускания. */
          draggable={false}
          clippedStart={bar.clippedStart}
          clippedEnd={bar.clippedEnd}
          /* Сравниваем строки, а не события: пока тянут, на экране
             лежит КОПИЯ события с новыми краями, и по самому событию
             обводка не нашлась бы никогда. */
          active={
            (drag?.kind === "move" || drag?.kind === "resize") &&
            drag.event.row === event.row
          }
          {...(onMove
            ? {
                onGrab: (ratio: number) => {
                  /* Клетка, за которую взялись: доля ширины полосы,
                     разложенная на её же дни. Иначе полоса прыгает
                     началом под курсор. */
                  const column = Math.min(
                    bar.span - 1,
                    Math.max(0, Math.floor(ratio * bar.span)),
                  );
                  const day = days[bar.start + column];
                  if (day) setDrag({ kind: "move", event, anchor: day, day });
                },
                onResize: (edge: "start" | "end") => {
                  /*
                   * Растягивать нечем, если у view не выбрано поле
                   * конца: второй границы у события нет. Ручка при этом
                   * рисуется всё равно — молчаливо отсутствующая
                   * читается как поломка, а не как ненастроенный view.
                   */
                  if (!toField) {
                    toast.error(t("calendar.needEndField"));
                    return;
                  }

                  setDrag({
                    kind: "resize",
                    event,
                    edge,
                    day: edge === "start" ? event.from : event.to,
                  });
                },
              }
            : {})}
          onDragStart={() => undefined}
          onDragEnd={() => undefined}
          onOpen={() => event.row.guid && onOpenRow(event.row.guid)}
        />
      </div>
    );
  };

  /** Выделение: между тем днём, где нажали, и тем, где сейчас курсор. */
  const renderSelection = (row: Item | null) => {
    if (!days[0]) return null;

    if (drag?.kind === "create" && (drag.row ?? null) === row) {
      const from = daysBetween(days[0], drag.anchor);
      const to = daysBetween(days[0], drag.day);
      const first = days[Math.min(from, to)];
      const last = days[Math.max(from, to)];

      return (
        <div
          style={{ left: Math.min(from, to) * width, width: (Math.abs(to - from) + 1) * width }}
          className="pointer-events-none absolute inset-y-1.5 flex items-center rounded-sm border border-accent bg-accent-subtle px-1.5"
        >
          {/* Даты словами: по одной подсветке в шапке не сказать,
              какое число под курсором, — так же подписывала протяжку
              и старая админка. */}
          {first && last && (
            <span className="text-2xs whitespace-nowrap text-accent-text">
              {dayRange(first, last, locale)}
            </span>
          )}
        </div>
      );
    }

    /* Призрак под курсором: пустая строка сама говорит, что по ней
       протягивают. Пока что-то тащат — не показываем: в руке уже есть
       что вести. */
    if (drag || !hover || hover.row !== row) return null;

    return (
      <div
        style={{ left: daysBetween(days[0], hover.day) * width, width }}
        className="pointer-events-none absolute inset-y-1.5 flex items-center justify-center rounded-sm border border-dashed border-border-strong text-fg-subtle"
      >
        <Icon as={IconPlus} size={12} className="shrink-0" />
        {/* Подпись вылезает за призрак: в колонку она не влезет, а знать,
            на какое число ставишь, надо. */}
        <span className="text-2xs absolute left-full ml-1 whitespace-nowrap">
          {dayRange(hover.day, hover.day, locale)}
        </span>
      </div>
    );
  };

  /** Строка: слева запись, справа её место на оси. */
  const renderRow = (
    row: Item,
    event: CalendarEvent | null,
    placeable: boolean,
    /** Уровней группировки над строкой: на столько она и сдвинута. */
    indent = 0,
  ) => (
    <div
      key={row.guid ?? String(event?.from)}
      className="group/row flex"
      style={{ height: ROW_HEIGHT }}
    >
      <RowLabel
        row={row}
        columns={columns}
        tableSlug={tableSlug}
        relations={byId}
        locale={locale}
        language={language}
        width={leftWidth}
        indent={indent}
        collapsed={collapsed}
        onOpen={() => row.guid && onOpenRow(row.guid)}
      />

      <Lane
        width={axisWidth}
        {...(placeable && onMove
          ? {
              onPick: (clientX: number) => {
                const day = dayUnder(clientX);
                if (day) setDrag({ kind: "create", row, anchor: day, day });
              },
              onHover: (clientX: number | null) => {
                const day = clientX === null ? null : dayUnder(clientX);
                setHover(day ? { row, day } : null);
              },
            }
          : {})}
      >
        {event && renderBar(event)}
        {renderSelection(row)}
      </Lane>
    </div>
  );

  /**
   * Заголовок группы. Значение рисует та же ячейка, что и в строке:
   * у статуса это цветная плашка, у связи — подпись по полям показа,
   * и рисовать их вторым способом значило бы однажды разойтись.
   */
  const renderGroup = (entry: { key: string; level: number; row: number; count: number }) => {
    const row = shown[entry.row]?.row;
    const group = groups[entry.level];
    if (!group || !row) return null;

    const isFolded = folded.has(entry.key);

    return (
      <SectionRow
        key={`group:${entry.row}`}
        left={leftWidth}
        width={axisWidth}
        level={entry.level}
        folded={isFolded}
        onToggle={() => toggleGroup(entry.key)}
        label={
          collapsed ? null : (
            <>
              <span className="flex min-w-0 items-center">
                {isBlank(row[group.slug]) ? (
                  <span className="text-fg-subtle">—</span>
                ) : (
                  <Cell
                    field={group}
                    row={row}
                    tableSlug={tableSlug}
                    relations={byId}
                    locale={locale}
                    language={language}
                  />
                )}
              </span>

              {/* Счёт загруженного, а не всей группы: остальная её часть
                  ещё на сервере — так же честно считает и таблица. */}
              <span className="text-2xs shrink-0 text-fg-subtle">{entry.count}</span>
            </>
          )
        }
      />
    );
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
        <div className="flex items-center">
          <button
            type="button"
            onClick={() => onCursor(shiftPeriod("MONTH", cursor, -1))}
            aria-label={t("action.previous")}
            className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconChevronLeft} size={16} />
          </button>
          <button
            type="button"
            onClick={() => onCursor(shiftPeriod("MONTH", cursor, 1))}
            aria-label={t("action.next")}
            className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconChevronRight} size={16} />
          </button>
        </div>

        <button
          type="button"
          /* Сегодня — это и день, и место на экране: ось прокручивается
             так, чтобы текущий день оказался по центру. */
          onClick={() => {
            const day = new Date();
            onCursor(day);
            setFocus((current) => ({ day, align: "center", id: current.id + 1 }));
          }}
          className="h-7 rounded-md border border-border px-2 text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
        >
          {t("calendar.today")}
        </button>

        {/* Капитель: Intl отдаёт месяц строчной буквой в русском
            и узбекском, а это заголовок экрана. */}
        <span className="text-sm font-medium first-letter:uppercase">
          {monthTitle(visible ?? cursor, locale)}
        </span>

        {/* Строк в диапазоне больше, чем приехало: молчать об этом
            нельзя — ни счётчика, ни номеров страниц на оси нет. */}
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
            tabs={TIMELINE_SCALES.map((item) => ({
              id: item,
              label: t(`timeline.scale.${item}` as const),
            }))}
            activeId={scale}
            onSelect={(id) => onScale(id as TimelineScale)}
          />
        </div>
      </div>

      <div
        ref={box}
        onScroll={onScroll}
        onPointerMove={(pointer) => {
          if (!drag) return;
          const day = dayUnder(pointer.clientX);
          if (day && dayKey(day) !== dayKey(drag.day)) setDrag({ ...drag, day });
        }}
        className="relative min-h-0 flex-1 overflow-auto bg-surface select-none"
      >
        <div style={{ width: leftWidth + axisWidth }}>
          {/* Шапка оси. Липнет к верху, а список записей внутри неё —
              к левому краю: у прокрутки два направления, и уехать
              не должно ни то, ни другое. */}
          <div className="sticky top-0 z-30 flex bg-surface">
            <div
              className="sticky left-0 z-10 flex shrink-0 items-center justify-between border-r border-b border-border bg-surface"
              style={{ width: leftWidth }}
            >
              {!collapsed && (
                <span className="truncate pl-2 text-2xs font-medium text-fg-subtle">
                  {t("timeline.records")}
                </span>
              )}
              <button
                type="button"
                onClick={() => setCollapsed((value) => !value)}
                aria-label={t(collapsed ? "timeline.expand" : "timeline.collapse")}
                title={t(collapsed ? "timeline.expand" : "timeline.collapse")}
                className="m-0.5 grid size-6 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
              >
                <Icon as={collapsed ? IconChevronsRight : IconChevronsLeft} size={14} />
              </button>
            </div>

            <div className="shrink-0 border-b border-border" style={{ width: axisWidth }}>
              {/* Месяц подписан один раз на все свои дни, и подпись едет
                  вместе с прокруткой: у месяца в тридцать колонок
                  название иначе уезжает с экрана раньше, чем сам месяц. */}
              <div className="flex h-6">
                {captions.map((caption) => (
                  <div
                    key={dayKey(caption.first)}
                    style={{ width: caption.span * width }}
                    className="relative shrink-0"
                  >
                    <span
                      style={{ left: leftWidth + 8 }}
                      className="sticky inline-block py-0.5 text-2xs font-medium text-fg-muted capitalize"
                    >
                      {scale === "WEEK"
                        ? dayRange(caption.first, addDays(caption.first, caption.span - 1), locale)
                        : monthTitle(caption.first, locale)}
                    </span>
                  </div>
                ))}
              </div>

              <div className="flex h-8">
                {days.map((day) => {
                  const key = dayKey(day);
                  const weekend = day.getDay() === 0 || day.getDay() === 6;
                  const lit = inRange(day, focused);
                  /*
                   * В узкой колонке числа стоят не у каждого дня:
                   * понедельник, сегодня и края подсвеченного отрезка.
                   * Так же считал и старый экран (TimeLineMonth.jsx:88) —
                   * иначе числа налезают друг на друга, а без них ось
                   * превращается в безымянную линейку.
                   */
                  const numbered =
                    width >= NUMBER_WIDTH ||
                    day.getDay() === 1 ||
                    key === today ||
                    (focused !== null &&
                      (key === dayKey(focused.from) || key === dayKey(focused.to)));

                  return (
                    /*
                     * Подсказка на каждой клетке — как в старой админке
                     * (TimeLineDayBlock.jsx: поповер «13 / Monday»). Здесь
                     * она нужнее: в масштабе недель нет дня недели,
                     * в масштабе месяцев нет и числа, а год не написан
                     * нигде.
                     */
                    <Tooltip key={key} label={fullDate(day, locale)}>
                    <div
                      style={{ width }}
                      className={`flex shrink-0 items-center justify-center gap-1 text-2xs ${
                        lit
                          ? "bg-surface-active font-medium text-fg"
                          : weekend
                            ? "text-fg-subtle"
                            : "text-fg-muted"
                      }`}
                    >
                      {width >= WEEKDAY_WIDTH && (
                        <span className="capitalize">{weekdayName(day, locale)}</span>
                      )}
                      {numbered && (
                        <span
                          className={
                            key === today
                              ? "grid size-4.5 place-items-center rounded-full bg-accent-solid text-accent-fg tabular-nums"
                              : "tabular-nums"
                          }
                        >
                          {day.getDate()}
                        </span>
                      )}
                    </div>
                    </Tooltip>
                  );
                })}
              </div>
            </div>
          </div>

          <div className="relative">
            {/* Колонки — одним слоем на все строки: выходные, сегодня
                и разделители. Рисовать их в каждой строке значило бы
                держать на экране день × запись пустых элементов. */}
            <div
              ref={axis}
              className="pointer-events-none absolute inset-y-0 flex"
              style={{ left: leftWidth, width: axisWidth }}
            >
              {/* Сегодня — линия через все строки, а не только подложка
                  колонки: подложку в масштабе месяца видно как ещё один
                  оттенок, а по линии сразу ясно, где мы. Точка сверху
                  ставит её на шкалу, как флажок. */}
              {todayIndex >= 0 && (
                <div
                  /* Без z-index: полосы событий проходят поверх линии,
                     а не перечёркиваются ею. */
                  className="absolute inset-y-0 w-0.5 -translate-x-1/2 rounded-full bg-accent"
                  style={{ left: (todayIndex + 0.5) * width }}
                >
                  <span className="absolute -top-1 left-1/2 size-2 -translate-x-1/2 rounded-full bg-accent" />
                </div>
              )}

              {days.map((day) => {
                const key = dayKey(day);
                const weekend = day.getDay() === 0 || day.getDay() === 6;

                return (
                  /* Подложки у сегодняшней колонки нет: широкая полоса
                     спорила с линией и читалась как ещё один оттенок
                     выходных. День отмечает линия и кружок на числе. */
                  <div
                    key={key}
                    style={{ width }}
                    className={`h-full shrink-0 ${weekend ? "bg-bg" : ""}`}
                  />
                );
              })}
            </div>

            {/* Строк столько же, сколько записей: у полосы нет соседей,
                с которыми ей пришлось бы расходиться. С группировкой
                между ними встают свёртываемые заголовки. */}
            {entries
              ? entries.map((entry) => {
                  if (entry.kind === "header") return renderGroup(entry);

                  const event = shown[entry.row];
                  return event ? renderRow(event.row, event, false, groups.length) : null;
                })
              : shown.map((event) => renderRow(event.row, event, false))}

            {/* Записи без дат: на оси их нет, и появляются они там
                протяжкой по своей же строке. */}
            {undated.length > 0 && (
              <>
                <SectionRow label={t("timeline.undated")} left={leftWidth} width={axisWidth} />
                {undated.map((row) => renderRow(row, null, true))}
              </>
            )}

            {/* Пустая строка внизу: протяжка по ней заводит запись
                сразу на выделенные дни. Щелчок — нет: промах по пустой
                строке иначе каждый раз открывал бы черновик. */}
            {onCreate && (
              <div className="group/row flex" style={{ height: ROW_HEIGHT }}>
                <div
                  className="sticky left-0 z-20 flex shrink-0 items-center gap-1 border-r border-b border-border bg-surface px-2 text-xs text-fg-subtle transition-colors group-hover/row:bg-surface-hover"
                  style={{ width: leftWidth }}
                >
                  <Icon as={IconPlus} size={14} className="shrink-0" />
                  {!collapsed && <span className="truncate">{t("timeline.newRow")}</span>}
                </div>

                <Lane
                  width={axisWidth}
                  onPick={(clientX) => {
                    const day = dayUnder(clientX);
                    if (day) setDrag({ kind: "create", row: null, anchor: day, day });
                  }}
                  onHover={(clientX) => {
                    const day = clientX === null ? null : dayUnder(clientX);
                    setHover(day ? { row: null, day } : null);
                  }}
                >
                  {renderSelection(null)}
                </Lane>
              </div>
            )}

            {/* Записей в диапазоне нет — это не пустая ось, а причина:
                иначе экран выглядит сломанным. */}
            {!events.length && !undated.length && (
              <p
                className="sticky left-0 p-8 text-xs text-fg-muted"
                style={{ width: LEFT_WIDTH * 2 }}
              >
                {t("timeline.noRows")}
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Место строки на оси: полоса, выделение и приём протяжки.
 *
 * Отдельным компонентом ради наведения: призрак будущей полосы должен
 * знать, над КАКОЙ строкой курсор, а знает это только сама строка.
 */
function Lane({
  width,
  children,
  onPick,
  onHover,
}: {
  width: number;
  children: React.ReactNode;
  /** Нажали в пустом месте строки. Не задан — протягивать нечего. */
  onPick?: ((clientX: number) => void) | undefined;
  /** Курсор ведут по пустому месту; null — ушёл со строки. */
  onHover?: ((clientX: number | null) => void) | undefined;
}) {
  return (
    <div
      /* Ни рамок, ни линеек: сетку рисует только подложка колонок —
         полоса на чистом фоне читается как полоса, а не как ещё одна
         клетка таблицы. */
      className={`relative flex shrink-0 items-center transition-colors group-hover/row:bg-surface-hover/60 ${
        onPick ? "cursor-crosshair" : ""
      }`}
      style={{ width }}
      {...(onPick
        ? {
            onPointerDown: (pointer: PointerEvent<HTMLDivElement>) => {
              // Только левой кнопкой: правой вызывают меню браузера.
              if (pointer.button === 0) onPick(pointer.clientX);
            },
          }
        : {})}
      {...(onHover
        ? {
            onPointerMove: (pointer: PointerEvent<HTMLDivElement>) => onHover(pointer.clientX),
            onPointerLeave: () => onHover(null),
          }
        : {})}
    >
      {children}
    </div>
  );
}

/** Запись слева: первая заполненная колонка view, как на карточке доски. */
function RowLabel({
  row,
  columns,
  tableSlug,
  relations,
  locale,
  language,
  width,
  indent,
  collapsed,
  onOpen,
}: {
  row: Item;
  columns: Field[];
  tableSlug: string;
  relations: Map<string, Relation>;
  locale: string;
  language: string;
  width: number;
  /** Сколько уровней группировки над строкой: её отступ. */
  indent: number;
  /** Список свёрнут: от подписи остаётся один значок типа поля. */
  collapsed: boolean;
  onOpen: () => void;
}) {
  const { t } = useTranslation();
  /* Первая ЗАПОЛНЕННАЯ, а не первая по списку: запись, заведённую
     протяжкой, ещё не назвали, и прочерк не сообщает ничего. */
  const title = columns.find((field) => !isBlank(row[field.slug]));

  return (
    /* Подсказка — про поле: какой колонкой подписана запись, по значению
       не видно, а слаг ещё и то имя, которым поле зовут в API. */
    <Tooltip label={title?.slug ?? ""}>
    <button
      type="button"
      onClick={onOpen}
      className={`sticky left-0 z-20 flex shrink-0 items-center gap-1.5 overflow-hidden border-r border-b border-border bg-surface text-left text-xs transition-colors group-hover/row:bg-surface-hover ${
        collapsed ? "justify-center px-1" : "pr-2"
      }`}
      /* Отступ по уровню группы: иначе строки стоят вровень со своим
         заголовком и вложенность не читается. */
      style={{ width, ...(collapsed ? {} : { paddingLeft: 8 + indent * 14 }) }}
    >
      {title && (
        <Icon as={fieldIcon(title.type)} size={14} className="shrink-0 text-fg-subtle" />
      )}

      {!collapsed &&
        (title ? (
          <Cell
            field={title}
            row={row}
            tableSlug={tableSlug}
            relations={relations}
            locale={locale}
            language={language}
          />
        ) : (
          <span className="truncate text-fg-subtle">{t("board.untitled")}</span>
        ))}
    </button>
    </Tooltip>
  );
}

/**
 * Заголовок раздела на всю ширину оси: и «Без дат», и группа.
 *
 * Один компонент на оба, потому что это одно и то же: строка-разделитель
 * с подписью слева. У группы к ней добавляется стрелка свёртки.
 */
function SectionRow({
  label,
  left,
  width,
  level = 0,
  folded,
  onToggle,
}: {
  label: ReactNode;
  left: number;
  /** Ширина оси. Заголовок её не красит, но занимает: см. ниже. */
  width: number;
  /** Уровень вложенности: им задан отступ. */
  level?: number;
  /** Группа свёрнута. Без onToggle не показывается вовсе. */
  folded?: boolean | undefined;
  onToggle?: (() => void) | undefined;
}) {
  const { t } = useTranslation();
  const inner = (
    <>
      {onToggle && (
        <Icon
          as={folded ? IconChevronRight : IconChevronDown}
          size={14}
          className="shrink-0 text-fg-muted"
        />
      )}
      {label}
    </>
  );

  return (
    <div className="flex h-7">
      {onToggle ? (
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={!folded}
          title={t(folded ? "timeline.groupExpand" : "timeline.groupCollapse")}
          className="sticky left-0 z-20 flex shrink-0 items-center gap-1.5 overflow-hidden border-r border-b border-border bg-bg pr-2 text-2xs font-medium text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          style={{ width: left, paddingLeft: 8 + level * 14 }}
        >
          {inner}
        </button>
      ) : (
        <div
          className="sticky left-0 z-20 flex shrink-0 items-center gap-1.5 overflow-hidden border-r border-b border-border bg-bg pr-2 text-2xs font-medium text-fg-muted"
          style={{ width: left, paddingLeft: 8 + level * 14 }}
        >
          {inner}
        </div>
      )}

      {/*
        Пустая распорка во всю ось. Ни фона, ни линии: полоса во всю
        ширину рвала бы колонки на куски, и при двух уровнях группировки
        вместо сетки получалась стопка чередующихся лент. Но ширина нужна:
        подпись слева липкая, а липнет она только в пределах своей строки —
        строка шириной со список уехала бы вместе с прокруткой вбок.
      */}
      <div className="shrink-0" style={{ width }} />
    </div>
  );
}

/** Попадает ли день в подсвеченный отрезок. Пусто — не попадает никто. */
function inRange(day: Date, range: { from: Date; to: Date } | null): boolean {
  if (!range) return false;

  const value = day.getTime();
  return value >= startOfDay(range.from).getTime() && value <= startOfDay(range.to).getTime();
}

/** «5 окт» или «5 окт – 10 окт»: одна дата, когда день один. */
function dayRange(from: Date, to: Date, locale: string): string {
  const format = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" });
  const first = format.format(from);
  const last = format.format(to);

  return first === last ? first : `${first} – ${last}`;
}

/** Тот же день, но с часами другого момента: на оси часов нет. */
function withTime(day: Date, source: Date): Date {
  return new Date(
    day.getFullYear(),
    day.getMonth(),
    day.getDate(),
    source.getHours(),
    source.getMinutes(),
  );
}

/** «понедельник, 3 марта 2026» — подсказка на клетке шапки. */
function fullDate(day: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, {
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
  }).format(day);
}

function weekdayName(day: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, { weekday: "short" }).format(day);
}

/** «Январь 2026» — через Intl: склонение месяца у него уже правильное. */
function monthTitle(day: Date, locale: string): string {
  return new Intl.DateTimeFormat(locale, { month: "long", year: "numeric" }).format(
    startOfDay(day),
  );
}
