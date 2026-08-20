import { IconCopy, IconExternalLink, IconMapPin, IconPaperclip, IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field, type FieldOption, type Relation } from "@/features/table";
import { fileName } from "@/shared/lib/file-kind";
import { toast } from "@/shared/lib/toast";
import { Checkbox } from "@/shared/ui/checkbox";
import { Chip, hexToChipColor } from "@/shared/ui/chip";
import { DynamicIcon } from "@/shared/ui/dynamic-icon";
import { openPreview } from "@/shared/ui/file-preview";
import { Icon } from "@/shared/ui/icon";
import { cellKind, editorKind } from "../model/cell-kind";
import { mapLink, parseCoords } from "../model/coords";
import { isBlank, toDateValue, toList, type DateKind } from "../model/cell-value";
import { relationSelection } from "../model/relation";
import type { Item } from "../model/types";
import { ButtonCell } from "./ButtonCell";
import { CodeCell } from "./CodeCell";
import { FormulaCell } from "./FormulaCell";
import { PolygonCell } from "./PolygonCell";

/**
 * Одна ячейка, только показ. Правка — в CellEditor: там своя разметка,
 * и делать один компонент, который умеет оба состояния, значит завести
 * в каждой ветке по флагу.
 *
 * Ветвление идёт по виду ячейки, а не по типу поля: типов сорок,
 * видов пятнадцать (см. model/cell-kind).
 */

/**
 * Пусто — прочерк, а не пустая ячейка: иначе строка выглядит поехавшей.
 */
const Empty = () => <span className="text-fg-subtle">—</span>;

export function Cell({
  field,
  row,
  tableSlug,
  relations,
  locale,
  language,
  wrap = false,
}: {
  field: Field;
  row: Item;
  /** Нужен полю-кнопке: вызов функции передаёт таблицу вместе со строкой. */
  tableSlug: string;
  /** Связи по id — для полей-ссылок. */
  relations: Map<string, Relation>;
  /** Локаль интерфейса: форматы дат и чисел. */
  locale: string;
  /** Язык данных: подписи вариантов. Это разные вещи, см. DataLanguage. */
  language: string;
  /**
   * Показать значение целиком. В строке таблицы ширину задаёт колонка,
   * и всё лишнее обрезается многоточием; в раскрытой ячейке — переносится.
   */
  wrap?: boolean;
}) {
  const value = row[field.slug];
  const line = wrap ? "whitespace-pre-wrap break-words" : "truncate";

  switch (cellKind(field.type)) {
    /*
     * Хеш пароля — такой же секрет, как сам пароль: его можно подобрать
     * офлайн. Бэкенд отдаёт колонку как есть, поэтому не показываем её
     * здесь. Скрыт сам факт значения, а не только символы: пустая строка
     * и заполненная должны отличаться, а содержимое — нет.
     */
    case "password":
      return isBlank(value) ? <Empty /> : <span className="tracking-widest">••••••</span>;

    case "link":
      return <LinkCell value={value} line={line} />;

    case "relation":
      return <RelationCell field={field} row={row} relations={relations} line={line} />;

    case "status":
      return <TagsCell field={field} value={value} language={language} dot wrap={wrap} />;

    case "multiselect":
      return <TagsCell field={field} value={value} language={language} wrap={wrap} />;

    case "date":
      return <DateCell value={value} kind="date" locale={locale} />;

    case "datetime":
      return <DateCell value={value} kind="datetime" locale={locale} />;

    case "datetime_naive":
      return <DateCell value={value} kind="datetime_naive" locale={locale} />;

    case "time":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <span className="tabular-nums">{String(value).slice(0, 5)}</span>
      );

    case "number":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <span className={`tabular-nums ${line}`}>{String(value)}</span>
      );

    case "boolean":
      /*
       * Настоящий чекбокс, а не галочка текстом: три состояния —
       * «да», «нет» и «не заполнено» — иначе неразличимы. Клик по нему
       * не ловится: значение переключает вся ячейка.
       */
      return isBlank(value) ? (
        <Empty />
      ) : (
        <Checkbox checked={Boolean(value)} readOnly tabIndex={-1} className="pointer-events-none" />
      );

    case "image":
      return <ImageCell value={value} wrap={wrap} />;

    case "file":
      return <FileCell value={value} wrap={wrap} />;

    case "color":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <span className="flex min-w-0 items-center gap-1.5">
          <span
            className="size-3.5 shrink-0 rounded-sm border border-border"
            style={{ background: String(value) }}
          />
          <span className={`font-mono text-xs ${line}`}>{String(value)}</span>
        </span>
      );

    case "icon":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <DynamicIcon name={String(value)} fallback={<span className={line}>{String(value)}</span>} />
      );

    /*
     * У кнопки значения нет — рисуем её саму, а не пустую ячейку.
     * Прочерк здесь читался бы как «не заполнено», хотя заполнять
     * тут нечего.
     */
    case "button":
      return <ButtonCell field={field} row={row} tableSlug={tableSlug} />;

    case "map":
      return <MapCell value={value} line={line} />;

    case "polygon":
      return isBlank(value) ? <Empty /> : <PolygonCell value={value} wrap={wrap} />;

    /*
     * Формула считается здесь и сейчас: в колонке FORMULA_FRONTEND
     * ничего не лежит, её значение существует только на экране.
     * Формулы нет — показываем то, что всё-таки лежит в строке.
     */
    case "formula": {
      const formula = field.attributes["formula"];
      if (typeof formula !== "string" || !formula.trim()) return <TextCell value={value} line={line} />;

      return <FormulaCell formula={formula} row={row} locale={locale} line={line} />;
    }

    case "qr":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <CodeCell type={field.type} value={String(value)} qr big={wrap} />
      );

    case "barcode":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <CodeCell type={field.type} value={String(value)} big={wrap} />
      );

    case "json":
      return isBlank(value) ? (
        <Empty />
      ) : (
        <span className={`font-mono text-xs ${line}`}>
          {typeof value === "string" ? value : JSON.stringify(value)}
        </span>
      );

    case "longtext":
      return <LongTextCell value={value} line={line} />;

    default:
      return <TextCell value={value} line={line} />;
  }
}

