import { Fragment, useRef, useState, type KeyboardEvent, type PointerEvent, type RefObject } from "react";
import {
  IconChartBar,
  IconChartDonut,
  IconChartFunnel,
  IconChartLine,
  IconColumns3,
  IconGridDots,
  IconGripVertical,
  IconLayoutColumns,
  IconNumber123,
  IconPlus,
  IconTrash,
  IconWaveSine,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field, type Relation } from "@/features/table";
import i18n, { type TranslationKey } from "@/shared/lib/i18n";
import { moveBefore } from "@/shared/lib/order";
import { Button } from "@/shared/ui/button";
import { CommitInput } from "@/shared/ui/commit-input";
import { Dropdown } from "@/shared/ui/dropdown";
import { Icon } from "@/shared/ui/icon";
import { Tooltip } from "@/shared/ui/tooltip";
import type { DateKind } from "@/shared/lib/date-value";
import {
  CHART_BUCKETS,
  CHART_KINDS,
  OTHER_KEY,
  MAX_ROWS,
  MAX_SPAN,
  MIN_ROWS,
  MIN_SPAN,
  aggregationAllowed,
  bucketKey,
  bucketLabel,
  chartReady,
  chartSeries,
  needsSplit,
  newChart,
  splittable,
  staged,
  type ChartBucket,
  type ChartConfig,
  type ChartKind,
  type ChartSeries,
  type ChartSplit,
} from "../model/chart";
import { cellKind } from "../model/cell-kind";
import { AGGREGATIONS, NUMERIC_FIELDS, type Aggregation } from "../model/pivot";
import { valueLabelOf } from "../model/relation";
import type { Item } from "../model/types";

/**
 * Экран CHART: несколько графиков по загруженным строкам.
 *
 * Конструктор, а не готовая панель: набор графиков, их порядок и ширину
 * задаёт админ и видят все. Поэтому настройки уезжают в view
 * (`attributes.charts`), а не в адрес, — в отличие от сводной, где то же
 * самое означает «как я сейчас смотрю».
 *
 * Правки уходят сразу, без кнопки «Сохранить»: PUT view подменяет кэш
 * ещё до ответа сервера (features/view/api/views, onMutate), и брошенный
 * мышью график встаёт на место мгновенно. Черновик поверх этого добавил
 * бы второе состояние, которое нужно мирить с первым.
 *
 * Режим правки — только чтобы не показывать ручки и списки тем, кто
 * пришёл посмотреть на числа.
 */
export function ChartView({
  charts,
  columns,
  relations,
  rows,
  language,
  loaded,
  total,
  sample,
  onSample,
  onCharts,
}: {
  charts: ChartConfig[];
  /** Колонки view: из них выбирают поля графика. */
  columns: Field[];
  /** Связи схемы: ими разворачиваются подписи полей-ссылок. */
  relations: Relation[];
  rows: Item[];
  language: string;
  /** Приехали не все строки — счётчик тогда врать не должен. */
  loaded: boolean;
  /** Сколько строк под этим отбором всего, по данным сервера. */
  total: number;
  /** Сколько строк спрошено: по ним и посчитано. */
  sample: number;
  onSample: (next: number) => void;
  /** Нет обработчика — нет и режима правки: роль пришла посмотреть. */
  onCharts?: ((next: ChartConfig[]) => void) | undefined;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [dragged, setDragged] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);

  const editable = Boolean(onCharts);
  const editMode = editing && Boolean(onCharts);

  const apply = (next: ChartConfig[]) => onCharts?.(next);

  const change = (id: string, patch: Partial<ChartConfig>) =>
    apply(charts.map((chart) => (chart.id === id ? { ...chart, ...patch } : chart)));

  /** Перестановка отдаётся списком целиком: порядок — одно значение. */
  const reorder = (ids: string[]) =>
    apply(
      ids
        .map((id) => charts.find((chart) => chart.id === id))
        .filter((chart): chart is ChartConfig => chart !== undefined),
    );

  const move = (id: string, delta: number) => {
    const ids = charts.map((chart) => chart.id);
    const from = ids.indexOf(id);
    const to = from + delta;
    if (from === -1 || to < 0 || to >= ids.length) return;

    const next = [...ids];
    const [moved] = next.splice(from, 1);
    if (moved) next.splice(to, 0, moved);
    reorder(next);
  };

  const drop = (target: string) => {
    if (dragged && dragged !== target) reorder(moveBefore(charts.map((chart) => chart.id), dragged, target));
    setDragged(null);
    setOver(null);
  };

  /*
   * Новый график сразу открывает режим правки: он приходит ненастроенным,
   * и оставить человека наедине с карточкой «выберите поле», у которой
   * не видно списков, — это тупик.
   */
  const add = () => {
    apply([...charts, newChart()]);
    setEditing(true);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/*
       * Полоса экрана. Кнопки правки — только тому, кому можно менять
       * view; выбор выборки — всем: это «как я сейчас смотрю», и оно же
       * решает, по скольким строкам посчитаны числа.
       */}
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3">
        {editable && (
          <>
            {/*
             * Обычная кнопка, а не бледная надпись: раскладка графиков
             * собирается руками, и вход в сборку — главное действие
             * экрана, а не служебная ссылка в углу.
             */}
            <Button
              type="button"
              size="sm"
              variant={editMode ? "primary" : "secondary"}
              onClick={() => setEditing((open) => !open)}
            >
              <Icon as={IconLayoutColumns} size={14} />
              {t(editMode ? "chart.done" : "chart.edit")}
            </Button>

            {editMode && (
              <>
                <Button type="button" size="sm" variant="secondary" onClick={add}>
                  <Icon as={IconPlus} size={14} />
                  {t("chart.add")}
                </Button>

                {/* Подсказка про перетаскивание: ручка на карточке
                    появляется только здесь, и сама себя не объясняет. */}
                <span className="text-2xs text-fg-subtle">{t("chart.dragHint")}</span>
              </>
            )}
          </>
        )}

        <Sample
          shown={rows.length}
          total={total}
          loaded={loaded}
          sample={sample}
          onSample={onSample}
        />
      </div>

      <div className="min-h-0 flex-1 overflow-auto p-3">
        {!charts.length ? (
          /* Пустой экран предлагает действие, а не описывает его:
             «нажмите изменить раскладку» — это инструкция к кнопке,
             которую всё равно надо найти глазами. */
          editable ? (
            <AddTile onClick={add} full />
          ) : (
            <p className="p-6 text-sm text-fg-muted">{t("chart.empty")}</p>
          )
        ) : (
          /* Двенадцать колонок, а не две: размер карточки задаётся
             ручкой на её краю, и шаг в четверть экрана — это шаг,
             а не выбор из двух вариантов. */
          <div className="grid gap-3 lg:grid-cols-12">
            {charts.map((chart) => (
              <ChartCard
                key={chart.id}
                config={chart}
                columns={columns}
                relations={relations}
                rows={rows}
                language={language}
                editing={editMode}
                dragged={dragged === chart.id}
                over={over === chart.id && Boolean(dragged)}
                onDragStart={() => setDragged(chart.id)}
                onDragEnd={() => {
                  setDragged(null);
                  setOver(null);
                }}
                onDragOver={() => setOver(chart.id)}
                onDrop={() => drop(chart.id)}
                onMove={(delta) => move(chart.id, delta)}
                onChange={(patch) => change(chart.id, patch)}
                onRemove={() => apply(charts.filter((item) => item.id !== chart.id))}
              />
            ))}

            {/* Место под следующий график — в самой сетке, а не только
                кнопкой в шапке: так видно, куда он встанет. */}
            {editMode && <AddTile onClick={add} />}
          </div>
        )}
      </div>
    </div>
  );
}

