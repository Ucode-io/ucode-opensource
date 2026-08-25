import { toDateValue, type DateKind } from "@/shared/lib/date-value";
import { pivotKey, pivotTable, reduce, toAggregation, toNumber, type Aggregation } from "./pivot";
import type { Item } from "./types";

/**
 * Графики экрана CHART.
 *
 * Считает КЛИЕНТ, по загруженным строкам, — ровно как сводная и по той
 * же причине: серверной агрегации нет вовсе (docs/backend-notes.md,
 * «Группировка»). Поэтому у каждого графика в подписи стоит, по скольким
 * строкам он посчитан, а не круглое «итого».
 *
 * Вся арифметика — из `pivot`: график это та же сводная, нарисованная
 * не таблицей. Одно поле разбивки — список полос; два — матрица
 * «поле × поле», из которой получаются группы, стопки, мульти-линия
 * и тепловая карта. Второй реализации «сгруппировать и свести» в проекте
 * нет и быть не должно: она разошлась бы со сводной на первом же крайнем
 * случае (пустое значение, текст в числовом поле, среднее по набору
 * без чисел).
 *
 * Настройки живут В VIEW, а не в адресе, — в отличие от сводной. Это
 * не «как я сейчас смотрю», а раскладка, которую админ собрал всем:
 * набор графиков, их порядок и размеры. Поэтому же у экрана есть режим
 * правки, а у сводной его нет.
 */

/**
 * Формы графика. Каждая отвечает на свой вопрос, и лишних здесь нет:
 *
 *   bar     — сравнить величины. Полосы горизонтальные: значения поля
 *             бывают длинными, и вертикальные столбцы их обрезают или
 *             ставят наискось. Со вторым полем — группа полос
 *             на категорию.
 *   stack   — доля в целом ПО КАЖДОЙ категории. Второе поле обязательно:
 *             без него стопка это одна полоса, то есть просто `bar`.
 *   line    — изменение во времени. Точки идут по возрастанию ключа,
 *             а не по величине: иначе это не время, а рейтинг.
 *             Со вторым полем — линия на каждое его значение.
 *   heatmap — «поле × поле», величина цветом. Там, где групп много
 *             в обе стороны, полосы кончаются раньше, чем данные.
 *   donut   — доля в целом по ВСЕЙ выборке.
 *   stat    — одно число. Отдельная форма, а не график из одной полосы:
 *             число, набранное крупно, читается сразу, а полоса требует
 *             сравнить себя с осью.
 */
export const CHART_KINDS = [
  "bar",
  "stack",
  "line",
  "heatmap",
  "funnel",
  "stream",
  "donut",
  "stat",
] as const;

export type ChartKind = (typeof CHART_KINDS)[number];

/**
 * Во что сворачивается дата перед группировкой.
 *
 * Без этого поле-дата бесполезно как поле разбивки: каждая строка
 * получает свой ключ до минуты, и «график по дням» выходит списком
 * из ста одиночных столбиков. Час и день недели — не украшение:
 * «когда к нам приходят заявки» отвечается только ими.
 *
 * Ключ бакета сортируемый, подпись — человеческая; собираются они
 * порознь (`bucketKey` и `bucketLabel`), потому что «пн» сортируется
 * не как «пн», а как первый день недели.
 */
export const CHART_BUCKETS = ["hour", "weekday", "day", "month"] as const;

export type ChartBucket = (typeof CHART_BUCKETS)[number] | "";

