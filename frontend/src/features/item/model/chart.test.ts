import { expect, test } from "vitest";
import {
  OTHER_KEY,
  aggregationAllowed,
  bucketKey,
  chartReady,
  chartSeries,
  newChart,
  splittable,
  toCharts,
  toChartsAttribute,
  type ChartConfig,
} from "./chart";
import type { Item } from "./types";

const config = (over: Partial<ChartConfig> = {}): ChartConfig => ({
  id: "c1",
  kind: "bar",
  title: "",
  groupSlug: "status",
  groupBucket: "",
  splitSlug: "",
  splitBucket: "",
  aggregation: "count",
  valueSlug: "",
  width: 6,
  height: 1,
  ...over,
});

/** По одной строке на значение: ключ виден в подписи, число — в полосе. */
const spread = (values: string[]): Item[] =>
  values.map((status, index) => ({ guid: String(index), status, amount: index + 1 }));

/** Строки с двумя полями: категория и разрез. */
const matrix = (pairs: [string, string][]): Item[] =>
  pairs.map(([status, city], index) => ({ guid: String(index), status, city, amount: index + 1 }));

test("полосы идут по убыванию, а линия — по возрастанию ключа", () => {
  const rows: Item[] = spread(["b", "a", "a", "c", "c", "c"]);

  expect(chartSeries(rows, config()).points.map((point) => point.key)).toEqual(["c", "a", "b"]);
  expect(chartSeries(rows, config({ kind: "line" })).points.map((point) => point.key)).toEqual([
    "a",
    "b",
    "c",
  ]);
});

test("хвост кольца сворачивается в «Прочее», и доли по-прежнему складываются в целое", () => {
  // Девять значений по одной строке: семь долей, восьмая — хвост из двух.
  const series = chartSeries(spread("abcdefghi".split("")), config({ kind: "donut" }));

  expect(series.points).toHaveLength(8);
  expect(series.points.at(-1)?.key).toBe(OTHER_KEY);
  expect(series.points.at(-1)?.value).toBe(2);
  expect(series.total).toBe(9);
  expect(series.hidden).toBe(2);
});

test("восемь долей помещаются целиком: сворачивать нечего", () => {
  const series = chartSeries(spread("abcdefgh".split("")), config({ kind: "donut" }));

  expect(series.points).toHaveLength(8);
  expect(series.points.some((point) => point.key === OTHER_KEY)).toBe(false);
  expect(series.hidden).toBe(0);
});

test("у полос хвост отбрасывается, а не сворачивается: это рейтинг", () => {
  const series = chartSeries(spread("abcdefghijklmno".split("")), config());

  expect(series.points).toHaveLength(12);
  expect(series.points.some((point) => point.key === OTHER_KEY)).toBe(false);
  expect(series.hidden).toBe(3);
});

test("строки без значения поля в график не идут", () => {
  const rows: Item[] = [
    { guid: "1", status: "todo" },
    { guid: "2", status: null },
    { guid: "3" },
  ];

  const series = chartSeries(rows, config());
  expect(series.points).toHaveLength(1);
  expect(series.points[0]?.key).toBe("todo");
  expect(series.count).toBe(1);
});

test("stat считает по всем строкам, а среднее — только по тем, где есть число", () => {
  const rows: Item[] = [
    { guid: "1", amount: 10 },
    { guid: "2", amount: 20 },
    { guid: "3", amount: "не число" },
  ];

  const stat = config({ kind: "stat", groupSlug: "", aggregation: "avg", valueSlug: "amount" });
  expect(chartSeries(rows, stat).total).toBe(15);
  expect(chartSeries(rows, stat).count).toBe(2);
});

test("без поля график не готов и не считается", () => {
  expect(chartReady(config({ groupSlug: "" }))).toBe(false);
  // Считать «сумму» нечем, пока не выбрано поле значения.
  expect(chartReady(config({ aggregation: "sum" }))).toBe(false);
  expect(chartSeries(spread(["a"]), config({ groupSlug: "" })).points).toEqual([]);
});

test("стопке и тепловой карте второе поле обязательно", () => {
  expect(chartReady(config({ kind: "stack" }))).toBe(false);
  expect(chartReady(config({ kind: "heatmap" }))).toBe(false);
  expect(chartReady(config({ kind: "stack", splitSlug: "city" }))).toBe(true);
  // Полосам и линии оно нужно не больше, чем раньше.
  expect(chartReady(config({ kind: "bar" }))).toBe(true);
});