/*
 * Ширина карточки классами: Tailwind не видит собранные строкой имена,
 * поэтому все двенадцать перечислены. До `lg` сетка одноколоночная —
 * там ширина не значит ничего.
 */
const SPAN: Record<number, string> = {
  1: "lg:col-span-1",
  2: "lg:col-span-2",
  3: "lg:col-span-3",
  4: "lg:col-span-4",
  5: "lg:col-span-5",
  6: "lg:col-span-6",
  7: "lg:col-span-7",
  8: "lg:col-span-8",
  9: "lg:col-span-9",
  10: "lg:col-span-10",
  11: "lg:col-span-11",
  12: "lg:col-span-12",
};

/** Высота ряда сетки: примерно столько карточка занимала всегда. */
const ROW_HEIGHT = 260;

/** Зазор сетки (`gap-3`) в пикселях: без него шаг колонки считается мимо. */
const GRID_GAP = 12;

/**
 * Ручка размера карточки: тянет вправо — колонки, вниз — ряды.
 *
 * Своя, а не `shared/ui/resize-handle`: та меряет и отдаёт ПИКСЕЛИ
 * и пишет ширину прямо в стиль. Здесь величина дискретна — колонка
 * сетки, — и во время перетаскивания карточка должна прыгать по
 * колонкам, а не ехать за курсором: иначе отпустил, а она встала
 * не туда, где была под рукой.
 *
 * Наружу размер уходит один раз, на отпускании: каждая пройденная
 * колонка иначе становится отдельным PUT view.
 */
function SizeHandle({
  edge,
  card,
  value,
  min,
  max,
  label,
  onPreview,
  onCommit,
}: {
  edge: "right" | "bottom";
  card: RefObject<HTMLElement | null>;
  value: number;
  min: number;
  max: number;
  label: string;
  onPreview: (value: number) => void;
  onCommit: (value: number) => void;
}) {
  const clamp = (next: number) => Math.round(Math.min(max, Math.max(min, next)));

  const start = (event: PointerEvent<HTMLDivElement>) => {
    const element = card.current;
    const grid = element?.parentElement;
    if (!element || !grid || event.button !== 0) return;

    // Ручка живёт внутри карточки, которую тащат мышью, — без этого
    // нажатие на неё начинает перетаскивание всей карточки.
    event.preventDefault();
    event.stopPropagation();

    const box = element.getBoundingClientRect();
    const step =
      edge === "right"
        ? (grid.getBoundingClientRect().width + GRID_GAP) / MAX_SPAN
        : ROW_HEIGHT + GRID_GAP;

    const sizeAt = (point: globalThis.PointerEvent) =>
      clamp(
        edge === "right"
          ? (point.clientX - box.left + GRID_GAP) / step
          : (point.clientY - box.top + GRID_GAP) / step,
      );

    let last = value;
    const onMove = (move: globalThis.PointerEvent) => {
      const next = sizeAt(move);
      if (next === last) return;
      last = next;
      onPreview(next);
    };

    const onUp = (up: globalThis.PointerEvent) => {
      document.removeEventListener("pointermove", onMove);
      document.body.style.cursor = "";
      onCommit(sizeAt(up));
    };

    document.body.style.cursor = edge === "right" ? "col-resize" : "row-resize";
    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp, { once: true });
  };

  // Стрелками — тоже: попасть мышью в полосу шириной 6px может не каждый.
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const keys = edge === "right" ? ["ArrowLeft", "ArrowRight"] : ["ArrowUp", "ArrowDown"];
    const towards = event.key === keys[0] ? -1 : event.key === keys[1] ? 1 : 0;
    if (!towards) return;

    event.preventDefault();
    onCommit(clamp(value + towards));
  };

  return (
    <div
      role="separator"
      aria-orientation={edge === "right" ? "vertical" : "horizontal"}
      aria-label={label}
      aria-valuenow={value}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onPointerDown={start}
      onKeyDown={onKeyDown}
      className={`absolute z-10 hidden items-center justify-center lg:flex ${
        edge === "right"
          ? "top-0 right-0 h-full w-2 cursor-col-resize"
          : "bottom-0 left-0 h-2 w-full cursor-row-resize"
      }`}
    >
      {/*
       * Полоска видна всегда, пока идёт правка, а не только под
       * курсором: ручку, о которой не знаешь, не ищут наведением.
       */}
      <span
        className={`rounded-full bg-border-strong transition-colors hover:bg-accent focus-visible:bg-accent ${
          edge === "right" ? "h-8 w-1" : "h-1 w-8"
        }`}
      />
    </div>
  );
}

/**
 * По скольким строкам считать — и сколько их всего.
 *
 * Серверной агрегации нет (docs/backend-notes.md, «Группировка»),
 * поэтому графики считают ЗАГРУЖЕННУЮ выборку. И это не случайная
 * выборка, а первые N строк в текущем порядке.
 *
 * Сказано об этом ровно настолько, насколько нужно: выбор размера
 * и рядом два числа. Полосы во всю ширину с восклицанием здесь не
 * стояло и не стоит — она кричала на каждом экране, где строк больше
 * порции, то есть почти всегда, и переставала читаться на второй день.
 * Неполнота видна цветом самого числа.
 */
function Sample({
  shown,
  total,
  loaded,
  sample,
  onSample,
}: {
  shown: number;
  total: number;
  loaded: boolean;
  sample: number;
  onSample: (next: number) => void;
}) {
  const { t, i18n } = useTranslation();
  const number = (value: number) => value.toLocaleString(i18n.language);
  /* `loaded` — про то, догружено ли всё; `total` бывает больше уже
     показанного и тогда, когда строки добавили в соседней вкладке. */
  const complete = loaded && shown >= total;

  return (
    <label className="ml-auto flex shrink-0 items-center gap-1.5 text-2xs">
      <span className="text-fg-subtle">{t("chart.sample")}</span>

      <Dropdown
        value={String(sample)}
        size="sm"
        ariaLabel={t("chart.sample")}
        className="w-20"
        items={SAMPLES.map((value) => ({ value: String(value), label: String(value) }))}
        onChange={(next) => onSample(Number(next))}
      />

      {/* Два числа вместо предупреждения. Совпали — серым, разошлись —
          цветом внимания: этого хватает, чтобы заметить, и не хватает,
          чтобы мешать. */}
      <span
        className={complete ? "text-fg-subtle" : "text-warning"}
        title={complete ? "" : t("chart.partial")}
      >
        {complete ? number(shown) : `${number(shown)} / ${number(total)}`}
      </span>
    </label>
  );
}

/**
 * Из скольких размеров выборки выбирают.
 *
 * Тысяча — потолок порции во всём приложении (`MAX_LIMIT`
 * в `GridFooter`), одним запросом больше не берётся. Кому мало —
 * сужает отбор, и полоса покрытия ровно об этом и просит.
 */