export type ChartConfig = {
  /** Ключ для порядка и перетаскивания. Не идентификатор в базе. */
  id: string;
  kind: ChartKind;
  /** Заголовок, заданный руками. Пусто — собирается из полей. */
  title: string;
  /** Поле, по значениям которого делится график. У `stat` не нужно. */
  groupSlug: string;
  /** Во что сворачивать дату поля разбивки. Пусто — брать как есть. */
  groupBucket: ChartBucket;
  /**
   * Второе поле: им категория делится на серии. Пусто — серия одна.
   *
   * Это `colSlug` сводной, ничего больше: матрицу она уже собирает, мы
   * её просто не спрашивали.
   */
  splitSlug: string;
  /** То же для второго поля: «день × час» — это одно поле двумя бакетами. */
  splitBucket: ChartBucket;
  aggregation: Aggregation;
  /** Поле, по которому считают. Для `count` не нужно. */
  valueSlug: string;
  /**
   * Ширина в колонках двенадцатиколоночной сетки: 6 — половина,
   * 12 — вся, 3 — четверть.
   *
   * Меньше четверти не бывает: в такую карточку не помещается ни одна
   * подпись, а график без подписей — это узор.
   */
  width: number;
  /**
   * Высота рядами сетки. Ряд — примерно та высота, которую карточка
   * занимала всегда; два и три нужны линии и ленте, где по вертикали
   * читается форма.
   */
  height: number;
};

/** Границы размера карточки. Ширина — колонки, высота — ряды. */
export const MIN_SPAN = 3;
export const MAX_SPAN = 12;
export const MIN_ROWS = 1;
export const MAX_ROWS = 3;

export const clampSpan = (value: number) =>
  Math.round(Math.min(MAX_SPAN, Math.max(MIN_SPAN, value)));

export const clampRows = (value: number) =>
  Math.round(Math.min(MAX_ROWS, Math.max(MIN_ROWS, value)));

/**
 * Формы, которые утверждают «части складываются в целое».
 *
 * Кольцо, стопка, воронка и лента: у всех подпись доли в процентах,
 * а доля бывает только от суммируемого. Для количества и суммы это
 * правда, для среднего, минимума и максимума — нет: половина кольца
 * «средний чек по Москве» не означает половину чего бы то ни было.
 * Поэтому у этих форм выбор способа сведения короче, а не «нарисуем,
 * а человек разберётся».
 */
const PART_KINDS = new Set<string>(["donut", "stack", "funnel", "stream"]);

/** Формы, которым второе поле обязательно: без него рисовать нечего. */
const NEEDS_SPLIT = new Set<string>(["stack", "heatmap"]);

/** Формы, у которых порядок задаёт список значений, а не их величина. */
const STAGE_KINDS = new Set<string>(["funnel", "stream"]);

export function aggregationAllowed(kind: ChartKind, aggregation: Aggregation): boolean {
  return !PART_KINDS.has(kind) || aggregation === "count" || aggregation === "sum";
}

/**
 * Умеет ли форма показать второе поле.
 *
 * Кольцу и числу делить нечего. Воронке и ленте — тоже: они про ОДИН
 * список стадий и доли в нём, а вторая ось превратила бы их в матрицу,
 * то есть в тепловую карту.
 */
export function splittable(kind: ChartKind): boolean {
  return kind === "stack" || !PART_KINDS.has(kind);
}

/**
 * Показывает ли форма стадии в их собственном порядке.
 *
 * У воронки порядок стадий — это и есть смысл: «новая заявка» стоит
 * перед «оплачено» не потому, что её больше. Отсортировать воронку
 * по величине значит стереть то, ради чего её рисуют.
 */
export function staged(kind: ChartKind): boolean {
  return STAGE_KINDS.has(kind);
}

/** Обязательно ли второе поле: без него у формы нет второй оси. */
export function needsSplit(kind: ChartKind): boolean {
  return NEEDS_SPLIT.has(kind);
}

/**
 * Сколько долей и серий рисуется, прежде чем хвост уходит в «Прочее».
 *
 * Семь — потолок палитры минус место под хвост, а не круглое число:
 * девятый оттенок пришлось бы выдумать, а выдуманный неотличим
 * от соседнего под дальтонизмом. Поэтому семь цветных и восьмое —
 * серое «Прочее».
 */
const SERIES_LIMIT = 7;

/** Сколько полос показывает рейтинг: это верхушка, а не весь список. */
const BAR_ROWS = 12;

/**
 * Столько же, но когда у каждой категории своя группа полос: двенадцать
 * категорий по восемь серий — это девяносто шесть полос в карточке.
 */
const GROUPED_ROWS = 8;