/**
 * Многострочный текст: рядом со значением — «скопировать», как в старой
 * админке (MultiLineCellFormElement). Значение длинное, и выделять его
 * мышью в обрезанной ячейке — мучение.
 */
function LongTextCell({ value, line }: { value: unknown; line: string }) {
  const { t } = useTranslation();

  if (isBlank(value) || typeof value === "object") return <TextCell value={value} line={line} />;

  const text = String(value);

  return (
    <span className="flex h-full min-w-0 flex-1 items-center gap-1">
      <span className={`min-w-0 ${line}`}>{text}</span>

      <button
        type="button"
        onClick={(event) => {
          // Клик по кнопке — только копирование: ячейка не раскрывается.
          event.stopPropagation();
          void navigator.clipboard.writeText(text);
          toast.success(t("cell.copied"));
        }}
        aria-label={t("cell.copy")}
        title={t("cell.copy")}
        className="ml-auto hidden size-6 shrink-0 place-items-center rounded-md text-fg-muted transition-colors group-hover/row:grid hover:bg-surface-active hover:text-fg"
      >
        <Icon as={IconCopy} size={14} />
      </button>
    </span>
  );
}

/**
 * Точка на карте.
 *
 * Показываются сами координаты, а рядом — переход на карту: встроенная
 * карта в ячейке высотой 36px бесполезна, а её загрузка стоит внешнего
 * скрипта и ключа API на каждую строку таблицы.
 *
 * Не разобралось в пару координат — показываем текст как есть: колонка
 * MAP это обычный VARCHAR, и в ней встречается всё что угодно.
 */
function MapCell({ value, line }: { value: unknown; line: string }) {
  const { t } = useTranslation();

  if (isBlank(value)) return <Empty />;

  const point = parseCoords(value);
  if (!point) return <span className={line}>{String(value)}</span>;

  return (
    <span className="flex w-full min-w-0 items-center gap-1.5">
      <Icon as={IconMapPin} size={14} className="shrink-0 text-fg-muted" />
      <span className={`tabular-nums ${line}`}>{`${point.lat}, ${point.lon}`}</span>

      <a
        href={mapLink(point)}
        target="_blank"
        rel="noreferrer noopener"
        // Клик по кнопке — только переход: ячейка при этом не раскрывается.
        onClick={(event) => event.stopPropagation()}
        aria-label={t("cell.openMap")}
        title={t("cell.openMap")}
        className="ml-auto hidden size-6 shrink-0 place-items-center rounded-md text-fg-muted transition-colors group-hover/row:grid hover:bg-surface-active hover:text-fg"
      >
        <Icon as={IconExternalLink} size={14} />
      </a>
    </span>
  );
}

function TextCell({ value, line }: { value: unknown; line: string }) {
  if (isBlank(value)) return <Empty />;

  // Объект незнакомой формы печатать как [object Object] нельзя.
  if (typeof value === "object") {
    return <span className={`font-mono text-xs ${line}`}>{JSON.stringify(value)}</span>;
  }

  return <span className={line}>{String(value)}</span>;
}