test("второе поле разбивает категорию на серии", () => {
  const rows = matrix([
    ["todo", "msk"],
    ["todo", "msk"],
    ["todo", "spb"],
    ["done", "spb"],
  ]);

  const series = chartSeries(rows, config({ splitSlug: "city" }));

  expect(series.splits.map((split) => split.key)).toEqual(["msk", "spb"]);
  const todo = series.points.find((point) => point.key === "todo");
  expect(todo?.value).toBe(3);
  expect(todo?.cells.get("msk")).toBe(2);
  expect(todo?.cells.get("spb")).toBe(1);
});

test("серии отбираются по величине, а показываются по ключу", () => {
  /*
   * Девять разрезов: восьмой и девятый по величине уходят в хвост,
   * а уцелевшие идут по алфавиту — цвет следует за значением, а не
   * за местом в рейтинге.
   */
  const rows = matrix(
    "abcdefghi"
      .split("")
      .flatMap((city, index) =>
        Array.from({ length: 9 - index }, () => ["todo", city] as [string, string]),
      ),
  );

  const series = chartSeries(rows, config({ splitSlug: "city" }));

  expect(series.splits.map((split) => split.key)).toEqual([
    "a",
    "b",
    "c",
    "d",
    "e",
    "f",
    "g",
    OTHER_KEY,
  ]);
  expect(series.hiddenSplits).toBe(2);
  // Хвост сложен, а не потерян: 2 + 1 строки девятого и восьмого города.
  expect(series.points[0]?.cells.get(OTHER_KEY)).toBe(3);
});

test("несуммируемый хвост серий отбрасывается, а не складывается", () => {
  const rows = matrix(
    "abcdefghi".split("").map((city) => ["todo", city] as [string, string]),
  );

  const series = chartSeries(
    rows,
    config({ splitSlug: "city", aggregation: "avg", valueSlug: "amount" }),
  );

  expect(series.splits).toHaveLength(7);
  expect(series.splits.some((split) => split.key === OTHER_KEY)).toBe(false);
  expect(series.hiddenSplits).toBe(2);
});

test("доля в целом не считается от среднего", () => {
  expect(aggregationAllowed("donut", "avg")).toBe(false);
  expect(aggregationAllowed("stack", "min")).toBe(false);
  expect(aggregationAllowed("stack", "sum")).toBe(true);
  // Остальным формам среднее не мешает: они ничего не обещают о целом.
  expect(aggregationAllowed("bar", "avg")).toBe(true);
  expect(aggregationAllowed("heatmap", "max")).toBe(true);
});

test("группа полос показывает меньше категорий, чем одиночные полосы", () => {
  const rows = matrix(
    "abcdefghijkl".split("").flatMap((status) => [
      [status, "msk"] as [string, string],
      [status, "spb"] as [string, string],
    ]),
  );

  expect(chartSeries(rows, config()).points).toHaveLength(12);
  expect(chartSeries(rows, config({ splitSlug: "city" })).points).toHaveLength(8);
});

test("час и день недели дают сортируемый ключ, а не подпись", () => {
  // Дата без пояса читается как настенные часы: 09:30 это девять утра
  // в любом браузере, а не «девять минус сдвиг».
  expect(bucketKey("2026-08-25 09:30", "datetime_naive", "hour")).toBe("09");
  // 25 августа 2026 — вторник, второй день недели.
  expect(bucketKey("2026-08-25 09:30", "datetime_naive", "weekday")).toBe("2");
  expect(bucketKey("2026-08-25 09:30", "datetime_naive", "day")).toBe("2026-08-25");
  expect(bucketKey("2026-08-25 09:30", "datetime_naive", "month")).toBe("2026-08");
  // Воскресенье у JS нулевое, а неделя кончается им.
  expect(bucketKey("2026-08-30", "date", "weekday")).toBe("7");
  // Не разобралось — строка в расчёт не идёт.
  expect(bucketKey("позавчера", "date", "day")).toBeNull();
  expect(bucketKey(null, "date", "hour")).toBeNull();
});

test("без бакета значение берётся как есть", () => {
  expect(bucketKey("todo", "date", "")).toBe("todo");
  expect(bucketKey(null, "date", "")).toBeNull();
});