/**
 * Строк и колонок у тепловой карты больше: клетка занимает меньше
 * места, чем полоса, а палитра ей не нужна — она красит одним тоном.
 * Двадцать четыре колонки — это сутки по часам, четырнадцать строк —
 * две недели.
 */
const HEATMAP_ROWS = 14;
const HEATMAP_SPLITS = 24;

/**
 * Потолок перечислимой оси времени: год по дням.
 *
 * Нужен не ради красоты, а против выборки, где одна отметка стоит
 * в 2019 году: без него ось растянулась бы на тысячи пустых дней.
 */
const DOMAIN_LIMIT = 366;

/** Ключ свёрнутого хвоста. Пустым он быть не может — так зовут «пусто». */
export const OTHER_KEY = " other";

/** Точка графика: ключ для React, подпись для человека, число. */
export type ChartPoint = {
  key: string;
  label: string;
  /** Итог по категории — сумма (или свод) по всем сериям. */
  value: number;
  /** Клетки по сериям. Пусто — серия одна, и всё сказано в `value`. */
  cells: Map<string, number>;
};

/** Серия: значение второго поля. */
export type ChartSplit = { key: string; label: string };

export type ChartSeries = {
  points: ChartPoint[];
  /** Значения второго поля в порядке показа. Пусто — серия одна. */
  splits: ChartSplit[];
  /** Сумма долей кольца. У остальных форм — итог по всем точкам. */
  total: number;
  /** Сколько строк вошло в расчёт: ими подписан график. */
  count: number;
  /** Сколько категорий не поместилось. Ноль — показаны все. */
  hidden: number;
  /** Сколько серий не поместилось. Ноль — показаны все. */
  hiddenSplits: number;
};

const EMPTY: ChartSeries = {
  points: [],
  splits: [],
  total: 0,
  count: 0,
  hidden: 0,
  hiddenSplits: 0,
};

/** Готов ли график к показу: без поля рисовать нечего. */
export function chartReady(config: ChartConfig): boolean {
  if (config.aggregation !== "count" && !config.valueSlug) return false;
  if (config.kind === "stat") return true;
  if (!config.groupSlug) return false;

  return !NEEDS_SPLIT.has(config.kind) || Boolean(config.splitSlug);
}

/**
 * Ось графика: как превратить значение поля в ключ и в подпись.
 *
 * `bucket` сворачивает дату (час, день недели, день, месяц); без него
 * значение берётся как есть. `label` разворачивает ключ в человеческую
 * подпись — у связи это подпись записи, у списка подпись варианта,
 * у бакета имя часа или дня недели.
 */
export type ChartAxis = {
  label?: ((row: Item, value: string) => string) | undefined;
  bucket?: ((value: unknown) => string | null) | undefined;
};

export type ChartContext = {
  group?: ChartAxis | undefined;
  split?: ChartAxis | undefined;
  /**
   * Порядок стадий для воронки и ленты: значения поля в том порядке,
   * в каком их завёл админ (варианты списка). Пусто — порядок берётся
   * по величине, как у полос.
   */
  order?: string[] | undefined;
};

/*
 * Синтетические колонки, в которые проецируются оси перед сводкой.
 *
 * Своя колонка на каждую ось, а не правка исходной: «день × час» —
 * это ОДНО поле, свёрнутое двумя разными бакетами, и в общей колонке
 * два ключа затёрли бы друг друга.
 */
const GROUP_KEY = "__chart_group";
const SPLIT_KEY = "__chart_split";

/**
 * Точки графика из загруженных строк.
 *
 * Порядок задаёт форма, а не настройка: рейтингу нужен порядок
 * по величине, времени — по ключу. Лишнего переключателя здесь нет
 * намеренно — «линия, отсортированная по убыванию» это не график,
 * а горка.
 *
 * `context` разворачивает значения в подписи и сворачивает даты
 * в бакеты. Подписи считаются по ИСХОДНЫМ строкам и один раз
 * на значение: связанная запись приезжает рядом со ссылкой, и после
 * свода её уже не достать.
 */