/**
 * STATUS и MULTISELECT. Варианты уже приведены к одной форме в
 * features/table/api/normalize: ключ поиска — сохранённое значение,
 * какой бы ключ ни занимал его в сыром виде.
 *
 * Значения без варианта показываем как есть: вариант могли удалить
 * из настроек поля, а в строках он остался. Скрывать такое нельзя —
 * данные есть, и человек должен их видеть.
 */
function TagsCell({
  field,
  value,
  language,
  dot,
  wrap,
}: {
  field: Field;
  value: unknown;
  language: string;
  dot?: boolean;
  wrap?: boolean;
}) {
  const items = toList(value);
  if (!items.length) return <Empty />;

  return (
    <span className={`flex min-w-0 gap-1 ${wrap ? "flex-wrap" : ""}`}>
      {items.map((item, index) => (
        <Chip key={index} dot={dot} color={optionColor(field, field.options.get(item))}>
          {optionLabel(field.options.get(item), item, language)}
        </Chip>
      ))}
    </span>
  );
}

/** Цвет варианта — только когда поле цветное: иначе чип нейтральный. */
export function optionColor(field: Field, option: FieldOption | undefined) {
  return field.hasColor && option?.color ? hexToChipColor(option.color) : "gray";
}

/** Три ступени: перевод, базовая подпись, само значение. Пустоты не бывает. */
export function optionLabel(
  option: FieldOption | undefined,
  value: string,
  language: string,
): string {
  return option ? localized(option.labels, language, option.label || option.value) : value;
}

/**
 * Ссылка. Значение остаётся текстом — его правят как текст, — но по
 * наведению на строку появляется кнопка «открыть». Без неё адрес
 * приходится выделять и копировать: клик по ячейке открывает редактор,
 * а не браузер, и иначе быть не может.
 *
 * Сама подпись ссылкой не делается сознательно: тогда клик по тексту
 * означал бы то переход, то правку — в зависимости от того, попал ли
 * курсор в буквы.
 */
function LinkCell({ value, line }: { value: unknown; line: string }) {
  const { t } = useTranslation();

  if (isBlank(value)) return <Empty />;

  const text = String(value);
  const href = toHref(text);

  return (
    <span className="flex w-full min-w-0 items-center gap-1">
      <span className={line}>{text}</span>

      {href && (
        <a
          href={href}
          target="_blank"
          rel="noreferrer noopener"
          // Клик по кнопке — только переход: ячейка при этом не раскрывается.
          onClick={(event) => event.stopPropagation()}
          aria-label={t("cell.openLink")}
          title={href}
          className="ml-auto hidden size-6 shrink-0 place-items-center rounded-md text-fg-muted transition-colors group-hover/row:grid hover:bg-surface-active hover:text-fg"
        >
          <Icon as={IconExternalLink} size={14} />
        </a>
      )}
    </span>
  );
}

/**
 * Адрес для перехода или null, если значение на адрес не похоже.
 *
 * Схема не угадывается ни для чего, кроме голого домена: без белого
 * списка `javascript:` из данных стал бы исполняемой ссылкой в нашей
 * странице. Всё непонятное получает https:// — такой адрес безопасен
 * даже если ведёт в никуда.
 */
export function toHref(value: string): string | null {
  const text = value.trim();
  if (!text || /\s/.test(text)) return null;

  if (/^(https?:\/\/|mailto:|tel:)/i.test(text)) return text;
  return text.includes(".") ? `https://${text}` : null;
}

/**
 * Поле-связь. Показываем не uuid, а поля связанной строки, которые
 * выбраны в настройках связи (view_fields). Связанная строка приходит
 * рядом со значением: author_id → author_id_data.
 */
function RelationCell({
  field,
  row,
  relations,
  line,
}: {
  field: Field;
  row: Item;
  relations: Map<string, Relation>;
  line: string;
}) {
  const { t } = useTranslation();
  const slugs = field.relationId ? relations.get(field.relationId)?.viewFieldSlugs : undefined;
  const parts = relationSelection(row, field, slugs)
    .map((item) => item.label)
    .filter(Boolean);

  if (parts.length) return <span className={line}>{parts.join(", ")}</span>;

  /*
   * Пусто — и связь настроена: предлагаем связать. Прочерк здесь врёт,
   * потому что ячейка не «без значения», а «ещё не заполнена», и
   * заполняется она одним кликом.
   */
  if (slugs?.length && editorKind(field)) {
    return (
      <span className="flex items-center gap-1 text-fg-subtle opacity-0 transition-opacity group-hover/row:opacity-100">
        <Icon as={IconPlus} size={14} />
        <span className="truncate">{t("cell.createRelation")}</span>
      </span>
    );
  }

  return <Empty />;
}