const SAMPLES = [200, 500, 1000];

/**
 * Пустое место под график: пунктирная плитка с плюсом.
 *
 * Тот же приём, что у «добавить поле» в таблице, и по той же причине:
 * добавление — это не пункт меню где-то наверху, а пустая клетка там,
 * где появится результат.
 */
function AddTile({ onClick, full = false }: { onClick: () => void; full?: boolean }) {
  const { t } = useTranslation();

  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex min-h-40 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border-strong text-sm text-fg-muted transition-colors hover:border-accent hover:bg-accent-subtle hover:text-accent-text ${
        full ? "w-full" : ""
      }`}
    >
      <Icon as={IconPlus} size={20} />
      {t("chart.add")}
    </button>
  );
}

function ChartCard({
  config,
  columns,
  relations,
  rows,
  language,
  editing,
  dragged,
  over,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
  onMove,
  onChange,
  onRemove,
}: {
  config: ChartConfig;
  columns: Field[];
  relations: Relation[];
  rows: Item[];
  language: string;
  editing: boolean;
  dragged: boolean;
  over: boolean;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDragOver: () => void;
  onDrop: () => void;
  onMove: (delta: number) => void;
  onChange: (patch: Partial<ChartConfig>) => void;
  onRemove: () => void;
}) {
  const { t, i18n } = useTranslation();

  const label = (field: Field) => localized(field.labels, language, field.label);
  const bySlug = new Map(columns.map((field) => [field.slug, field]));
  const groupField = bySlug.get(config.groupSlug);
  const splitField = bySlug.get(config.splitSlug);
  const valueField = bySlug.get(config.valueSlug);

  /*
   * Ось: чем свернуть значение в ключ и чем развернуть ключ в подпись.
   * У списка в строке лежит код варианта, у связи — uuid, у даты
   * с бакетом — час или день недели, и подпись у каждого своя.
   */
  const axis = (field: Field | undefined, bucket: ChartBucket) => {
    if (!field) return undefined;

    const kind = cellKind(field.type);
    const dateKind = DATE_KINDS.has(kind) ? (kind as DateKind) : null;
    if (!dateKind || !bucket) {
      return { label: valueLabelOf(field, relations, language) };
    }

    return {
      bucket: (value: unknown) => bucketKey(value, dateKind, bucket),
      // Подпись бакета собирается из ключа, а не из строки: «08» — это
      // час, а не значение, которое где-то лежит.
      label: (_row: Item, key: string) => bucketLabel(key, bucket, i18n.language),
    };
  };

  const series = chartSeries(rows, config, {
    group: axis(groupField, config.groupBucket),
    split: axis(splitField, config.splitBucket),
    /* Порядок стадий воронки — порядок вариантов поля, как их завёл
       админ. У поля без вариантов порядок остаётся по величине. */
    ...(staged(config.kind) && groupField
      ? { order: [...groupField.options.keys()] }
      : {}),
  });

  /*
   * Заголовок по умолчанию собирается из того, что настроено: «Сумма
   * amount · по status». Пустая карточка без подписи — это число,
   * о котором нельзя сказать, что оно считает.
   */
  const derived = [
    t(`pivot.aggregation.${config.aggregation}` as const),
    valueField && config.aggregation !== "count" ? label(valueField) : "",
    groupField && config.kind !== "stat" ? `· ${label(groupField)}` : "",
    splitField && series.splits.length ? `× ${label(splitField)}` : "",
  ]
    .filter(Boolean)
    .join(" ");

  /*
   * Размер во время перетаскивания ручки — местный: иначе каждая
   * пройденная колонка это отдельный PUT view. Наружу уходит один раз,
   * на отпускании, — тот же уговор, что и у ручки ширины сайдбара.
   */
  const [sizing, setSizing] = useState<{ width: number; height: number } | null>(null);
  const width = sizing?.width ?? config.width;
  const height = sizing?.height ?? config.height;
  const card = useRef<HTMLElement>(null);

  return (
    <section
      ref={card}
      draggable={editing}
      /*
       * Высота именно ЗАДАНА, а не «не меньше»: с `min-height` карточка
       * росла по содержимому, и у списка из двенадцати полос ручка внизу
       * двигалась вхолостую — число менялось, а на экране ничего.
       * Не поместилось — прокручивается внутри, как и положено плитке
       * на панели.
       */
      style={{ height: `${height * ROW_HEIGHT}px` }}
      /* Выделение текста в заголовке — не бросок карточки: у поля ввода
         внутри draggable-родителя браузер начинает перетаскивание
         вместо выделения. */
      onDragStart={(event) => {
        if (event.target instanceof HTMLElement && event.target.closest("input, textarea")) {
          event.preventDefault();
          return;
        }
        onDragStart();
      }}
      onDragEnd={onDragEnd}
      /* preventDefault обязателен: без него браузер запрещает бросок,
         и onDrop не случается вовсе. */
      onDragOver={(event) => {
        if (!editing) return;
        event.preventDefault();
        onDragOver();
      }}
      onDrop={(event) => {
        if (!editing) return;
        event.preventDefault();
        onDrop();
      }}
      className={`relative flex flex-col gap-2 rounded-lg border bg-surface p-3 transition-colors ${
        SPAN[width] ?? SPAN[6]
      } ${dragged ? "opacity-40" : ""} ${over ? "border-accent bg-accent-subtle" : "border-border"}`}
    >
      {/* Ручки размера — только в правке и только там, где сетка
          многоколоночная: на узком экране всё и так в одну колонку. */}
      {editing && (
        <>
          <SizeHandle
            edge="right"
            card={card}
            value={width}
            min={MIN_SPAN}
            max={MAX_SPAN}
            label={t("chart.resizeWidth")}
            onPreview={(next) => setSizing({ width: next, height })}
            onCommit={(next) => {
              setSizing(null);
              if (next !== config.width) onChange({ width: next });
            }}
          />

          <SizeHandle
            edge="bottom"
            card={card}
            value={height}
            min={MIN_ROWS}
            max={MAX_ROWS}
            label={t("chart.resizeHeight")}
            onPreview={(next) => setSizing({ width, height: next })}
            onCommit={(next) => {
              setSizing(null);
              if (next !== config.height) onChange({ height: next });
            }}
          />
        </>
      )}
      <header className="flex min-w-0 items-center gap-1.5">
        {editing && (
          <button
            type="button"
            aria-label={t("chart.move")}
            title={t("chart.move")}
            onKeyDown={(event) => {
              if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
              // Иначе стрелка прокрутит страницу под карточкой.
              event.preventDefault();
              onMove(event.key === "ArrowLeft" ? -1 : 1);
            }}
            className="grid size-5 shrink-0 cursor-grab place-items-center rounded text-fg-subtle transition-colors hover:text-fg"
          >
            <Icon as={IconGripVertical} size={14} />
          </button>
        )}

        {editing ? (
          <CommitInput
            value={config.title}
            placeholder={derived}
            label={t("chart.title")}
            allowEmpty
            onCommit={(title) => onChange({ title })}
            className="h-7 min-w-0 flex-1 text-sm"
          />
        ) : (
          <h3 className="min-w-0 flex-1 truncate text-sm font-medium text-fg">
            {config.title || derived}
          </h3>
        )}

        {/*
         * Сколько строк попало в ЭТОТ график. У соседних карточек число
         * другое: строки с пустым полем разбивки в расчёт не идут.
         * Про выборку целиком говорит полоса над сеткой — здесь про неё
         * ни слова, иначе две строки спорили бы об одном.
         *
         * У ненастроенного графика счётчика нет: он сообщал бы «ноль
         * строк», то есть отвечал на вопрос, которого не задавали.
         */}
        {chartReady(config) && (
          <span className="shrink-0 text-2xs text-fg-subtle">
            {t("chart.rows", { count: series.count })}
          </span>
        )}

        {editing && (
          <button
            type="button"
            aria-label={t("chart.remove")}
            title={t("chart.remove")}
            onClick={onRemove}
            className="grid size-6 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-danger"
          >
            <Icon as={IconTrash} size={14} />
          </button>
        )}
      </header>

      {editing && (
        <ChartSetup config={config} columns={columns} language={language} onChange={onChange} />
      )}

      {/*
       * Место под сам график — весь остаток карточки. Формы, у которых
       * форма читается по вертикали (линия, лента), растягиваются на
       * него; списки полос остаются сверху и прокручиваются, если
       * не поместились.
       */}
      <div className="flex min-h-0 flex-1 flex-col overflow-auto">
        {!chartReady(config) ? (
          <p className="grid flex-1 place-items-center text-sm text-fg-muted">
            {/* Стопке и тепловой карте не хватает именно ВТОРОГО поля —
                сказать «выберите поле», когда первое уже выбрано, значит
                отправить человека искать несуществующую ошибку. */}
            {config.groupSlug && !config.splitSlug ? t("chart.needSplit") : t("pivot.pick")}
          </p>
        ) : !series.points.length ? (
          <p className="grid flex-1 place-items-center text-sm text-fg-muted">{t("pivot.empty")}</p>
        ) : config.kind === "stat" ? (
          <Stat series={series} locale={i18n.language} />
        ) : config.kind === "donut" ? (
          <Donut series={series} locale={i18n.language} />
        ) : config.kind === "heatmap" ? (
          <Heatmap series={series} locale={i18n.language} />
        ) : config.kind === "funnel" ? (
          <Funnel series={series} locale={i18n.language} />
        ) : config.kind === "stream" ? (
          <Stream series={series} locale={i18n.language} />
        ) : config.kind === "stack" ? (
          <Stack series={series} locale={i18n.language} />
        ) : config.kind === "line" ? (
          <Line series={series} locale={i18n.language} />
        ) : (
          <Bars series={series} locale={i18n.language} />
        )}
      </div>

      {/* Легенда обязательна, как только серий больше одной: опознавать
          их цветом и только цветом нельзя. У кольца она своя — там доли
          и есть серии. */}
      {series.splits.length > 1 && config.kind !== "heatmap" && <Legend splits={series.splits} />}

      {/* Сколько значений осталось за кадром. Молча обрезанный рейтинг
          выглядит как весь список. */}
      {(series.hidden > 0 || series.hiddenSplits > 0) && config.kind !== "donut" && (
        <p className="text-2xs text-fg-subtle">
          {[
            series.hidden > 0 ? t("chart.hidden", { count: series.hidden }) : "",
            series.hiddenSplits > 0 ? t("chart.hiddenSplits", { count: series.hiddenSplits }) : "",
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
      )}
    </section>
  );
}

/** Настройки одного графика. Видны только в режиме правки. */
function ChartSetup({
  config,
  columns,
  language,
  onChange,
}: {
  config: ChartConfig;
  columns: Field[];
  language: string;
  onChange: (patch: Partial<ChartConfig>) => void;
}) {
  const { t } = useTranslation();
  const label = (field: Field) => localized(field.labels, language, field.label);
  const options = (fields: Field[]) =>
    fields.map((field) => ({ value: field.slug, label: label(field) }));

  const bySlug = new Map(columns.map((field) => [field.slug, field]));

  /** Считать можно по числовому полю; количество — по любому. */
  const numeric = columns.filter((field) => NUMERIC_FIELDS.has(field.type));

  /*
   * Что вообще можно посчитать на этом экране.
   *
   * Кольцо и стопка складывают доли в целое, поэтому среднего, минимума
   * и максимума в их списке нет вовсе (см. aggregationAllowed). А если
   * у view нет ни одного числового поля, то нечего и суммировать: выбор
   * «Сумма» открывал бы пустой список полей — тупик, из которого
   * человек выбирается только отменой.
   */
  const aggregations = AGGREGATIONS.filter(
    (item) =>
      aggregationAllowed(config.kind, item) && (item === "count" || numeric.length > 0),
  );

  return (
    <div className="flex flex-wrap items-center gap-1.5 border-t border-border pt-2">
      <Dropdown
        value={config.kind}
        ariaLabel={t("chart.kind")}
        className="w-36"
        items={CHART_KINDS.map((kind) => ({
          value: kind,
          label: t(`chart.kind.${kind}` as const),
          icon: <Icon as={KIND_ICONS[kind]} size={14} className="text-fg-muted" />,
        }))}
        onChange={(value) => {
          const kind = value as ChartKind;
          onChange({
            kind,
            // Смена формы не должна оставлять кольцо со средним.
            ...(aggregationAllowed(kind, config.aggregation)
              ? {}
              : { aggregation: "count" as Aggregation }),
            // Кольцу и числу второе поле не нужно; уносим его с собой,
            // чтобы оно не всплыло при возврате к полосам как чужое.
            ...(splittable(kind) ? {} : { splitSlug: "" }),
          });
        }}
      />

      {/* У одного числа поля разбивки нет: считать нечего по чему. */}
      {config.kind !== "stat" && (
        <>
          <Dropdown
            value={config.groupSlug}
            ariaLabel={t("chart.group")}
            placeholder={t("pivot.pick")}
            className="w-40"
            items={options(columns)}
            onChange={(groupSlug) =>
              // Поле сменили — прежняя единица времени к нему отношения
              // не имеет: «по часам» у поля-списка означает пустой график.
              onChange({ groupSlug, groupBucket: dated(bySlug.get(groupSlug)) ? config.groupBucket : "" })
            }
          />

          {dated(bySlug.get(config.groupSlug)) && (
            <Dropdown
              value={config.groupBucket}
              ariaLabel={t("chart.bucket")}
              className="w-40"
              items={bucketOptions(t)}
              onChange={(value) => onChange({ groupBucket: value as ChartBucket })}
            />
          )}
        </>
      )}

      {/* Второе поле: им категория делится на серии. Стопке и тепловой
          карте оно обязательно, полосам и линии — по желанию, поэтому
          в списке есть «без разбивки». */}
      {splittable(config.kind) && (
        <>
          <Dropdown
            value={config.splitSlug}
            ariaLabel={t("chart.split")}
            placeholder={t("chart.splitNone")}
            className="w-40"
            items={[
              ...(needsSplit(config.kind) ? [] : [{ value: "", label: t("chart.splitNone") }]),
              /*
               * По себе делить нечего — кроме свёрнутой даты: «день × час»
               * это ОДНО поле, разложенное двумя единицами времени, и ради
               * этого случая поле остаётся в списке.
               */
              ...options(
                columns.filter(
                  (field) => field.slug !== config.groupSlug || Boolean(config.groupBucket),
                ),
              ),
            ]}
            onChange={(splitSlug) =>
              onChange({
                splitSlug,
                splitBucket: dated(bySlug.get(splitSlug)) ? config.splitBucket : "",
              })
            }
          />

          {dated(bySlug.get(config.splitSlug)) && (
            <Dropdown
              value={config.splitBucket}
              ariaLabel={t("chart.bucket")}
              className="w-40"
              items={bucketOptions(t)}
              onChange={(value) => onChange({ splitBucket: value as ChartBucket })}
            />
          )}
        </>
      )}

      <Dropdown
        value={config.aggregation}
        ariaLabel={t("pivot.value")}
        className="w-32"
        items={aggregations.map((item) => ({
          value: item,
          label: t(`pivot.aggregation.${item}` as const),
        }))}
        onChange={(value) => onChange({ aggregation: value as Aggregation })}
      />

      {/* Поле значения нужно всем, кроме количества. */}
      {config.aggregation !== "count" && (
        <Dropdown
          value={config.valueSlug}
          ariaLabel={t("pivot.value")}
          placeholder={t("pivot.pick")}
          className="w-40"
          items={options(numeric)}
          onChange={(valueSlug) => onChange({ valueSlug })}
        />
      )}

      {/* Размер карточки — ручками на её краях, а не кнопкой здесь:
          «во всю ширину» было выбором из двух, а сетка даёт десять. */}
      <span className="ml-auto text-2xs tabular-nums text-fg-subtle">
        {config.width}/{MAX_SPAN}
        {config.height > 1 ? ` · ${config.height}` : ""}
      </span>
    </div>
  );
}

/**
 * Полосы: сравнение величин.
 *
 * Без второго поля — один цвет на все полосы, а не радуга: сравнивают
 * здесь длину, и восемь оттенков подряд означали бы, что цвет тоже
 * что-то значит. Со вторым полем цвет наконец значит серию, и у каждой
 * категории появляется своя группа тонких полос.
 *
 * Длина считается от общего максимума по ВСЕМ сериям, а не по каждой
 * своей: иначе полосы разных серий нельзя было бы сравнить глазами,
 * а ради этого график и рисуют.
 */
function Bars({ series, locale }: { series: ChartSeries; locale: string }) {
  const grouped = series.splits.length > 1;
  const top = grouped
    ? Math.max(...series.points.flatMap((point) => [...point.cells.values()]), 0)
    : Math.max(...series.points.map((point) => point.value), 0);

  /*
   * Отрицательное значение полосой не рисуется: его место слева от нуля,
   * а это уже двусторонняя ось. Число в строке остаётся, и минус в нём
   * виден.
   * ponytail: двусторонняя ось — когда попросят min по полю с минусами.
   */
  const width = (value: number) => `${top > 0 ? Math.max(0, value / top) * 100 : 0}%`;

  return (
    <div className="flex flex-col gap-2">
      {series.points.map((point) => (
        <div key={point.key} className="flex items-start gap-2">
          <span
            className="w-28 shrink-0 truncate pt-0.5 text-xs text-fg-muted"
            title={point.label}
          >
            {point.label}
          </span>

          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            {grouped ? (
              series.splits.map((split, index) => (
                <Row
                  key={split.key}
                  label={`${point.label} · ${splitName(split)}: ${show(point.cells.get(split.key) ?? 0, locale)}`}
                  width={width(point.cells.get(split.key) ?? 0)}
                  color={seriesFill(split.key, index)}
                  value={show(point.cells.get(split.key) ?? 0, locale)}
                  thin
                />
              ))
            ) : (
              <Row
                label={`${point.label}: ${show(point.value, locale)}`}
                width={width(point.value)}
                color="bg-chart-1"
                value={show(point.value, locale)}
              />
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

/** Одна полоса с числом на конце. Тонкая — когда их несколько в группе. */
function Row({
  label,
  width,
  color,
  value,
  thin = false,
}: {
  label: string;
  width: string;
  color: string;
  value: string;
  thin?: boolean;
}) {
  return (
    <div className="flex items-center gap-2">
      <Tooltip label={label}>
        <div className={`${thin ? "h-2.5" : "h-4"} min-w-0 flex-1`}>
          <div className={`h-full rounded-r-[4px] ${color}`} style={{ width }} />
        </div>
      </Tooltip>

      <span className="w-14 shrink-0 text-right text-xs tabular-nums text-fg">{value}</span>
    </div>
  );
}

/**
 * Стопка: доля в целом по каждой категории.
 *
 * Сегменты в процентах от итога СВОЕЙ строки — вопрос здесь «из чего
 * состоит эта категория», а не «какая из них больше». Величину категорий
 * сравнивают полосами, и подменять один вопрос другим внутри одной формы
 * нельзя.
 */
function Stack({ series, locale }: { series: ChartSeries; locale: string }) {
  return (
    <div className="flex flex-col gap-2">
      {series.points.map((point) => (
        <div key={point.key} className="flex items-center gap-2">
          <span className="w-28 shrink-0 truncate text-xs text-fg-muted" title={point.label}>
            {point.label}
          </span>

          {/* Зазор поверхностью, а не обводкой: разделяет пустота —
              тот же приём, что у кольца. */}
          <div className="flex h-4 min-w-0 flex-1 gap-0.5 overflow-hidden rounded-[4px]">
            {series.splits.map((split, index) => {
              const value = point.cells.get(split.key) ?? 0;
              if (value <= 0) return null;

              return (
                <Tooltip
                  key={split.key}
                  label={`${point.label} · ${splitName(split)}: ${show(value, locale)}`}
                >
                  <div
                    className={`h-full ${seriesFill(split.key, index)}`}
                    style={{ width: `${point.value > 0 ? (value / point.value) * 100 : 0}%` }}
                  />
                </Tooltip>
              );
            })}
          </div>

          <span className="w-14 shrink-0 text-right text-xs tabular-nums text-fg">
            {show(point.value, locale)}
          </span>
        </div>
      ))}
    </div>
  );
}

/**
 * Воронка: стадии в своём порядке, число и доля от всех.
 *
 * Порядок стадий приходит из вариантов поля и не сортируется по
 * величине — в воронке «новая заявка» стоит перед «оплачено» не потому,
 * что её больше.
 *
 * Доля считается от ВСЕХ строк, а не от предыдущей стадии: «сколько
 * сделок сейчас на этом шаге», а не «сколько дошло». Переходов между
 * стадиями в наших данных нет — в строке лежит только текущая стадия,
 * а история переходов живёт в журнале, которого у графика нет.
 */
function Funnel({ series, locale }: { series: ChartSeries; locale: string }) {
  const top = Math.max(...series.points.map((point) => point.value), 0);

  return (
    <div className="flex flex-col gap-1">
      {series.points.map((point) => {
        const share = series.total > 0 ? (point.value / series.total) * 100 : 0;

        return (
          <div key={point.key} className="flex items-center gap-2">
            <span className="w-28 shrink-0 truncate text-xs text-fg-muted" title={point.label}>
              {point.label}
            </span>

            <Tooltip label={`${point.label}: ${show(point.value, locale)} · ${percent(share)}`}>
              <div className="h-5 min-w-0 flex-1">
                <div
                  className="h-full rounded-r-[4px] bg-chart-1"
                  style={{ width: `${top > 0 ? Math.max(0, point.value / top) * 100 : 0}%` }}
                />
              </div>
            </Tooltip>

            {/* Число и доля рядом: «54» отвечает «сколько», «10.4%» —
                «много это или мало». По отдельности каждое неполно. */}
            <span className="w-20 shrink-0 text-right text-xs tabular-nums">
              <span className="font-medium text-fg">{show(point.value, locale)}</span>
              <span className="ml-1.5 text-fg-subtle">{percent(share)}</span>
            </span>
          </div>
        );
      })}
    </div>
  );
}

/**
 * Лента: те же стадии и доли, но одной непрерывной полосой.
 *
 * Толщина в стадии — её доля от всех строк. Это РАСПРЕДЕЛЕНИЕ, а не
 * поток: лента не говорит, что из первой стадии во вторую кто-то перешёл,
 * — таких данных у нас нет (см. Funnel). Поэтому у неё нет ни стрелок,
 * ни сужения «сколько дошло»: она просто толще там, где сделок больше.
 *
 * Кривые — кубические Безье с ручками на середине между стадиями:
 * ломаная из прямых читалась бы как измеренная траектория, а измерены
 * только точки в стадиях.
 */
function Stream({ series, locale }: { series: ChartSeries; locale: string }) {
  const count = series.points.length;
  const top = Math.max(...series.points.map((point) => point.value), 0);

  /** Полутолщина в единицах системы: 45 из 100 — почти вся высота. */
  const half = (value: number) => (top > 0 ? Math.max(0.6, (value / top) * 45) : 0.6);
  const x = (index: number) => (count > 1 ? (index / (count - 1)) * 100 : 50);

  /** Одна сторона ленты: точки в стадиях, между ними — плавный переход. */
  const edge = (sign: number) =>
    series.points
      .map((point, index) => {
        const y = 50 + sign * half(point.value);
        if (index === 0) return `${x(index)},${y}`;

        const previous = series.points[index - 1];
        const middle = (x(index - 1) + x(index)) / 2;
        const from = 50 + sign * half(previous?.value ?? 0);
        return `C ${middle},${from} ${middle},${y} ${x(index)},${y}`;
      })
      .join(" ");

  const back = [...series.points]
    .map((point, index) => ({ point, index }))
    .reverse()
    .map(({ point, index }, at) => {
      const y = 50 - half(point.value);
      if (at === 0) return `L ${x(index)},${y}`;

      const next = series.points[index + 1];
      const middle = (x(index) + x(index + 1)) / 2;
      const from = 50 - half(next?.value ?? 0);
      return `C ${middle},${from} ${middle},${y} ${x(index)},${y}`;
    })
    .join(" ");

  return (
    <div className="flex h-full min-h-0 flex-col gap-1">
      {/* Стадии подписаны сверху: имя, число и доля. Внутри ленты
          текста нет — он растянулся бы вместе с системой координат. */}
      <div className="flex">
        {series.points.map((point) => (
          <div
            key={point.key}
            className="min-w-0 flex-1 border-l border-chart-grid px-1 first:border-l-0"
          >
            <p className="truncate text-2xs text-fg-subtle" title={point.label}>
              {point.label}
            </p>
            <p className="text-sm font-semibold text-fg">{show(point.value, locale)}</p>
            <p className="text-2xs text-fg-subtle">
              {percent(series.total > 0 ? (point.value / series.total) * 100 : 0)}
            </p>
          </div>
        ))}
      </div>

      <div className="relative min-h-0 flex-1">
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="size-full" role="presentation">
          <path d={`M ${edge(1)} ${back} Z`} className="fill-chart-1 opacity-70" />
        </svg>

        {/* Полосы поверх ленты — и подсказка, и разделители стадий:
            попасть в саму ленту мышью можно только там, где она
            толстая, а без линий непонятно, где кончается стадия. */}
        <div className="absolute inset-0 flex">
          {series.points.map((point) => (
            <Tooltip
              key={point.key}
              label={`${point.label}: ${show(point.value, locale)} · ${percent(
                series.total > 0 ? (point.value / series.total) * 100 : 0,
              )}`}
            >
              <div className="h-full flex-1 border-l border-chart-grid first:border-l-0" />
            </Tooltip>
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * Тепловая карта: «поле × поле», величина цветом.
 *
 * Шкала — один тон в пять ступеней прозрачности, а не радуга: здесь
 * сравнивают величину, а у величины нет «своего цвета», у неё есть
 * «больше» и «меньше». Ступени прозрачности, а не отдельные токены:
 * тот же приём, что у подложки колонки доски (CHIP_SURFACE) — на светлом
 * фоне получается светлее, на тёмном темнее, и обе темы выходят сами.
 *
 * Число стоит прямо в клетке: цвет отвечает «где густо», число —
 * «сколько именно», и второй вопрос задают сразу после первого.
 */
function Heatmap({ series, locale }: { series: ChartSeries; locale: string }) {
  const { t } = useTranslation();
  const top = Math.max(...series.points.flatMap((point) => [...point.cells.values()]), 0);

  return (
    <div className="overflow-x-auto">
      <div
        className="grid min-w-full gap-0.5"
        /*
         * Колонка подписей — по содержимому, но не шире 7rem
         * (`fit-content`). Жёсткие 7rem оставляли слева пустую полосу
         * шириной в полстроки: «пн» и «10:00» занимают вчетверо меньше,
         * а место под них резервировалось всегда.
         *
         * Клетки делят остаток поровну и не ужимаются меньше 1.5rem:
         * колонок бывает двадцать четыре — по часу на каждую, — и тогда
         * сетка уезжает в прокрутку, а не сплющивается в полоску.
         */
        style={{
          gridTemplateColumns: `fit-content(7rem) repeat(${series.splits.length}, minmax(1.5rem, 1fr)) 3rem`,
        }}
      >
        <span />
        {series.splits.map((split) => (
          <span
            key={split.key}
            className="truncate pb-1 text-center text-2xs text-fg-subtle"
            title={splitName(split)}
          >
            {splitName(split)}
          </span>
        ))}
        <span className="pb-1 text-right text-2xs text-fg-subtle">{t("chart.total")}</span>

        {series.points.map((point) => (
          <Fragment key={point.key}>
            <span className="truncate pr-2 text-xs text-fg-muted" title={point.label}>
              {point.label}
            </span>

            {series.splits.map((split) => {
              const value = point.cells.get(split.key);

              return (
                <Tooltip
                  key={split.key}
                  label={`${point.label} · ${splitName(split)}: ${value === undefined ? "—" : show(value, locale)}`}
                >
                  {/* Пустая клетка — серая подложка без числа, а не
                      бледная ступень шкалы: у шкалы свой смысл
                      («мало»), а здесь не было ни одной строки. */}
                  {value === undefined ? (
                    <div className="h-6 rounded-sm bg-surface-hover" role="presentation" />
                  ) : (
                    <div
                      className={`grid h-6 place-items-center rounded-sm text-2xs tabular-nums ${heatFill(value, top)} ${heatInk(value, top)}`}
                    >
                      {/*
                       * Число прямо в клетке: цвет отвечает «где густо»,
                       * а число — «сколько именно», и второй вопрос
                       * задают сразу после первого. Цвет текста выбран
                       * по яркости заливки — иначе на тёмной клетке
                       * тёмные цифры пропадают.
                       */}
                      {show(value, locale)}
                    </div>
                  )}
                </Tooltip>
              );
            })}

            {/* Итог строки: у карты «день × час» именно он отвечает
                «сколько всего в этот день». */}
            <span className="self-center text-right text-xs font-medium tabular-nums text-fg">
              {show(point.value, locale)}
            </span>
          </Fragment>
        ))}
      </div>

      {/* Шкала: без неё непонятно, что означает «темнее». Числами
          по краям, а не подписью у каждой ступени, — ступеней пять,
          а подписей хватит двух. */}
      <div className="mt-2 flex items-center gap-1.5 text-2xs text-fg-subtle">
        <span className="tabular-nums">0</span>
        {HEAT_FILL.map((fill) => (
          <span key={fill} className={`size-3 rounded-sm ${fill}`} />
        ))}
        <span className="tabular-nums">{show(top, locale)}</span>
      </div>
    </div>
  );
}

/**
 * Кольцо: доля в целом.
 *
 * Окружность ровно 100 единиц (r = 100 / 2π) — тогда доля в процентах
 * и есть длина штриха, и пересчитывать нечего. Зазор между долями —
 * тот же приём, что и везде: разделяет пустота, а не обводка.
 */
function Donut({ series, locale }: { series: ChartSeries; locale: string }) {
  const { t } = useTranslation();
  /* Одна доля — кольцо целиком, и зазор ему не нужен: он превратился бы
     в надрез, за которым ничего нет. */
  const gap = series.points.length > 1 ? 0.6 : 0;
  let offset = 0;

  return (
    <div className="flex items-center gap-4">
      <svg viewBox="0 0 42 42" className="size-32 shrink-0" role="presentation">
        {series.points.map((point, index) => {
          const share = series.total > 0 ? (point.value / series.total) * 100 : 0;
          // Доля тоньше зазора всё равно должна быть видна волоском.
          const length = Math.max(share - gap, 0.4);
          const dash = `${length} ${100 - length}`;
          // 25 — четверть окружности: кольцо начинается сверху, а не справа.
          const rotate = 25 - offset;
          offset += share;

          return (
            <circle
              key={point.key}
              cx="21"
              cy="21"
              r="15.9155"
              fill="none"
              strokeWidth="5"
              strokeDasharray={dash}
              strokeDashoffset={rotate}
              className={seriesStroke(point.key, index)}
            />
          );
        })}
      </svg>

      {/* Легенда обязательна: доли различаются только цветом, а цветом
          одним опознавать нельзя. Своя, а не общая: здесь у каждой доли
          есть ещё и число, а у серий его нет — оно на самих полосах. */}
      <ul className="flex min-w-0 flex-1 flex-col gap-1">
        {series.points.map((point, index) => (
          <li key={point.key} className="flex min-w-0 items-center gap-1.5 text-xs">
            <span className={`size-2 shrink-0 rounded-full ${seriesDot(point.key, index)}`} />
            <span className="min-w-0 flex-1 truncate text-fg-muted">
              {point.key === OTHER_KEY ? t("chart.other") : point.label}
            </span>
            <span className="shrink-0 tabular-nums text-fg">{show(point.value, locale)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Легенда серий: цвет и подпись.
 *
 * Одна на полосы, стопку и линию. Без неё серию можно опознать только
 * по цвету, а цвет — не единственный канал: под дальтонизмом соседние
 * оттенки сходятся, и остаётся подпись.
 */
function Legend({ splits }: { splits: ChartSplit[] }) {
  const { t } = useTranslation();

  return (
    <ul className="flex flex-wrap items-center gap-x-3 gap-y-1 pt-1">
      {splits.map((split, index) => (
        <li key={split.key} className="flex min-w-0 items-center gap-1.5 text-2xs">
          <span className={`size-2 shrink-0 rounded-full ${seriesDot(split.key, index)}`} />
          <span className="truncate text-fg-muted">
            {split.key === OTHER_KEY ? t("chart.other") : split.label}
          </span>
        </li>
      ))}
    </ul>
  );
}

/**
 * Линия: изменение во времени.
 *
 * Растянутая система координат (`preserveAspectRatio="none"`) вместо
 * измерения контейнера: карточка бывает любой ширины, а мерить её
 * пришлось бы наблюдателем размера. Толщину линии растяжение не портит
 * — `vector-effect` считает её в пикселях экрана, а не в единицах
 * системы. Текста внутри поэтому нет вовсе: он растянулся бы вместе
 * с осями. Подписи — обычной разметкой вокруг.
 */
function Line({ series, locale }: { series: ChartSeries; locale: string }) {
  const multi = series.splits.length > 1;
  /* Ось общая для всех линий: у каждой своя означало бы, что рядом
     нарисованы графики с разной ценой деления. */
  const values = multi
    ? series.points.flatMap((point) => series.splits.map((split) => point.cells.get(split.key) ?? 0))
    : series.points.map((point) => point.value);

  const top = Math.max(...values, 0);
  const bottom = Math.min(...values, 0);
  const span = top - bottom || 1;
  const count = series.points.length;

  /*
   * Поле сверху и снизу: линия по краю коробки обрезается своей же
   * толщиной пополам — ровно это и видно, когда все значения равны
   * и линия ложится на верхнюю границу.
   */
  const PAD = 4;

  const path = (valueAt: (index: number) => number) =>
    series.points
      .map((_, index) => {
        const x = count > 1 ? (index / (count - 1)) * 100 : 50;
        const y = PAD + (1 - (valueAt(index) - bottom) / span) * (100 - PAD * 2);
        return `${x},${y}`;
      })
      .join(" ");

  const lines = multi
    ? series.splits.map((split, index) => ({
        key: split.key,
        color: seriesStroke(split.key, index),
        points: path((at) => series.points[at]?.cells.get(split.key) ?? 0),
      }))
    : [{ key: "", color: "stroke-chart-1", points: path((at) => series.points[at]?.value ?? 0) }];

  const first = series.points[0];
  const last = series.points[count - 1];

  return (
    <div className="flex h-full min-h-0 flex-col gap-1">
      <div className="flex min-h-0 flex-1 items-stretch gap-2">
        <span className="w-14 shrink-0 text-right text-2xs tabular-nums text-fg-subtle">
          {show(top, locale)}
        </span>

        {/* Поле графика — весь остаток по высоте: у линии форма читается
            именно по вертикали, и растянуть её значит показать больше. */}
        <div className="relative min-h-0 min-w-0 flex-1 border-b border-chart-grid">
          <svg
            viewBox="0 0 100 100"
            preserveAspectRatio="none"
            className="size-full"
            role="presentation"
          >
            {lines.map((line) => (
              <polyline
                key={line.key}
                points={line.points}
                fill="none"
                strokeWidth="2"
                strokeLinejoin="round"
                strokeLinecap="round"
                vectorEffect="non-scaling-stroke"
                className={line.color}
              />
            ))}
          </svg>

          {/*
           * Подсказка по наведению — полосами поверх линии: попасть
           * в саму линию мышью нельзя.
           * ponytail: полосы равной ширины, а точки стоят по краям, —
           * у первой и последней подсказка срабатывает на полполосы
           * в сторону. Считать точные границы — когда станет мешать.
           */}
          <div className="absolute inset-0 flex">
            {series.points.map((point) => (
              <Tooltip
                key={point.key}
                label={
                  multi ? (
                    /* У мульти-линии в одной точке несколько значений:
                       подсказка перечисляет все, иначе она отвечает
                       за одну линию, а навели на все сразу. Разметкой,
                       а не переводом строки: подсказка рисует текст
                       в одну строку, и «\n» в ней схлопывается. */
                    <span className="flex flex-col gap-0.5">
                      <span className="font-medium">{point.label}</span>
                      {series.splits.map((split) => (
                        <span key={split.key}>
                          {splitName(split)}: {show(point.cells.get(split.key) ?? 0, locale)}
                        </span>
                      ))}
                    </span>
                  ) : (
                    `${point.label}: ${show(point.value, locale)}`
                  )
                }
              >
                <div className="h-full flex-1" />
              </Tooltip>
            ))}
          </div>
        </div>
      </div>

      {/* Ось времени: края подписаны, середину читают подсказкой. */}
      <div className="flex gap-2 pl-16 text-2xs text-fg-subtle">
        <span className="min-w-0 flex-1 truncate">{first?.label}</span>
        {count > 1 && <span className="min-w-0 flex-1 truncate text-right">{last?.label}</span>}
      </div>
    </div>
  );
}

/**
 * Одно число.
 *
 * Крупно и обычными цифрами, а не моноширинными: `tabular-nums` даёт
 * каждой цифре ширину нуля, и на крупном кегле число рассыпается.
 * Моноширинные — там, где числа стоят столбиком.
 */
function Stat({ series, locale }: { series: ChartSeries; locale: string }) {
  return (
    <p className="py-4 text-3xl font-semibold text-fg">
      {show(series.points[0]?.value ?? 0, locale)}
    </p>
  );
}

/**
 * Значок формы. Рядом с названием, а не вместо него: силуэт читается
 * быстрее слова, но по одному силуэту в 14px «стопку» от «полос»
 * не отличить.
 *
 * Проверено в списке на настоящем размере, как и значки типов view:
 *   stack   — рамка, разрезанная на три доли (`IconColumns3`). Первым
 *             стоял `IconLayoutDistributeVertical` — две палки со скобкой
 *             посередине, читается как «выровнять», а не «разделить
 *             на части».
 *   heatmap — сетка точек: клетки, а не график.
 *   stat    — «123»: цифры и есть форма.
 */
const KIND_ICONS: Record<ChartKind, TablerIcon> = {
  bar: IconChartBar,
  stack: IconColumns3,
  line: IconChartLine,
  heatmap: IconGridDots,
  /* Воронка — свой значок Tabler, хотя силуэт у него тот же, что
     у кнопки отбора в шапке. Спутать негде: этот список показывает
     формы графика, а не действия. */
  funnel: IconChartFunnel,
  stream: IconWaveSine,
  donut: IconChartDonut,
  stat: IconNumber123,
};

/**
 * Типы ячейки, у которых значение — дата. Их и можно сворачивать
 * в бакеты; остальным поле «единица времени» не показывается вовсе.
 */
const DATE_KINDS = new Set<string>(["date", "datetime", "datetime_naive"]);

/** Поле-дата: только у него есть смысл спрашивать единицу времени. */
function dated(field: Field | undefined): boolean {
  return Boolean(field) && DATE_KINDS.has(cellKind(field!.type));
}

/** Список единиц времени. «Как есть» — каждая отметка своим значением. */
function bucketOptions(t: (key: TranslationKey) => string) {
  return [
    { value: "", label: t("chart.bucketNone") },
    ...CHART_BUCKETS.map((bucket) => ({
      value: bucket,
      label: t(`chart.bucket.${bucket}` as TranslationKey),
    })),
  ];
}

/*
 * Цвета серий, по порядку и без перебора по кругу. Классы перечислены
 * целиком: Tailwind не видит собранные строкой имена. Девятой серии
 * не бывает — хвост сворачивается в «Прочее» серым (см. model/chart).
 */
const SERIES_STROKE = [
  "stroke-chart-1",
  "stroke-chart-2",
  "stroke-chart-3",
  "stroke-chart-4",
  "stroke-chart-5",
  "stroke-chart-6",
  "stroke-chart-7",
  "stroke-chart-8",
];

const SERIES_BG = [
  "bg-chart-1",
  "bg-chart-2",
  "bg-chart-3",
  "bg-chart-4",
  "bg-chart-5",
  "bg-chart-6",
  "bg-chart-7",
  "bg-chart-8",
];

function seriesStroke(key: string, index: number): string {
  if (key === OTHER_KEY) return "stroke-chart-other";
  return SERIES_STROKE[index] ?? "stroke-chart-other";
}

function seriesDot(key: string, index: number): string {
  if (key === OTHER_KEY) return "bg-chart-other";
  return SERIES_BG[index] ?? "bg-chart-other";
}

/** Заливка полосы или сегмента. Тот же порядок, что у точки легенды. */
const seriesFill = seriesDot;

/** Подпись серии. «Прочее» переводится, остальное — значение поля. */
function splitName(split: ChartSplit): string {
  return split.key === OTHER_KEY ? i18n.t("chart.other") : split.label;
}

/*
 * Ступени тепловой карты: один тон, пять шагов насыщенности. Классы
 * целиком — Tailwind не видит собранные строкой имена.
 *
 * Прозрачность ЦВЕТА (`/15`), а не элемента (`opacity-15`): выцветший
 * элемент выцвечивает и число внутри себя. И не пять новых токенов:
 * подложка ложится на фон карточки, и в тёмной теме та же доля даёт
 * такой же приглушённый оттенок, что и в светлой, — тот же приём,
 * что у подложки колонки доски (CHIP_SURFACE).
 */
const HEAT_FILL = [
  "bg-chart-1/15",
  "bg-chart-1/35",
  "bg-chart-1/55",
  "bg-chart-1/75",
  "bg-chart-1",
];

function heatIndex(value: number, top: number): number {
  if (top <= 0) return 0;
  return Math.min(HEAT_FILL.length - 1, Math.floor((value / top) * HEAT_FILL.length));
}

function heatFill(value: number, top: number): string {
  return HEAT_FILL[heatIndex(value, top)] ?? "bg-chart-1";
}

/**
 * Цвет числа в клетке — по яркости заливки: на двух верхних ступенях
 * тёмные цифры пропадают, на трёх нижних пропадают белые.
 */
function heatInk(value: number, top: number): string {
  return heatIndex(value, top) >= 3 ? "text-accent-fg" : "text-fg";
}

/** Доля от целого в подписи. Один знак: «10.4%» читается, «10.42%» — нет. */
function percent(share: number): string {
  return `${share.toFixed(1)}%`;
}

/**
 * Число на графике: с разделителем разрядов и до двух знаков после
 * запятой. Больше знаков в графике не значит точнее — точное значение
 * есть в таблице.
 */
function show(value: number, locale: string): string {
  return value.toLocaleString(locale, { maximumFractionDigits: 2 });
}