export function chartSeries(
  rows: Item[],
  config: ChartConfig,
  context: ChartContext = {},
): ChartSeries {
  if (!chartReady(config)) return EMPTY;

  if (config.kind === "stat") {
    const values =
      config.aggregation === "count"
        ? rows.map(() => 1)
        : rows
            .map((row) => toNumber(row[config.valueSlug]))
            .filter((value): value is number => value !== null);

    const value = reduce(values, config.aggregation);
    return {
      ...EMPTY,
      points: [{ key: "", label: "", value, cells: new Map() }],
      total: value,
      count: values.length,
    };
  }

  const split = splittable(config.kind) ? config.splitSlug : "";
  /*
   * Оси проецируются в свои колонки: там уже свёрнутая дата, если бакет
   * задан, и исходное значение, если нет. Дальше ни сводка, ни подписи
   * про бакеты не знают — они просто группируют по ключу.
   */
  const projected = project(rows, config.groupSlug, split, context);

  const table = pivotTable(projected, {
    rowSlugs: [GROUP_KEY],
    colSlug: split ? SPLIT_KEY : "",
    valueSlug: config.valueSlug,
    aggregation: config.aggregation,
    /*
     * По ключу — линия, воронка и всё, что свёрнуто в единицы времени:
     * «понедельник, вторник, среда» это не рейтинг, и ставить среду
     * первой потому, что её больше, значит сломать ось. Остальные —
     * по величине. Стадии досортировываются ниже: сводка знает только
     * про подпись и итог, а порядок стадий приходит извне.
     */
    sort:
      config.kind === "line" || staged(config.kind) || config.groupBucket
        ? { by: { kind: "label" }, desc: false }
        : { by: { kind: "total" }, desc: true },
    /*
     * Строки без значения поля в график не идут. У сводной это выбор
     * человека, здесь — правило: «пусто» отдельной полосой съедает
     * место и цвет, а на кольце превращается в долю ничего.
     */
    skipEmpty: true,
  });

  const summable = config.aggregation === "count" || config.aggregation === "sum";
  const name = (slug: string, axis: ChartAxis | undefined) =>
    labelMap(projected, slug, axis?.label);

  /*
   * Серии. Отбираются по величине — иначе за потолком палитры остались бы
   * самые крупные, — а показываются по ключу: цвет обязан следовать
   * за значением, а не за его местом в рейтинге, иначе отбор строк
   * перекрашивает уцелевшие серии.
   *
   * ponytail: набор серий всё-таки зависит от выборки — отфильтровали
   * строки, восьмая серия ушла, цвета сдвинулись. Закреплять цвет
   * за значением намертво — когда попросят.
   */
  const splitLabels = name(SPLIT_KEY, context.split);
  /*
   * Ось времени перечисляется целиком, а не по встреченным значениям:
   * «сделки по часам» без пустых часов — это не сутки, а список тех
   * часов, когда что-то было. Пустая клетка здесь такая же измеренная,
   * как и полная.
   */
  const splitDomain = bucketDomain(config.splitBucket, table.columns);
  const splitKeys = splitDomain ?? table.columns;

  const ranked = splitKeys
    .map((key) => ({
      key,
      label: splitLabels.get(key) ?? labelOfKey(context.split, key),
      value: table.columnTotals.get(key) ?? 0,
    }))
    .sort((a, b) => b.value - a.value);

  /* Потолок серий — от палитры, а у тепловой карты палитры нет: она
     красит одним тоном, и двадцать четыре часа ей не мешают. */
  const seriesLimit = config.kind === "heatmap" ? HEATMAP_SPLITS : SERIES_LIMIT;
  const kept = splitDomain
    ? { items: ranked, hidden: 0 }
    : foldTail(ranked, seriesLimit, summable);

  const splits = split
    ? kept.items.map(({ key, label }) => ({ key, label })).sort(bySplitKey)
    : [];

  const groupLabels = name(GROUP_KEY, context.group);
  const shown = new Set(splits.map((item) => item.key));

  const groupDomain = bucketDomain(config.groupBucket, table.rows.map((row) => row.key));
  const byKey = new Map(table.rows.map((row) => [row.key, row]));

  const all = (groupDomain ?? table.rows.map((row) => row.key)).map((key) => {
    const row = byKey.get(key);

    return {
      key,
      label: groupLabels.get(key) ?? labelOfKey(context.group, key),
      value: row?.total ?? 0,
      cells: cellsOf(row?.cells ?? new Map(), shown, summable),
    };
  });

  /*
   * Стадии выстраиваются в свой порядок — тот, в каком варианты завёл
   * админ. Значения, которых в списке нет (данные старше настройки),
   * уходят в хвост: выбрасывать их нельзя, они тоже чьи-то сделки.
   *
   * Списка нет вовсе — у поля без вариантов, например у связи —
   * и тогда порядок по величине: «своего» порядка у таких значений
   * не существует, а сортировка по ключу выстроила бы воронку
   * по алфавиту uuid.
   */
  if (staged(config.kind)) {
    const at = new Map((context.order ?? []).map((key, index) => [key, index]));
    all.sort(
      (a, b) =>
        (at.get(a.key) ?? Infinity) - (at.get(b.key) ?? Infinity) || b.value - a.value,
    );
  }

  if (config.kind === "donut") {
    const slices = foldTail(
      all.map(({ key, label, value }) => ({ key, label, value })),
      SERIES_LIMIT,
      summable,
    );

    const points = slices.items.map((item) => ({ ...item, cells: new Map<string, number>() }));

    return {
      points,
      splits: [],
      total: points.reduce((sum, point) => sum + point.value, 0),
      count: table.count,
      hidden: slices.hidden,
      hiddenSplits: 0,
    };
  }

  /*
   * У полос и стопки хвост категорий именно отбрасывается, а не
   * сворачивается: это рейтинг, и «Прочее» в нём — полоса, которую
   * не с чем сравнивать. Сколько осталось за кадром, экран говорит числом.
   *
   * У оси времени отбрасывается НАЧАЛО, а не хвост: «последние
   * четырнадцать дней» — это то, что спрашивают, а «первые
   * четырнадцать за всю историю» не спрашивает никто.
   */
  const rowLimit = rowCap(config.kind, config.groupBucket, splits.length);
  const points =
    rowLimit === null
      ? all
      : config.groupBucket
        ? all.slice(-rowLimit)
        : all.slice(0, rowLimit);

  return {
    points,
    splits,
    total: table.total,
    count: table.count,
    hidden: all.length - points.length,
    hiddenSplits: kept.hidden,
  };
}