/**
 * Даты форматируются по языку интерфейса. Форматтер кэшируется: Intl
 * стоит дорого, а в таблице ячейки считаются сотнями.
 *
 * Пояс — часть ключа: значение без пояса печатается в UTC, иначе браузер
 * пересчитает его в местное время и сдвинет то, что сдвигать нельзя.
 */
const formatters = new Map<string, Intl.DateTimeFormat>();

function formatter(locale: string, withTime: boolean, utc: boolean): Intl.DateTimeFormat {
  const key = `${locale}:${withTime}:${utc}`;
  let cached = formatters.get(key);

  if (!cached) {
    cached = new Intl.DateTimeFormat(locale, {
      dateStyle: "medium",
      ...(withTime ? { timeStyle: "short" as const } : {}),
      ...(utc ? { timeZone: "UTC" } : {}),
    });
    formatters.set(key, cached);
  }

  return cached;
}

function DateCell({ value, kind, locale }: { value: unknown; kind: DateKind; locale: string }) {
  if (isBlank(value)) return <Empty />;

  const parsed = toDateValue(value, kind);

  // Бэкенд отдаёт даты в нескольких форматах, и не все разбираются.
  // Нечитаемую дату показываем как есть, а не как «Invalid Date».
  if (!parsed) return <span className="truncate">{String(value)}</span>;

  return (
    <span className="truncate tabular-nums whitespace-nowrap">
      {formatter(locale, kind !== "date", parsed.naive).format(parsed.date)}
    </span>
  );
}

/** До трёх картинок и счётчик: строка высотой 36px больше не вмещает. */
const PREVIEW = 3;

function ImageCell({ value, wrap }: { value: unknown; wrap?: boolean }) {
  const urls = toList(value);
  if (!urls.length) return <Empty />;

  const shown = wrap ? urls : urls.slice(0, PREVIEW);

  return (
    <span className={`flex min-w-0 items-center gap-1 ${wrap ? "flex-wrap" : ""}`}>
      {shown.map((_url, index) => (
        <Thumb key={index} urls={urls} index={index} />
      ))}
      {shown.length < urls.length && (
        /* Остальные не влезли, но открыть их можно: щелчок ведёт
           в просмотр к первому из спрятанных. */
        <button
          type="button"
          onClick={(event) => {
            event.stopPropagation();
            openPreview(urls, shown.length, "image");
          }}
          className="text-xs text-fg-subtle transition-colors hover:text-fg"
        >
          +{urls.length - shown.length}
        </button>
      )}
    </span>
  );
}

/**
 * Картинка-миниатюра. Щелчок открывает просмотр, а не раскрывает ячейку:
 * на картинку жмут, чтобы её РАЗГЛЯДЕТЬ, а редактор открывается по
 * пустому месту ячейки.
 *
 * `draggable={false}` обязателен: без него браузер тащит саму картинку
 * (и её адрес) вместо карточки на доске или строки в списке — курсор
 * показывает ссылку, а перетаскивание не начинается вовсе.
 */
function Thumb({ urls, index }: { urls: string[]; index: number }) {
  const { t } = useTranslation();

  return (
    <button
      type="button"
      onClick={(event) => {
        event.stopPropagation();
        openPreview(urls, index, "image");
      }}
      aria-label={t("cell.preview")}
      className="shrink-0"
    >
      <img
        src={urls[index]}
        alt=""
        loading="lazy"
        draggable={false}
        className="size-6 rounded-sm border border-border object-cover"
      />
    </button>
  );
}

function FileCell({ value, wrap }: { value: unknown; wrap?: boolean }) {
  const urls = toList(value);
  if (!urls.length) return <Empty />;

  return (
    <span className={`flex min-w-0 items-center gap-2 ${wrap ? "flex-wrap" : ""}`}>
      {urls.map((url, index) => (
        <a
          key={index}
          href={url}
          target="_blank"
          rel="noreferrer"
          onClick={(event) => {
            // Клик по ссылке не должен заодно раскрывать ячейку.
            event.stopPropagation();
            /*
             * Обычный щелчок открывает просмотр, а не соседнюю вкладку:
             * PDF и картинку смотрят на месте. Ссылка при этом остаётся
             * ссылкой — с Cmd/Ctrl и средней кнопкой она по-прежнему
             * открывается вкладкой, как её и ждут.
             */
            if (event.metaKey || event.ctrlKey || event.shiftKey) return;
            event.preventDefault();
            openPreview(urls, index);
          }}
          className="flex min-w-0 items-center gap-1 text-accent-text hover:underline"
        >
          <Icon as={IconPaperclip} size={14} />
          <span className="truncate">{fileName(url)}</span>
        </a>
      ))}
    </span>
  );
}


