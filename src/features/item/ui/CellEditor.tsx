import {
  Suspense,
  lazy,
  useDeferredValue,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  IconAdjustments,
  IconCheck,
  IconExternalLink,
  IconPlus,
  IconTrash,
  IconUpload,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { IconPicker } from "@/features/icons";
import { STATUS_GROUPS, type Field, type FieldOption, type Relation } from "@/features/table";
import { Anchored } from "@/shared/ui/anchored";
import { Chip } from "@/shared/ui/chip";
import { openPreview } from "@/shared/ui/file-preview";
import { Icon } from "@/shared/ui/icon";
import { fileName } from "@/shared/lib/file-kind";
import { toast } from "@/shared/lib/toast";
import { useUploadFiles, uploadFolder } from "../api/files";
import { useCreateItem, useLinkRelation, useRelationItems } from "../api/relations";
import { editorKind, isMultiValue } from "../model/cell-kind";
import { formatCoords, mapLink, parseCoords } from "../model/coords";
import {
  fromDateInput,
  fromTimeInput,
  isBlank,
  sameValue,
  toDateInput,
  toDayInput,
  toList,
  toNumber,
  toTimeInput,
  type DateKind,
} from "../model/cell-value";
import { isLinkable, relationLabel, relationSelection } from "../model/relation";
import type { Item } from "../model/types";
import { cellError } from "../model/validate";
import { Cell, optionColor, optionLabel } from "./Cell";
import { CodeCell } from "./CodeCell";
import { MapPicker } from "./MapPicker";
import { PolygonCell } from "./PolygonCell";

/**
 * Раскрытая ячейка: редактор или просто значение целиком.
 *
 * Ячейка в строке — это прямоугольник фиксированной ширины, и всё
 * длинное в ней обрезано. Клик раскрывает её поверх таблицы: у поля,
 * которое можно править, — редактором, у остальных — карточкой со всем
 * значением. Второе не украшение: колонка шириной 180px прячет и текст,
 * и половину чипов, и без раскрытия их нечем прочитать.
 *
 * Правка применяется по-разному, и это сознательно:
 *   выбор, дата, связь   сразу, одним кликом
 *   текст и число        при закрытии; Escape отменяет
 *
 * Выбор — законченное действие, текст — нет. Отправлять запрос на каждое
 * нажатие клавиши значит слать их полсотни на одно слово; ждать закрытия
 * там, где человек уже ткнул в нужный день, — заставлять подтверждать
 * сделанное.
 */

/*
 * Календарь грузится отдельным куском и только когда открыли дату.
 * react-day-picker с его локалями — это 100 КБ, то есть четверть всего
 * приложения ради экрана, который открывают не каждый день. В общий
 * бандл он попадать не должен: его качает и тот, кто просто вошёл.
 */
const Calendar = lazy(() =>
  import("@/shared/ui/calendar").then((module) => ({ default: module.Calendar })),
);
const TimeList = lazy(() =>
  import("@/shared/ui/calendar").then((module) => ({ default: module.TimeList })),
);

/** Место под календарь, пока он едет: без него карточка прыгает. */
const CalendarFallback = () => <div className="h-64 w-64" />;

/* Цвет рамки задаётся на месте: две color-утилиты в одной строке классов
   борются не по порядку в строке, а по порядку в стилях. */
const card = "rounded-md border bg-surface shadow-popover";

export function ActiveCell({
  field,
  row,
  guid,
  tableSlug,
  anchor,
  relations,
  locale,
  language,
  heading,
  onSettings,
  onEdit,
  onLink,
  onClose,
}: {
  field: Field;
  row: Item;
  /** Первичный ключ строки. Без него править нечего — только смотреть. */
  guid: string | undefined;
  /** Слаг таблицы: нужен ручкам связи, они пишут не только в эту строку. */
  tableSlug: string;
  anchor: DOMRect;
  relations: Map<string, Relation>;
  locale: string;
  language: string;
  /** Заголовок строки в drawer: правится тем же кеглем, каким показан. */
  heading?: boolean | undefined;
  /** Настройки поля из ячейки: варианты выбора правят, глядя на список. */
  onSettings?: ((field: Field, anchor: DOMRect) => void) | undefined;
  onEdit: (value: unknown) => void;
  /**
   * Выбранная строка связи вместо запроса «связать».
   *
   * Нужно черновику новой строки: связывать ещё нечего — строки в базе
   * нет, и PUT ушёл бы по несуществующему guid. Значение вместе со
   * связанной записью ложится в черновик и уезжает вместе с ним.
   */
  onLink?: ((item: Item | null) => void) | undefined;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const kind = editorKind(field);
  const value = row[field.slug];

  /**
   * Значение не проходит проверку поля — текст, который надо показать.
   *
   * Сообщение админа важнее нашего: он писал его про конкретное поле
   * («номер в формате +998XX») и на языке проекта. Своё — только когда
   * его не задали.
   */
  const check = (next: unknown) => {
    const error = cellError(field, next);
    if (!error) return null;

    return error.message || t(error.kind === "required" ? "cell.required" : "cell.invalid");
  };

  /*
   * Правка, прошедшая проверку. Выбор, дата, цвет и файл применяются
   * одним кликом, и места под сообщение у них нет — поэтому уведомление,
   * а не подпись под полем. Не прошло — не отправляем: иначе значение
   * уедет и вернётся ошибкой сервера или, хуже, ляжет в базу.
   */
  const edit = (next: unknown) => {
    const error = check(next);
    if (error) return toast.error(error);

    onEdit(next);
  };

  const relation = field.relationId ? relations.get(field.relationId) : undefined;

  /*
   * У связи правится не значение, а сама связь, и только Many2One:
   * остальные виды лежат не в этой строке (см. model/relation). Такую
   * ячейку раскрываем как обычную — показать целиком, но не трогать.
   */
  const relational = kind === "relation";
  const linkable = relational && relation && isLinkable(relation.type);

  if (!kind || !guid || (relational && !linkable)) {
    return (
      <Anchored anchor={anchor} onClose={onClose}>
        <div className={`${card} border-border max-h-72 max-w-md overflow-auto px-2 py-1.5 text-sm`}>
          <Cell
            field={field}
            row={row}
            tableSlug={tableSlug}
            relations={relations}
            locale={locale}
            language={language}
            wrap
          />
        </div>
      </Anchored>
    );
  }

  if (linkable && relation) {
    return (
      <RelationEditor
        field={field}
        relation={relation}
        row={row}
        rowGuid={guid}
        tableSlug={tableSlug}
        anchor={anchor}
        onLink={onLink}
        onClose={onClose}
      />
    );
  }

  /**
   * «Настроить поле» под списком вариантов — как в меню колонки.
   *
   * Ячейка при этом закрывается: панель настроек всплывает на том же
   * якоре, и без закрытия две карточки лежат друг на друге, пока первый
   * же клик не уберёт нижнюю.
   */
  const settings = onSettings
    ? () => {
        onSettings(field, anchor);
        onClose();
      }
    : undefined;

  switch (kind) {
    case "status":
      return (
        <StatusEditor
          field={field}
          value={value}
          anchor={anchor}
          language={language}
          onSettings={settings}
          onEdit={edit}
          onClose={onClose}
        />
      );

    case "multiselect":
      return (
        <MultiselectEditor
          field={field}
          value={value}
          anchor={anchor}
          language={language}
          onSettings={settings}
          onEdit={edit}
          onClose={onClose}
        />
      );

    case "image":
    case "file":
      return (
        <FileEditor
          field={field}
          value={value}
          anchor={anchor}
          images={kind === "image"}
          onEdit={edit}
          onClose={onClose}
        />
      );

    case "color":
      return <ColorEditor value={value} anchor={anchor} onEdit={edit} onClose={onClose} />;

    case "icon":
      return <IconEditor value={value} anchor={anchor} onEdit={edit} onClose={onClose} />;

    case "map":
      return (
        <MapEditor
          value={value}
          // Точка, с которой открывается ПУСТАЯ ячейка: её задаёт админ
          // в настройках поля (`attributes.lat`/`long`). Без неё
          // заполнять координаты пришлось бы с нуля каждый раз.
          center={{ lat: field.attributes["lat"], lon: field.attributes["long"] }}
          anchor={anchor}
          onEdit={edit}
          onClose={onClose}
        />
      );

    case "json":
      return <JsonEditor value={value} anchor={anchor} onEdit={edit} onClose={onClose} />;

    /*
     * Область правится списком координат — рисовать её мышью нечем:
     * своей карты у нас нет (см. PolygonCell). Зато форма видна прямо
     * над текстом и перерисовывается по мере правки, поэтому опечатка
     * в координате заметна сразу, а не при следующем открытии.
     */
    case "polygon":
      return (
        <JsonEditor
          value={value}
          anchor={anchor}
          preview={(text) => <PolygonCell value={text} wrap />}
          onEdit={edit}
          onClose={onClose}
        />
      );

    case "date":
    case "datetime":
    case "datetime_naive":
      return (
        <DateEditor
          kind={kind}
          value={value}
          anchor={anchor}
          locale={locale}
          onEdit={edit}
          onClose={onClose}
        />
      );

    case "time":
      return <TimeEditor value={value} anchor={anchor} onEdit={edit} onClose={onClose} />;

    default:
      return (
        <TextEditor
          value={value}
          anchor={anchor}
          multiline={kind === "longtext"}
          numeric={kind === "number"}
          heading={heading}
          check={check}
          /*
           * У QR и штрихкода правится строка, но смотрят на рисунок —
           * и рисунок обязан меняться вместе с ней: иначе непонятно,
           * закодировалось ли набранное. Значение сюда приходит из
           * черновика, а не из строки.
           */
          preview={
            kind === "qr" || kind === "barcode"
              ? (text) =>
                  text.trim() ? (
                    <CodeCell type={field.type} value={text} qr={kind === "qr"} big />
                  ) : null
              : undefined
          }
          onEdit={edit}
          onClose={onClose}
        />
      );
  }
}

/**
 * Текст, число и многострочный текст — один редактор: разница между ними
 * в разборе значения и в том, что делает Enter.
 *
 * Всегда textarea, даже для одной строки: поле должно вырасти под длинное
 * значение, а не прокручиваться внутри 180 пикселей. Ради этого раскрытие
 * и делалось.
 */
function TextEditor({
  value,
  anchor,
  multiline,
  numeric,
  heading,
  check,
  preview,
  onEdit,
  onClose,
}: {
  value: unknown;
  anchor: DOMRect;
  multiline: boolean;
  numeric: boolean;
  /** Тем же кеглем, что и показанное значение: заголовок не ужимается
      до 13px на время правки и обратно — текст на месте не прыгает. */
  heading?: boolean | undefined;
  /** Проверка поля: текст ошибки или null. */
  check: (value: unknown) => string | null;
  /** Что показать над полем ввода: рисунок кода по набранному значению. */
  preview?: ((value: string) => ReactNode) | undefined;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState(() => (isBlank(value) ? "" : String(value)));
  /*
   * Ошибка появляется на вводе, а не на попытке сохранить: человек
   * должен видеть, что набирает не то, до того как отвёл взгляд. Но
   * ровно на своём вводе — открытая ячейка с уже неподходящим значением
   * из базы краснеть не должна, он его не писал.
   */
  const [error, setError] = useState<string | null>(null);
  const area = useRef<HTMLTextAreaElement>(null);
  const latest = useRef(draft);

  /* Поле растёт под текст. auto перед замером обязателен: без него
     scrollHeight помнит прежнюю, большую высоту, и поле не сжимается. */
  useLayoutEffect(() => {
    const element = area.current;
    if (!element) return;

    element.style.height = "auto";
    element.style.height = `${element.scrollHeight}px`;
  }, [draft]);

  // Курсор в конец: autoFocus сам ставит его в начало, и дописать
  // к значению нельзя не промотав.
  useLayoutEffect(() => {
    const element = area.current;
    element?.setSelectionRange(element.value.length, element.value.length);
  }, []);

  const commit = () => {
    const next = numeric ? toNumber(latest.current) : latest.current;

    // Пустое число — null: пустая строка в колонку FLOAT не ложится.
    if (sameValue(next, value)) return onClose();

    /*
     * Не проходит проверку — не сохраняем и НЕ закрываемся: иначе
     * набранное исчезает вместе с карточкой и восстановить его нечем.
     * Escape по-прежнему отменяет — так же ведёт себя JSON.
     */
    const problem = check(next);
    if (problem) return setError(problem);

    onEdit(next);
    onClose();
  };

  return (
    <Anchored anchor={anchor} onClose={commit} onCancel={onClose}>
      <div className={`${card} ${error ? "border-danger" : "border-accent"} p-1.5`}>
        {preview && <div className="mb-1.5 flex justify-center">{preview(draft)}</div>}

        <textarea
          ref={area}
          autoFocus
          rows={1}
          value={draft}
          inputMode={numeric ? "decimal" : undefined}
          onChange={(event) => {
            const text = event.target.value;
            latest.current = text;
            setDraft(text);
            setError(check(numeric ? toNumber(text) : text));
          }}
          onKeyDown={(event) => {
            // В многострочном поле Enter — перенос строки, и сохраняет
            // его Cmd/Ctrl+Enter. В однострочном наоборот.
            const save = multiline ? event.metaKey || event.ctrlKey : true;
            if (event.key === "Enter" && save) {
              event.preventDefault();
              commit();
            }
          }}
          className={`max-h-64 w-full resize-none bg-transparent px-0.5 text-fg outline-none ${
            heading ? "text-2xl leading-8 font-semibold" : "text-sm"
          }`}
        />

        {error && <p className="px-0.5 pt-1 text-2xs text-danger">{error}</p>}
      </div>
    </Anchored>
  );
}

/**
 * Дата и время. Календарь и списки часов/минут, а не браузерное поле
 * ввода: у нативного `<input type="date">` свой вид в каждой системе,
 * свой формат и своя раскладка кнопок — рядом с остальным интерфейсом
 * он выглядит чужим, а в тёмной теме показывает светлый календарь.
 *
 * Правка применяется сразу, а не при закрытии: выбор дня — законченное
 * действие, как выбор варианта. Поэтому у даты нет отмены по Escape,
 * только «очистить».
 */
function DateEditor({
  kind,
  value,
  anchor,
  locale,
  onEdit,
  onClose,
}: {
  kind: DateKind;
  value: unknown;
  anchor: DOMRect;
  locale: string;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const withTime = kind !== "date";
  const [day = "", time = ""] = toDateInput(value, kind).split("T");

  const commit = (nextDay: string, nextTime: string) => {
    // Время без даты бессмысленно, поэтому день по умолчанию — сегодня;
    // дата без времени — полночь, и это ровно то, что человек имел в виду.
    const next = withTime ? `${nextDay}T${nextTime || "00:00"}` : nextDay;
    const ready = fromDateInput(next, kind);

    if (!sameValue(ready, value)) onEdit(ready);
  };

  return (
    <Anchored anchor={anchor} onClose={onClose}>
      <div className={`${card} border-border p-2`}>
        <Suspense fallback={<CalendarFallback />}>
        <div className="flex gap-2">
          <Calendar
            locale={locale}
            // Строка «ГГГГ-ММ-ДД» без пояса читается как местная полночь,
            // а не как UTC: календарь рисует тот же день, что и ячейка.
            value={day ? new Date(`${day}T00:00:00`) : undefined}
            onSelect={(date) => {
              commit(toDayInput(date), time);
              // У даты без времени выбор дня — это всё: закрываемся.
              if (!withTime) onClose();
            }}
          />

          {withTime && (
            <TimeList
              value={time}
              onChange={(next) => commit(day || toDayInput(new Date()), next)}
            />
          )}
        </div>
        </Suspense>

        {!isBlank(value) && (
          <ClearButton
            onClick={() => {
              onEdit(null);
              onClose();
            }}
          />
        )}
      </div>
    </Anchored>
  );
}

/** TIME — то же самое без календаря: в поле лежат только часы и минуты. */
function TimeEditor({
  value,
  anchor,
  onEdit,
  onClose,
}: {
  value: unknown;
  anchor: DOMRect;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  return (
    <Anchored anchor={anchor} onClose={onClose}>
      <div className={`${card} border-border p-2`}>
        <Suspense fallback={<div className="h-56 w-24" />}>
          <TimeList
            value={toTimeInput(value)}
            onChange={(next) => {
              const ready = fromTimeInput(next);
              if (!sameValue(ready, value)) onEdit(ready);
            }}
          />
        </Suspense>

        {!isBlank(value) && (
          <ClearButton
            onClick={() => {
              onEdit(null);
              onClose();
            }}
          />
        )}
      </div>
    </Anchored>
  );
}

function ClearButton({ onClick }: { onClick: () => void }) {
  const { t } = useTranslation();

  return (
    <button
      type="button"
      onClick={onClick}
      className="mt-1 h-7 w-full rounded-md text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      {t("cell.clear")}
    </button>
  );
}


/**
 * Связь. Поиск по связанной таблице, выбор строки и создание новой.
 *
 * Запросы дёргаются прямо отсюда, а не приходят обработчиками сверху:
 * связанная таблица у каждой ячейки своя, и протащить её список через
 * DataGrid значило бы держать в гриде состояние поиска по чужой таблице.
 * Хуки живут в api/, как и все остальные.
 *
 * «Создать» — это ровно то, чего не хватало: чаще всего нужной строки
 * в связанной таблице ещё нет, и без создания на месте выбор упирается
 * в переход на другой экран и обратно.
 */
function RelationEditor({
  field,
  relation,
  row,
  rowGuid,
  tableSlug,
  anchor,
  onLink,
  onClose,
}: {
  field: Field;
  relation: Relation;
  row: Item;
  rowGuid: string;
  /** Таблица, в которой правим строку. Нужна обеим ручкам связи. */
  tableSlug: string;
  anchor: DOMRect;
  /** Есть — выбор не уезжает запросом, а отдаётся вызывающему. */
  onLink?: ((item: Item | null) => void) | undefined;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  /*
   * Запрос отстаёт от ввода на один кадр: без этого каждая буква
   * порождает свой запрос к чужой таблице. useDeferredValue вместо
   * таймера — отменять и чистить нечего.
   */
  const search = useDeferredValue(query);

  const slugs = relation.viewFieldSlugs;

  /*
   * Строки грузятся и без настроенных полей показа. Показывать их
   * идентификаторами — плохо, но связать строку всё равно можно,
   * а пустая коробка читается как сломанный экран: ровно на ней
   * и застревали после создания связи, где поля показа не выбрали.
   */
  const { items, isLoading } = useRelationItems(relation.toSlug, search);

  /*
   * Поиск отсеивает ещё и на клиенте. Серверный `search` ищет только
   * по полям, помеченным как искомые, и на многих таблицах не отсеивает
   * ничего: человек печатает, а список стоит на месте. Из двадцати
   * пришедших строк отобрать нужные — одна строка кода.
   */
  const needle = search.trim().toLowerCase();
  const visible = needle
    ? items.filter((item) => relationLabel(item, slugs).toLowerCase().includes(needle))
    : items;
  const link = useLinkRelation(tableSlug);
  const create = useCreateItem(relation.toSlug);

  const selected = relationSelection(row, field, slugs);
  const selectedGuids = new Set(selected.map((item) => item.guid));

  /* Уезжает guid, но наружу отдаётся строка целиком: черновику нужна
     и связанная запись — из неё ячейка берёт, что показать. */
  const write = (item: Item | null) => {
    if (onLink) return onLink(item);

    const itemGuid = item ? String(item["guid"] ?? "") : null;
    link.mutate({ fieldSlug: field.slug, rowGuid, itemGuid });
  };

  const pick = (item: Item) => {
    // Ссылка одна: повторный выбор того же значения снимает её.
    write(selectedGuids.has(String(item["guid"] ?? "")) ? null : item);
    onClose();
  };

  /**
   * Новая строка связанной таблицы. Заполняется первым полем показа —
   * тем, что человек только что набрал в поиске: остальные поля он
   * дозаполнит в самой таблице, а связь нужна сейчас.
   */
  const createAndLink = () => {
    const text = query.trim();
    const first = slugs[0];
    if (!text || !first) return;

    const created: Item = { guid: crypto.randomUUID(), [first]: text };
    create.mutate(created, {
      onSuccess: () => {
        write(created);
        onClose();
      },
    });
    setQuery("");
  };

  return (
    <Anchored anchor={anchor} onClose={onClose}>
      <div className={`${card} border-accent w-80 overflow-hidden`}>
        <div className="flex flex-wrap items-center gap-1 border-b border-border p-1.5">
          {selected.map((item) => (
            <Chip
              key={item.guid}
              onRemove={() => write(null)}
              removeLabel={t("cell.remove")}
            >
              {item.label || item.guid}
            </Chip>
          ))}

          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={selected.length ? "" : t("cell.searchRelation")}
            className="h-5 min-w-24 flex-1 bg-transparent text-sm text-fg outline-none placeholder:text-fg-subtle"
          />
        </div>

        <div className="max-h-72 overflow-y-auto p-1">
          {/* Список из одинаковых на вид идентификаторов сам по себе
              бесполезен — говорим, чего не хватает, но список оставляем:
              связать строку можно и так. */}
          {!slugs.length && (
            <p className="px-2 py-2 text-xs text-fg-subtle">{t("cell.relationNotConfigured")}</p>
          )}

          <>
            {visible.map((item) => {
                const guid = String(item["guid"] ?? "");
                const label = relationLabel(item, slugs);

                return (
                  <button
                    key={guid}
                    type="button"
                    onClick={() => pick(item)}
                    className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-surface-hover"
                  >
                    <span className="truncate">{label || guid}</span>
                    {selectedGuids.has(guid) && (
                      <Icon as={IconCheck} size={14} className="ml-auto text-accent-text" />
                    )}
                  </button>
                );
              })}

            {!visible.length && !isLoading && (
              <p className="px-2 py-2 text-xs text-fg-subtle">{t("table.noOptions")}</p>
            )}

            {/* Новую строку заполнять нечем, пока не выбрано ни одного
                поля показа: писать текст в guid нельзя. */}
            {query.trim() && slugs.length > 0 && (
                <button
                  type="button"
                  onClick={createAndLink}
                  disabled={create.isPending}
                  className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-50"
                >
                  <Icon as={IconPlus} size={14} />
                  <span className="truncate">{t("cell.createNamed", { name: query.trim() })}</span>
              </button>
            )}
          </>
        </div>
      </div>
    </Anchored>
  );
}

/**
 * STATUS. Варианты разложены по стадиям — так они и заведены в схеме:
 * три отдельных списка todo / progress / complete. Плоский список
 * потерял бы единственное, что отличает статус от обычного выбора.
 *
 * Выбор одиночный, поэтому клик по варианту сразу и сохраняет, и
 * закрывает. Повторный клик по текущему — снимает значение.
 *
 * Поиск набором с клавиатуры: у статуса вариантов бывает два десятка,
 * и тянуться мышью к нужному в списке из трёх стадий дольше, чем
 * набрать три буквы.
 */
function StatusEditor({
  field,
  value,
  anchor,
  language,
  onSettings,
  onEdit,
  onClose,
}: {
  field: Field;
  value: unknown;
  anchor: DOMRect;
  language: string;
  onSettings?: (() => void) | undefined;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");

  const current = isBlank(value) ? "" : String(value);
  const search = query.trim().toLowerCase();
  const options = [...field.options.values()].filter((option) =>
    optionLabel(option, option.value, language).toLowerCase().includes(search),
  );

  const groups = STATUS_GROUPS.map((group) => ({
    group,
    options: options.filter((option) => option.group === group),
  })).filter((entry) => entry.options.length > 0);

  // Вариант без стадии в схеме STATUS невозможен, но данные переживают
  // смену типа поля. Потерять его молча нельзя.
  const ungrouped = options.filter((option) => !option.group);

  const pick = (next: string) => {
    if (next !== current) onEdit(next);
    onClose();
  };

  return (
    <Anchored anchor={anchor} onClose={onClose}>
      <div className={`${card} border-accent w-64 overflow-hidden`}>
        {/* Текущее значение остаётся на месте ячейки — под ним и
            открылось меню. Крестик снимает его. */}
        <div className="flex min-h-9 flex-wrap items-center gap-1 border-b border-border px-2 py-1.5">
          {current && (
            <Chip
              dot
              color={optionColor(field, field.options.get(current))}
              onRemove={() => {
                onEdit("");
                onClose();
              }}
              removeLabel={t("cell.clear")}
            >
              {optionLabel(field.options.get(current), current, language)}
            </Chip>
          )}

          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && options.length === 1) {
                event.preventDefault();
                pick(options[0]!.value);
              }
            }}
            placeholder={current ? "" : t("cell.selectOption")}
            className="h-5 min-w-16 flex-1 bg-transparent text-sm text-fg outline-none placeholder:text-fg-subtle"
          />
        </div>

        <div className="max-h-72 overflow-y-auto p-1">
          {groups.map(({ group, options: list }) => (
            <div key={group}>
              <p className="px-1.5 pt-1.5 pb-1 text-2xs text-fg-muted">{t(`status.${group}`)}</p>
              {list.map((option) => (
                <OptionRow
                  key={option.value}
                  field={field}
                  option={option}
                  language={language}
                  dot
                  onClick={() => pick(option.value)}
                />
              ))}
            </div>
          ))}

          {ungrouped.map((option) => (
            <OptionRow
              key={option.value}
              field={field}
              option={option}
              language={language}
              dot
              onClick={() => pick(option.value)}
            />
          ))}

          {!options.length && (
            <p className="px-2 py-2 text-xs text-fg-subtle">{t("table.noOptions")}</p>
          )}
        </div>

        <SettingsRow onClick={onSettings} />
      </div>
    </Anchored>
  );
}

/**
 * «Настроить поле» под списком вариантов.
 *
 * Нужного варианта в списке нет — значит его надо завести, а не искать
 * дальше. Без этой строки путь до настроек лежит через закрытие ячейки,
 * поиск нужной колонки и её меню.
 */
function SettingsRow({ onClick }: { onClick?: (() => void) | undefined }) {
  const { t } = useTranslation();
  if (!onClick) return null;

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-8 w-full items-center gap-2 border-t border-border px-2.5 text-left text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={IconAdjustments} size={14} />
      <span className="truncate">{t("column.settings")}</span>
    </button>
  );
}

/**
 * MULTISELECT. Выбранное показано чипами наверху, поиск — там же:
 * список вариантов бывает длинным, а колонка узкой.
 *
 * Каждое переключение сохраняется сразу, но меню не закрывается —
 * значений несколько, и закрывать его после первого значило бы
 * открывать заново на каждое.
 */
function MultiselectEditor({
  field,
  value,
  anchor,
  language,
  onSettings,
  onEdit,
  onClose,
}: {
  field: Field;
  value: unknown;
  anchor: DOMRect;
  language: string;
  onSettings?: (() => void) | undefined;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");

  const selected = toList(value);
  const search = query.trim().toLowerCase();

  const visible = [...field.options.values()].filter((option) =>
    optionLabel(option, option.value, language).toLowerCase().includes(search),
  );

  const toggle = (option: string) =>
    onEdit(
      selected.includes(option)
        ? selected.filter((item) => item !== option)
        : [...selected, option],
    );

  return (
    <Anchored anchor={anchor} onClose={onClose}>
      <div className={`${card} border-accent w-72 overflow-hidden`}>
        <div className="flex flex-wrap items-center gap-1 border-b border-border p-1.5">
          {selected.map((item) => (
            <Chip
              key={item}
              color={optionColor(field, field.options.get(item))}
              onRemove={() => toggle(item)}
              removeLabel={t("cell.remove")}
            >
              {optionLabel(field.options.get(item), item, language)}
            </Chip>
          ))}

          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              // Backspace на пустом поиске снимает последний чип —
              // так же, как в любом поле с тегами.
              if (event.key === "Backspace" && !query && selected.length) {
                toggle(selected[selected.length - 1]!);
              }
              if (event.key === "Enter" && visible.length === 1) {
                event.preventDefault();
                toggle(visible[0]!.value);
                setQuery("");
              }
            }}
            className="h-5 min-w-16 flex-1 bg-transparent text-sm text-fg outline-none placeholder:text-fg-subtle"
            placeholder={selected.length ? "" : t("cell.selectOption")}
          />
        </div>

        <div className="max-h-72 overflow-y-auto p-1">
          {visible.map((option) => (
            <OptionRow
              key={option.value}
              field={field}
              option={option}
              language={language}
              checked={selected.includes(option.value)}
              onClick={() => toggle(option.value)}
            />
          ))}

          {!visible.length && (
            <p className="px-2 py-2 text-xs text-fg-subtle">{t("table.noOptions")}</p>
          )}
        </div>

        <SettingsRow onClick={onSettings} />
      </div>
    </Anchored>
  );
}

function OptionRow({
  field,
  option,
  language,
  checked,
  dot,
  onClick,
}: {
  field: Field;
  option: FieldOption;
  language: string;
  checked?: boolean | undefined;
  dot?: boolean | undefined;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-8 w-full items-center gap-2 rounded-md px-1.5 text-left transition-colors hover:bg-surface-hover"
    >
      <Chip dot={dot} color={optionColor(field, option)}>
        {optionLabel(option, option.value, language)}
      </Chip>

      {checked && <Icon as={IconCheck} size={14} className="ml-auto text-accent-text" />}
    </button>
  );
}

/**
 * Файлы и картинки.
 *
 * Загрузка идёт прямо из ячейки: в старом ucode ради одной картинки
 * открывалась модалка на пол-экрана с зумом, поворотом и предпросмотром —
 * всё это нужно при просмотре, а не при заполнении таблицы.
 *
 * Значение пишется одним разом: множественному типу — список, одиночному
 * — строка. Массив в колонку PHOTO не ложится (см. isMultiValue).
 *
 * Перетаскивание работает, вставка из буфера — нет: ячейка не поле ввода,
 * и ловить Ctrl+V пришлось бы на документе.
 */
function FileEditor({
  field,
  value,
  anchor,
  images,
  onEdit,
  onClose,
}: {
  field: Field;
  value: unknown;
  anchor: DOMRect;
  /** Картинки показываем плитками, остальное — строками со ссылкой. */
  images: boolean;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const upload = useUploadFiles();

  const urls = toList(value);
  const multiple = isMultiValue(field.type);

  const write = (next: string[]) => onEdit(multiple ? next : (next[0] ?? null));

  const add = (files: FileList | null) => {
    const picked = files ? [...files] : [];
    if (!picked.length) return;

    upload.mutate(
      // Одиночному полю второй файл некуда девать — берём первый.
      { files: multiple ? picked : picked.slice(0, 1), folder: uploadFolder(field.attributes) },
      { onSuccess: (added) => write(multiple ? [...urls, ...added] : added) },
    );
  };

  return (
    <Anchored anchor={anchor} onClose={onClose}>
      <div
        onDragOver={(event) => {
          event.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          add(event.dataTransfer.files);
        }}
        className={`${card} w-72 p-1.5 ${dragging ? "border-accent" : "border-border"}`}
      >
        {urls.length > 0 && (
          <div className={`mb-1.5 ${images ? "flex flex-wrap gap-1.5" : "flex flex-col gap-0.5"}`}>
            {urls.map((url) => (
              <div key={url} className="group/file relative flex min-w-0 items-center gap-1.5">
                {/* Щелчок открывает просмотр, а не соседнюю вкладку:
                    загруженный файл смотрят прямо здесь, не теряя
                    открытого редактора. */}
                {images ? (
                  <button type="button" onClick={() => openPreview(urls, urls.indexOf(url), "image")}>
                    <img
                      src={url}
                      alt=""
                      loading="lazy"
                      draggable={false}
                      className="size-14 rounded-md border border-border object-cover"
                    />
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() => openPreview(urls, urls.indexOf(url))}
                    className="min-w-0 flex-1 truncate text-left text-sm text-accent-text hover:underline"
                  >
                    {fileName(url)}
                  </button>
                )}

                <button
                  type="button"
                  onClick={() => write(urls.filter((item) => item !== url))}
                  aria-label={t("cell.remove")}
                  className={`grid size-6 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger ${
                    images ? "absolute top-0.5 right-0.5 bg-surface/80" : ""
                  }`}
                >
                  <Icon as={IconTrash} size={14} />
                </button>
              </div>
            ))}
          </div>
        )}

        <input
          ref={input}
          type="file"
          hidden
          multiple={multiple}
          accept={images ? "image/*" : field.type === "VIDEO" ? "video/*" : undefined}
          onChange={(event) => {
            add(event.target.files);
            // Тот же файл, выбранный второй раз, иначе не даёт события.
            event.target.value = "";
          }}
        />

        <button
          type="button"
          onClick={() => input.current?.click()}
          disabled={upload.isPending}
          className="flex h-8 w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-border-strong text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-50"
        >
          <Icon as={IconUpload} size={14} />
          {upload.isPending ? t("cell.uploading") : t("cell.upload")}
        </button>
      </div>
    </Anchored>
  );
}

/**
 * Цвет. Системная пипетка, а не своя палитра: в COLOR лежит произвольный
 * HEX (фирменный цвет, подсветка строки), и ограничивать его девятью
 * оттенками чипов нельзя — это не вариант выбора, а значение.
 *
 * Рядом поле ввода: цвет чаще вставляют из макета, чем подбирают на глаз.
 */
function ColorEditor({
  value,
  anchor,
  onEdit,
  onClose,
}: {
  value: unknown;
  anchor: DOMRect;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(() => (isBlank(value) ? "" : String(value)));
  const latest = useRef(draft);

  const set = (next: string) => {
    latest.current = next;
    setDraft(next);
  };

  const commit = () => {
    const next = latest.current.trim();
    if (!sameValue(next, value)) onEdit(next || null);
    onClose();
  };

  return (
    <Anchored anchor={anchor} onClose={commit} onCancel={onClose}>
      <div className={`${card} border-accent flex w-56 items-center gap-1.5 p-1.5`}>
        <input
          type="color"
          value={/^#[0-9a-f]{6}$/i.test(draft) ? draft : "#000000"}
          onChange={(event) => set(event.target.value)}
          aria-label={t("cell.color")}
          className="size-8 shrink-0 cursor-pointer rounded-md border border-border bg-transparent p-0.5"
        />

        <input
          autoFocus
          value={draft}
          placeholder="#45aeff"
          onChange={(event) => set(event.target.value)}
          onKeyDown={(event) => event.key === "Enter" && commit()}
          className="h-8 min-w-0 flex-1 rounded-md border border-border-strong bg-surface px-2 font-mono text-sm text-fg outline-none focus:border-accent"
        />

        {draft && (
          <button
            type="button"
            onClick={() => {
              onEdit(null);
              onClose();
            }}
            aria-label={t("cell.clear")}
            className="grid size-8 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconTrash} size={14} />
          </button>
        )}
      </div>
    </Anchored>
  );
}

/**
 * Иконка. Тот же выбор, что и у пункта меню: имя из Iconify, наш CDN
 * или чужая ссылка — все три вида умеет DynamicIcon.
 *
 * Правка применяется при закрытии: в поле рядом с сеткой имя набирают
 * руками, и запрос на каждую букву — это запрос на каждую букву.
 */
function IconEditor({
  value,
  anchor,
  onEdit,
  onClose,
}: {
  value: unknown;
  anchor: DOMRect;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState(() => (isBlank(value) ? "" : String(value)));
  const latest = useRef(draft);

  const commit = () => {
    const next = latest.current.trim();
    if (!sameValue(next, value)) onEdit(next || null);
    onClose();
  };

  return (
    <Anchored anchor={anchor} onClose={commit} onCancel={onClose}>
      <div className={`${card} border-accent w-72 p-1.5`}>
        <IconPicker
          value={draft}
          type="ICON"
          onChange={(icon) => {
            latest.current = icon;
            setDraft(icon);
          }}
        />
      </div>
    </Anchored>
  );
}

/**
 * JSON и MAP.
 *
 * Уезжает ТЕКСТ, а не разобранный объект, хотя разбором он проверяется.
 * Колонка под полем типа JSON заводится как varchar, и объект в неё
 * не кладётся вовсе: «unable to encode map[string]interface {} into text
 * format for varchar». Строку же примет и varchar, и jsonb — там она
 * разберётся на стороне базы.
 *
 * Непонятный текст не сохраняется и не закрывает карточку — иначе
 * правка исчезает вместе с ней. Escape отменяет.
 */
/**
 * Точка на карте: два поля — широта и долгота.
 *
 * Не встроенная карта: за неё пришлось бы платить внешним скриптом,
 * ключом API в настройках поля (старая админка спрашивала его у каждого
 * поля MAP отдельно) и запросом к чужому серверу на каждое открытие
 * ячейки. Координаты правятся руками и проверяются кнопкой «на карте» —
 * ссылка открывается в новой вкладке и ничего не грузит на страницу.
 */
function MapEditor({
  value,
  center,
  anchor,
  onEdit,
  onClose,
}: {
  value: unknown;
  /**
   * Точка по умолчанию из настроек поля. Подставляется только в пустую
   * ячейку и только в поля ввода: пока человек ничего не тронул,
   * в строке по-прежнему пусто — значение уедет, если он подтвердит.
   */
  center?: { lat: unknown; lon: unknown };
  anchor: DOMRect;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  /*
   * Настройки поля приходят как есть из свободного мешка attributes:
   * координата бывает и числом, и строкой. Приводим к паре тем же
   * разбором, что и значение ячейки, — заодно отсеиваются мусор
   * и координаты вне глобуса.
   */
  const coord = (raw: unknown) =>
    typeof raw === "number" || typeof raw === "string" ? String(raw).trim() : "";

  const point =
    parseCoords(value) ??
    (isBlank(value) ? parseCoords(formatCoords(coord(center?.lat), coord(center?.lon))) : null);
  const [lat, setLat] = useState(point ? String(point.lat) : "");
  const [lon, setLon] = useState(point ? String(point.lon) : "");
  const latest = useRef({ lat, lon });

  const set = (next: { lat?: string; lon?: string }) => {
    latest.current = { ...latest.current, ...next };
    if (next.lat !== undefined) setLat(next.lat);
    if (next.lon !== undefined) setLon(next.lon);
  };

  const commit = () => {
    const next = formatCoords(latest.current.lat, latest.current.lon);
    if (!sameValue(next, value)) onEdit(next || null);
    onClose();
  };

  const draft = parseCoords(formatCoords(lat, lon));
  const field =
    "h-8 w-full rounded-md border border-border-strong bg-surface px-2 text-sm tabular-nums text-fg outline-none focus:border-accent";

  return (
    <Anchored anchor={anchor} onClose={commit} onCancel={onClose}>
      <div className={`${card} border-accent w-80 p-1.5`}>
        {/* Точка ставится кликом по карте — как в старой админке; поля
            ввода остаются для точных координат и правят ту же пару. */}
        <MapPicker
          point={draft}
          onPick={(picked) => set({ lat: String(picked.lat), lon: String(picked.lon) })}
        />

        <div className="mt-1.5 flex gap-1.5">
          <label className="min-w-0 flex-1">
            <span className="mb-0.5 block text-2xs text-fg-muted">{t("cell.latitude")}</span>
            <input
              autoFocus
              value={lat}
              inputMode="decimal"
              placeholder="41.311081"
              onChange={(event) => set({ lat: event.target.value })}
              onKeyDown={(event) => event.key === "Enter" && commit()}
              className={field}
            />
          </label>

          <label className="min-w-0 flex-1">
            <span className="mb-0.5 block text-2xs text-fg-muted">{t("cell.longitude")}</span>
            <input
              value={lon}
              inputMode="decimal"
              placeholder="69.240562"
              onChange={(event) => set({ lon: event.target.value })}
              onKeyDown={(event) => event.key === "Enter" && commit()}
              className={field}
            />
          </label>
        </div>

        {/* Кнопка появляется, только когда пара действительно координаты:
            ссылка на «Ташкент,» открывала бы карту мира. */}
        {draft && (
          <a
            href={mapLink(draft)}
            target="_blank"
            rel="noreferrer noopener"
            className="mt-1.5 flex h-7 items-center justify-center gap-1.5 rounded-md border border-border text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconExternalLink} size={14} />
            {t("cell.openMap")}
          </a>
        )}
      </div>
    </Anchored>
  );
}

function JsonEditor({
  value,
  anchor,
  preview,
  onEdit,
  onClose,
}: {
  value: unknown;
  anchor: DOMRect;
  /** Что показать над текстом: форма области по набранным координатам. */
  preview?: ((value: string) => ReactNode) | undefined;
  onEdit: (value: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(() => toJsonText(value));
  const [invalid, setInvalid] = useState(false);
  const latest = useRef(draft);

  const commit = () => {
    const text = latest.current.trim();

    if (!text) {
      if (!isBlank(value)) onEdit(null);
      return onClose();
    }

    try {
      JSON.parse(text);
    } catch {
      setInvalid(true);
      return;
    }

    if (!sameValue(text, toJsonText(value))) onEdit(text);
    onClose();
  };

  return (
    <Anchored anchor={anchor} onClose={commit} onCancel={onClose}>
      <div className={`${card} ${invalid ? "border-danger" : "border-accent"} w-80 p-1.5`}>
        {preview && <div className="mb-1.5 flex justify-center">{preview(draft)}</div>}

        <textarea
          autoFocus
          rows={8}
          value={draft}
          spellCheck={false}
          onChange={(event) => {
            latest.current = event.target.value;
            setDraft(event.target.value);
            setInvalid(false);
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
              event.preventDefault();
              commit();
            }
          }}
          className="max-h-72 w-full resize-none bg-transparent px-0.5 font-mono text-xs text-fg outline-none"
        />

        {invalid && <p className="px-0.5 pt-1 text-2xs text-danger">{t("cell.jsonInvalid")}</p>}
      </div>
    </Anchored>
  );
}

/** Объект — с отступами, строку из данных — как есть: она может быть не JSON. */
function toJsonText(value: unknown): string {
  if (isBlank(value)) return "";
  return typeof value === "string" ? value : JSON.stringify(value, null, 2);
}