/**
 * Сколько категорий помещается. null — сколько есть.
 *
 * У линии это ось времени, у воронки — список стадий: обрезанная
 * воронка врёт процентами, а обрезанная ось времени просто кончается
 * не там, где данные.
 *
 * У часов и дней недели ось конечна сама по себе — двадцать четыре
 * и семь, — и резать её нечего: «последние двенадцать часов суток»
 * это не сутки.
 */
function rowCap(kind: ChartKind, bucket: ChartBucket, splits: number): number | null {
  if (kind === "line" || staged(kind)) return null;
  if (bucket === "hour" || bucket === "weekday") return null;
  if (kind === "heatmap") return HEATMAP_ROWS;

  return splits > 1 ? GROUPED_ROWS : BAR_ROWS;
}

/**
 * Полная ось для свёрнутого времени: `null` — перечислять нечего
 * (значение взято как есть, и «всех возможных» значений у него нет).
 *
 * У часа и дня недели ось известна заранее. У дня и месяца — отрезок
 * от первой отметки до последней: пропущенный день это ноль сделок,
 * и линия, которая его перескакивает, рисует неправду о своей форме.
 */
function bucketDomain(bucket: ChartBucket, keys: string[]): string[] | null {
  const pad = (n: number) => String(n).padStart(2, "0");

  if (bucket === "hour") return Array.from({ length: 24 }, (_, hour) => pad(hour));
  if (bucket === "weekday") return Array.from({ length: 7 }, (_, day) => String(day + 1));
  if (bucket !== "day" && bucket !== "month") return null;
  if (!keys.length) return [];

  const sorted = [...keys].sort();
  const first = sorted[0] ?? "";
  const last = sorted[sorted.length - 1] ?? "";

  const parse = (key: string) => key.split("-").map(Number);
  const [fromYear = 0, fromMonth = 1, fromDay = 1] = parse(first);
  const [toYear = 0, toMonth = 1, toDay = 1] = parse(last);

  const out: string[] = [];

  if (bucket === "month") {
    for (
      let cursor = new Date(Date.UTC(fromYear, fromMonth - 1, 1));
      cursor <= new Date(Date.UTC(toYear, toMonth - 1, 1)) && out.length < DOMAIN_LIMIT;
      cursor.setUTCMonth(cursor.getUTCMonth() + 1)
    ) {
      out.push(`${cursor.getUTCFullYear()}-${pad(cursor.getUTCMonth() + 1)}`);
    }

    return out;
  }

  for (
    let cursor = new Date(Date.UTC(fromYear, fromMonth - 1, fromDay));
    cursor <= new Date(Date.UTC(toYear, toMonth - 1, toDay)) && out.length < DOMAIN_LIMIT;
    cursor.setUTCDate(cursor.getUTCDate() + 1)
  ) {
    out.push(
      `${cursor.getUTCFullYear()}-${pad(cursor.getUTCMonth() + 1)}-${pad(cursor.getUTCDate())}`,
    );
  }

  return out;
}