test("бакет сворачивает даты в общие столбцы", () => {
  const rows: Item[] = [
    { guid: "1", at: "2026-08-25 09:10" },
    { guid: "2", at: "2026-08-25 09:50" },
    { guid: "3", at: "2026-08-26 14:00" },
  ];

  const byHour = chartSeries(rows, config({ groupSlug: "at", groupBucket: "hour" }), {
    group: { bucket: (value) => bucketKey(value, "datetime_naive", "hour") },
  });

  // Сутки перечислены целиком: пустой час — такой же час, как и полный.
  expect(byHour.points).toHaveLength(24);
  expect(byHour.points[9]).toMatchObject({ key: "09", value: 2 });
  expect(byHour.points[14]).toMatchObject({ key: "14", value: 1 });
  expect(byHour.points[3]).toMatchObject({ key: "03", value: 0 });
});

test("свёрнутое время идёт по часам, а не по величине", () => {
  const rows: Item[] = [
    { guid: "1", at: "2026-08-25 18:00" },
    { guid: "2", at: "2026-08-25 09:00" },
    { guid: "3", at: "2026-08-25 18:30" },
  ];

  // 18 часов больше по количеству, но 09 стоит раньше: это ось, а не рейтинг.
  const series = chartSeries(rows, config({ groupSlug: "at", groupBucket: "hour" }), {
    group: { bucket: (value) => bucketKey(value, "datetime_naive", "hour") },
  });

  expect(series.points.map((point) => point.key)).toEqual(
    Array.from({ length: 24 }, (_, hour) => String(hour).padStart(2, "0")),
  );
  expect(series.points[9]?.value).toBe(1);
  expect(series.points[18]?.value).toBe(2);
});

test("дни идут подряд: пропущенный день это ноль, а не разрыв оси", () => {
  const rows: Item[] = [
    { guid: "1", at: "2026-08-25" },
    { guid: "2", at: "2026-08-28" },
  ];

  const series = chartSeries(rows, config({ kind: "line", groupSlug: "at", groupBucket: "day" }), {
    group: { bucket: (value) => bucketKey(value, "date", "day") },
  });

  expect(series.points.map((point) => point.key)).toEqual([
    "2026-08-25",
    "2026-08-26",
    "2026-08-27",
    "2026-08-28",
  ]);
  expect(series.points[1]?.value).toBe(0);
});

test("у оси времени обрезается начало, а не хвост: спрашивают последние дни", () => {
  // Двадцать дней подряд при потолке тепловой карты в четырнадцать.
  const rows: Item[] = Array.from({ length: 20 }, (_, index) => ({
    guid: String(index),
    at: `2026-08-${String(index + 1).padStart(2, "0")}`,
  }));

  const series = chartSeries(
    rows,
    config({ kind: "heatmap", groupSlug: "at", groupBucket: "day", splitSlug: "at" }),
    {
      group: { bucket: (value) => bucketKey(value, "date", "day") },
      split: { bucket: (value) => bucketKey(value, "date", "hour") },
    },
  );

  expect(series.points).toHaveLength(14);
  expect(series.points[0]?.key).toBe("2026-08-07");
  expect(series.points.at(-1)?.key).toBe("2026-08-20");
  expect(series.hidden).toBe(6);
});

test("день × час: одно поле двумя бакетами не затирает само себя", () => {
  const rows: Item[] = [
    { guid: "1", at: "2026-08-25 09:10" },
    { guid: "2", at: "2026-08-25 14:00" },
    { guid: "3", at: "2026-08-26 09:30" },
  ];

  const series = chartSeries(
    rows,
    config({ kind: "heatmap", groupSlug: "at", splitSlug: "at" }),
    {
      group: { bucket: (value) => bucketKey(value, "datetime_naive", "day") },
      split: { bucket: (value) => bucketKey(value, "datetime_naive", "hour") },
    },
  );

  expect(series.splits.map((split) => split.key)).toEqual(["09", "14"]);
  const first = series.points.find((point) => point.key === "2026-08-25");
  expect(first?.cells.get("09")).toBe(1);
  expect(first?.cells.get("14")).toBe(1);
  expect(first?.value).toBe(2);
});

test("воронка идёт по порядку стадий, а не по величине", () => {
  const rows = spread(["paid", "paid", "paid", "new", "call"]);
  const order = ["new", "call", "paid"];

  const funnel = chartSeries(rows, config({ kind: "funnel" }), { order });
  expect(funnel.points.map((point) => point.key)).toEqual(order);

  // Полосам тот же порядок не навязывается: там рейтинг.
  expect(chartSeries(rows, config(), { order }).points[0]?.key).toBe("paid");
});