/**
 * Подпись ключа, которого не было в данных.
 *
 * У свёрнутого времени подпись собирается из самого ключа и строка
 * для неё не нужна — этим пустые часы и отличаются от пустых значений
 * поля, которых просто нет.
 */
function labelOfKey(axis: ChartAxis | undefined, key: string): string {
  return axis?.label?.({} as Item, key) || key;
}

/**
 * Оси в свои колонки: свёрнутая дата или значение как есть.
 *
 * Строки копируются поверхностно — исходные принадлежат кэшу запроса,
 * и трогать их нельзя. Копия дешёвая: на экран графиков приезжает
 * не больше порции строк.
 */
function project(rows: Item[], groupSlug: string, splitSlug: string, context: ChartContext): Item[] {
  const key = (row: Item, slug: string, axis: ChartAxis | undefined): unknown => {
    if (!slug) return null;
    return axis?.bucket ? axis.bucket(row[slug]) : row[slug];
  };

  return rows.map((row) => ({
    ...row,
    [GROUP_KEY]: key(row, groupSlug, context.group),
    [SPLIT_KEY]: key(row, splitSlug, context.split),
  }));
}

/** Порядок серий — по ключу, «Прочее» всегда последним. */
function bySplitKey(a: ChartSplit, b: ChartSplit): number {
  if (a.key === OTHER_KEY) return 1;
  if (b.key === OTHER_KEY) return -1;

  return a.key.localeCompare(b.key);
}

/**
 * Клетки категории по показанным сериям.
 *
 * Хвост складывается в «Прочее» там, где значения суммируемы, — иначе
 * стопка перестала бы быть целым. Где не суммируемы (среднее, минимум,
 * максимум), хвост просто не показывается: среднее средних — не среднее.
 */
function cellsOf(
  cells: Map<string, number>,
  shown: ReadonlySet<string>,
  summable: boolean,
): Map<string, number> {
  if (!shown.size) return new Map();

  const kept = new Map<string, number>();
  let other = 0;

  for (const [key, value] of cells) {
    if (shown.has(key)) kept.set(key, value);
    else if (summable) other += value;
  }

  if (shown.has(OTHER_KEY) && other) kept.set(OTHER_KEY, other);
  return kept;
}

type Tail = { key: string; label: string; value: number };

/**
 * Верхушка списка и его хвост.
 *
 * Суммируемое сворачивается в «Прочее»: кольцо и стопка без него
 * перестали бы быть целым, и каждая доля врала бы о своём размере.
 * Несуммируемое отбрасывается — сложить средние нельзя, — и вызывающий
 * говорит числом, сколько не поместилось.
 */
function foldTail(items: Tail[], limit: number, summable: boolean): { items: Tail[]; hidden: number } {
  if (items.length <= limit + 1) return { items, hidden: 0 };

  const head = items.slice(0, limit);
  const tail = items.slice(limit);

  if (!summable) return { items: head, hidden: tail.length };

  const other = tail.reduce((sum, item) => sum + item.value, 0);
  return { items: [...head, { key: OTHER_KEY, label: "", value: other }], hidden: tail.length };
}

/**
 * Значение → подпись, по первой встреченной строке с этим значением.
 *
 * Пустая подпись — значит разворачивать нечем (у связи не заданы поля
 * показа), и остаётся само значение: пустая полоса хуже uuid.
 */
function labelMap(
  rows: Item[],
  slug: string,
  labelOf: ((row: Item, value: string) => string) | undefined,
): Map<string, string> {
  const labels = new Map<string, string>();
  if (!slug || !labelOf) return labels;

  for (const row of rows) {
    const value = pivotKey(row[slug]);
    if (!labels.has(value)) labels.set(value, labelOf(row, value));
  }

  return labels;
}

/**
 * Дата → ключ бакета. `null` — значение не разобралось: такая строка
 * в расчёт не идёт, как и строка с пустым полем.
 *
 * Ключ сортируемый, а не читаемый: час это «08», день недели —
 * «1»…«7» от понедельника, месяц — «2026-08». Порядок ключей задаёт
 * порядок столбцов, и «пятница» между «понедельником» и «средой»
 * по алфавиту — не то, что нужно.
 */
export function bucketKey(value: unknown, kind: DateKind, bucket: ChartBucket): string | null {
  if (!bucket) return value === null || value === undefined ? null : String(value);

  const parsed = toDateValue(value, kind);
  if (!parsed) return null;

  // Значение без пояса лежит в Date как UTC — так же его и читаем,
  // иначе браузер пересчитает в местное время и сдвинет день.
  const date = parsed.date;
  const get = parsed.naive
    ? {
        year: date.getUTCFullYear(),
        month: date.getUTCMonth() + 1,
        day: date.getUTCDate(),
        hour: date.getUTCHours(),
        weekday: date.getUTCDay(),
      }
    : {
        year: date.getFullYear(),
        month: date.getMonth() + 1,
        day: date.getDate(),
        hour: date.getHours(),
        weekday: date.getDay(),
      };

  const pad = (n: number) => String(n).padStart(2, "0");

  switch (bucket) {
    case "hour":
      return pad(get.hour);
    case "weekday":
      // Воскресенье у JS нулевое, а неделя начинается с понедельника.
      return String(get.weekday === 0 ? 7 : get.weekday);
    case "day":
      return `${get.year}-${pad(get.month)}-${pad(get.day)}`;
    case "month":
      return `${get.year}-${pad(get.month)}`;
  }
}

/** Ключ бакета → подпись на языке интерфейса. */
export function bucketLabel(key: string, bucket: ChartBucket, locale: string): string {
  if (!bucket) return key;

  switch (bucket) {
    case "hour":
      return `${key}:00`;
    case "weekday": {
      /* 1 января 2024 — понедельник; от него и отсчитываем, чтобы имена
         дней приехали из Intl, а не из списка в коде. */
      const date = new Date(Date.UTC(2024, 0, Number(key)));
      return new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(date);
    }
    case "day": {
      const [year, month, day] = key.split("-").map(Number);
      const date = new Date(Date.UTC(year ?? 0, (month ?? 1) - 1, day ?? 1));
      return new Intl.DateTimeFormat(locale, {
        day: "2-digit",
        month: "short",
        timeZone: "UTC",
      }).format(date);
    }
    case "month": {
      const [year, month] = key.split("-").map(Number);
      const date = new Date(Date.UTC(year ?? 0, (month ?? 1) - 1, 1));
      return new Intl.DateTimeFormat(locale, {
        month: "short",
        year: "numeric",
        timeZone: "UTC",
      }).format(date);
    }
  }
}

/**
 * Новый график — пустое место под настройку, а не готовая карточка.
 *
 * Поле НЕ подставляется. Подставлялось первое из колонок, и это давало
 * ровно то, на что жалуются: добавил график — получил близнеца соседнего,
 * потому что у обоих «количество по первому полю». Новая карточка обязана
 * выглядеть новой; её первое состояние — «выберите поле», и оно же
 * говорит, что делать дальше.
 *
 * Форма при этом задана: полосы отвечают на самый частый вопрос
 * («чего сколько»), и менять её приходится реже, чем поле.
 */