test("без списка стадий воронка идёт по величине, а не по алфавиту ключа", () => {
  // У поля-связи вариантов нет вовсе, и «своего» порядка у значений
  // не существует: uuid по алфавиту — это не порядок стадий.
  const series = chartSeries(spread(["b", "a", "a", "c", "c", "c"]), config({ kind: "funnel" }));
  expect(series.points.map((point) => point.key)).toEqual(["c", "a", "b"]);
});

test("стадия, которой нет в списке вариантов, уходит в хвост, а не пропадает", () => {
  const rows = spread(["new", "древняя"]);
  const funnel = chartSeries(rows, config({ kind: "funnel" }), { order: ["new", "paid"] });

  expect(funnel.points.map((point) => point.key)).toEqual(["new", "древняя"]);
});

test("воронка и лента второго поля не берут: у них одна ось", () => {
  expect(splittable("funnel")).toBe(false);
  expect(splittable("stream")).toBe(false);
  expect(splittable("stack")).toBe(true);
  // И доли считают только от суммируемого — как кольцо.
  expect(aggregationAllowed("funnel", "avg")).toBe(false);
});

test("воронка не обрезается: обрезанная врёт процентами", () => {
  const series = chartSeries(spread("abcdefghijklmno".split("")), config({ kind: "funnel" }));
  expect(series.points).toHaveLength(15);
  expect(series.hidden).toBe(0);
});

test("новый график рождается пустым, а не близнецом соседнего", () => {
  const chart = newChart();

  // Поле не подставляется: иначе два добавленных подряд графика
  // выглядят одинаково — «количество по первому полю» у обоих.
  expect(chart.groupSlug).toBe("");
  expect(chartReady(chart)).toBe(false);
  // А форма задана: полосы отвечают на самый частый вопрос.
  expect(chart.kind).toBe("bar");
  // Идентификаторы разные — иначе перетаскивание путает карточки.
  expect(newChart().id).not.toBe(chart.id);
});

test("мусор в attributes выбрасывается, а не роняет экран", () => {
  expect(toCharts(undefined)).toEqual([]);
  expect(toCharts("charts")).toEqual([]);
  // Форма — единственное обязательное поле: без неё рисовать нечем.
  expect(toCharts([null, 7, {}, { kind: "pie" }])).toEqual([]);
});

test("кольцо от среднего приводится к количеству при чтении", () => {
  const [chart] = toCharts([{ kind: "donut", aggregation: "avg", group_slug: "status" }]);
  expect(chart?.aggregation).toBe("count");
});

test("кольцу и числу второе поле не достаётся даже из базы", () => {
  const [donut] = toCharts([{ kind: "donut", group_slug: "status", split_slug: "city" }]);
  expect(donut?.splitSlug).toBe("");
});

test("запись и чтение сходятся: график переживает круг через attributes", () => {
  const charts = [
    config({ kind: "stack", splitSlug: "city", aggregation: "sum", valueSlug: "amount", width: 12 }),
  ];

  expect(toCharts(toChartsAttribute(charts))).toEqual(charts);
});

test("старая ширина переезжает в двенадцать колонок сама", () => {
  // «1 — половина, 2 — вся» лежит в проектах; в новой шкале минимум
  // четверть, поэтому единица и двойка читаются однозначно.
  expect(toCharts([{ kind: "bar", width: 1 }])[0]?.width).toBe(6);
  expect(toCharts([{ kind: "bar", width: 2 }])[0]?.width).toBe(12);
  // Новые значения проходят как есть, мусор — к умолчанию и границам.
  expect(toCharts([{ kind: "bar", width: 4 }])[0]?.width).toBe(4);
  expect(toCharts([{ kind: "bar", width: 99 }])[0]?.width).toBe(12);
  expect(toCharts([{ kind: "bar" }])[0]?.width).toBe(6);
});

test("высота держится в границах рядов", () => {
  expect(toCharts([{ kind: "bar", height: 2 }])[0]?.height).toBe(2);
  expect(toCharts([{ kind: "bar", height: 9 }])[0]?.height).toBe(3);
  expect(toCharts([{ kind: "bar" }])[0]?.height).toBe(1);
});