export function newChart(): ChartConfig {
  return {
    id: crypto.randomUUID(),
    kind: "bar",
    title: "",
    groupSlug: "",
    groupBucket: "",
    splitSlug: "",
    splitBucket: "",
    aggregation: "count",
    valueSlug: "",
    width: 6,
    height: MIN_ROWS,
  };
}

/**
 * Графики из `attributes.charts`.
 *
 * Ключ наш, старая админка про него не знает, — но разбирается он всё
 * равно с недоверием: attributes это свободный JSONB, куда пишет и она,
 * и скрипты миграций, и рука в базе. Мусорная запись должна пропасть
 * из списка, а не уронить экран.
 */
export function toCharts(value: unknown): ChartConfig[] {
  if (!Array.isArray(value)) return [];

  return value
    .map((item, index): ChartConfig | null => {
      if (typeof item !== "object" || item === null) return null;
      const raw = item as Record<string, unknown>;

      const kind = CHART_KINDS.find((name) => name === raw["kind"]);
      if (!kind) return null;

      const aggregation = toAggregation(
        typeof raw["aggregation"] === "string" ? raw["aggregation"] : undefined,
      );
      const split = typeof raw["split_slug"] === "string" ? raw["split_slug"] : "";
      const bucket = (value: unknown): ChartBucket =>
        CHART_BUCKETS.find((item) => item === value) ?? "";

      return {
        /*
         * Своего id у графика может не быть — например, у записи,
         * дописанной руками в базу. Порядковый номер годится: список
         * короткий и переписывается целиком при каждой правке.
         */
        id: typeof raw["id"] === "string" && raw["id"] ? raw["id"] : `chart-${index}`,
        kind,
        title: typeof raw["title"] === "string" ? raw["title"] : "",
        groupSlug: typeof raw["group_slug"] === "string" ? raw["group_slug"] : "",
        groupBucket: bucket(raw["group_bucket"]),
        // Кольцу и числу делить нечего: чужая настройка не должна
        // всплыть при следующей смене формы.
        splitSlug: splittable(kind) ? split : "",
        splitBucket: splittable(kind) ? bucket(raw["split_bucket"]) : "",
        // Доля от среднего не бывает: настройка из базы приводится
        // к допустимой, иначе экран рисовал бы бессмыслицу.
        aggregation: aggregationAllowed(kind, aggregation) ? aggregation : "count",
        valueSlug: typeof raw["value_slug"] === "string" ? raw["value_slug"] : "",
        width: toSpan(raw["width"]),
        height: clampRows(typeof raw["height"] === "number" ? raw["height"] : MIN_ROWS),
      };
    })
    .filter((chart): chart is ChartConfig => chart !== null);
}

/**
 * Ширина из хранилища.
 *
 * До двенадцатиколоночной сетки ширина была «1 — половина, 2 — вся»,
 * и такие графики лежат в проектах. Значения 1 и 2 в новой шкале
 * не встречаются вовсе — минимум четверть, то есть 3, — поэтому
 * старое читается однозначно и переезжает само, без миграции в базе.
 */
function toSpan(value: unknown): number {
  if (value === 1) return 6;
  if (value === 2) return MAX_SPAN;

  return clampSpan(typeof value === "number" ? value : 6);
}

/** Обратно в attributes. Ключи змейкой — как всё остальное в этом объекте. */
export function toChartsAttribute(charts: ChartConfig[]): Record<string, unknown>[] {
  return charts.map((chart) => ({
    id: chart.id,
    kind: chart.kind,
    title: chart.title,
    group_slug: chart.groupSlug,
    group_bucket: chart.groupBucket,
    split_slug: chart.splitSlug,
    split_bucket: chart.splitBucket,
    aggregation: chart.aggregation,
    value_slug: chart.valueSlug,
    width: chart.width,
    height: chart.height,
  }));
}
